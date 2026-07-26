// MessageInput — textarea auto-expansible + botón enviar + botón cancelar.
// Atajos: Enter = enviar, Shift+Enter = newline.

import { useEffect, useRef, useState, type KeyboardEvent } from 'react'
import { cn } from '@/lib/utils'

interface MessageInputProps {
  onEnviar: (texto: string) => void
  onCancelar?: () => void
  enviando: boolean
  disabled?: boolean
  placeholder?: string
  className?: string
}

const MAX_ALTURA_PX = 200

export function MessageInput({
  onEnviar,
  onCancelar,
  enviando,
  disabled,
  placeholder = 'Escribe un mensaje a Liz…',
  className,
}: MessageInputProps) {
  const [texto, setTexto] = useState('')
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  // Auto-resize del textarea.
  useEffect(() => {
    const el = textareaRef.current
    if (!el) return
    el.style.height = 'auto'
    el.style.height = `${Math.min(el.scrollHeight, MAX_ALTURA_PX)}px`
  }, [texto])

  // Focus inicial.
  useEffect(() => {
    textareaRef.current?.focus()
  }, [])

  const enviar = () => {
    const t = texto.trim()
    if (!t || enviando || disabled) return
    onEnviar(t)
    setTexto('')
  }

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
      e.preventDefault()
      enviar()
    }
  }

  const handlePaste = (e: React.ClipboardEvent<HTMLTextAreaElement>) => {
    // Si pegas una imagen o archivo, ignorar (P8.5 puede añadir soporte de adjuntos).
    if (e.clipboardData.files.length > 0) {
      e.preventDefault()
    }
  }

  return (
    <div
      className={cn(
        'border-t border-border bg-surface/80 px-4 py-3 backdrop-blur dark:border-border-dark dark:bg-surface-dark/80',
        className,
      )}
    >
      <div className="mx-auto max-w-3xl">
        <div
          className={cn(
            'flex items-end gap-2 rounded-xl border border-border bg-surface px-3 py-2 transition-colors',
            'focus-within:border-liz-500 focus-within:ring-1 focus-within:ring-liz-500',
            'dark:border-border-dark dark:bg-surface-dark-subtle',
          )}
        >
          <textarea
            ref={textareaRef}
            value={texto}
            onChange={(e) => setTexto(e.target.value)}
            onKeyDown={handleKeyDown}
            onPaste={handlePaste}
            disabled={disabled}
            placeholder={placeholder}
            rows={1}
            className={cn(
              'flex-1 resize-none bg-transparent text-sm text-text placeholder:text-text-subtle',
              'focus:outline-none disabled:opacity-50',
              'dark:text-text-dark dark:placeholder:text-text-dark-subtle',
              'max-h-[200px] min-h-[24px]',
            )}
            aria-label="Mensaje para Liz"
          />
          {enviando ? (
            <button
              onClick={onCancelar}
              className="btn-ghost shrink-0 border border-border text-rose-600 hover:bg-rose-50 dark:border-border-dark dark:text-rose-400 dark:hover:bg-rose-950/30"
              title="Cancelar envío"
              aria-label="Cancelar envío"
            >
              ✕ Cancelar
            </button>
          ) : (
            <button
              onClick={enviar}
              disabled={!texto.trim() || disabled}
              className="btn-primary shrink-0"
              title="Enviar (Enter)"
              aria-label="Enviar mensaje"
            >
              <svg
                width="14"
                height="14"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2.5"
                strokeLinecap="round"
                strokeLinejoin="round"
              >
                <path d="M5 12h14M13 5l7 7-7 7" />
              </svg>
              <span className="hidden sm:inline">Enviar</span>
            </button>
          )}
        </div>
        <div className="mt-1.5 flex items-center justify-between text-[11px] text-text-subtle dark:text-text-dark-subtle">
          <span>
            <kbd className="rounded border border-border px-1 py-0.5 font-mono dark:border-border-dark">
              Enter
            </kbd>{' '}
            envía ·{' '}
            <kbd className="rounded border border-border px-1 py-0.5 font-mono dark:border-border-dark">
              Shift+Enter
            </kbd>{' '}
            nueva línea
          </span>
          {texto.length > 0 && <span>{texto.length} chars</span>}
        </div>
      </div>
    </div>
  )
}
