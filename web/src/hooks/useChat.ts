// Hook central del chat — orquesta mensajes locales + SSE streaming.
// Expone: mensajes, enviando, error, enviar(), limpiar().

import { useCallback, useEffect, useRef, useState } from 'react'
import { streamChat } from '@/lib/sse'
import { chatApi } from '@/lib/endpoints'
import type { CategoriaTarea, ChunkStream, SolicitudChat } from '@/types/api'
import { shortId } from '@/lib/utils'

// Mensaje local — combina MensajeChat del backend + estado de UI (streaming, error).
export interface UIMensaje {
  id: string
  rol: 'usuario' | 'asistente'
  contenido: string
  timestamp: number
  categoria?: CategoriaTarea
  modeloUsado?: string
  herramientasUsadas?: string[]
  pasosEjecutados?: number
  duracionMs?: number
  tokens?: number
  error?: string
  // Estado de UI:
  streaming?: boolean
  pending?: boolean // mensaje del usuario aún no confirmado por el backend
}

interface UseChatOptions {
  sesionId?: string
  usuarioId?: string
  proyecto?: string
  // Hook para notificar cuando se completa un mensaje (ej: guardar en historial).
  onMensajeCompletado?: (mensaje: UIMensaje) => void
}

interface UseChatReturn {
  mensajes: UIMensaje[]
  enviando: boolean
  error: string | null
  categoriaActual: CategoriaTarea | null
  modeloUsado: string | null
  enviar: (texto: string) => Promise<void>
  cancelar: () => void
  limpiar: () => void
  setSesionId: (id: string | undefined) => void
}

