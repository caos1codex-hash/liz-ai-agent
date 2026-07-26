// MessageList — lista virtualizada-sencilla de mensajes con auto-scroll.
// Cuando el usuario está "abajo", sigue el stream. Si hace scroll up, se queda quieto.

import { useAutoScroll } from '@/hooks/useAutoScroll'
import { Message } from './Message'
import type { UIMensaje } from '@/hooks/useChat'
import { cn } from '@/lib/utils'

interface MessageListProps {
  mensajes: UIMensaje[]
  enviando: boolean
  className?: string
}

export function MessageList({ mensajes, enviando, className }: MessageListProps) {
  const { containerRef, isAtBottom, scrollToBottom } = useAutoScroll<HTMLDivElement>(
    [mensajes, enviando],
  )

  // Lista vacía: welcome state.
  if (mensajes.length === 0) {
    return (
      <div
        className={cn(
          'flex min-h-0 flex-1 items-center justify-center p-6',
          className,
        )}
      >
        <div className="max-w-md text-center animate-fade-in">
          <img
            src="/liz.svg"
            alt="Liz"
            className="mx-auto mb-4 h-16 w-16 rounded-2xl shadow-lg shadow-liz-600/20"
          />
          <h2 className="text-xl font-semibold text-text dark:text-text-dark">
            Hola, soy Liz
          </h2>
          <p className="mt-2 text-sm text-text-muted dark:text-text-dark-muted">
            Tu agente de IA para Linux. Pregúntame lo que quieras — archivos,
            procesos, código, instalación de software. Si no tengo una herramienta,
            la creo.
          </p>
          <div className="mt-5 grid grid-cols-1 gap-2 text-left sm:grid-cols-2">
            <ExamplePrompt
              text="Lista los procesos que consumen más RAM"
              hint="gestión de procesos"
            />
            <ExamplePrompt
              text="Busca todos los .log del mes pasado y dime cuánto ocupan"
              hint="búsqueda de archivos"
            />
            <ExamplePrompt
              text="Crea un servidor HTTP en Go con auth JWT"
              hint="escritura de código"
            />
            <ExamplePrompt
              text="Instala ripgrep y verifica que quedó accesible"
              hint="instalación de software"
            />
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className={cn('relative min-h-0 flex-1', className)}>
      <div
        ref={containerRef}
        className="h-full overflow-y-auto overflow-x-hidden"
        role="log"
        aria-live="polite"
        aria-busy={enviando}
      >
        <div className="mx-auto max-w-3xl pb-32 pt-2">
          {mensajes.map((m) => (
            <Message key={m.id} mensaje={m} />
          ))}
        </div>
      </div>
      {/* Botón "bajar" cuando el usuario se aleja del fondo durante streaming */}
      {!isAtBottom && (
        <button
          onClick={() => scrollToBottom()}
          className="absolute bottom-4 left-1/2 -translate-x-1/2 rounded-full border border-border bg-surface px-3 py-1.5 text-xs font-medium shadow-lg transition-all hover:bg-surface-muted dark:border-border-dark dark:bg-surface-dark-subtle dark:hover:bg-surface-dark-muted"
          aria-label="Ir al último mensaje"
        >
          ↓ Último mensaje
        </button>
      )}
    </div>
  )
}

function ExamplePrompt({ text, hint }: { text: string; hint: string }) {
  return (
    <div className="rounded-lg border border-border bg-surface p-3 text-xs dark:border-border-dark dark:bg-surface-dark-subtle">
      <div className="font-medium text-text dark:text-text-dark">{text}</div>
      <div className="mt-1 text-text-subtle dark:text-text-dark-subtle">{hint}</div>
    </div>
  )
}
