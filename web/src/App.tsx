// App root — orquesta theme + sidebar + chat + header.
// Estado de sesión activa se eleva aquí para que sidebar y chat estén sincronizados.

import { useCallback, useState } from 'react'
import { useTheme } from '@/hooks/useTheme'
import { AppShell } from '@/components/AppShell'
import { ChatWindow } from '@/components/ChatWindow'
import { Sidebar } from '@/components/Sidebar'
import { StatusDot } from '@/components/StatusDot'

export default function App() {
  useTheme()

  // Sesión activa: string vacío = "nueva conversación sin sesión" (modo anónimo).
  // null = ninguna seleccionada (mostrar welcome).
  const [sesionActivaId, setSesionActivaId] = useState<string | null>(null)

  const handleSeleccionarSesion = useCallback((id: string) => {
    // id === '' → nueva conversación anónima (sin sesión creada todavía)
    setSesionActivaId(id === '' ? null : id)
  }, [])

  const handleNuevaSesionCreada = useCallback(
    (sesion: { id: string; titulo?: string }) => {
      setSesionActivaId(sesion.id)
    },
    [],
  )

  // Cuando el backend asigna una sesión nueva tras el primer mensaje,
  // lo notificamos al sidebar para que refresque.
  const handleSesionBackendCreada = useCallback((id: string) => {
    setSesionActivaId(id)
    // El sidebar se auto-refresca porque useSesiones tiene su propia lógica,
    // pero necesitamos forzar el refresh. Lo hacemos via custom event.
    window.dispatchEvent(new CustomEvent('liz:refrescar-sesiones'))
  }, [])

  return (
    <AppShell
      sidebar={
        <Sidebar
          sesionActivaId={sesionActivaId}
          onSeleccionarSesion={handleSeleccionarSesion}
          onNuevaSesionCreada={handleNuevaSesionCreada}
        />
      }
      header={
        <div className="flex w-full items-center justify-between">
          <div className="flex items-center gap-2">
            <img src="/liz.svg" alt="Liz" className="h-7 w-7 rounded-md" />
            <span className="font-semibold text-text dark:text-text-dark">Liz AI</span>
            <span className="ml-2 rounded-full bg-liz-100 px-2 py-0.5 text-xs font-medium text-liz-700 dark:bg-liz-900/40 dark:text-liz-300">
              Fase 8 · P8.3
            </span>
          </div>
          <div className="flex items-center gap-3 text-xs text-text-muted dark:text-text-dark-muted">
            <span className="hidden sm:inline">Sidebar + sesiones CRUD</span>
            <StatusDot />
          </div>
        </div>
      }
    >
      <ChatWindow
        sesionId={sesionActivaId ?? undefined}
        onSesionCreada={handleSesionBackendCreada}
      />
    </AppShell>
  )
}
