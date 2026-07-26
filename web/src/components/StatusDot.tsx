// Indicador visual del estado del backend (punto de color + tooltip).
// Verde = online, rojo = offline, ámbar = checking.

import { useBackendHealth } from '@/hooks/useBackendHealth'
import { cn } from '@/lib/utils'

const STATUS_COLORS: Record<string, string> = {
  online: 'bg-emerald-500',
  offline: 'bg-rose-500',
  checking: 'bg-amber-500 animate-pulse-soft',
}

const STATUS_LABELS: Record<string, string> = {
  online: 'Backend en línea',
  offline: 'Backend fuera de línea',
  checking: 'Conectando…',
}

export function StatusDot({ className }: { className?: string }) {
  const { status } = useBackendHealth()
  return (
    <span
      className={cn('inline-block h-2.5 w-2.5 rounded-full', STATUS_COLORS[status], className)}
      title={STATUS_LABELS[status]}
      role="status"
      aria-label={STATUS_LABELS[status]}
    />
  )
}
