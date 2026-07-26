// App root — orquesta theme + sidebar + chat + header + proyecto selector + toasts.

import { useCallback, useState } from 'react'
import { useTheme } from '@/hooks/useTheme'
import { AppShell } from '@/components/AppShell'
import { ChatWindow } from '@/components/ChatWindow'
import { Sidebar } from '@/components/Sidebar'
import { Header } from '@/components/Header'
import { ToastProvider, useToast } from '@/components/Toast'
import type { UIMensaje } from '@/hooks/useChat'

function AppInner() {
  // Theme system (oscuro por defecto, persistencia localStorage).
  useTheme()

  // Sesión activa: null = nueva conversación anónima.
  const [sesionActivaId, setSesionActivaId] = useState<string | null>(null)

  // Proyecto de contexto actual (afecta el prompt del pipeline).
  const [proyectoActual, setProyectoActual] = useState<string | null>(null)

  // Modelo usado en el último mensaje (para el badge del header).
  const [ultimoModelo, setUltimoModelo] = useState<string | null>(null)

  // Drawer móvil del sidebar.
  const [drawerOpen, setDrawerOpen] = useState(false)

  const toast = useToast()

  const handleSeleccionarSesion = useCallback((id: string) => {
    setSesionActivaId(id === '' ? null : id)
    setDrawerOpen(false)
  }, [])

  const handleNuevaSesionCreada = useCallback((sesion: { id: string; titulo?: string }) => {
    setSesionActivaId(sesion.id)
  }, [])

  const handleSesionBackendCreada = useCallback((id: string) => {
    setSesionActivaId(id)
    window.dispatchEvent(new CustomEvent('liz:refrescar-sesiones'))
  }, [])

  const handleMensajeCompletado = useCallback(
    (msg: UIMensaje) => {
      if (msg.modeloUsado) {
        setUltimoModelo(msg.modeloUsado)
      }
      if (msg.error) {
        toast.showError(`Error en respuesta: ${msg.error}`)
      }
    },
    [toast],
  )

  return (
    <AppShell
      sidebarOpen={drawerOpen}
      onToggleSidebar={() => setDrawerOpen((v) => !v)}
      sidebar={
        <Sidebar
          sesionActivaId={sesionActivaId}
          onSeleccionarSesion={handleSeleccionarSesion}
          onNuevaSesionCreada={handleNuevaSesionCreada}
        />
      }
      header={
        <Header
          proyectoActual={proyectoActual}
          onCambiarProyecto={setProyectoActual}
          modeloEnUso={ultimoModelo}
          onToggleSidebar={() => setDrawerOpen((v) => !v)}
        />
      }
    >
      <ChatWindow
        sesionId={sesionActivaId ?? undefined}
        proyecto={proyectoActual ?? undefined}
        onMensajeCompletado={handleMensajeCompletado}
        onSesionCreada={handleSesionBackendCreada}
      />
    </AppShell>
  )
}

export default function App() {
  return (
    <ToastProvider>
      <AppInner />
    </ToastProvider>
  )
}
