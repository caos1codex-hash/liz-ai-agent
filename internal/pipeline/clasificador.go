package pipeline

import (
	"context"
	"fmt"
	"strings"
)

// Clasificador determina la intención del usuario y la categoría de la tarea.
// Usa un enfoque híbrido: primero heurísticas rápidas, luego LLM para casos ambiguos.
type Clasificador struct {
	orquestador OrquestadorCliente
}

// nuevoClasificador crea un Clasificador con el orquestador inyectado.
func nuevoClasificador(orch OrquestadorCliente) *Clasificador {
	return &Clasificador{orquestador: orch}
}

// Clasificar determina la categoría de la tarea del mensaje del usuario.
// Primero intenta con heurísticas (rápido), y si la confianza es baja,
// usa el LLM para clasificar con mayor precisión.
func (c *Clasificador) Clasificar(ctx context.Context, mensaje string, proyecto string) (*ResultadoClasificacion, error) {
	// 1. Heurísticas rápidas primero
	result := c.clasificarPorHeuristica(mensaje, proyecto)
	if result != nil && result.Confianza >= 0.7 {
		return result, nil
	}

	// 2. Si hay orquestador, usar LLM para clasificación precisa
	if c.orquestador != nil {
		resultLLM, err := c.clasificarPorLLM(ctx, mensaje, proyecto)
		if err == nil && resultLLM != nil {
			// Si la heurística dio algo, promediar; si no, usar LLM
			if result != nil {
				if resultLLM.Confianza > result.Confianza {
					return resultLLM, nil
				}
			}
			return resultLLM, nil
		}
	}

	// 3. Fallback: usar resultado heurístico o default
	if result == nil {
		return &ResultadoClasificacion{
			Categoria:   CategoriaConversacion,
			Confianza:    0.5,
			Razonamiento: "No se pudo determinar la categoría con confianza. Tratado como conversación.",
			Prioridad:   3,
		}, nil
	}
	return result, nil
}

// clasificarPorHeuristica usa reglas basadas en palabras clave para clasificar rápido.
func (c *Clasificador) clasificarPorHeuristica(mensaje, proyecto string) *ResultadoClasificacion {
	msg := strings.ToLower(mensaje)

	// Patrones por categoría con orden de prioridad (más específico primero)
	reglas := []struct {
		categoria       CategoriaTarea
		palabras        []string
		confianza       float64
		requiereContexto bool
	}{
		{CategoriaAutoCreacion, []string{"crea una herramienta", "crea un programa", "programa una herramienta", "necesitas crear", "no tienes la herramienta", "crea algo que", "inventa una herramienta"}, 0.85, false},
		{CategoriaAutoCreacion, []string{"auto-crea", "auto crear", "genera herramienta"}, 0.8, false},
		{CategoriaEjecucionComando, []string{"ejecuta ", "run ", "corre ", "ejecutar ", "run: ", "comando:", "shell:"}, 0.85, false},
		{CategoriaMonitorizacion, []string{"monitor", "métrica", "cpu", "ram", "memoria", "disco", "red", "temperatura", "uptime", "load", "procesos activos", "estado del sistema", "estado del servidor", "recursos"}, 0.8, false},
		{CategoriaInstalacion, []string{"instala", "instalar", "desinstala", "actualiza", "upgrade", "apt ", "snap ", "dnf ", "pacman ", "brew ", "pip install", "npm install", "cargo install"}, 0.85, false},
		{CategoriaProcesos, []string{"mata el proceso", "mata proceso", "kill", "proceso en el puerto", "qué procesos", "listar procesos", "procesos que consumen", "terminar proceso", "detener proceso"}, 0.85, false},
		{CategoriaArchivos, []string{"crea el archivo", "edita el archivo", "borra ", "elimina ", "mueve ", "copia ", "directorio", "carpeta", "archivo", "lee el archivo", "escribe en", "crea directorio", "permisos del archivo"}, 0.75, false},
		{CategoriaBusqueda, []string{"busca ", "buscar ", "encuentra ", "dónde está", "find ", "grep ", "que archivos", "qué archivos", "filtrar", "localizar"}, 0.8, false},
		{CategoriaCodigo, []string{"función", "funcion", "clase", "struct", "método", "variable", "refactoriza", "implementa", "escribe código", "crea un servidor", "crea una api", "endpoint", "migra", "import", "export", "compila", "build", "debug", "fix ", "bug", "error en", "línea "}, 0.75, true},
		{CategoriaCodigo, []string{"go ", "python", "rust ", "java ", "javascript", "typescript", "código", "codigo", "programa", "script", ".go", ".py", ".rs", ".ts", ".js"}, 0.7, true},
		{CategoriaAnalisis, []string{"analiza", "analiza el código", "revisión", "review", "explica este código", "qué hace", "cómo funciona", "arquitectura", "diseña", "planifica", "complejidad", "optimiza", "performance"}, 0.8, true},
		{CategoriaConversacion, []string{"hola", "buenos días", "buenas tardes", "buenas noches", "gracias", "adiós", "chao", "qué eres", "quién eres", "qué puedes hacer", "ayuda", "help", "cómo te llamas", "qué eres liz"}, 0.9, false},
	}

	for _, regla := range reglas {
		if tienePalabrasClave(msg, regla.palabras) {
			return &ResultadoClasificacion{
				Categoria:       regla.categoria,
				Confianza:        regla.confianza,
				Razonamiento:     fmt.Sprintf("Heurística: se detectaron palabras clave de la categoría '%s'", regla.categoria),
				NecesitaContexto: regla.requiereContexto,
				Prioridad:       c.prioridadDesdeCategoria(regla.categoria),
			}
		}
	}

	// Verificar si hay un proyecto activo — sesgo hacia código
	if proyecto != "" {
		return &ResultadoClasificacion{
			Categoria:       CategoriaCodigo,
			Confianza:        0.6,
			Razonamiento:    "Hay un proyecto activo y no se detectó otra categoría clara; se asume tarea de código.",
			NecesitaContexto: true,
			Prioridad:       2,
		}
	}

	return nil // No se pudo clasificar con heurísticas
}