export function useChat(opts: UseChatOptions = {}): UseChatReturn {
  const { sesionId: initialSesionId, usuarioId = 'usuario_default', proyecto } = opts

  const [mensajes, setMensajes] = useState<UIMensaje[]>([])
  const [enviando, setEnviando] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [categoriaActual, setCategoriaActual] = useState<CategoriaTarea | null>(null)
  const [modeloUsado, setModeloUsado] = useState<string | null>(null)
  const [sesionId, setSesionId] = useState<string | undefined>(initialSesionId)

  const abortRef = useRef<AbortController | null>(null)
  const asistenteMsgIdRef = useRef<string | null>(null)

  // Sincronizar sesión entrante.
  useEffect(() => {
    setSesionId(initialSesionId)
  }, [initialSesionId])

  // Cargar mensajes de una sesión existente (si hay sesionId).
  useEffect(() => {
    if (!sesionId) {
      setMensajes([])
      return
    }
    let cancelled = false
    ;(async () => {
      try {
        const mensajesBackend = await chatApi.mensajesSesion(sesionId)
        if (cancelled) return
        const uiMensajes: UIMensaje[] = mensajesBackend.map((m) => ({
          id: m.id,
          rol: m.rol === 'usuario' ? 'usuario' : 'asistente',
          contenido: m.contenido,
          timestamp: new Date(m.timestamp).getTime(),
        }))
        setMensajes(uiMensajes)
      } catch (err) {
        // Silencioso: si no hay sesión previa, empezamos vacío.
        if (!cancelled) setMensajes([])
        console.warn('[useChat] No se pudieron cargar mensajes previos:', err)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [sesionId])

  const cancelar = useCallback(() => {
    abortRef.current?.abort()
    abortRef.current = null
    setEnviando(false)
    // Marcar el mensaje asistente como no-streaming (preserva lo que se haya recibido).
    setMensajes((prev) =>
      prev.map((m) =>
        m.id === asistenteMsgIdRef.current ? { ...m, streaming: false } : m,
      ),
    )
    asistenteMsgIdRef.current = null
  }, [])

  const enviar = useCallback(
    async (texto: string) => {
      const textoLimpio = texto.trim()
      if (!textoLimpio || enviando) return

      setError(null)
      setEnviando(true)

      // Mensaje del usuario (local, optimista).
      const userMsg: UIMensaje = {
        id: shortId(),
        rol: 'usuario',
        contenido: textoLimpio,
        timestamp: Date.now(),
        pending: true,
      }

      // Mensaje asistente placeholder (se va llenando con chunks SSE).
      const asistenteId = shortId()
      const asistenteMsg: UIMensaje = {
        id: asistenteId,
        rol: 'asistente',
        contenido: '',
        timestamp: Date.now(),
        streaming: true,
      }

      asistenteMsgIdRef.current = asistenteId
      setMensajes((prev) => [...prev, userMsg, asistenteMsg])

      // Acumuladores para el mensaje asistente
      let textoAcumulado = ''
      const herramientasUsadas = new Set<string>()
      let categoriaLocal: CategoriaTarea | null = null
      let modeloLocal: string | null = null
      let pasosEjecutados = 0

      const solicitud: SolicitudChat = {
        mensaje: textoLimpio,
        usuario_id: usuarioId,
        sesion_id: sesionId,
        proyecto: proyecto || undefined,
        stream: true,
      }

      const controller = new AbortController()
      abortRef.current = controller

      const actualizarAsistente = (patch: Partial<UIMensaje>) => {
        setMensajes((prev) =>
          prev.map((m) => (m.id === asistenteId ? { ...m, ...patch } : m)),
        )
      }

      try {
        await streamChat({
          solicitud,
          signal: controller.signal,
          onChunk: (chunk: ChunkStream) => {
            switch (chunk.tipo) {
              case 'texto':
                textoAcumulado += chunk.contenido
                actualizarAsistente({ contenido: textoAcumulado })
                break
              case 'herramienta':
                if (chunk.contenido) herramientasUsadas.add(chunk.contenido)
                if (chunk.datos && typeof chunk.datos === 'object') {
                  const datos = chunk.datos as { nombre?: string; herramienta?: string }
                  if (datos.nombre) herramientasUsadas.add(datos.nombre)
                  if (datos.herramienta) herramientasUsadas.add(datos.herramienta)
                }
                actualizarAsistente({ herramientasUsadas: Array.from(herramientasUsadas) })
                break
              case 'estado':
                // El backend envía estados del pipeline ("Iniciando...", "Clasificando...", etc.)
                actualizarAsistente({
                  contenido: textoAcumulado || chunk.contenido,
                })
                break
              case 'error':
                actualizarAsistente({ error: chunk.contenido })
                break
              case 'pensamiento':
                // Por ahora mostramos los pensamientos como parte del flujo (P8.5 puede añadir UI dedicada).
                break
              default:
                // Chunk desconocido: ignorar.
                break
            }
            if (chunk.modelo) {
              modeloLocal = chunk.modelo
              setModeloUsado(modeloLocal)
              actualizarAsistente({ modeloUsado: modeloLocal })
            }
          },
          onFinal: (resumen) => {
            actualizarAsistente({
              streaming: false,
              modeloUsado: resumen.modelo,
              tokens: resumen.tokens,
              pasosEjecutados: resumen.pasos_ejecutados,
              duracionMs: resumen.duracion_ms,
            })
            if (resumen.sesion_id && !sesionId) {
              setSesionId(resumen.sesion_id)
            }
          },
          onError: (err) => {
            setError(err.message)
            actualizarAsistente({
              streaming: false,
              error: err.message,
              contenido: textoAcumulado || '⚠️ No se pudo generar la respuesta.',
            })
          },
        })

        // Marcar mensaje usuario como confirmado.
        setMensajes((prev) =>
          prev.map((m) => (m.id === userMsg.id ? { ...m, pending: false } : m)),
        )

        // Notificar completado.
        if (opts.onMensajeCompletado) {
          const finalMensaje: UIMensaje = {
            ...asistenteMsg,
            contenido: textoAcumulado,
            streaming: false,
            categoria: categoriaLocal ?? undefined,
            modeloUsado: modeloLocal ?? undefined,
            herramientasUsadas: Array.from(herramientasUsadas),
            pasosEjecutados,
          }
          opts.onMensajeCompletado(finalMensaje)
        }
      } catch (err) {
        const msg = (err as Error).message
        setError(msg)
        actualizarAsistente({
          streaming: false,
          error: msg,
          contenido: textoAcumulado || '⚠️ Error en la comunicación con el backend.',
        })
      } finally {
        setEnviando(false)
        abortRef.current = null
        asistenteMsgIdRef.current = null
      }
    },
    [enviando, proyecto, sesionId, usuarioId, opts],
  )

  const limpiar = useCallback(() => {
    setMensajes([])
    setError(null)
    setCategoriaActual(null)
    setModeloUsado(null)
  }, [])

  return {
    mensajes,
    enviando,
    error,
    categoriaActual,
    modeloUsado,
    enviar,
    cancelar,
    limpiar,
    setSesionId,
  }
}
