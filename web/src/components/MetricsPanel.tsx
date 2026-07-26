// MetricsPanel — panel desplegable con métricas detalladas del pipeline.
// Muestra: mensajes procesados, duración promedio, último uso, modelo más usado,
// distribución por categoría.

import { usePipelineMetricas } from '@/hooks/usePipelineMetricas'
import { MessageBadge } from './MessageBadge'
import { cn, formatDuration, formatRelative } from '@/lib/utils'
import type { CategoriaTarea } from '@/types/api'

interface MetricsPanelProps {
  abierto: boolean
  onCerrar: () => void
  className?: string
}

const CATEGORIA_LABELS: Partial<Record<CategoriaTarea, string>> = {
  conversacion: '💬 Conversación',
  codigo: '💻 Código',
  archivos: '📁 Archivos',
  procesos: '⚙️ Procesos',
  monitorizacion: '📊 Monitor',
  instalacion: '📦 Instalación',
  busqueda: '🔍 Búsqueda',
  analisis: '🔬 Análisis',
  auto_creacion: '🛠️ Auto-creación',
  ejecucion_comando: '⌨️ Comando shell',
}

export function MetricsPanel({ abierto, onCerrar, className }: MetricsPanelProps) {
  const { estado, cargando, error, refrescar } = usePipelineMetricas()

  if (!abierto) return null

  return (
    <>
      {/* Overlay clickeable para cerrar */}
      <div
        className="fixed inset-0 z-40 bg-black/20 backdrop-blur-sm md:hidden"
        onClick={onCerrar}
        aria-hidden="true"
      />
      <div
        className={cn(
          'absolute right-2 top-full z-50 mt-1 w-[320px] max-w-[calc(100vw-1rem)] rounded-xl border border-border bg-surface p-3 shadow-xl animate-slide-up dark:border-border-dark dark:bg-surface-dark-subtle',
          className,
        )}
      >
        <div className="mb-2 flex items-center justify-between">
          <h3 className="text-sm font-semibold text-text dark:text-text-dark">
            Métricas del pipeline
          </h3>
          <button
            onClick={onCerrar}
            className="btn-ghost px-1 py-0.5 text-xs"
            aria-label="Cerrar métricas"
          >
            ✕
          </button>
        </div>

        {cargando && !estado ? (
          <div className="py-4 text-center text-xs text-text-subtle dark:text-text-dark-subtle">
            Cargando métricas…
          </div>
        ) : error ? (
          <div className="py-2 text-xs text-rose-600 dark:text-rose-400">
            <p className="font-medium">Error</p>
            <p className="mt-0.5 break-all font-mono text-[10px]">{error}</p>
            <button onClick={refrescar} className="mt-1 underline">
              Reintentar
            </button>
          </div>
        ) : estado ? (
          <div className="space-y-2">
            <div className="grid grid-cols-2 gap-2 text-xs">
              <StatCard label="Mensajes" value={String(estado.mensajes_procesados)} />
              <StatCard
                label="Duración prom."
                value={formatDuration(parseDurationToMs(estado.promedio_duracion))}
              />
            </div>

            {estado.modelo_mas_usado && (
              <div className="rounded-lg bg-surface-muted p-2 dark:bg-surface-dark-muted">
                <div className="text-[10px] uppercase tracking-wide text-text-subtle dark:text-text-dark-subtle">
                  Modelo más usado
                </div>
                <div className="mt-0.5 font-mono text-xs text-text dark:text-text-dark">
                  {estado.modelo_mas_usado}
                </div>
              </div>
            )}

            {estado.ultimo_uso && (
              <div className="text-[11px] text-text-subtle dark:text-text-dark-subtle">
                Último uso: {formatRelative(estado.ultimo_uso)}
              </div>
            )}

            {estado.categorias && Object.keys(estado.categorias).length > 0 && (
              <div>
                <div className="mb-1 text-[10px] uppercase tracking-wide text-text-subtle dark:text-text-dark-subtle">
                  Por categoría
                </div>
                <div className="flex flex-wrap gap-1">
                  {Object.entries(estado.categorias).map(([cat, count]) => (
                    <MessageBadge key={cat} variant="categoria">
                      {CATEGORIA_LABELS[cat as CategoriaTarea] ?? cat}: {count as number}
                    </MessageBadge>
                  ))}
                </div>
              </div>
            )}

            <button
              onClick={refrescar}
              className="btn-ghost w-full justify-center text-xs"
              title="Refrescar métricas"
            >
              ↻ Refrescar
            </button>
          </div>
        ) : (
          <div className="py-4 text-center text-xs text-text-subtle dark:text-text-dark-subtle">
            Sin datos de métricas.
          </div>
        )}
      </div>
    </>
  )
}

function StatCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg bg-surface-muted p-2 dark:bg-surface-dark-muted">
      <div className="text-[10px] uppercase tracking-wide text-text-subtle dark:text-text-dark-subtle">
        {label}
      </div>
      <div className="mt-0.5 font-mono text-sm font-medium text-text dark:text-text-dark">
        {value}
      </div>
    </div>
  )
}

// Convierte "1.234s" / "350ms" / "1m30s" → miliseconds.
// El backend envía time.Duration.String() de Go.
function parseDurationToMs(s: string): number {
  if (!s) return 0
  // Acepta formatos tipo "1.5s", "350ms", "2m30s", "1h5m"
  const re = /(\d+(?:\.\d+)?)(ns|µs|ms|s|m|h)/g
  let totalMs = 0
  let match: RegExpExecArray | null
  while ((match = re.exec(s)) !== null) {
    const val = parseFloat(match[1])
    const unit = match[2]
    switch (unit) {
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
