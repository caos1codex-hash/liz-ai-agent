package pipeline

import (
	"context"
	"fmt"
	"strings"
)

// Respondedor genera la respuesta final usando el LLM.
// Soporta tanto respuesta completa (JSON) como streaming (SSE).
type Respondedor struct {
	orquestador OrquestadorCliente
	memoria     MemoriaGestor
}

// nuevoRespondedor crea un Respondedor con las dependencias inyectadas.
func nuevoRespondedor(orch OrquestadorCliente, mem MemoriaGestor) *Respondedor {
	return &Respondedor{
		orquestador: orch,
		memoria:     mem,
	}
}

// GenerarRespuesta construye el prompt completo y genera la respuesta del LLM.
// Retorna la respuesta completa como string.
func (r *Respondedor) GenerarRespuesta(ctx context.Context, mensaje string, sesion *SesionInfo, clasif *ResultadoClasificacion, resultados []ResultadoPaso, modelo string) (string, string, int, error) {
	prompt := r.construirPrompt(mensaje, sesion, clasif, resultados, modelo)

	respuesta, err := r.orquestador.Completar(ctx, prompt, clasif.PrioridadModelo())
	if err != nil {
		return "", "", 0, fmt.Errorf("error generando respuesta: %w", err)
	}

	modeloUsado := r.orquestador.ModeloActual()
	tokens := estimarTokens(respuesta)

	return respuesta, modeloUsado, tokens, nil
}

// GenerarRespuestaStream genera la respuesta con streaming via callback.
// Cada fragmento se envía al callback a medida que llega del LLM.
func (r *Respondedor) GenerarRespuestaStream(ctx context.Context, mensaje string, sesion *SesionInfo, clasif *ResultadoClasificacion, resultados []ResultadoPaso, modelo string, callback func(*ChunkStream)) (string, string, int, error) {
	prompt := r.construirPrompt(mensaje, sesion, clasif, resultados, modelo)

	var respuestaCompleta strings.Builder
	modeloUsado := modelo
	totalTokens := 0

	chunkCh, err := r.orquestador.CompletarStream(ctx, prompt, clasif.PrioridadModelo())
	if err != nil {
		return "", "", 0, fmt.Errorf("error iniciando stream: %w", err)
	}

	for chunk := range chunkCh {
		if chunk.Error != nil {
			_ = chunk.Error
			continue
		}
		if chunk.Delta != "" {
			respuestaCompleta.WriteString(chunk.Delta)
			totalTokens += estimarTokens(chunk.Delta)

			if callback != nil {
				callback(&ChunkStream{
					Tipo:      "texto",
					Contenido: chunk.Delta,
					Modelo:    modeloUsado,
				})
			}
		}
		if chunk.Modelo != "" {
			modeloUsado = chunk.Modelo
		}
	}

	// Si no hay streaming, fallback a completar normal
	if respuestaCompleta.Len() == 0 && r.orquestador != nil {
		resp, mod, tokens, err := r.GenerarRespuesta(ctx, mensaje, sesion, clasif, resultados, modelo)
		if err != nil {
			return "", "", 0, err
		}
		return resp, mod, tokens, nil
	}

	return respuestaCompleta.String(), modeloUsado, totalTokens, nil
}

