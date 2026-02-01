import { useEffect, useState, useMemo } from 'react'
import { api } from '../api'
import type { Scan, Service, Metric } from '../api'
import { navigate } from '../lib/router'
import { formatNumber, formatDate } from '../lib/format'
import { DataTable, type Column } from '../components/DataTable'
import { Breadcrumb } from '../components/Breadcrumb'
import { Loading } from '../components/Loading'

interface MetricsPageProps {
  scanId: number
  serviceName: string
}

export function MetricsPage({ scanId, serviceName }: MetricsPageProps) {
  const [scans, setScans] = useState<Scan[]>([])
  const [selectedScanId, setSelectedScanId] = useState<number>(scanId)
  const [_, setService] = useState<Service | null>(null)
  const [metrics, setMetrics] = useState<Metric[]>([])
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')

  // Load all scans for dropdown
  useEffect(() => {
    async function loadScans() {
      try {
        const data = await api.getScans()
        setScans(data || [])
      } catch (e) {
        console.error(e)
      }
    }
    loadScans()
  }, [])

  // Load service and metrics when scan changes
  useEffect(() => {
    async function load() {
      setLoading(true)
      try {
        const [serviceData, metricsData] = await Promise.all([
          api.getService(selectedScanId, serviceName),
          api.getMetrics(selectedScanId, serviceName),
        ])
        setService(serviceData)
        setMetrics(metricsData || [])
      } catch (e) {
        console.error(e)
      }
      setLoading(false)
    }
    load()
  }, [selectedScanId, serviceName])

  // Update URL when snapshot changes
  useEffect(() => {
    if (selectedScanId !== scanId) {
      navigate({ page: 'metrics', scanId: selectedScanId, serviceName })
    }
  }, [selectedScanId, scanId, serviceName])

  // Filter metrics by search
  const filteredMetrics = useMemo(() => {
    if (!search) return metrics
    const lower = search.toLowerCase()
    return metrics.filter(m => m.name.toLowerCase().includes(lower))
  }, [metrics, search])

  if (loading) return <Loading />

  const columns: Column<Metric>[] = [
    {
      key: 'name',
      header: 'Metric',
      render: (m) => <span className="font-mono text-gray-900">{m.name}</span>,
    },
    {
      key: 'series',
      header: 'Series',
      align: 'right',
      render: (m) => <span className="text-gray-600">{formatNumber(m.series_count)}</span>,
    },
    {
      key: 'labels',
      header: 'Labels',
      align: 'right',
      render: (m) => <span className="text-gray-500">{m.label_count}</span>,
    },
  ]

  function handleRowClick(m: Metric) {
    navigate({ page: 'labels', scanId: selectedScanId, serviceName, metricName: m.name })
  }

  return (
    <div className="space-y-6">
      <Breadcrumb
        items={[
          { label: 'Services', onClick: () => navigate({ page: 'scans' }) },
          { label: serviceName },
        ]}
      />

      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <select
            value={selectedScanId}
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
            {formatNumber(filteredMetrics.length)} metrics · {formatNumber(filteredMetrics.reduce((sum, m) => sum + m.series_count, 0))} series
          </span>
        </div>
        <input
          type="text"
          placeholder="Search metrics..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="px-4 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-gray-200 w-64"
        />
      </div>

      <DataTable
        columns={columns}
        data={filteredMetrics}
        keyExtractor={(m) => m.id}
        onRowClick={handleRowClick}
      />
    </div>
  )
}
