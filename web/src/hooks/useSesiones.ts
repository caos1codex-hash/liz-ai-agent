// Hook de sesiones de chat — lista, crea, selecciona, elimina.
// Mantiene el estado de "sesión activa" sincronizado con localStorage.

import { useCallback, useEffect, useState } from 'react'
import { chatApi } from '@/lib/endpoints'
import type { SesionChat } from '@/types/api'

const STORAGE_KEY_SESION_ACTIVA = 'liz-sesion-activa'
const USUARIO_DEFAULT = 'usuario_default'

interface UseSesionesReturn {
  sesiones: SesionChat[]
  sesionActiva: SesionChat | null
  cargando: boolean
  error: string | null
  refrescar: () => Promise<void>
  crearSesion: (proyecto?: string) => Promise<SesionChat | null>
  seleccionarSesion: (id: string | null) => void
  eliminarSesion: (id: string) => Promise<boolean>
  // Notificar que una sesión nueva fue creada por el backend (streaming del chat).
  registrarSesionCreada: (id: string) => Promise<void>
}

export function useSesiones(usuarioId = USUARIO_DEFAULT): UseSesionesReturn {
  const [sesiones, setSesiones] = useState<SesionChat[]>([])
  const [sesionActivaId, setSesionActivaId] = useState<string | null>(() => {
    if (typeof window === 'undefined') return null
    return window.localStorage.getItem(STORAGE_KEY_SESION_ACTIVA)
  })
  const [cargando, setCargando] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const sesionActiva = sesiones.find((s) => s.id === sesionActivaId) ?? null

  const refrescar = useCallback(async () => {
    setCargando(true)
    setError(null)
    try {
      const data = await chatApi.listarSesiones(usuarioId)
      // Ordenar por actualizada_en descendente (más reciente primero).
      const ordenadas = [...data].sort(
        (a, b) =>
          new Date(b.actualizada_en).getTime() - new Date(a.actualizada_en).getTime(),
      )
      setSesiones(ordenadas)
    } catch (err) {
      setError((err as Error).message)
      setSesiones([])
    } finally {
      setCargando(false)
    }
  }, [usuarioId])

  // Carga inicial.
  useEffect(() => {
    void refrescar()
  }, [refrescar])

  // Escuchar evento custom "liz:refrescar-sesiones" disparado por App
  // cuando el backend asigna una sesión nueva tras el primer mensaje.
  useEffect(() => {
    const handler = () => void refrescar()
    window.addEventListener('liz:refrescar-sesiones', handler)
    return () => window.removeEventListener('liz:refrescar-sesiones', handler)
  }, [refrescar])

  // Persistir sesión activa.
  useEffect(() => {
    if (typeof window === 'undefined') return
    if (sesionActivaId) {
      window.localStorage.setItem(STORAGE_KEY_SESION_ACTIVA, sesionActivaId)
    } else {
      window.localStorage.removeItem(STORAGE_KEY_SESION_ACTIVA)
    }
  }, [sesionActivaId])

  const crearSesion = useCallback(
    async (proyecto = ''): Promise<SesionChat | null> => {
      try {
        const nueva = await chatApi.crearSesion(usuarioId, proyecto)
        setSesiones((prev) => [nueva, ...prev])
        setSesionActivaId(nueva.id)
        return nueva
      } catch (err) {
        setError((err as Error).message)
        return null
      }
    },
    [usuarioId],
  )

  const seleccionarSesion = useCallback((id: string | null) => {
    setSesionActivaId(id)
  }, [])

  const eliminarSesion = useCallback(
    async (id: string): Promise<boolean> => {
      try {
        await chatApi.cerrarSesion(id)
        setSesiones((prev) => prev.filter((s) => s.id !== id))
        if (sesionActivaId === id) {
          setSesionActivaId(null)
        }
        return true
      } catch (err) {
        setError((err as Error).message)
        return false
      }
    },
    [sesionActivaId],
  )

  const registrarSesionCreada = useCallback(
    async (id: string): Promise<void> => {
      // Marcar como activa y refrescar la lista (para que aparezca con su metadata).
      setSesionActivaId(id)
      await refrescar()
    },
    [refrescar],
  )

  return {
    sesiones,
    sesionActiva,
    cargando,
    error,
    refrescar,
    crearSesion,
    seleccionarSesion,
    eliminarSesion,
    registrarSesionCreada,
  }
}