// clasificarPorLLM usa el orquestador para clasificar la intención con mayor precisión.
func (c *Clasificador) clasificarPorLLM(ctx context.Context, mensaje, proyecto string) (*ResultadoClasificacion, error) {
	prompt := fmt.Sprintf(`Clasifica la siguiente intención del usuario en UNA de estas categorías:
- conversacion: saludo, despedida, pregunta sobre Liz, ayuda general
- ejecucion_comando: ejecutar un comando shell específico
- archivos: crear, editar, eliminar, mover, copiar archivos o directorios
- procesos: listar, monitorear, matar procesos del sistema
- monitorizacion: revisar métricas (CPU, RAM, disco, red)
- instalacion: instalar, desinstalar, actualizar software
- busqueda: buscar archivos o contenido en archivos
- codigo: escribir, leer, modificar, analizar código fuente
- analisis: análisis profundo de código, arquitectura, rendimiento
- auto_creacion: necesita crear una herramienta que Liz no tiene

Mensaje del usuario: "%s"
Proyecto activo: "%s"

Responde SOLO en formato JSON:
{"categoria": "nombre_categoria", "confianza": 0.0-1.0, "razonamiento": "explicación breve", "necesita_contexto": true/false, "prioridad": 1-3}`, mensaje, proyecto)

	respuesta, err := c.orquestador.Completar(ctx, prompt, "general")
	if err != nil {
		return nil, fmt.Errorf("error clasificando con LLM: %w", err)
	}

	result, err := parsearClasificacion(respuesta)
	if err != nil {
		return nil, fmt.Errorf("error parseando respuesta de clasificación: %w", err)
	}

	return result, nil
}

// parsearClasificacion extrae el resultado de clasificación de la respuesta del LLM.
func parsearClasificacion(texto string) (*ResultadoClasificacion, error) {
	// Intentar extraer JSON de la respuesta
	jsonStr := extraerJSON(texto)
	if jsonStr == "" {
		return nil, fmt.Errorf("no se encontró JSON en la respuesta del LLM")
	}

	result := &ResultadoClasificacion{}
	if err := parsearJSON(jsonStr, result); err != nil {
		return nil, err
	}

	if !result.Categoria.Valida() {
		result.Categoria = CategoriaConversacion
	}

	if result.Confianza <= 0 {
		result.Confianza = 0.5
	}
	if result.Confianza > 1 {
		result.Confianza = 1.0
	}
	if result.Prioridad == 0 {
		result.Prioridad = 2
	}

	return result, nil
}

// prioridadDesdeCategoria devuelve la prioridad default para una categoría.
func (c *Clasificador) prioridadDesdeCategoria(cat CategoriaTarea) int {
	switch cat {
	case CategoriaEjecucionComando:
		return 1
	case CategoriaMonitorizacion, CategoriaProcesos:
		return 1
	case CategoriaInstalacion, CategoriaAutoCreacion:
		return 2
	case CategoriaCodigo, CategoriaArchivos, CategoriaBusqueda, CategoriaAnalisis:
		return 2
	default:
		return 3
	}
}
