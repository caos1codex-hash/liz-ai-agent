package pipeline

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Pipeline es el coordinador principal que conecta todos los componentes
// del chat en un flujo end-to-end coherente.
//
// Flujo: Receptor → Clasificador → Planificador → Ejecutor → Respondedor
//
// El Pipeline es thread-safe y puede manejar múltiples solicitudes concurrentes.
type Pipeline struct {
	mu sync.RWMutex

	// Componentes del pipeline
	receptor     *Receptor
	clasificador *Clasificador
	planificador *Planificador
	ejecutor     *Ejecutor
	respondedor  *Respondedor

	// Dependencias inyectadas
	memoria       MemoriaGestor
	orquestador   OrquestadorCliente
	catalogo      CatalogoCliente
	autoGestor    AutoCreacionGestor
	contextoCoord ContextoCoordinador

	// Métricas
	mensajesProcesados int64
	ultimaDuracion     time.Duration
	ultimoUso          time.Time
	categoriaCount     map[CategoriaTarea]int
	modeloCount        map[string]int
}

// NuevasOpciones permite configurar el Pipeline con dependencias opcionales.
type NuevasOpciones struct {
	Orquestador   OrquestadorCliente
	Catalogo      CatalogoCliente
	Memoria       MemoriaGestor
	AutoGestor    AutoCreacionGestor
	ContextoCoord ContextoCoordinador
}

// Nuevo crea un Pipeline con las dependencias inyectadas.
// Todas las dependencias son opcionales — el pipeline degrada gracefully.
func Nuevo(opts NuevasOpciones) *Pipeline {
	p := &Pipeline{
		memoria:        opts.Memoria,
		orquestador:    opts.Orquestador,
		catalogo:       opts.Catalogo,
		autoGestor:     opts.AutoGestor,
		contextoCoord:  opts.ContextoCoord,
		categoriaCount: make(map[CategoriaTarea]int),
		modeloCount:    make(map[string]int),
	}

	// Inicializar componentes (nil-safe: cada componente maneja deps nulas internamente)
	p.receptor = nuevoReceptor(opts.Memoria)
	p.clasificador = nuevoClasificador(opts.Orquestador)
	p.planificador = nuevoPlanificador(opts.Orquestador, opts.Catalogo, opts.AutoGestor, opts.ContextoCoord)
	p.ejecutor = nuevoEjecutor(opts.Catalogo, opts.AutoGestor)
	p.respondedor = nuevoRespondedor(opts.Orquestador, opts.Memoria)

	return p
}

// Procesar ejecuta el pipeline completo para un mensaje del usuario.
// Retorna la respuesta completa como RespuestaPipeline.
func (p *Pipeline) Procesar(ctx context.Context, sol *SolicitudChat) (*RespuestaPipeline, error) {
	inicio := time.Now()

	// 1. Validar y recibir mensaje
	mensaje, sesion, err := p.receptor.Recibir(ctx, sol)
	if err != nil {
		return nil, fmt.Errorf("error en receptor: %w", err)
	}

	// 2. Clasificar intención
	clasif, err := p.clasificador.Clasificar(ctx, mensaje.Contenido, sol.Proyecto)
	if err != nil {
		return nil, fmt.Errorf("error en clasificador: %w", err)
	}

	// 3. Obtener historial para el planificador
	var historial []turnoHistorial
	if p.memoria != nil {
		msgs := p.memoria.ObtenerMensajesRecientes(sesion.ID, 10)
		for _, m := range msgs {
			historial = append(historial, turnoHistorial{Rol: m.Rol, Contenido: m.Contenido})
		}
	}

	// 4. Planificar
	plan, err := p.planificador.Planificar(ctx, mensaje.Contenido, clasif, sesion, historial)
	if err != nil {
		return nil, fmt.Errorf("error en planificador: %w", err)
	}

	// 5. Ejecutar pasos que requieren herramientas
	resultados := make([]ResultadoPaso, 0)
	if clasif.RequiereHerramientas() {
		resultados, err = p.ejecutor.EjecutarPlan(ctx, plan, nil)
		if err != nil {
			return nil, fmt.Errorf("error en ejecutor: %w", err)
		}
	}

	// 6. Generar respuesta
	respuestaTexto := ""
	modeloUsado := ""
	tokensUsados := 0

	if p.orquestador != nil {
		respuestaTexto, modeloUsado, tokensUsados, err = p.respondedor.GenerarRespuesta(
			ctx, mensaje.Contenido, sesion, clasif, resultados, "",
		)
		if err != nil {
			// Fallback: respuesta genérica
			respuestaTexto = fmt.Sprintf("Procesé tu solicitud pero tuve un error generando la respuesta: %v", err)
			modeloUsado = "fallback"
		}
	} else {
		respuestaTexto = p.respuestaSinOrquestador(mensaje.Contenido, clasif, resultados)
		modeloUsado = "none"
	}

	// 7. Actualizar métricas
	p.actualizarMetricas(clasif, modeloUsado, time.Since(inicio))

	// 8. Construir respuesta final
	resp := &RespuestaPipeline{
		ID:              generarUUID(),
		SesionID:        sesion.ID,
		Mensaje:         respuestaTexto,
		Categoria:       clasif.Categoria,
		PasosEjecutados: len(resultados),
		Resultados:      resultados,
		ModeloUsado:     modeloUsado,
		TokensUsados:    tokensUsados,
		DuracionTotal:   time.Since(inicio),
		Timestamp:       time.Now(),
		Metadata: map[string]interface{}{
			"confianza_clasificacion": clasif.Confianza,
			"pasos_totales":           len(plan.Pasos),
			"requiere_herramientas":   clasif.RequiereHerramientas(),
		},
	}

	return resp, nil
}

