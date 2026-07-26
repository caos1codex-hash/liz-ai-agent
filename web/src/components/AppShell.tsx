// Layout shell de la app — estructura base con sidebar responsive.
// En móvil (<md): sidebar como overlay drawer.
// En desktop (≥md): sidebar fijo a la izquierda.

import { useEffect, useState, type ReactNode } from 'react'
import { cn } from '@/lib/utils'

interface AppShellProps {
  children: ReactNode
  sidebar?: ReactNode
  header?: ReactNode
  // Control externo del sidebar en móvil (opcional).
  sidebarOpen?: boolean
  onToggleSidebar?: () => void
}

export function AppShell({
  children,
  sidebar,
  header,
  sidebarOpen = false,
  onToggleSidebar,
}: AppShellProps) {
  // Estado interno del drawer si no hay control externo.
  const [internalOpen, setInternalOpen] = useState(false)
  const drawerOpen = onToggleSidebar ? sidebarOpen : internalOpen

  // Cerrar el drawer con tecla Escape.
  useEffect(() => {
    if (!drawerOpen) return
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onToggleSidebar ? onToggleSidebar() : setInternalOpen(false)
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [drawerOpen, onToggleSidebar])

  return (
    <div className="flex h-full w-full overflow-hidden bg-surface dark:bg-surface-dark">
      {/* Sidebar desktop (fijo) */}
      {sidebar && (
        <aside className="hidden w-72 shrink-0 border-r border-border dark:border-border-dark md:flex md:flex-col">
          {sidebar}
        </aside>
      )}

      {/* Sidebar móvil (drawer overlay) */}
      {sidebar && (
        <>
          {/* Backdrop */}
          {drawerOpen && (
            <div
              className="fixed inset-0 z-40 bg-black/40 backdrop-blur-sm md:hidden animate-fade-in"
              onClick={() => (onToggleSidebar ? onToggleSidebar() : setInternalOpen(false))}
              aria-hidden="true"
            />
          )}
          <aside
            className={cn(
              'fixed inset-y-0 left-0 z-50 w-72 max-w-[85vw] border-r border-border bg-surface shadow-2xl transition-transform duration-200 dark:border-border-dark dark:bg-surface-dark md:hidden',
              drawerOpen ? 'translate-x-0' : '-translate-x-full',
            )}
          >
            {sidebar}
          </aside>
        </>
      )}

      {/* Main */}
      <div className="flex min-w-0 flex-1 flex-col">
        {header && (
          <header className="relative flex h-14 shrink-0 items-center border-b border-border px-2 dark:border-border-dark md:px-4">
            {header}
          </header>
        )}
        <main className="flex min-h-0 flex-1 flex-col">{children}</main>
      </div>
    </div>
  )
}