// construirPrompt ensambla el prompt completo con toda la información disponible.
// Sigue el patrón "Contexto Bajo Demanda" de Liz.
func (r *Respondedor) construirPrompt(mensaje string, sesion *SesionInfo, clasif *ResultadoClasificacion, resultados []ResultadoPaso, modelo string) string {
	var sb strings.Builder

	// 1. Rol del sistema
	sb.WriteString(ContextoParaPrompt())
	sb.WriteString("\n\n")

	// 2. Contexto de memoria del usuario (hechos)
	if r.memoria != nil && sesion != nil {
		hechos := r.memoria.ObtenerHechos(sesion.UsuarioID, 20)
		if hechos != "" {
			sb.WriteString("## Memoria del usuario (hechos conocidos)\n")
			sb.WriteString(hechos)
			sb.WriteString("\n\n")
		}

		// Historial reciente de la sesión
		historial := r.memoria.ObtenerMensajesRecientes(sesion.ID, 10)
		if len(historial) > 0 {
			sb.WriteString("## Historial reciente de la conversación\n")
			for _, msg := range historial {
				sb.WriteString(fmt.Sprintf("- **%s**: %s\n", msg.Rol, truncarTexto(msg.Contenido, 300)))
			}
			sb.WriteString("\n")
		}
	}

	// 3. Resultados de herramientas ejecutadas
	if len(resultados) > 0 {
		sb.WriteString("## Resultados de las herramientas ejecutadas\n")
		sb.WriteString(FormatearResultados(resultados))
		sb.WriteString("\n")
	}

	// 4. Contexto del proyecto (si aplica)
	if sesion != nil && sesion.Proyecto != "" && clasif.NecesitaContexto {
		sb.WriteString(fmt.Sprintf("## Proyecto activo: %s\n", sesion.Proyecto))
		sb.WriteString("Si necesitas ver archivos específicos del proyecto, solicítalos.\n\n")
	}

	// 5. Instrucción específica según categoría
	sb.WriteString(fmt.Sprintf("## Tipo de tarea: %s\n", clasif.Categoria))
	sb.WriteString(r.instruccionCategoria(clasif.Categoria))

	// 6. Mensaje del usuario
	sb.WriteString(fmt.Sprintf("\n## Mensaje del usuario\n%s\n", mensaje))

	// 7. Instrucción final
	sb.WriteString("\n## Instrucción\n")
	sb.WriteString("Responde de forma directa y útil. Si ejecutaste herramientas, resume los resultados clave. ")
	sb.WriteString("Si hubo errores, explica qué pasó. Si necesitas más información, pídela.\n")

	return sb.String()
}

// instruccionCategoria devuelve instrucciones específicas para cada tipo de tarea.
func (r *Respondedor) instruccionCategoria(cat CategoriaTarea) string {
	switch cat {
	case CategoriaConversacion:
		return "Responde de forma natural y conversacional. Sé amigable pero directo.\n"
	case CategoriaEjecucionComando:
		return "Resume la salida del comando ejecutado. Si hay errores, sugiere soluciones.\n"
	case CategoriaArchivos:
		return "Describe las operaciones realizadas en archivos. Muestra las rutas afectadas.\n"
	case CategoriaProcesos:
		return "Resume la información de procesos. Destaca los más relevantes.\n"
	case CategoriaMonitorizacion:
		return "Presenta las métricas de forma clara. Usa formato tabular si es apropiado. Indica valores normales vs. alertas.\n"
	case CategoriaInstalacion:
		return "Detalla el proceso de instalación. Informa de paquetes instalados/actualizados.\n"
	case CategoriaBusqueda:
		return "Presenta los resultados de búsqueda de forma organizada. Destalla los más relevantes.\n"
	case CategoriaCodigo:
		return "Analiza y explica el código. Si escribiste código, muestra los cambios. Si analizaste código, explica hallazgos.\n"
	case CategoriaAnalisis:
		return "Proporciona un análisis profundo y detallado. Usa estructura clara con secciones. Incluye recomendaciones si aplica.\n"
	case CategoriaAutoCreacion:
		return "Explica la herramienta que se creó, qué hace y cómo usarla. Muestra ejemplos de uso.\n"
	default:
		return "Responde de forma útil y directa.\n"
	}
}

// GenerarRespuestaSimple genera una respuesta sin herramientas (solo LLM).
// Usado para conversaciones simples.
func (r *Respondedor) GenerarRespuestaSimple(ctx context.Context, mensaje string, sesion *SesionInfo, modelo string) (string, string, int, error) {
	clasif := &ResultadoClasificacion{
		Categoria:        CategoriaConversacion,
		Confianza:        0.9,
		Razonamiento:     "Respuesta directa sin herramientas",
		NecesitaContexto: false,
		Prioridad:        3,
	}

	return r.GenerarRespuesta(ctx, mensaje, sesion, clasif, nil, modelo)
}

// GenerarRespuestaSimpleStream genera una respuesta simple con streaming.
func (r *Respondedor) GenerarRespuestaSimpleStream(ctx context.Context, mensaje string, sesion *SesionInfo, modelo string, callback func(*ChunkStream)) (string, string, int, error) {
	clasif := &ResultadoClasificacion{
		Categoria:        CategoriaConversacion,
		Confianza:        0.9,
		Razonamiento:     "Respuesta directa sin herramientas",
		NecesitaContexto: false,
		Prioridad:        3,
	}

	return r.GenerarRespuestaStream(ctx, mensaje, sesion, clasif, nil, modelo, callback)
}
