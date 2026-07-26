package pipeline

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// CategoriaTarea representa el tipo de tarea clasificada.
type CategoriaTarea string

const (
	// CategoriaConversacion indica un mensaje conversacional sin acción directa.
	CategoriaConversacion CategoriaTarea = "conversacion"

	// CategoriaCodigo indica una tarea relacionada con código fuente.
	CategoriaCodigo CategoriaTarea = "codigo"

	// CategoriaArchivos indica operaciones sobre el sistema de archivos.
	CategoriaArchivos CategoriaTarea = "archivos"

	// CategoriaProcesos indica gestión de procesos del sistema.
	CategoriaProcesos CategoriaTarea = "procesos"

	// CategoriaMonitorizacion indica lectura de métricas del sistema.
	CategoriaMonitorizacion CategoriaTarea = "monitorizacion"

	// CategoriaInstalacion indica instalación/desinstalación de software.
	CategoriaInstalacion CategoriaTarea = "instalacion"

	// CategoriaBusqueda indica búsqueda de archivos o contenido.
	CategoriaBusqueda CategoriaTarea = "busqueda"

	// CategoriaAnalisis indica análisis profundo de código o datos.
	CategoriaAnalisis CategoriaTarea = "analisis"

	// CategoriaAutoCreacion indica que se necesita crear una herramienta nueva.
	CategoriaAutoCreacion CategoriaTarea = "auto_creacion"

	// CategoriaEjecucionComando indica ejecución directa de un comando shell.
	CategoriaEjecucionComando CategoriaTarea = "ejecucion_comando"
)

// TodasCategorias devuelve la lista de todas las categorías válidas.
func TodasCategorias() []CategoriaTarea {
	return []CategoriaTarea{
		CategoriaConversacion,
		CategoriaCodigo,
		CategoriaArchivos,
		CategoriaProcesos,
		CategoriaMonitorizacion,
		CategoriaInstalacion,
		CategoriaBusqueda,
		CategoriaAnalisis,
		CategoriaAutoCreacion,
		CategoriaEjecucionComando,
	}
}

// String implementa Stringer para CategoryTarea.
func (c CategoriaTarea) String() string {
	return string(c)
}

// Valida verifica que la categoría sea una de las predefinidas.
func (c CategoriaTarea) Valida() bool {
	for _, cat := range TodasCategorias() {
		if cat == c {
			return true
		}
	}
	return false
}

// RequiereHerramientas indica si la categoría necesita ejecución de herramientas.
func (c *ResultadoClasificacion) RequiereHerramientas() bool {
	return c.Categoria != CategoriaConversacion
}

// PrioridadModelo devuelve el tipo de tarea para selección de modelo.
func (c *ResultadoClasificacion) PrioridadModelo() string {
	switch c.Categoria {
	case CategoriaCodigo, CategoriaAutoCreacion:
		return "codigo"
	case CategoriaAnalisis:
		return "razonamiento"
	case CategoriaMonitorizacion, CategoriaProcesos, CategoriaInstalacion:
		return "general"
	default:
		return "general"
	}
}

// MensajeChat representa un mensaje completo en el pipeline.
type MensajeChat struct {
	ID              string                 `json:"id"`
	SesionID        string                 `json:"sesion_id"`
	UsuarioID       string                 `json:"usuario_id"`
	Contenido       string                 `json:"contenido"`
	Rol             string                 `json:"rol"` // "usuario", "asistente"
	Timestamp       time.Time              `json:"timestamp"`
	TokensEstimados int                    `json:"tokens_estimados"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// SolicitudChat es la entrada del usuario para el pipeline.
type SolicitudChat struct {
	Mensaje   string `json:"mensaje"`
	UsuarioID string `json:"usuario_id,omitempty"`
	SesionID  string `json:"sesion_id,omitempty"`
	Proyecto  string `json:"proyecto,omitempty"` // Proyecto de contexto opcional
	Stream    bool   `json:"stream,omitempty"`   // true para SSE
}

// Validar verifica que la solicitud tenga los campos requeridos.
func (s *SolicitudChat) Validar() error {
	if strings.TrimSpace(s.Mensaje) == "" {
		return fmt.Errorf("campo 'mensaje' es requerido y no puede estar vacío")
	}
	if len(s.Mensaje) > 50000 {
		return fmt.Errorf("campo 'mensaje' excede el límite de 50,000 caracteres")
	}
	if s.UsuarioID == "" {
		s.UsuarioID = "usuario_default"
	}
	return nil
}

// ResultadoClasificacion contiene la clasificación de intención.
type ResultadoClasificacion struct {
	Categoria        CategoriaTarea `json:"categoria"`
	Confianza        float64        `json:"confianza"`         // 0.0 a 1.0
	Razonamiento     string         `json:"razonamiento"`      // Por qué eligió esta categoría
	NecesitaContexto bool           `json:"necesita_contexto"` // Si necesita contexto de proyecto
	Prioridad        int            `json:"prioridad"`         // 1=urgente, 2=normal, 3=bajo
}

// PasoTarea representa un paso individual dentro de un plan.
type PasoTarea struct {
	ID              int                    `json:"id"`
	Descripcion     string                 `json:"descripcion"`
	Herramienta     string                 `json:"herramienta"`          // Nombre de la herramienta (vacío si es solo LLM)
	Parametros      map[string]interface{} `json:"parametros,omitempty"` // Parámetros para la herramienta
	DependeDe       []int                  `json:"depende_de,omitempty"` // IDs de pasos que deben completarse primero
	TimeoutSegundos int                    `json:"timeout_segundos,omitempty"`
	RequiereLLM     bool                   `json:"requiere_llm,omitempty"` // Si este paso necesita LLM antes/después
	PromptLLM       string                 `json:"prompt_llm,omitempty"`   // Prompt específico para este paso
}

// PlanEjecucion es el plan completo generado por el planificador.
type PlanEjecucion struct {
	ID                string         `json:"id"`
	Pasos             []PasoTarea    `json:"pasos"`
	DescripcionGlobal string         `json:"descripcion_global"`
	Categoria         CategoriaTarea `json:"categoria"`
	EstimacionPasos   int            `json:"estimacion_pasos"`
	PuedeParalelizar  bool           `json:"puede_paralelizar"`
	NecesitaAutoCrear bool           `json:"necesita_auto_crear"` // Si alguna herramienta no existe
}

// ResultadoPaso contiene el resultado de ejecutar un paso.
type ResultadoPaso struct {
	PasoID    int                    `json:"paso_id"`
	Exito     bool                   `json:"exito"`
	Datos     interface{}            `json:"datos,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Duracion  time.Duration          `json:"duracion"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	ToolUsada string                 `json:"tool_usada,omitempty"`
}

