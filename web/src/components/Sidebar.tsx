// Sidebar — panel lateral con historial de conversaciones.
// Lista sesiones, permite crear nueva, seleccionar, eliminar.

import { useState } from 'react'
import { useSesiones } from '@/hooks/useSesiones'
import { SidebarHeader } from './SidebarHeader'
import { SidebarItem } from './SidebarItem'
import { cn } from '@/lib/utils'

interface SidebarProps {
  usuarioId?: string
  sesionActivaId: string | null
  onSeleccionarSesion: (id: string) => void
  onNuevaSesionCreada: (sesion: { id: string; titulo?: string }) => void
  className?: string
}

export function Sidebar({
  usuarioId = 'usuario_default',
  sesionActivaId,
  onSeleccionarSesion,
  onNuevaSesionCreada,
  className,
}: SidebarProps) {
  const {
    sesiones,
    cargando,
    error,
    refrescar,
    crearSesion,
    eliminarSesion,
  } = useSesiones(usuarioId)

  const [creando, setCreando] = useState(false)
  const [confirmarEliminar, setConfirmarEliminar] = useState<string | null>(null)

  const handleNueva = async () => {
    setCreando(true)
    const nueva = await crearSesion()
    setCreando(false)
    if (nueva) {
      onSeleccionarSesion(nueva.id)
      onNuevaSesionCreada({ id: nueva.id, titulo: nueva.titulo })
    }
  }

  const handleEliminar = async (id: string) => {
    if (confirmarEliminar !== id) {
      setConfirmarEliminar(id)
      // Auto-reset tras 3s si no confirma.
      setTimeout(() => {
        setConfirmarEliminar((curr) => (curr === id ? null : curr))
      }, 3000)
      return
    }
    setConfirmarEliminar(null)
    const ok = await eliminarSesion(id)
    if (ok && sesionActivaId === id) {
      // Si eliminamos la activa, el padre decide qué hacer (normalmente: ir a "nueva conversación").
      onSeleccionarSesion('' as string) // vacío = sin sesión
    }
  }

  return (
    <div className={cn('flex min-h-0 flex-1 flex-col', className)}>
      <SidebarHeader onNuevaConversacion={handleNueva} creando={creando} />

      {/* Estado: cargando / error / vacío / lista */}
      <div className="min-h-0 flex-1 overflow-y-auto p-2">
        {cargando && sesiones.length === 0 ? (
          <div className="space-y-2 p-2">
            {[1, 2, 3].map((i) => (
              <div
                key={i}
                className="h-12 animate-pulse-soft rounded-lg bg-surface-muted dark:bg-surface-dark-muted"
              />
            ))}
          </div>
        ) : error ? (
          <div className="p-3">
            <div className="rounded-lg border border-rose-300 bg-rose-50 p-3 text-xs text-rose-700 dark:border-rose-800 dark:bg-rose-950/30 dark:text-rose-300">
              <p className="font-medium">No se pudieron cargar las conversaciones</p>
              <p className="mt-1 break-all font-mono text-[10px]">{error}</p>
              <button onClick={refrescar} className="btn-ghost mt-2 text-xs">
                Reintentar
              </button>
            </div>
          </div>
        ) : sesiones.length === 0 ? (
          <div className="px-4 py-8 text-center">
            <div className="mb-2 text-3xl">💬</div>
            <p className="text-sm font-medium text-text dark:text-text-dark">
              Sin conversaciones todavía
            </p>
            <p className="mt-1 text-xs text-text-muted dark:text-text-dark-muted">
              Crea una nueva con el botón de arriba, o simplemente escribe un mensaje.
            </p>
          </div>
        ) : (
          <div className="space-y-0.5">
            {sesiones.map((s) => (
              <SidebarItem
                key={s.id}
                sesion={s}
                activa={s.id === sesionActivaId}
                onClick={() => onSeleccionarSesion(s.id)}
                onEliminar={() => handleEliminar(s.id)}
              />
            ))}
            {confirmarEliminar && (
              <div className="mt-2 rounded-lg border border-amber-300 bg-amber-50 p-2 text-xs text-amber-800 dark:border-amber-800 dark:bg-amber-950/30 dark:text-amber-200">
                ¿Eliminar?{' '}
                <button
                  onClick={() => handleEliminar(confirmarEliminar)}
                  className="font-semibold underline"
                >
                  Sí
                </button>{' '}
                ·{' '}
                <button
                  onClick={() => setConfirmarEliminar(null)}
                  className="font-semibold underline"
                >
                  No
                </button>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Footer del sidebar */}
      <div className="border-t border-border px-3 py-2 text-[10px] text-text-subtle dark:border-border-dark dark:text-text-dark-subtle">
        {sesiones.length > 0 && (
          <span>{sesiones.length} conversación{sesiones.length !== 1 ? 'es' : ''}</span>
        )}
      </div>
    </div>
  )
}
