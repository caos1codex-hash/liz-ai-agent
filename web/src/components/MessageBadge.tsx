// MessageBadge — pill informativo (modelo, herramienta, categoría, métricas).
// Variantes de color por tipo.

import { cn } from '@/lib/utils'

type BadgeVariant = 'modelo' | 'herramienta' | 'categoria' | 'metrica' | 'error'

const VARIANT_STYLES: Record<BadgeVariant, string> = {
  modelo:
    'bg-liz-100 text-liz-700 dark:bg-liz-900/40 dark:text-liz-300',
  herramienta:
    'bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-300',
  categoria:
    'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300',
  metrica:
    'bg-surface-muted text-text-muted dark:bg-surface-dark-muted dark:text-text-dark-muted',
  error:
    'bg-rose-100 text-rose-700 dark:bg-rose-900/40 dark:text-rose-300',
}

interface MessageBadgeProps {
  variant: BadgeVariant
  children: React.ReactNode
  title?: string
  className?: string
}

export function MessageBadge({ variant, children, title, className }: MessageBadgeProps) {
  return (
    <span
      title={title}
      className={cn(
        'inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium leading-none',
        VARIANT_STYLES[variant],
        className,
      )}
    >
      {children}
    </span>
  )
}
