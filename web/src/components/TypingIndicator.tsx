// TypingIndicator — tres puntos animados para "Liz está pensando...".

export function TypingIndicator() {
  return (
    <div className="flex items-center gap-1.5 py-1" aria-label="Liz está escribiendo">
      <span className="h-2 w-2 animate-pulse-soft rounded-full bg-liz-500 [animation-delay:0ms]" />
      <span className="h-2 w-2 animate-pulse-soft rounded-full bg-liz-500 [animation-delay:200ms]" />
      <span className="h-2 w-2 animate-pulse-soft rounded-full bg-liz-500 [animation-delay:400ms]" />
      <span className="ml-1.5 text-xs text-text-subtle dark:text-text-dark-subtle">
        Liz está pensando…
      </span>
    </div>
  )
}