// ProcesarStream ejecuta el pipeline con streaming SSE.
// Cada fragmento de la respuesta se envía al callback.
func (p *Pipeline) ProcesarStream(ctx context.Context, sol *SolicitudChat, callback func(*ChunkStream)) (*RespuestaPipeline, error) {
	inicio := time.Now()

	// 1. Recibir mensaje
	mensaje, sesion, err := p.receptor.Recibir(ctx, sol)
	if err != nil {
		return nil, fmt.Errorf("error en receptor: %w", err)
	}

	// Notificar inicio
	if callback != nil {
		callback(nuevoChunk("estado", "Clasificando intención..."))
	}

	// 2. Clasificar
	clasif, err := p.clasificador.Clasificar(ctx, mensaje.Contenido, sol.Proyecto)
	if err != nil {
		return nil, fmt.Errorf("error en clasificador: %w", err)
	}

	if callback != nil {
		callback(nuevoChunkConDatos("estado", fmt.Sprintf("Categoría: %s (confianza: %.0f%%)", clasif.Categoria, clasif.Confianza*100),
			map[string]interface{}{"categoria": clasif.Categoria, "confianza": clasif.Confianza}))
	}

	// 3. Planificar
	if callback != nil {
		callback(nuevoChunk("estado", "Planificando pasos..."))
	}

	historial := p.obtenerHistorial(sesion.ID)
	plan, err := p.planificador.Planificar(ctx, mensaje.Contenido, clasif, sesion, historial)
	if err != nil {
		return nil, fmt.Errorf("error en planificador: %w", err)
	}

	// 4. Ejecutar pasos
	resultados := make([]ResultadoPaso, 0)
	if clasif.RequiereHerramientas() && len(plan.Pasos) > 0 {
		if callback != nil {
			callback(nuevoChunk("estado", fmt.Sprintf("Ejecutando %d pasos...", len(plan.Pasos))))
		}
		resultados, err = p.ejecutor.EjecutarPlan(ctx, plan, callback)
		if err != nil {
			return nil, fmt.Errorf("error en ejecutor: %w", err)
		}
	}

	// 5. Generar respuesta con streaming
	respuestaTexto := ""
	modeloUsado := ""
	tokensUsados := 0

	if callback != nil {
		callback(nuevoChunk("estado", "Generando respuesta..."))
	}

	if p.orquestador != nil {
		respuestaTexto, modeloUsado, tokensUsados, err = p.respondedor.GenerarRespuestaStream(
			ctx, mensaje.Contenido, sesion, clasif, resultados, "", callback,
		)
		if err != nil {
			respuestaTexto = fmt.Sprintf("Error generando respuesta: %v", err)
			modeloUsado = "fallback"
		}
	} else {
		respuestaTexto = p.respuestaSinOrquestador(mensaje.Contenido, clasif, resultados)
		modeloUsado = "none"
	}

	// Notificar finalización
	if callback != nil {
		callback(nuevoChunk("estado", fmt.Sprintf("Completado en %v", time.Since(inicio).Round(time.Millisecond))))
	}

	// Actualizar métricas
	p.actualizarMetricas(clasif, modeloUsado, time.Since(inicio))

	return &RespuestaPipeline{
		ID:              generarUUID(),
		SesionID:        sesion.ID,
		Mensaje:         respuestaTexto,
		Categoria:       clasif.Categoria,
		PasosEjecutados: len(resultados),
		Resultados:      resultados,
		ModeloUsado:     modeloUsado,
		TokensUsados:    tokensUsados,
		DuracionTotal:   time.Since(inicio),
		Timestamp:       time.Now(),
		Metadata: map[string]interface{}{
			"confianza_clasificacion": clasif.Confianza,
			"pasos_totales":           len(plan.Pasos),
		},
	}, nil
}

