// Auto-scroll para listas de mensajes — se mantiene pegado al final
// si el usuario está "abajo", pero permite hacer scroll up sin pegar saltos.

import { useCallback, useEffect, useRef, useState } from 'react'

interface UseAutoScrollOptions {
  // Distancia al fondo (px) bajo la cual consideramos que el usuario está "abajo".
  threshold?: number
}

interface UseAutoScrollReturn<T extends HTMLElement> {
  containerRef: React.RefObject<T>
  isAtBottom: boolean
  scrollToBottom: (behavior?: ScrollBehavior) => void
}

export function useAutoScroll<T extends HTMLElement = HTMLDivElement>(
  deps: unknown[],
  opts: UseAutoScrollOptions = {},
): UseAutoScrollReturn<T> {
  const { threshold = 80 } = opts
  const containerRef = useRef<T>(null)
  const [isAtBottom, setIsAtBottom] = useState(true)

  // Scroll al bottom cuando cambian las deps Y el usuario está abajo.
  useEffect(() => {
    const el = containerRef.current
    if (!el || !isAtBottom) return
    el.scrollTo({ top: el.scrollHeight, behavior: 'smooth' })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps)

  // Track scroll position.
  useEffect(() => {
    const el = containerRef.current
    if (!el) return
    const handleScroll = () => {
      const distanceFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight
      setIsAtBottom(distanceFromBottom < threshold)
    }
    el.addEventListener('scroll', handleScroll, { passive: true })
    handleScroll()
    return () => el.removeEventListener('scroll', handleScroll)
  }, [threshold])

  const scrollToBottom = useCallback((behavior: ScrollBehavior = 'smooth') => {
    const el = containerRef.current
    if (!el) return
    el.scrollTo({ top: el.scrollHeight, behavior })
    setIsAtBottom(true)
  }, [])

  return { containerRef, isAtBottom, scrollToBottom }
}
