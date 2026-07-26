// Avatar — burbuja de avatar para usuario/asistente.

import { cn } from '@/lib/utils'

interface AvatarProps {
  rol: 'usuario' | 'asistente'
  className?: string
}

export function Avatar({ rol, className }: AvatarProps) {
  if (rol === 'usuario') {
    return (
      <div
        className={cn(
          'flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-surface-muted text-sm font-medium text-text-muted dark:bg-surface-dark-muted dark:text-text-dark-muted',
          className,
        )}
        aria-label="Usuario"
      >
        👤
      </div>
    )
  }
  return (
    <div
      className={cn(
        'flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-liz-500 to-liz-700 text-white shadow-sm shadow-liz-600/30',
        className,
      )}
      aria-label="Liz"
    >
      <img src="/liz.svg" alt="" className="h-5 w-5" />
    </div>
  )
}
