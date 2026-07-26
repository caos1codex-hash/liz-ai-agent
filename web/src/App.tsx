// App root — orquesta el theme provider y el routing mínimo (single page por ahora).

import { useTheme } from '@/hooks/useTheme'
import { AppShell } from '@/components/AppShell'
import { StatusPage } from '@/pages/StatusPage'

export default function App() {
  // Inicializa el theme system (oscuro por defecto, persistencia localStorage).
  useTheme()

  return (
    <AppShell
      header={
        <div className="flex w-full items-center justify-between">
          <div className="flex items-center gap-2">
            <img src="/liz.svg" alt="Liz" className="h-7 w-7 rounded-md" />
            <span className="font-semibold text-text dark:text-text-dark">Liz AI</span>
            <span className="ml-2 rounded-full bg-liz-100 px-2 py-0.5 text-xs font-medium text-liz-700 dark:bg-liz-900/40 dark:text-liz-300">
              Fase 8 · P8.1
            </span>
          </div>
          <div className="text-xs text-text-muted dark:text-text-dark-muted">
            Scaffolding listo · proximas fases: chat + sidebar + header
          </div>
        </div>
      }
    >
      <StatusPage />
    </AppShell>
  )
}