// RespuestaPipeline es la respuesta final del pipeline.
type RespuestaPipeline struct {
	ID              string                 `json:"id"`
	SesionID        string                 `json:"sesion_id"`
	Mensaje         string                 `json:"mensaje"`
	Categoria       CategoriaTarea         `json:"categoria"`
	PasosEjecutados int                    `json:"pasos_ejecutados"`
	Resultados      []ResultadoPaso        `json:"resultados,omitempty"`
	ModeloUsado     string                 `json:"modelo_usado"`
	TokensUsados    int                    `json:"tokens_usados"`
	DuracionTotal   time.Duration          `json:"duracion_total"`
	Timestamp       time.Time              `json:"timestamp"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// ChunkStream representa un fragmento de la respuesta SSE.
type ChunkStream struct {
	Tipo      string      `json:"tipo"` // "texto", "herramienta", "error", "estado", "hecho", "pensamiento"
	Contenido string      `json:"contenido"`
	Datos     interface{} `json:"datos,omitempty"`
	PasoID    int         `json:"paso_id,omitempty"`
	Modelo    string      `json:"modelo,omitempty"`
}

// EstadoPipeline contiene métricas del pipeline.
type EstadoPipeline struct {
	MensajesProcesados int64                  `json:"mensajes_procesados"`
	PromedioDuracion   time.Duration          `json:"promedio_duracion"`
	UltimoUso          time.Time              `json:"ultimo_uso"`
	CategoriaCount     map[CategoriaTarea]int `json:"categoria_count"`
	ModeloMasUsado     string                 `json:"modelo_mas_usado"`
}

// contextoParaLLM construye el prompt completo para el LLM.
type contextoParaLLM struct {
	RolSistema       string           `json:"rol_sistema"`
	Historial        []turnoHistorial `json:"historial,omitempty"`
	HechosMemoria    string           `json:"hechos_memoria,omitempty"`
	ContextoProyecto string           `json:"contexto_proyecto,omitempty"`
	ResultadosTools  string           `json:"resultados_tools,omitempty"`
	Instruccion      string           `json:"instruccion"`
}

// turnoHistorial representa un turno de conversación.
type turnoHistorial struct {
	Rol       string `json:"rol"`
	Contenido string `json:"contenido"`
}

// Serializar convierte el chunk a JSON para SSE.
func (c *ChunkStream) Serializar() ([]byte, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("error serializando chunk stream: %w", err)
	}
	return data, nil
}

// nuevoChunk crea un chunk de stream con el tipo y contenido dados.
func nuevoChunk(tipo, contenido string) *ChunkStream {
	return &ChunkStream{
		Tipo:      tipo,
		Contenido: contenido,
	}
}

// NuevoChunk es la versión exportada de nuevoChunk.
func NuevoChunk(tipo, contenido string) *ChunkStream {
	return nuevoChunk(tipo, contenido)
}

// nuevoChunkConDatos crea un chunk con datos adicionales.
func nuevoChunkConDatos(tipo, contenido string, datos interface{}) *ChunkStream {
	return &ChunkStream{
		Tipo:      tipo,
		Contenido: contenido,
		Datos:     datos,
	}
}

// estimarTokens calcula una estimación de tokens (~4 chars = 1 token).
func estimarTokens(texto string) int {
	if texto == "" {
		return 0
	}
	return len(texto) / 4
}

// ContextoParaPrompt construye el prompt del sistema base para Liz.
func ContextoParaPrompt() string {
	return `Eres Liz, un agente de IA autónomo para Linux con control total del sistema operativo.

## Tus Capacidades
- Control total del sistema: archivos, procesos, red, paquetes
- Ejecución de comandos con permisos completos
- Escritura y edición de código en cualquier lenguaje
- Auto-creación de herramientas: si necesitas algo que no tienes, lo programas
- Análisis profundo de código y proyectos
- Monitoreo del sistema en tiempo real
- Búsqueda y filtrado de archivos

## Tu Personalidad
- Directo y preciso: no divagues, ve al grano
- Proactivo: si detectas algo relevante, menciónalo
- Transparente: informa qué herramientas usas y por qué
- Profesional pero amigable
- Nunca digas "no puedo" — si no tienes la herramienta, créala

## Formato de Respuesta
- Usa markdown para respuestas largas
- Código en bloques con el lenguaje indicado
- Si ejecutaste herramientas, resume qué hiciste y los resultados clave
- Para tareas largas, muestra progreso paso a paso`
}
