// Placeholder page para P8.1 — confirma que el frontend arranca y conecta con el backend.
// Será reemplazada por el ChatPage real en P8.2.

import { useBackendHealth } from '@/hooks/useBackendHealth'
import { StatusDot } from '@/components/StatusDot'
import { formatRelative } from '@/lib/utils'

export function StatusPage() {
  const { status, info, error, refresh } = useBackendHealth()

  return (
    <div className="flex min-h-0 flex-1 items-center justify-center p-6">
      <div className="card w-full max-w-md animate-fade-in p-6 text-center">
        <img
          src="/liz.svg"
          alt="Liz"
          className="mx-auto mb-4 h-16 w-16 rounded-2xl shadow-lg shadow-liz-600/20"
        />
        <h1 className="text-2xl font-bold text-text dark:text-text-dark">Liz AI Agent</h1>
        <p className="mt-1 text-sm text-text-muted dark:text-text-dark-muted">
          Fase 8 — Frontend (P8.1: scaffolding + health check)
        </p>

        <div className="mt-6 flex items-center justify-center gap-2 text-sm">
          <StatusDot />
          <span className="font-medium capitalize">{status}</span>
          {info?.version && (
            <span className="text-text-subtle dark:text-text-dark-subtle">· v{info.version}</span>
          )}
        </div>

        {info && (
          <div className="mt-4 space-y-1 text-left text-xs text-text-muted dark:text-text-dark-muted">
            <div className="flex justify-between">
              <span>Estado:</span>
              <span className="font-mono">{info.estado}</span>
            </div>
            {info.uptime && (
              <div className="flex justify-between">
                <span>Uptime:</span>
                <span className="font-mono">{info.uptime}</span>
              </div>
            )}
            {info.version && (
              <div className="flex justify-between">
                <span>Versión:</span>
                <span className="font-mono">{info.version}</span>
              </div>
            )}
          </div>
        )}

        {error && (
          <div className="mt-4 rounded-lg border border-rose-300 bg-rose-50 p-3 text-xs text-rose-700 dark:border-rose-800 dark:bg-rose-950/40 dark:text-rose-300">
            <p className="font-semibold">No se pudo conectar al backend</p>
            <p className="mt-1 font-mono break-all">{error}</p>
            <p className="mt-2 text-rose-600 dark:text-rose-400">
              Asegúrate de que Liz esté corriendo en <code>http://localhost:3000</code>.
            </p>
          </div>
        )}

        <div className="mt-6 flex justify-center gap-2">
          <button onClick={refresh} className="btn-primary">
            Reintentar
          </button>
        </div>

        <p className="mt-6 text-xs text-text-subtle dark:text-text-dark-subtle">
          {info ? `Última verificación: ${formatRelative(new Date().toISOString())}` : ''}
        </p>
      </div>
    </div>
  )
}
