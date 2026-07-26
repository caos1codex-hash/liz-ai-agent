// ChatWindow — orquesta MessageList + MessageInput usando useChat().
// Recibe sesión/proyecto opcionales y notifica al padre cuando el backend asigna sesión.

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
    onMensajeCompletado,
    onSesionAsignada: (id) => {
      // El backend creó/asignó una sesión tras el primer mensaje.
      onSesionCreada?.(id)
    },
  })

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {error && (
        <div className="border-b border-rose-300 bg-rose-50 px-4 py-2 text-xs text-rose-700 dark:border-rose-800 dark:bg-rose-950/30 dark:text-rose-300">
          <strong>Error:</strong> {error}
        </div>
      )}
      <MessageList mensajes={mensajes} enviando={enviando} />
      <MessageInput onEnviar={enviar} onCancelar={cancelar} enviando={enviando} />
    </div>
  )
}
