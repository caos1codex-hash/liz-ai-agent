// Cliente SSE para el endpoint /api/v1/chat en modo streaming.
// El backend envía eventos "data: <json>\n\n" con chunks del pipeline.

import { APIError, api } from './api'
import type { ChunkStream, ChunkFinal, SolicitudChat } from '@/types/api'

export type ChunkHandler = (chunk: ChunkStream) => void
export type FinalHandler = (resumen: ChunkFinal) => void
export type ErrorHandler = (err: Error) => void

interface StreamChatOptions {
  solicitud: SolicitudChat
  onChunk: ChunkHandler
  onFinal?: FinalHandler
  onError?: ErrorHandler
  signal?: AbortSignal
}

/**
 * Envía un mensaje al pipeline en modo streaming y procesa los chunks SSE.
 *
 * Formato SSE del backend:
 *   data: {"tipo": "texto", "contenido": "...", ...}\n\n
 *   data: {"tipo": "completado", "sesion_id": "...", "modelo": "...", ...}\n\n
 *
 * Si el backend responde con un error HTTP, se llama onError con un APIError.
 * Si el stream se corta sin un chunk "completado", no se llama onFinal.
 */
export async function streamChat(opts: StreamChatOptions): Promise<void> {
  const { solicitud, onChunk, onFinal, onError, signal } = opts

  let response: Response
  try {
    response = await api.stream('/chat', { ...solicitud, stream: true }, { signal })
  } catch (err) {
    if (onError) onError(err as Error)
    else throw err
    return
  }

  if (!response.ok) {
    const err = new APIError(
      `HTTP ${response.status} ${response.statusText}`,
      response.status,
      '/chat',
    )
    if (onError) onError(err)
    else throw err
    return
  }

  if (!response.body) {
    const err = new APIError('Respuesta sin body (ReadableStream no soportado)', 0, '/chat')
    if (onError) onError(err)
    else throw err
    return
  }

  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  try {
    for (;;) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })

      // Procesar líneas "data: <json>\n\n"
      const events = buffer.split('\n\n')
      buffer = events.pop() ?? '' // último fragmento puede estar incompleto

      for (const event of events) {
        const line = event.trim()
        if (!line.startsWith('data:')) continue
        const jsonStr = line.slice(5).trim()
        if (!jsonStr) continue
        try {
          const chunk = JSON.parse(jsonStr) as ChunkStream | ChunkFinal
          if (chunk.tipo === 'completado') {
            onFinal?.(chunk as ChunkFinal)
          } else {
            onChunk(chunk as ChunkStream)
          }
        } catch (parseErr) {
          // Ignorar JSON malformado pero no abortar el stream
          console.warn('[SSE] chunk no-parseable:', jsonStr, parseErr)
        }
      }
    }
  } catch (err) {
    if ((err as Error).name === 'AbortError') return
    if (onError) onError(err as Error)
    else throw err
  } finally {
    try {
      reader.releaseLock()
    } catch {
      /* noop */
    }
  }
}
