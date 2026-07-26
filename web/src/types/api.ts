// Tipos compartidos — espejo de los structs Go del backend.
// Mantener sincronizado con:
//   - internal/pipeline/tipos.go (SolicitudChat, RespuestaPipeline, ChunkStream, EstadoPipeline)
//   - internal/nucleo/memoria/sesiones.go (Sesion)
//   - internal/nucleo/servidor/servidor.go (RespuestaAPI)
//   - internal/nucleo/orquestador/orquestador.go (Modelo, Metricas)

// RespuestaAPI — formato estándar de todas las respuestas del backend.
export interface RespuestaAPI<T = unknown> {
  exito: boolean
  mensaje?: string
  datos?: T
  error?: string
  timestamp: string
}

// CategoríaTarea — tipo de tarea clasificada por el pipeline.
export type CategoriaTarea =
  | 'conversacion'
  | 'codigo'
  | 'archivos'
  | 'procesos'
  | 'monitorizacion'
  | 'instalacion'
  | 'busqueda'
  | 'analisis'
  | 'auto_creacion'
  | 'ejecucion_comando'

// SolicitudChat — entrada del usuario para el pipeline.
export interface SolicitudChat {
  mensaje: string
  usuario_id?: string
  sesion_id?: string
  proyecto?: string
  stream?: boolean
}

// MensajeChat — un mensaje completo en el pipeline.
export interface MensajeChat {
  id: string
  sesion_id: string
  usuario_id: string
  contenido: string
  rol: 'usuario' | 'asistente'
  timestamp: string
  tokens_estimados: number
  metadata?: Record<string, unknown>
}

// ResultadoPaso — resultado de un paso del plan.
export interface ResultadoPaso {
  paso_id: number
  exito: boolean
  datos?: unknown
  error?: string
  duracion: number // nanoseconds en Go, interpretar como ms
  metadata?: Record<string, unknown>
  tool_usada?: string
}

// RespuestaPipeline — respuesta final del pipeline (modo JSON no-streaming).
export interface RespuestaPipeline {
  id: string
  sesion_id: string
  mensaje: string
  categoria: CategoriaTarea
  pasos_ejecutados: number
  resultados?: ResultadoPaso[]
  modelo_usado: string
  tokens_usados: number
  duracion_total: number
  timestamp: string
  metadata?: Record<string, unknown>
}

// ChunkStream — fragmento SSE del pipeline.
// tipos: "texto" | "herramienta" | "error" | "estado" | "hecho" | "pensamiento"
export interface ChunkStream {
  tipo: string
  contenido: string
  datos?: unknown
  paso_id?: number
  modelo?: string
}

// ChunkFinal — chunk "completado" enviado al final del stream.
export interface ChunkFinal {
  tipo: 'completado'
  sesion_id: string
  modelo: string
  tokens: number
  duracion_ms: number
  pasos_ejecutados: number
}

// EstadoPipeline — métricas devueltas por GET /api/v1/chat.
export interface EstadoPipeline {
  mensajes_procesados: number
  promedio_duracion: string
  ultimo_uso: string
  categorias: Record<CategoriaTarea, number>
  modelo_mas_usado: string
  fase: string
}

// SesionChat — sesión conversacional en memoria.
export interface SesionChat {
  id: string
  usuario_id: string
  proyecto: string
  titulo: string
  creada_en: string
  actualizada_en: string
  cerrada: boolean
  mensajes?: MensajeChat[]
}

// ModeloOrquestador — info de un modelo NVIDIA.
export interface ModeloOrquestador {
  id: string
  nombre: string
  tipo: string[]
  velocidad: string
  prioridad: number
  disponible?: boolean
}

// InfoHerramienta — info de una herramienta del catálogo.
export interface InfoHerramienta {
  nombre: string
  descripcion: string
  parametros?: ParametroHerramienta[]
}

export interface ParametroHerramienta {
  nombre: string
  tipo: string
  requerido: boolean
  descripcion: string
}

// HealthStatus — /api/v1/health
export interface HealthStatus {
  estado: string
  version: string
  uptime?: string
}

// ProyectoContexto — /api/v1/contexto/proyectos
export interface ProyectoContexto {
  nombre: string
  ruta: string
  archivos_indexados?: number
  ultimo_indexado?: string
}
