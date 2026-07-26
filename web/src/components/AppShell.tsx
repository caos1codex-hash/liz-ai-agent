// Layout shell de la app — estructura base para P8.1.
// En P8.2-P8.4 se rellenará con sidebar + header + chat window.

import type { ReactNode } from 'react'

interface AppShellProps {
  children: ReactNode
  sidebar?: ReactNode
  header?: ReactNode
}

export function AppShell({ children, sidebar, header }: AppShellProps) {
  return (
    <div className="flex h-full w-full overflow-hidden bg-surface dark:bg-surface-dark">
      {sidebar && (
        <aside className="hidden w-72 shrink-0 border-r border-border dark:border-border-dark md:flex md:flex-col">
          {sidebar}
        </aside>
      )}
      <div className="flex min-w-0 flex-1 flex-col">
        {header && (
          <header className="flex h-14 shrink-0 items-center border-b border-border px-4 dark:border-border-dark">
            {header}
          </header>
        )}
        <main className="flex min-h-0 flex-1 flex-col">{children}</main>
      </div>
    </div>
  )
}
