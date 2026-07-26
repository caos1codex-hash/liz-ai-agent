// Hook de proyectos de contexto indexados en el backend.
// Carga una vez al montar. Permite refrescar manualmente tras indexar uno nuevo.

import { useCallback, useEffect, useState } from 'react'
import { contextoApi } from '@/lib/endpoints'
import type { ProyectoContexto } from '@/types/api'

interface UseProyectosReturn {
  proyectos: ProyectoContexto[]
  cargando: boolean
  error: string | null
  refrescar: () => Promise<void>
}

export function useProyectos(): UseProyectosReturn {
  const [proyectos, setProyectos] = useState<ProyectoContexto[]>([])
  const [cargando, setCargando] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const refrescar = useCallback(async () => {
    setError(null)
    try {
      const data = await contextoApi.proyectos()
      setProyectos(data ?? [])
    } catch (err) {
      setError((err as Error).message)
      setProyectos([])
    } finally {
      setCargando(false)
    }
  }, [])

  useEffect(() => {
    void refrescar()
  }, [refrescar])

  return { proyectos, cargando, error, refrescar }
}
