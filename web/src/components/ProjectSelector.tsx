// ProjectSelector — dropdown para elegir proyecto de contexto.
// Lista proyectos indexados en el backend (GET /api/v1/contexto/proyectos).
// Permite "Sin proyecto" (default).

import { useEffect, useRef, useState } from 'react'
import { useProyectos } from '@/hooks/useProyectos'
import { cn, truncate } from '@/lib/utils'

interface ProjectSelectorProps {
  proyectoActual: string | null
  onCambiar: (proyecto: string | null) => void
  className?: string
}

export function ProjectSelector({
  proyectoActual,
  onCambiar,
  className,
}: ProjectSelectorProps) {
  const { proyectos, cargando, error, refrescar } = useProyectos()
  const [abierto, setAbierto] = useState(false)
  const dropdownRef = useRef<HTMLDivElement>(null)

  // Cerrar al click fuera.
  useEffect(() => {
    if (!abierto) return
    const handler = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setAbierto(false)
      }
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [abierto])

  const label = proyectoActual ? truncate(proyectoActual, 20) : 'Sin proyecto'

  return (
    <div ref={dropdownRef} className={cn('relative', className)}>
      <button
        onClick={() => setAbierto((v) => !v)}
        className="btn-ghost text-xs"
        title={proyectoActual ? `Proyecto: ${proyectoActual}` : 'Sin proyecto'}
        aria-haspopup="listbox"
        aria-expanded={abierto}
      >
        <span className="text-text-subtle dark:text-text-dark-subtle">📁</span>
        <span className="max-w-[120px] truncate">{label}</span>
        <svg
          width="10"
          height="10"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="3"
          className={cn('transition-transform', abierto && 'rotate-180')}
        >
          <path d="M6 9l6 6 6-6" />
        </svg>
      </button>

      {abierto && (
        <div
          className="absolute right-0 top-full z-50 mt-1 min-w-[220px] max-w-[300px] rounded-lg border border-border bg-surface p-1 shadow-lg dark:border-border-dark dark:bg-surface-dark-subtle"
          role="listbox"
        >
          <button
            onClick={() => {
              onCambiar(null)
              setAbierto(false)
            }}
            className={cn(
              'flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-xs',
              !proyectoActual
                ? 'bg-liz-100 text-liz-700 dark:bg-liz-900/40 dark:text-liz-300'
                : 'text-text hover:bg-surface-muted dark:text-text-dark dark:hover:bg-surface-dark-muted',
            )}
          >
            <span className="opacity-60">🚫</span>
            <span>Sin proyecto</span>
          </button>

          <div className="my-1 border-t border-border dark:border-border-dark" />

          {cargando ? (
            <div className="px-2 py-2 text-xs text-text-subtle dark:text-text-dark-subtle">
              Cargando proyectos…
            </div>
          ) : error ? (
            <div className="px-2 py-2 text-xs text-rose-600 dark:text-rose-400">
              <div className="font-medium">Error</div>
              <div className="mt-0.5 break-all font-mono text-[10px]">{error}</div>
              <button onClick={refrescar} className="mt-1 underline">
                Reintentar
              </button>
            </div>
          ) : proyectos.length === 0 ? (
            <div className="px-2 py-2 text-xs text-text-subtle dark:text-text-dark-subtle">
              Sin proyectos indexados.
              <br />
              <span className="text-[10px]">
                Indexa uno vía <code className="font-mono">POST /api/v1/contexto/proyectos</code>
              </span>
            </div>
          ) : (
            <>
              <div className="px-2 py-1 text-[10px] font-medium uppercase tracking-wide text-text-subtle dark:text-text-dark-subtle">
                {proyectos.length} proyecto{proyectos.length !== 1 ? 's' : ''}
              </div>
              {proyectos.map((p) => (
                <button
                  key={p.nombre}
                  onClick={() => {
                    onCambiar(p.nombre)
                    setAbierto(false)
                  }}
                  className={cn(
                    'flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-xs',
                    proyectoActual === p.nombre
                      ? 'bg-liz-100 text-liz-700 dark:bg-liz-900/40 dark:text-liz-300'
                      : 'text-text hover:bg-surface-muted dark:text-text-dark dark:hover:bg-surface-dark-muted',
                  )}
                  title={p.ruta}
                >
                  <span className="opacity-60">📁</span>
                  <div className="min-w-0 flex-1">
                    <div className="truncate font-medium">{p.nombre}</div>
                    {p.archivos_indexados !== undefined && (
                      <div className="truncate text-[10px] text-text-subtle dark:text-text-dark-subtle">
                        {p.archivos_indexados} archivos
                      </div>
                    )}
                  </div>
                </button>
              ))}
            </>
          )}
        </div>
      )}
    </div>
  )
}
