// SidebarHeader — cabecera del sidebar con logo + botón "Nueva conversación".

import { cn } from '@/lib/utils'

interface SidebarHeaderProps {
  onNuevaConversacion: () => void
  creando: boolean
  className?: string
}

export function SidebarHeader({
  onNuevaConversacion,
  creando,
  className,
}: SidebarHeaderProps) {
  return (
    <div
      className={cn(
        'flex items-center justify-between border-b border-border px-3 py-3 dark:border-border-dark',
        className,
      )}
    >
      <div className="flex items-center gap-2">
        <img src="/liz.svg" alt="Liz" className="h-7 w-7 rounded-md" />
        <div className="flex flex-col leading-tight">
          <span className="text-sm font-semibold text-text dark:text-text-dark">Liz AI</span>
          <span className="text-[10px] text-text-subtle dark:text-text-dark-subtle">
            Agente Linux
          </span>
        </div>
      </div>
      <button
        onClick={onNuevaConversacion}
        disabled={creando}
        className="btn-ghost"
        title="Nueva conversación"
        aria-label="Nueva conversación"
      >
        {creando ? (
          <span className="animate-pulse-soft">…</span>
        ) : (
          <svg
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <path d="M12 5v14M5 12h14" />
          </svg>
        )}
        <span className="hidden lg:inline">Nueva</span>
      </button>
    </div>
  )
}
