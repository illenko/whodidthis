import { useEffect, useState, useCallback } from 'react'
import { api } from '../api'
import type { Scan, Service } from '../api'
import { navigate } from '../lib/router'
import { formatNumber, formatDate } from '../lib/format'
import { DataTable, type Column } from '../components/DataTable'
import { Loading, EmptyState } from '../components/Loading'

export function ScansPage() {
  const [scans, setScans] = useState<Scan[]>([])
  const [selectedScanId, setSelectedScanId] = useState<number | null>(null)
  const [services, setServices] = useState<Service[]>([])
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')

  // Load all scans and select the latest one
  useEffect(() => {
    async function loadInitial() {
      setLoading(true)
      try {
        const scansData = await api.getScans()
        setScans(scansData || [])
        if (scansData && scansData.length > 0) {
          setSelectedScanId(scansData[0].id) // Latest scan is first
        }
      } catch (e) {
        console.error(e)
      }
      setLoading(false)
    }
    loadInitial()
  }, [])

  // Load services when scan changes
  const loadServices = useCallback(async () => {
    if (!selectedScanId) return
    try {
      const data = await api.getServices(selectedScanId, { search: search || undefined })
      setServices(data || [])
    } catch (e) {
      console.error(e)
    }
  }, [selectedScanId, search])

  useEffect(() => {
    if (selectedScanId) {
      loadServices()
    }
  }, [selectedScanId, loadServices])

  if (loading) return <Loading />

  if (scans.length === 0) {
    return (
      <EmptyState
        title="No scans yet"
        description="Run a scan to discover services and metrics"
      />
    )
  }

  const columns: Column<Service>[] = [
    {
      key: 'name',
      header: 'Service',
      render: (svc) => <span className="font-mono text-gray-900">{svc.name}</span>,
    },
    {
      key: 'series',
      header: 'Series',
      align: 'right',
      render: (svc) => <span className="text-gray-600">{formatNumber(svc.total_series)}</span>,
    },
    {
      key: 'metrics',
      header: 'Metrics',
      align: 'right',
      render: (svc) => <span className="text-gray-500">{svc.metric_count}</span>,
    },
  ]

  function handleRowClick(svc: Service) {
    if (selectedScanId) {
      navigate({ page: 'metrics', scanId: selectedScanId, serviceName: svc.name })
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <select
            value={selectedScanId || ''}
            onChange={(e) => setSelectedScanId(Number(e.target.value))}
            className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-gray-200 bg-white"
          >
            {scans.map((scan) => (
              <option key={scan.id} value={scan.id}>
                Snapshot #{scan.id} — {formatDate(scan.collected_at)}
              </option>
            ))}
          </select>
          <span className="text-sm text-gray-500">
            {formatNumber(services.length)} services · {formatNumber(services.reduce((sum, s) => sum + s.total_series, 0))} total series
          </span>
        </div>
        <input
          type="text"
          placeholder="Search services..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="px-4 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-gray-200 w-64"
        />
      </div>

      <DataTable
        columns={columns}
        data={services}
        keyExtractor={(svc) => svc.id}
        onRowClick={handleRowClick}
      />
    </div>
  )
}
