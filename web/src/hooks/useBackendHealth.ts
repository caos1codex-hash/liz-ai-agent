// Hook de salud del backend — ping a /api/v1/health cada 30s.
// Expone estado: 'online' | 'offline' | 'checking'.

import { useEffect, useState, useCallback } from 'react'
import { healthApi } from '@/lib/endpoints'
import type { HealthStatus } from '@/types/api'

export type BackendStatus = 'online' | 'offline' | 'checking'

interface UseHealthReturn {
  status: BackendStatus
  info: HealthStatus | null
  error: string | null
  refresh: () => Promise<void>
}

const POLL_INTERVAL_MS = 30_000

export function useBackendHealth(): UseHealthReturn {
  const [status, setStatus] = useState<BackendStatus>('checking')
  const [info, setInfo] = useState<HealthStatus | null>(null)
  const [error, setError] = useState<string | null>(null)

  const check = useCallback(async () => {
    setStatus((prev) => (prev === 'online' || prev === 'offline' ? 'checking' : prev))
    try {
      const data = await healthApi.status()
      setInfo(data)
      setError(null)
      setStatus('online')
    } catch (err) {
      setInfo(null)
      setError((err as Error).message)
      setStatus('offline')
    }
  }, [])

  useEffect(() => {
    void check()
    const id = setInterval(check, POLL_INTERVAL_MS)
    return () => clearInterval(id)
  }, [check])

  return { status, info, error, refresh: check }
}
