import { useEffect, useState, useMemo } from 'react'
import { api } from '../api'
import type { Scan, Metric } from '../api'
import { navigate } from '../lib/router'
import { formatNumber, formatDate } from '../lib/format'
import { useDebounce } from '../hooks/useDebounce'
import { DataTable, type Column } from '../components/DataTable'
import { Breadcrumb } from '../components/Breadcrumb'
import { Loading } from '../components/Loading'
import { Input } from '../components/Input'
import { Select } from '../components/Select'

interface MetricsPageProps {
  scanId: number
  serviceName: string
}

export function MetricsPage({ scanId, serviceName }: MetricsPageProps) {
  const [scans, setScans] = useState<Scan[]>([])
  const [selectedScanId, setSelectedScanId] = useState<number>(scanId)
  const [metrics, setMetrics] = useState<Metric[]>([])
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const debouncedSearch = useDebounce(search, 300)

  // Load all scans for dropdown
  useEffect(() => {
    async function loadScans() {
      try {
        const data = await api.getScans()
        setScans(data || [])
      } catch (err) {
        console.error('Failed to load scans:', err)
      }
    }
    loadScans()
  }, [])

  // Load service and metrics when scan changes
  useEffect(() => {
    async function load() {
      setLoading(true)
      try {
        const metricsData = await api.getMetrics(selectedScanId, serviceName)
        setMetrics(metricsData || [])
      } catch (err) {
        console.error('Failed to load metrics:', err)
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
    if (!debouncedSearch) return metrics
    const lower = debouncedSearch.toLowerCase()
    return metrics.filter(m => m.name.toLowerCase().includes(lower))
  }, [metrics, debouncedSearch])

  if (loading) return <Loading />

  const columns: Column<Metric>[] = [
    {
      key: 'name',
      header: 'Metric',
      render: (m) => <span className="font-mono text-gray-900 dark:text-gray-100">{m.name}</span>,
    },
    {
      key: 'series',
      header: 'Series',
      align: 'right',
      render: (m) => <span className="text-gray-600 dark:text-gray-400">{formatNumber(m.series_count)}</span>,
    },
    {
      key: 'labels',
      header: 'Labels',
      align: 'right',
      render: (m) => <span className="text-gray-500 dark:text-gray-500">{m.label_count}</span>,
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

      <div className="flex items-center justify-between flex-wrap gap-4">
        <div className="flex items-center gap-4 flex-wrap">
          <Select
            value={selectedScanId}
            onChange={(e) => setSelectedScanId(Number(e.target.value))}
            aria-label="Select snapshot"
          >
            {scans.map((scan) => (
              <option key={scan.id} value={scan.id}>
                Snapshot #{scan.id} — {formatDate(scan.collected_at)}
              </option>
            ))}
          </Select>
          <span className="text-sm text-gray-500 dark:text-gray-400">
            {formatNumber(filteredMetrics.length)} metrics · {formatNumber(filteredMetrics.reduce((sum, m) => sum + m.series_count, 0))} series
          </span>
        </div>
        <Input
          type="text"
          placeholder="Search metrics..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="w-64"
          aria-label="Search metrics"
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
