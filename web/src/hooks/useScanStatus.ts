import { useEffect, useState, useCallback } from 'react'
import { api, type ScanStatus } from '../api'
import { POLLING_INTERVAL_MS } from '../lib/constants'

export function useScanStatus() {
  const [scanStatus, setScanStatus] = useState<ScanStatus | null>(null)
  const [error, setError] = useState<string | null>(null)

  const loadScanStatus = useCallback(async () => {
    try {
      const status = await api.getScanStatus()
      setScanStatus(status)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load scan status')
      console.error('Failed to load scan status:', err)
    }
  }, [])

  useEffect(() => {
    loadScanStatus()
    const interval = setInterval(loadScanStatus, POLLING_INTERVAL_MS)
    return () => clearInterval(interval)
  }, [loadScanStatus])

  const triggerScan = useCallback(async () => {
    try {
      await api.triggerScan()
      await loadScanStatus()
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to trigger scan')
      console.error('Failed to trigger scan:', err)
    }
  }, [loadScanStatus])

  return { scanStatus, error, triggerScan, reload: loadScanStatus }
}
