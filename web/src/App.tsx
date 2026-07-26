// App root — orquesta theme + sidebar + chat + header + proyecto selector.
// Estado elevado: sesión activa, proyecto actual, modelo en uso, drawer móvil.

import { useCallback, useState } from 'react'
import { useTheme } from '@/hooks/useTheme'
import { AppShell } from '@/components/AppShell'
import { ChatWindow } from '@/components/ChatWindow'
import { Sidebar } from '@/components/Sidebar'
import { Header } from '@/components/Header'
import type { UIMensaje } from '@/hooks/useChat'

export default function App() {
  // Inicializa el theme system (oscuro por defecto, persistencia localStorage).
  // Se llama dos veces intencionalmente: aquí para garantizar setup temprano,
  // y el Header también lo usa para el toggle (mismo hook, mismo storage key).
  useTheme()

  // Sesión activa: null = nueva conversación anónima.
  const [sesionActivaId, setSesionActivaId] = useState<string | null>(null)

  // Proyecto de contexto actual (afecta el prompt del pipeline).
  const [proyectoActual, setProyectoActual] = useState<string | null>(null)

  // Modelo usado en el último mensaje (para el badge del header).
  const [ultimoModelo, setUltimoModelo] = useState<string | null>(null)

  // Drawer móvil del sidebar.
  const [drawerOpen, setDrawerOpen] = useState(false)

  const handleSeleccionarSesion = useCallback((id: string) => {
    setSesionActivaId(id === '' ? null : id)
    setDrawerOpen(false) // cerrar drawer en móvil
  }, [])

  const handleNuevaSesionCreada = useCallback((sesion: { id: string; titulo?: string }) => {
    setSesionActivaId(sesion.id)
  }, [])

  const handleSesionBackendCreada = useCallback((id: string) => {
    setSesionActivaId(id)
    window.dispatchEvent(new CustomEvent('liz:refrescar-sesiones'))
  }, [])

  const handleMensajeCompletado = useCallback((msg: UIMensaje) => {
    if (msg.modeloUsado) {
      setUltimoModelo(msg.modeloUsado)
    }
  }, [])

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
