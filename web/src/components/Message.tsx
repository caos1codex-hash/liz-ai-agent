// Message — un mensaje individual en la conversación (usuario o asistente).
// Soporta streaming progresivo, markdown, badges de herramientas/modelo, y errores.

import { Avatar } from './Avatar'
import { Markdown } from './Markdown'
import { TypingIndicator } from './TypingIndicator'
import { MessageBadge } from './MessageBadge'
import { cn, formatDuration, formatRelative } from '@/lib/utils'
import type { UIMensaje } from '@/hooks/useChat'

interface MessageProps {
  mensaje: UIMensaje
}

export function Message({ mensaje }: MessageProps) {
  const esUsuario = mensaje.rol === 'usuario'
  const esStreaming = mensaje.streaming
  const tieneError = Boolean(mensaje.error)
  const vacio = !mensaje.contenido && esStreaming

  return (
    <div
      className={cn(
        'flex w-full gap-3 px-4 py-4 animate-slide-up',
        esUsuario ? 'justify-end' : 'justify-start',
      )}
      data-rol={mensaje.rol}
      data-streaming={esStreaming || undefined}
    >
      {!esUsuario && <Avatar rol="asistente" />}
      <div
        className={cn(
          'flex min-w-0 max-w-[85%] flex-col gap-1.5',
          esUsuario && 'items-end',
        )}
      >
        {/* Header del mensaje */}
        <div
          className={cn(
            'flex items-center gap-2 text-[11px] text-text-subtle dark:text-text-dark-subtle',
            esUsuario && 'flex-row-reverse',
          )}
        >
          <span className="font-medium">
            {esUsuario ? 'Tú' : 'Liz'}
          </span>
          <span>·</span>
          <time dateTime={new Date(mensaje.timestamp).toISOString()}>
            {formatRelative(new Date(mensaje.timestamp).toISOString())}
          </time>
          {mensaje.pending && (
            <MessageBadge variant="metrica" title="Mensaje aún no confirmado por el backend">
              enviando…
            </MessageBadge>
          )}
        </div>

        {/* Cuerpo del mensaje */}
        {esUsuario ? (
          <div
            className={cn(
              'rounded-2xl rounded-tr-sm bg-liz-600 px-4 py-2.5 text-sm text-white shadow-sm',
              'whitespace-pre-wrap break-words',
            )}
          >
            {mensaje.contenido}
          </div>
        ) : (
          <div
            className={cn(
              'rounded-2xl rounded-tl-sm border px-4 py-3 shadow-sm',
              tieneError
                ? 'border-rose-300 bg-rose-50 dark:border-rose-800 dark:bg-rose-950/30'
                : 'border-border bg-surface dark:border-border-dark dark:bg-surface-dark-subtle',
            )}
          >
            {vacio ? (
              <TypingIndicator />
            ) : (
              <>
                <Markdown content={mensaje.contenido} />
                {esStreaming && (
                  <span
                    className="ml-0.5 inline-block h-4 w-1.5 animate-blink bg-liz-500 align-middle"
                    aria-hidden="true"
                  />
                )}
                {tieneError && (
                  <div className="mt-2 rounded border-t border-rose-300 pt-2 text-xs text-rose-700 dark:border-rose-800 dark:text-rose-300">
                    ⚠️ {mensaje.error}
                  </div>
                )}
              </>
            )}
          </div>
        )}

        {/* Footer del mensaje asistente: badges informativos */}
        {!esUsuario && !esStreaming && (
          <div className="flex flex-wrap items-center gap-1.5">
            {mensaje.modeloUsado && (
              <MessageBadge variant="modelo" title="Modelo usado">
                🧠 {mensaje.modeloUsado}
              </MessageBadge>
            )}
            {mensaje.herramientasUsadas && mensaje.herramientasUsadas.length > 0 && (
              <>
                {mensaje.herramientasUsadas.map((h) => (
                  <MessageBadge key={h} variant="herramienta" title={`Herramienta usada: ${h}`}>
                    🔧 {h}
                  </MessageBadge>
                ))}
              </>
            )}
            {mensaje.pasosEjecutados !== undefined && mensaje.pasosEjecutados > 0 && (
              <MessageBadge variant="metrica" title="Pasos ejecutados">
                {mensaje.pasosEjecutados} paso{mensaje.pasosEjecutados !== 1 ? 's' : ''}
              </MessageBadge>
            )}
            {mensaje.duracionMs !== undefined && (
              <MessageBadge variant="metrica" title="Duración">
                {formatDuration(mensaje.duracionMs, 'ms')}
              </MessageBadge>
            )}
            {mensaje.tokens !== undefined && mensaje.tokens > 0 && (
              <MessageBadge variant="metrica" title="Tokens usados">
                {mensaje.tokens} tok
              </MessageBadge>
            )}
          </div>
        )}
      </div>
      {esUsuario && <Avatar rol="usuario" />}
    </div>
  )
}
