// App root — orquesta el theme provider y el ChatPage.

import { useTheme } from '@/hooks/useTheme'
import { AppShell } from '@/components/AppShell'
import { ChatWindow } from '@/components/ChatWindow'
import { StatusDot } from '@/components/StatusDot'

export default function App() {
  // Inicializa el theme system (oscuro por defecto, persistencia localStorage).
  useTheme()

  return (
    <AppShell
      header={
        <div className="flex w-full items-center justify-between">
          <div className="flex items-center gap-2">
            <img src="/liz.svg" alt="Liz" className="h-7 w-7 rounded-md" />
            <span className="font-semibold text-text dark:text-text-dark">Liz AI</span>
            <span className="ml-2 rounded-full bg-liz-100 px-2 py-0.5 text-xs font-medium text-liz-700 dark:bg-liz-900/40 dark:text-liz-300">
              Fase 8 · P8.2
            </span>
          </div>
          <div className="flex items-center gap-3 text-xs text-text-muted dark:text-text-dark-muted">
            <span className="hidden sm:inline">Chat core + SSE streaming</span>
            <StatusDot />
          </div>
        </div>
      }
    >
      <ChatWindow
        usuarioId="usuario_default"
        onMensajeCompletado={() => {
          /* P8.3: notificar al sidebar para refresh de sesiones */
        }}
        onSesionCreada={() => {
          /* P8.3: añadir nueva sesión al sidebar */
        }}
      />
    </AppShell>
  )
}
