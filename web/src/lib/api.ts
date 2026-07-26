// API client base — wrapper sobre fetch con timeout, JSON parsing, y manejo de errores.
// Todos los endpoints van bajo /api/v1/* (proxy Vite → localhost:3000 en dev).

import type { RespuestaAPI } from '@/types/api'

const DEFAULT_TIMEOUT_MS = 30_000

export class APIError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly endpoint: string,
    public readonly cause?: unknown,
  ) {
    super(message)
    this.name = 'APIError'
  }
}

interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE'
  body?: unknown
  signal?: AbortSignal
  timeoutMs?: number
  // Si true, retorna el Response raw (útil para SSE / blob).
  raw?: boolean
  // Si false, no lanza error y retorna la respuesta cruda.
  throwOnError?: boolean
}

/**
 * Realiza una petición HTTP al backend de Liz.
 *
 * @param endpoint — path relativo bajo /api/v1 (ej: "/chat", "/health")
 * @param opts — opciones de la petición
 * @returns datos deserializados (o Response si opts.raw=true)
 *
 * @throws APIError si status >= 400 o si el backend devuelve exito=false
 */
export async function apiRequest<T = unknown>(
  endpoint: string,
  opts: RequestOptions = {},
): Promise<T> {
  const {
    method = 'GET',
    body,
    signal,
    timeoutMs = DEFAULT_TIMEOUT_MS,
    raw = false,
    throwOnError = true,
  } = opts

  const url = endpoint.startsWith('/api/')
    ? endpoint
    : `/api/v1${endpoint.startsWith('/') ? endpoint : `/${endpoint}`}`

  // Componer AbortSignal: combina timeout + signal externo si viene.
  const controller = new AbortController()
  const timeoutId = setTimeout(() => controller.abort(), timeoutMs)
  if (signal) {
    signal.addEventListener('abort', () => controller.abort())
  }

  const headers: Record<string, string> = { Accept: 'application/json' }
  let bodyText: string | undefined
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json'
    bodyText = JSON.stringify(body)
  }

  let response: Response
  try {
    response = await fetch(url, {
      method,
      headers,
      body: bodyText,
      signal: controller.signal,
      credentials: 'same-origin',
    })
  } catch (err) {
    clearTimeout(timeoutId)
    if (err instanceof DOMException && err.name === 'AbortError') {
      throw new APIError(`Timeout o cancelado: ${url}`, 408, endpoint, err)
    }
    throw new APIError(
      `No se pudo conectar al backend (${url}). ¿Está Liz corriendo en :3000?`,
      0,
      endpoint,
      err,
    )
  } finally {
    clearTimeout(timeoutId)
  }

  if (raw) {
    if (throwOnError && !response.ok) {
      throw new APIError(
        `HTTP ${response.status} ${response.statusText} en ${url}`,
        response.status,
        endpoint,
      )
    }
    return response as unknown as T
  }

  if (!response.ok) {
    let errMsg = `HTTP ${response.status} ${response.statusText}`
    try {
      const errBody = (await response.json()) as RespuestaAPI
      if (errBody?.error) errMsg = errBody.error
      else if (errBody?.mensaje) errMsg = errBody.mensaje
    } catch {
      /* noop */
    }
    if (throwOnError) {
      throw new APIError(errMsg, response.status, endpoint)
    }
    return { exito: false, error: errMsg, timestamp: new Date().toISOString() } as unknown as T
  }

  const data = (await response.json()) as RespuestaAPI<T>
  if (throwOnError && data.exito === false) {
    throw new APIError(data.error ?? 'Error desconocido', response.status, endpoint)
  }
  return (data.datos ?? data) as T
}

// Atajos comunes.
export const api = {
  get: <T = unknown>(endpoint: string, opts?: Omit<RequestOptions, 'method' | 'body'>) =>
    apiRequest<T>(endpoint, { ...opts, method: 'GET' }),

  post: <T = unknown>(endpoint: string, body?: unknown, opts?: Omit<RequestOptions, 'method' | 'body'>) =>
    apiRequest<T>(endpoint, { ...opts, method: 'POST', body }),

  put: <T = unknown>(endpoint: string, body?: unknown, opts?: Omit<RequestOptions, 'method' | 'body'>) =>
    apiRequest<T>(endpoint, { ...opts, method: 'PUT', body }),

  delete: <T = unknown>(endpoint: string, opts?: Omit<RequestOptions, 'method' | 'body'>) =>
    apiRequest<T>(endpoint, { ...opts, method: 'DELETE' }),

  // Para SSE / streaming: retorna el Response raw.
  stream: (endpoint: string, body: unknown, opts?: Omit<RequestOptions, 'method' | 'body' | 'raw'>) =>
    apiRequest<Response>(endpoint, {
      ...opts,
      method: 'POST',
      body,
      raw: true,
      timeoutMs: 0, // sin timeout para streams
    }),
}
