// Toast — notificación efímera para errores globales.
// Uso: const { showError } = useToast(); showError('No se pudo conectar').
// Implementación mínima con context + portal al body.

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import { createPortal } from 'react-dom'
import { cn } from '@/lib/utils'

type ToastVariant = 'error' | 'success' | 'info'

interface ToastItem {
  id: string
  variant: ToastVariant
  message: string
  durationMs: number
}

interface ToastContextValue {
  showError: (message: string, durationMs?: number) => void
  showSuccess: (message: string, durationMs?: number) => void
  showInfo: (message: string, durationMs?: number) => void
}

const ToastContext = createContext<ToastContextValue | null>(null)

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([])
  const idCounter = useRef(0)

  const remove = useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id))
  }, [])

  const show = useCallback(
    (variant: ToastVariant, message: string, durationMs = 5000) => {
      const id = `toast-${++idCounter.current}`
      setToasts((prev) => [...prev, { id, variant, message, durationMs }])
      if (durationMs > 0) {
        setTimeout(() => remove(id), durationMs)
      }
    },
    [remove],
  )

  const value = useMemo<ToastContextValue>(
    () => ({
      showError: (m, d) => show('error', m, d),
      showSuccess: (m, d) => show('success', m, d),
      showInfo: (m, d) => show('info', m, d),
    }),
    [show],
  )

  return (
    <ToastContext.Provider value={value}>
      {children}
      {typeof document !== 'undefined' &&
        createPortal(
          <div
            className="pointer-events-none fixed bottom-4 right-4 z-[100] flex max-w-[calc(100vw-2rem)] flex-col gap-2"
            aria-live="assertive"
            aria-atomic="true"
          >
            {toasts.map((t) => (
              <Toast key={t.id} item={t} onClose={() => remove(t.id)} />
            ))}
          </div>,
          document.body,
        )}
    </ToastContext.Provider>
  )
}

function Toast({ item, onClose }: { item: ToastItem; onClose: () => void }) {
  const variantStyles: Record<ToastVariant, string> = {
    error:
      'border-rose-300 bg-rose-50 text-rose-800 dark:border-rose-800 dark:bg-rose-950/90 dark:text-rose-200',
    success:
      'border-emerald-300 bg-emerald-50 text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950/90 dark:text-emerald-200',
    info: 'border-border bg-surface text-text dark:border-border-dark dark:bg-surface-dark-subtle dark:text-text-dark',
  }
  const icon: Record<ToastVariant, string> = {
    error: '⚠️',
    success: '✅',
    info: 'ℹ️',
  }

  // Slide-in animation on mount.
  const [mounted, setMounted] = useState(false)
  useEffect(() => {
    const raf = requestAnimationFrame(() => setMounted(true))
    return () => cancelAnimationFrame(raf)
  }, [])

  return (
    <div
      className={cn(
        'pointer-events-auto flex items-start gap-2 rounded-lg border px-3 py-2 text-sm shadow-lg backdrop-blur transition-all',
        variantStyles[item.variant],
        mounted ? 'translate-x-0 opacity-100' : 'translate-x-8 opacity-0',
      )}
      role="alert"
    >
      <span className="shrink-0" aria-hidden="true">
        {icon[item.variant]}
      </span>
      <span className="min-w-0 flex-1 break-words">{item.message}</span>
      <button
        onClick={onClose}
        className="shrink-0 rounded p-0.5 hover:bg-black/5 dark:hover:bg-white/10"
        aria-label="Cerrar notificación"
      >
        ✕
      </button>
    </div>
  )
}

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext)
  if (!ctx) {
    // Fallback silencioso (no romper la app si no hay provider).
    const noop = () => {}
    return { showError: noop, showSuccess: noop, showInfo: noop }
  }
  return ctx
}
