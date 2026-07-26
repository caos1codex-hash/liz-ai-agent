// Endpoints de la API agrupados por dominio.
// Cada función llama al endpoint correspondiente y retorna los datos tipados.

import { api } from './api'
import type {
  EstadoPipeline,
  HealthStatus,
  MensajeChat,
  ModeloOrquestador,
  ProyectoContexto,
  RespuestaPipeline,
  SesionChat,
  SolicitudChat,
  InfoHerramienta,
} from '@/types/api'

// --- Health ---
export const healthApi = {
  status: (signal?: AbortSignal) => api.get<HealthStatus>('/health', { signal }),
}

// --- Pipeline de Chat ---
export const chatApi = {
  estado: () => api.get<EstadoPipeline>('/chat'),
  metricas: () => api.get<EstadoPipeline>('/chat/metricas'),

  // Modo JSON (no streaming). Para streaming usar lib/sse.ts → streamChat().
  enviar: (solicitud: SolicitudChat) =>
    api.post<RespuestaPipeline>('/chat', { ...solicitud, stream: false }),

  // Listar sesiones del usuario (con query param correcto).
  listarSesiones: (usuarioId = 'usuario_default') =>
    api.get<SesionChat[]>(`/chat/sesiones?usuario_id=${encodeURIComponent(usuarioId)}`),

  crearSesion: (usuarioId = 'usuario_default', proyecto = '') =>
    api.post<SesionChat>('/chat/sesiones', { usuario_id: usuarioId, proyecto }),

  obtenerSesion: (id: string) => api.get<SesionChat>(`/chat/sesiones/${id}`),

  cerrarSesion: (id: string) => api.delete<void>(`/chat/sesiones/${id}`),

  mensajesSesion: (id: string) =>
    api.get<MensajeChat[]>(`/chat/sesiones/${id}`).then((s) => {
      // El backend retorna la sesión completa; los mensajes vienen dentro.
      const sesion = s as unknown as SesionChat & { mensajes?: MensajeChat[] }
      return sesion.mensajes ?? []
    }),
}

// --- Orquestador ---
export const orquestadorApi = {
  estado: () => api.get<{ disponible: boolean; modelos: number }>('/orquestador'),
  modelos: () => api.get<ModeloOrquestador[]>('/orquestador/modelos'),
  metricas: () => api.get<Record<string, unknown>>('/orquestador/metricas'),
}

// --- Herramientas ---
export const herramientasApi = {
  listar: () => api.get<InfoHerramienta[]>('/herramientas'),
  obtener: (nombre: string) => api.get<InfoHerramienta>(`/herramientas/${nombre}`),
  metricas: () => api.get<Record<string, unknown>>('/herramientas/metricas'),
}

// --- Contexto (proyectos) ---
export const contextoApi = {
  proyectos: () => api.get<ProyectoContexto[]>('/contexto/proyectos'),
  indexar: (ruta: string, nombre?: string) =>
    api.post<ProyectoContexto>('/contexto/proyectos', { ruta, nombre }),
  eliminar: (nombre: string) => api.delete<void>(`/contexto/proyectos/${nombre}`),
}
