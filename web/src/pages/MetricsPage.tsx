import { useEffect, useState } from 'react'
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
  const [scan, setScan] = useState<Scan | null>(null)
  const [service, setService] = useState<Service | null>(null)
  const [metrics, setMetrics] = useState<Metric[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    async function load() {
      setLoading(true)
      try {
        const [scanData, serviceData, metricsData] = await Promise.all([
          api.getScan(scanId),
          api.getService(scanId, serviceName),
          api.getMetrics(scanId, serviceName),
        ])
        setScan(scanData)
        setService(serviceData)
        setMetrics(metricsData || [])
      } catch (e) {
        console.error(e)
      }
      setLoading(false)
    }
    load()
  }, [scanId, serviceName])

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
    navigate({ page: 'labels', scanId, serviceName, metricName: m.name })
  }

  return (
    <div className="space-y-6">
      <Breadcrumb
        items={[
          { label: 'Scans', onClick: () => navigate({ page: 'scans' }) },
          { label: scan ? formatDate(scan.collected_at) : `Scan #${scanId}`, onClick: () => navigate({ page: 'services', scanId }) },
          { label: serviceName },
        ]}
      />

      <div className="text-sm text-gray-500">
        {service ? formatNumber(service.total_series) : '–'} series &middot; {metrics.length} metrics
      </div>

      <DataTable
        columns={columns}
        data={metrics}
        keyExtractor={(m) => m.id}
        onRowClick={handleRowClick}
      />
    </div>
  )
}
