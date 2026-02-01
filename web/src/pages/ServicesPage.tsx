import { useEffect, useState, useCallback } from 'react'
import { api } from '../api'
import type { Scan, Service } from '../api'
import { navigate } from '../lib/router'
import { formatNumber, formatDate } from '../lib/format'
import { DataTable, type Column } from '../components/DataTable'
import { Breadcrumb } from '../components/Breadcrumb'
import { Loading } from '../components/Loading'

interface ServicesPageProps {
  scanId: number
}

export function ServicesPage({ scanId }: ServicesPageProps) {
  const [scan, setScan] = useState<Scan | null>(null)
  const [services, setServices] = useState<Service[]>([])
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')

  const loadServices = useCallback(async () => {
    try {
      const data = await api.getServices(scanId, { search: search || undefined })
      setServices(data || [])
    } catch (e) {
      console.error(e)
    }
  }, [scanId, search])

  useEffect(() => {
    async function load() {
      setLoading(true)
      try {
        const [scanData, servicesData] = await Promise.all([
          api.getScan(scanId),
          api.getServices(scanId),
        ])
        setScan(scanData)
        setServices(servicesData || [])
      } catch (e) {
        console.error(e)
      }
      setLoading(false)
    }
    load()
  }, [scanId])

  useEffect(() => {
    if (!loading) loadServices()
  }, [search, loadServices, loading])

  if (loading) return <Loading />

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
    navigate({ page: 'metrics', scanId, serviceName: svc.name })
  }

  return (
    <div className="space-y-6">
      <Breadcrumb
        items={[
          { label: 'Scans', onClick: () => navigate({ page: 'scans' }) },
          { label: scan ? formatDate(scan.collected_at) : `Scan #${scanId}` },
        ]}
      />

      <div className="flex items-center justify-between">
        <div className="text-sm text-gray-500">
          {formatNumber(services.length)} services &middot; {scan ? formatNumber(scan.total_series) : '–'} total series
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
