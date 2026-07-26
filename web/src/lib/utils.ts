import { clsx, type ClassValue } from 'clsx'

/**
 * Helper para componer classNames condicionalmente.
 * Equivalente ligero a cn() de shadcn/ui.
 *
 * @example
 *   cn('px-2', isActive && 'bg-liz-600', { 'opacity-50': disabled })
 */
export function cn(...inputs: ClassValue[]): string {
  return clsx(inputs)
}

/**
 * Formatea una duración en nanoseconds (formato Go time.Duration) o en miliseconds.
 * Retorna un string legible: "1.2s", "350ms", "2.5min".
 */
export function formatDuration(nsOrMs: number, unit: 'ns' | 'ms' = 'ns'): string {
  const ms = unit === 'ns' ? nsOrMs / 1_000_000 : nsOrMs
  if (ms < 1) return '<1ms'
  if (ms < 1000) return `${Math.round(ms)}ms`
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)}s`
  return `${(ms / 60_000).toFixed(1)}min`
}

/**
 * Formatea una fecha ISO a un string relativo corto.
 * Ej: "hace 5 min", "hace 2 h", "ayer", "12 jul".
 */
export function formatRelative(iso: string): string {
  const date = new Date(iso)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffMin = Math.floor(diffMs / 60_000)
  const diffH = Math.floor(diffMs / 3_600_000)
  const diffDays = Math.floor(diffMs / 86_400_000)

  if (diffMin < 1) return 'ahora'
  if (diffMin < 60) return `hace ${diffMin} min`
  if (diffH < 24) return `hace ${diffH} h`
  if (diffDays === 1) return 'ayer'
  if (diffDays < 7) return `hace ${diffDays} d`
  return date.toLocaleDateString('es', { day: 'numeric', month: 'short' })
}

/**
 * Trunca texto a N caracteres con elipsis.
 */
export function truncate(text: string, max = 60): string {
  if (text.length <= max) return text
  return text.slice(0, max - 1) + '…'
}

/**
 * Genera un ID corto aleatorio (para claves React temporales).
 */
export function shortId(): string {
  return Math.random().toString(36).slice(2, 10)
}
