// ChatWindow — orquesta MessageList + MessageInput usando useChat().
// Recibe sesión/proyecto opcionales y notifica al padre cuando llega un chunk "completado".

import { useChat, type UIMensaje } from '@/hooks/useChat'
import { MessageList } from './MessageList'
import { MessageInput } from './MessageInput'

interface ChatWindowProps {
  sesionId?: string
  usuarioId?: string
  proyecto?: string
  onMensajeCompletado?: (mensaje: UIMensaje) => void
  onSesionCreada?: (sesionId: string) => void
}

export function ChatWindow({
  sesionId,
  usuarioId,
  proyecto,
  onMensajeCompletado,
  onSesionCreada,
}: ChatWindowProps) {
  const { mensajes, enviando, error, enviar, cancelar } = useChat({
    sesionId,
    usuarioId,
    proyecto,
    onMensajeCompletado: (msg) => {
      onMensajeCompletado?.(msg)
      // Si el backend asignó una sesión nueva, notificar.
      // El hook setSesionId se encarga internamente; el padre se entera vía callback.
    },
  })

  // Hook adicional: si llega un mensaje completado con sesión, propagar.
  // El hook useChat internamente actualiza su sesión; aquí lo propagamos al padre.
  // (En P8.3 conectaremos esto con el sidebar.)
  const handleEnviar = async (texto: string) => {
    await enviar(texto)
    // Después de enviar, si la sesión era nueva, el backend la habrá creado.
    // El hook useChat setSesionId() lo maneja internamente.
    // El padre recibirá el callback onMensajeCompletado si está suscrito.
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {error && (
        <div className="border-b border-rose-300 bg-rose-50 px-4 py-2 text-xs text-rose-700 dark:border-rose-800 dark:bg-rose-950/30 dark:text-rose-300">
          <strong>Error:</strong> {error}
        </div>
      )}
      <MessageList mensajes={mensajes} enviando={enviando} />
      <MessageInput onEnviar={handleEnviar} onCancelar={cancelar} enviando={enviando} />
      {/* Spacer para que el sidebar/header de P8.3/P8.4 puedan acoplarse */}
      <SessionProbe onSesionCreada={onSesionCreada} sesionIdActual={sesionId} />
    </div>
  )
}

/**
 * Helper interno para propagar el ID de sesión nueva al padre.
 * No renderiza nada visible — solo observa el cambio de sesión vía effect secundario.
 *
 * TODO(P8.3): reemplazar con un callback directo desde useChat cuando se
 * sincronice el estado de sesión entre sidebar y chat.
 */
function SessionProbe({
  onSesionCreada,
  sesionIdActual,
}: {
  onSesionCreada?: (id: string) => void
  sesionIdActual?: string
}) {
  // No-op: el hook useChat ya notifica via onMensajeCompletado.
  // Este componente existe como punto de extensión futuro.
  if (!onSesionCreada || !sesionIdActual) return null
  return null
}
