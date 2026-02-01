import { useEffect, useState } from 'react'
import { api } from '../api'
import type { Scan } from '../api'
import { navigate } from '../lib/router'
import { formatNumber, formatDate, formatDuration } from '../lib/format'
import { DataTable, type Column } from '../components/DataTable'
import { Loading, EmptyState } from '../components/Loading'

export function ScansPage() {
  const [scans, setScans] = useState<Scan[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    loadScans()
  }, [])

  async function loadScans() {
    setLoading(true)
    try {
      const data = await api.getScans()
      setScans(data || [])
    } catch (e) {
      console.error(e)
    }
    setLoading(false)
  }

  if (loading) return <Loading />

  if (scans.length === 0) {
    return (
      <EmptyState
        title="No scans yet"
        description="Run a scan to discover services and metrics"
      />
    )
  }

  const columns: Column<Scan>[] = [
    {
      key: 'date',
      header: 'Date',
      render: (scan) => <span className="text-gray-900">{formatDate(scan.collected_at)}</span>,
    },
    {
      key: 'services',
      header: 'Services',
      align: 'right',
      render: (scan) => <span className="text-gray-600">{formatNumber(scan.total_services)}</span>,
    },
    {
      key: 'series',
      header: 'Total Series',
      align: 'right',
      render: (scan) => <span className="text-gray-600">{formatNumber(scan.total_series)}</span>,
    },
    {
      key: 'duration',
      header: 'Duration',
      align: 'right',
      render: (scan) => <span className="text-gray-500">{formatDuration(scan.duration_ms)}</span>,
    },
  ]

  function handleRowClick(scan: Scan) {
    navigate({ page: 'services', scanId: scan.id })
  }

  return (
    <div className="space-y-6">
      <h2 className="text-lg font-medium text-gray-900">Scan History</h2>
      <DataTable
        columns={columns}
        data={scans}
        keyExtractor={(scan) => scan.id}
        onRowClick={handleRowClick}
      />
    </div>
  )
}
