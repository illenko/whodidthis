import { useEffect, useState, useCallback } from 'react'
import { api } from '../api'
import type { Scan, Service } from '../api'
import { navigate } from '../lib/router'
import { formatNumber } from '../lib/format'
import { useDebounce } from '../hooks/useDebounce'
import { DataTable, type Column } from '../components/DataTable'
import { Breadcrumb } from '../components/Breadcrumb'
import { Loading } from '../components/Loading'
import { Input } from '../components/Input'

interface ServicesPageProps {
  scanId: number
}

export function ServicesPage({ scanId }: ServicesPageProps) {
  const [scan, setScan] = useState<Scan | null>(null)
  const [services, setServices] = useState<Service[]>([])
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const debouncedSearch = useDebounce(search, 300)

  const loadServices = useCallback(async () => {
    try {
      const data = await api.getServices(scanId, { search: debouncedSearch || undefined })
      setServices(data || [])
    } catch (err) {
      console.error('Failed to load services:', err)
    }
  }, [scanId, debouncedSearch])

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
      } catch (err) {
        console.error('Failed to load scan data:', err)
      }
      setLoading(false)
    }
    load()
  }, [scanId])

  useEffect(() => {
    if (!loading) loadServices()
  }, [debouncedSearch, loadServices, loading])

  if (loading) return <Loading />

  const columns: Column<Service>[] = [
    {
      key: 'name',
      header: 'Service',
      render: (svc) => <span className="font-mono text-gray-900 dark:text-gray-100">{svc.name}</span>,
    },
    {
      key: 'series',
      header: 'Series',
      align: 'right',
      render: (svc) => <span className="text-gray-600 dark:text-gray-400">{formatNumber(svc.total_series)}</span>,
    },
    {
      key: 'metrics',
      header: 'Metrics',
      align: 'right',
      render: (svc) => <span className="text-gray-500 dark:text-gray-500">{svc.metric_count}</span>,
    },
  ]

  function handleRowClick(svc: Service) {
    navigate({ page: 'metrics', scanId, serviceName: svc.name })
  }

  return (
    <div className="space-y-6">
      <Breadcrumb
        items={[
          { label: 'Home', onClick: () => navigate({ page: 'scans' }) },
          { label: scan ? `Snapshot #${scan.id}` : `Snapshot #${scanId}` },
        ]}
      />

      <div className="flex items-center justify-between flex-wrap gap-4">
        <div className="text-sm text-gray-500 dark:text-gray-400">
          {formatNumber(services.length)} services · {scan ? formatNumber(scan.total_series) : '–'} total series
        </div>
        <Input
          type="text"
          placeholder="Search services..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="w-64"
          aria-label="Search services"
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
