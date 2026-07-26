// SidebarItem — una conversación en la lista del sidebar.
// Estado activo/inactivo, hover para mostrar botón eliminar.

import { cn, formatRelative, truncate } from '@/lib/utils'
import type { SesionChat } from '@/types/api'

interface SidebarItemProps {
  sesion: SesionChat
  activa: boolean
  onClick: () => void
  onEliminar: () => void
}

export function SidebarItem({ sesion, activa, onClick, onEliminar }: SidebarItemProps) {
  const titulo = sesion.titulo?.trim() || 'Conversación sin título'
  const proyectoLabel = sesion.proyecto?.trim()

  return (
    <div
      role="button"
      tabIndex={0}
      onClick={onClick}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault()
          onClick()
        }
      }}
      className={cn(
        'group flex cursor-pointer items-start gap-2 rounded-lg px-3 py-2 text-sm transition-colors',
        activa
          ? 'bg-liz-100 text-liz-900 dark:bg-liz-900/40 dark:text-liz-100'
          : 'text-text hover:bg-surface-muted dark:text-text-dark dark:hover:bg-surface-dark-muted',
      )}
      aria-current={activa ? 'true' : undefined}
    >
      <div className="min-w-0 flex-1">
        <div className="flex items-center justify-between gap-2">
          <span
            className={cn(
              'truncate font-medium',
              activa ? 'text-liz-900 dark:text-liz-100' : 'text-text dark:text-text-dark',
            )}
            title={titulo}
          >
            {truncate(titulo, 40)}
          </span>
          <button
            onClick={(e) => {
              e.stopPropagation()
              onEliminar()
            }}
            className={cn(
              'shrink-0 rounded p-0.5 text-text-subtle opacity-0 transition-all hover:bg-rose-100 hover:text-rose-700 group-hover:opacity-100',
              'dark:text-text-dark-subtle dark:hover:bg-rose-900/40 dark:hover:text-rose-300',
              activa && 'opacity-100',
            )}
            title="Eliminar conversación"
            aria-label={`Eliminar conversación ${titulo}`}
          >
            <svg
              width="14"
              height="14"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <path d="M3 6h18M19 6l-2 14a2 2 0 01-2 2H9a2 2 0 01-2-2L5 6m5 0V4a2 2 0 012-2h0a2 2 0 012 2v2" />
            </svg>
          </button>
        </div>
        <div className="mt-0.5 flex items-center gap-1.5 text-[10px] text-text-subtle dark:text-text-dark-subtle">
          {proyectoLabel && (
            <>
              <span className="truncate" title={`Proyecto: ${proyectoLabel}`}>
                📁 {truncate(proyectoLabel, 18)}
              </span>
              <span>·</span>
            </>
          )}
          <span>{formatRelative(sesion.actualizada_en)}</span>
          {sesion.cerrada && (
            <>
              <span>·</span>
              <span className="text-amber-600 dark:text-amber-400">cerrada</span>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
