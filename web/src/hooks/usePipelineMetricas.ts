// Hook de métricas del pipeline de chat.
// Poll /api/v1/chat cada 60s (las métricas no cambian tan seguido).

import { useCallback, useEffect, useState } from 'react'
import { chatApi } from '@/lib/endpoints'
import type { EstadoPipeline } from '@/types/api'

const POLL_INTERVAL_MS = 60_000

interface UsePipelineMetricasReturn {
  estado: EstadoPipeline | null
  cargando: boolean
  error: string | null
  refrescar: () => Promise<void>
}

export function usePipelineMetricas(): UsePipelineMetricasReturn {
  const [estado, setEstado] = useState<EstadoPipeline | null>(null)
  const [cargando, setCargando] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const refrescar = useCallback(async () => {
    setError(null)
    try {
      const data = await chatApi.estado()
      setEstado(data)
    } catch (err) {
      // Silencioso: el header muestra el dot offline por separado.
      setError((err as Error).message)
      setEstado(null)
    } finally {
      setCargando(false)
    }
  }, [])

  useEffect(() => {
    void refrescar()
    const id = setInterval(refrescar, POLL_INTERVAL_MS)
    return () => clearInterval(id)
  }, [refrescar])

  return { estado, cargando, error, refrescar }
}
