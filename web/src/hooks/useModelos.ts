// Hook de modelos del orquestador NVIDIA.
// Carga una sola vez al montar (los modelos no cambian en runtime).

import { useCallback, useEffect, useState } from 'react'
import { orquestadorApi } from '@/lib/endpoints'
import type { ModeloOrquestador } from '@/types/api'

interface UseModelosReturn {
  modelos: ModeloOrquestador[]
  cargando: boolean
  error: string | null
  refrescar: () => Promise<void>
}

export function useModelos(): UseModelosReturn {
  const [modelos, setModelos] = useState<ModeloOrquestador[]>([])
  const [cargando, setCargando] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const refrescar = useCallback(async () => {
    setError(null)
    try {
      const data = await orquestadorApi.modelos()
      setModelos(data ?? [])
    } catch (err) {
      // Silencioso: si no hay orquestador, el header simplemente no muestra el badge.
      setError((err as Error).message)
      setModelos([])
    } finally {
      setCargando(false)
    }
  }, [])

  useEffect(() => {
    void refrescar()
  }, [refrescar])

  return { modelos, cargando, error, refrescar }
}
