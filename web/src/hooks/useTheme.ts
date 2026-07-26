// Theme system — Dark por defecto, persistencia en localStorage, toggle.
// La clase 'dark' se aplica al <html> (Tailwind darkMode: 'class').

import { useCallback, useEffect, useState } from 'react'

export type Theme = 'dark' | 'light'
const STORAGE_KEY = 'liz-theme'

function getInitialTheme(): Theme {
  if (typeof window === 'undefined') return 'dark'
  const saved = window.localStorage.getItem(STORAGE_KEY) as Theme | null
  if (saved === 'dark' || saved === 'light') return saved
  // Default: dark (Liz es oscura por diseño)
  return 'dark'
}

export function useTheme() {
  const [theme, setThemeState] = useState<Theme>(getInitialTheme)

  useEffect(() => {
    const root = document.documentElement
    if (theme === 'dark') {
      root.classList.add('dark')
    } else {
      root.classList.remove('dark')
    }
    window.localStorage.setItem(STORAGE_KEY, theme)
  }, [theme])

  const toggleTheme = useCallback(() => {
    setThemeState((prev) => (prev === 'dark' ? 'light' : 'dark'))
  }, [])

  const setTheme = useCallback((t: Theme) => setThemeState(t), [])

  return { theme, toggleTheme, setTheme }
}
