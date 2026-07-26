// Header — cabecera de la app con todos los indicadores.
// Responsive: en móvil se colapsa a iconos.

import { useState } from 'react'
import { useBackendHealth } from '@/hooks/useBackendHealth'
import { usePipelineMetricas } from '@/hooks/usePipelineMetricas'
import { useModelos } from '@/hooks/useModelos'
import { StatusDot } from './StatusDot'
import { MetricsPanel } from './MetricsPanel'
import { ProjectSelector } from './ProjectSelector'
import { useTheme } from '@/hooks/useTheme'
import { cn, formatDuration, truncate } from '@/lib/utils'

interface HeaderProps {
  proyectoActual: string | null
  onCambiarProyecto: (proyecto: string | null) => void
  modeloEnUso?: string | null // Modelo usado en el último mensaje del chat
  onToggleSidebar?: () => void // En móvil, abrir/cerrar sidebar
  className?: string
}

export function Header({
  proyectoActual,
  onCambiarProyecto,
  modeloEnUso,
  onToggleSidebar,
  className,
}: HeaderProps) {
  const { theme, toggleTheme } = useTheme()
  const { status: backendStatus } = useBackendHealth()
  const { estado: pipelineEstado } = usePipelineMetricas()
  const { modelos } = useModelos()
  const [metricsOpen, setMetricsOpen] = useState(false)

  const modeloMasUsado = modeloEnUso ?? pipelineEstado?.modelo_mas_usado ?? null
  const totalMensajes = pipelineEstado?.mensajes_procesados ?? 0

  return (
    <div
      className={cn(
        'flex h-full w-full items-center justify-between gap-2 px-2 md:px-4',
        className,
      )}
    >
      {/* Izquierda: hamburger (móvil) + branding */}
      <div className="flex min-w-0 items-center gap-2">
        {onToggleSidebar && (
          <button
            onClick={onToggleSidebar}
            className="btn-ghost p-1.5 md:hidden"
            aria-label="Abrir/cerrar sidebar"
          >
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
              <path d="M3 12h18M3 6h18M3 18h18" />
            </svg>
          </button>
        )}
        <img src="/liz.svg" alt="Liz" className="h-7 w-7 rounded-md" />
        <div className="flex flex-col leading-tight">
          <span className="text-sm font-semibold text-text dark:text-text-dark">Liz AI</span>
          <span className="hidden text-[10px] text-text-subtle dark:text-text-dark-subtle sm:inline">
            Agente de IA para Linux
          </span>
        </div>
        <span className="ml-2 hidden rounded-full bg-liz-100 px-2 py-0.5 text-xs font-medium text-liz-700 dark:bg-liz-900/40 dark:text-liz-300 md:inline">
          Fase 8 · P8.4
        </span>
      </div>

      {/* Derecha: indicadores + acciones */}
      <div className="flex items-center gap-1 md:gap-2">
        {/* Modelo actual o más usado */}
        {modeloMasUsado && (
          <div
            className="hidden items-center gap-1 rounded-full bg-liz-100 px-2 py-0.5 text-xs text-liz-700 dark:bg-liz-900/40 dark:text-liz-300 sm:flex"
            title={modeloEnUso ? 'Modelo usado en el último mensaje' : 'Modelo más usado'}
          >
            <span>🧠</span>
            <span className="max-w-[100px] truncate font-mono text-[11px]">
              {truncate(modeloMasUsado, 22)}
            </span>
          </div>
        )}

        {/* Mensajes procesados (click → abre metrics panel) */}
        <button
          onClick={() => setMetricsOpen((v) => !v)}
          className="hidden items-center gap-1 rounded-full bg-surface-muted px-2 py-0.5 text-xs text-text-muted hover:bg-surface dark:bg-surface-dark-muted dark:text-text-dark-muted dark:hover:bg-surface-dark md:flex"
          title="Mensajes procesados (clic para ver métricas)"
          aria-expanded={metricsOpen}
        >
          <span>💬</span>
          <span className="font-mono text-[11px]">{totalMensajes}</span>
        </button>

        {/* Modelos disponibles */}
        {modelos.length > 0 && (
          <div
            className="hidden items-center gap-1 rounded-full bg-surface-muted px-2 py-0.5 text-xs text-text-muted dark:bg-surface-dark-muted dark:text-text-dark-muted lg:flex"
            title={`${modelos.length} modelos NVIDIA disponibles`}
          >
            <span>⚡</span>
            <span className="font-mono text-[11px]">{modelos.length} modelos</span>
          </div>
        )}

        {/* Duración promedio (compacto) */}
        {pipelineEstado?.promedio_duracion && (
          <div
            className="hidden items-center gap-1 rounded-full bg-surface-muted px-2 py-0.5 text-xs text-text-muted dark:bg-surface-dark-muted dark:text-text-dark-muted lg:flex"
            title={`Duración promedio: ${pipelineEstado.promedio_duracion}`}
          >
            <span>⏱</span>
            <span className="font-mono text-[11px]">
              {formatDuration(parseDurMs(pipelineEstado.promedio_duracion))}
            </span>
          </div>
        )}

        {/* Project selector */}
        <ProjectSelector
          proyectoActual={proyectoActual}
          onCambiar={onCambiarProyecto}
          className="hidden md:block"
        />

        {/* Theme toggle */}
        <button
          onClick={toggleTheme}
          className="btn-ghost p-1.5"
          aria-label={theme === 'dark' ? 'Cambiar a tema claro' : 'Cambiar a tema oscuro'}
          title={theme === 'dark' ? 'Tema claro' : 'Tema oscuro'}
        >
          {theme === 'dark' ? (
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="12" cy="12" r="5" />
              <path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42" />
            </svg>
          ) : (
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
            </svg>
          )}
        </button>

        {/* Status dot */}
        <div className="relative flex items-center gap-1">
          <StatusDot />
          <span className="hidden text-[11px] capitalize text-text-muted dark:text-text-dark-muted md:inline">
            {backendStatus}
          </span>
          <MetricsPanel abierto={metricsOpen} onCerrar={() => setMetricsOpen(false)} />
        </div>
      </div>
    </div>
  )
}

// Convierte "1.234s" / "350ms" → ms (igual que en MetricsPanel pero compacto).
function parseDurMs(s: string): number {
  if (!s) return 0
  const re = /(\d+(?:\.\d+)?)(ns|µs|ms|s|m|h)/g
  let totalMs = 0
  let match: RegExpExecArray | null
  while ((match = re.exec(s)) !== null) {
    const val = parseFloat(match[1])
    switch (match[2]) {
      case 'ns':
        totalMs += val / 1_000_000
        break
      case 'µs':
        totalMs += val / 1000
        break
      case 'ms':
        totalMs += val
        break
      case 's':
        totalMs += val * 1000
        break
      case 'm':
        totalMs += val * 60_000
        break
      case 'h':
        totalMs += val * 3_600_000
        break
    }
  }
  return totalMs
}