// Estado retorna las métricas actuales del pipeline.
func (p *Pipeline) Estado() *EstadoPipeline {
	p.mu.RLock()
	defer p.mu.RUnlock()

	modeloMasUsado := ""
	maxCount := 0
	for m, c := range p.modeloCount {
		if c > maxCount {
			maxCount = c
			modeloMasUsado = m
		}
	}

	promedio := time.Duration(0)
	if p.mensajesProcesados > 0 {
		promedio = time.Duration(int64(p.ultimaDuracion) / p.mensajesProcesados)
	}

	catCount := make(map[CategoriaTarea]int)
	for k, v := range p.categoriaCount {
		catCount[k] = v
	}

	return &EstadoPipeline{
		MensajesProcesados: p.mensajesProcesados,
		PromedioDuracion:   promedio,
		UltimoUso:          p.ultimoUso,
		CategoriaCount:     catCount,
		ModeloMasUsado:     modeloMasUsado,
	}
}

// respuestaSinOrquestador genera una respuesta cuando no hay LLM disponible.
func (p *Pipeline) respuestaSinOrquestador(mensaje string, clasif *ResultadoClasificacion, resultados []ResultadoPaso) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[Modo sin LLM] Categoría detectada: %s\n\n", clasif.Categoria))

	if len(resultados) > 0 {
		sb.WriteString("Resultados de las herramientas:\n")
		for _, r := range resultados {
			sb.WriteString(fmt.Sprintf("- Paso %d (%s): %s\n", r.PasoID, r.ToolUsada, estadoExito(r.Exito)))
			if r.Error != "" {
				sb.WriteString(fmt.Sprintf("  Error: %s\n", r.Error))
			}
			if r.Datos != nil {
				datosStr, err := serializarJSON(r.Datos)
				if err == nil {
					sb.WriteString(fmt.Sprintf("  Datos: %s\n", truncarTexto(datosStr, 500)))
				}
			}
		}
	} else {
		sb.WriteString("No se ejecutaron herramientas. Para obtener respuestas inteligentes, ")
		sb.WriteString("configura una API key de NVIDIA en liz.yaml.\n")
	}

	return sb.String()
}

// obtenerHistorial obtiene el historial de la sesión.
func (p *Pipeline) obtenerHistorial(sesionID string) []turnoHistorial {
	if p.memoria == nil {
		return nil
	}
	msgs := p.memoria.ObtenerMensajesRecientes(sesionID, 10)
	var historial []turnoHistorial
	for _, m := range msgs {
		historial = append(historial, turnoHistorial{Rol: m.Rol, Contenido: m.Contenido})
	}
	return historial
}

// actualizarMetricas actualiza las métricas internas del pipeline.
func (p *Pipeline) actualizarMetricas(clasif *ResultadoClasificacion, modelo string, duracion time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.mensajesProcesados++
	p.ultimaDuracion = duracion
	p.ultimoUso = time.Now()
	p.categoriaCount[clasif.Categoria]++

	if modelo != "" {
		p.modeloCount[modelo]++
	}
}
