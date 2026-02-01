import { useEffect, useState, useCallback } from 'react'
import { api } from './api'
import type { Scan, Service, Metric, Label, ScanStatus } from './api'

// Utilities
function formatNumber(n: number): string {
  return n.toLocaleString()
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString()
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  const s = Math.floor(ms / 1000)
  if (s < 60) return `${s}s`
  const m = Math.floor(s / 60)
  const rs = s % 60
  return `${m}m ${rs}s`
}

// Route types
type Route =
  | { page: 'scans' }
  | { page: 'services'; scanId: number }
  | { page: 'metrics'; scanId: number; serviceName: string }
  | { page: 'labels'; scanId: number; serviceName: string; metricName: string }

function parseRoute(): Route {
  const hash = window.location.hash.slice(1) // Remove #
  if (!hash || hash === '/') return { page: 'scans' }

  const parts = hash.split('/').filter(Boolean)

  // /scans/{id}/services/{name}/metrics/{metric}
  if (parts[0] === 'scans' && parts.length >= 2) {
    const scanId = parseInt(parts[1], 10)
    if (isNaN(scanId)) return { page: 'scans' }

    if (parts[2] === 'services' && parts[3]) {
      const serviceName = decodeURIComponent(parts[3])

      if (parts[4] === 'metrics' && parts[5]) {
        const metricName = decodeURIComponent(parts[5])
        return { page: 'labels', scanId, serviceName, metricName }
      }

      return { page: 'metrics', scanId, serviceName }
    }

    return { page: 'services', scanId }
  }

  return { page: 'scans' }
}

function navigate(route: Route) {
  let hash = '#/'
  if (route.page === 'services') {
    hash = `#/scans/${route.scanId}`
  } else if (route.page === 'metrics') {
    hash = `#/scans/${route.scanId}/services/${encodeURIComponent(route.serviceName)}`
  } else if (route.page === 'labels') {
    hash = `#/scans/${route.scanId}/services/${encodeURIComponent(route.serviceName)}/metrics/${encodeURIComponent(route.metricName)}`
  }
  window.location.hash = hash
}

// Main App
function App() {
  const [route, setRoute] = useState<Route>(parseRoute)
  const [scanStatus, setScanStatus] = useState<ScanStatus | null>(null)

  useEffect(() => {
    const onHashChange = () => setRoute(parseRoute())
    window.addEventListener('hashchange', onHashChange)
    return () => window.removeEventListener('hashchange', onHashChange)
  }, [])

  useEffect(() => {
    loadScanStatus()
    const interval = setInterval(loadScanStatus, 5000)
    return () => clearInterval(interval)
  }, [])

  async function loadScanStatus() {
    try {
      setScanStatus(await api.getScanStatus())
    } catch (e) {
      console.error(e)
    }
  }

  async function handleScan() {
    await api.triggerScan()
    loadScanStatus()
  }

  return (
    <div className="min-h-screen bg-white">
      <header className="border-b border-gray-200">
        <div className="max-w-6xl mx-auto px-6 py-4 flex items-center justify-between">
          <button
            onClick={() => navigate({ page: 'scans' })}
            className="text-xl font-semibold text-gray-900 hover:text-gray-700"
          >
            Metrics Audit
          </button>
          <div className="flex items-center gap-4">
            {scanStatus?.running && (
              <span className="text-sm text-gray-500 flex items-center gap-2">
                <span className="w-2 h-2 bg-green-500 rounded-full animate-pulse" />
                {scanStatus.progress || 'Scanning...'}
              </span>
            )}
            <button
              onClick={handleScan}
              disabled={scanStatus?.running}
              className="px-4 py-2 text-sm bg-gray-900 text-white rounded-lg hover:bg-gray-800 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              Run Scan
            </button>
          </div>
        </div>
      </header>

      <main className="max-w-6xl mx-auto px-6 py-8">
        {route.page === 'scans' && <ScansPage onNavigate={navigate} />}
        {route.page === 'services' && <ServicesPage scanId={route.scanId} onNavigate={navigate} />}
        {route.page === 'metrics' && (
          <MetricsPage scanId={route.scanId} serviceName={route.serviceName} onNavigate={navigate} />
        )}
        {route.page === 'labels' && (
          <LabelsPage scanId={route.scanId} serviceName={route.serviceName} metricName={route.metricName} onNavigate={navigate} />
        )}
      </main>
    </div>
  )
}

// Scans List Page
function ScansPage({ onNavigate }: { onNavigate: (r: Route) => void }) {
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

  if (loading) return <div className="text-gray-500">Loading...</div>

  if (scans.length === 0) {
    return (
      <div className="text-center py-12">
        <div className="text-gray-500 mb-4">No scans yet</div>
        <div className="text-sm text-gray-400">Run a scan to discover services and metrics</div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <h2 className="text-lg font-medium text-gray-900">Scan History</h2>
      <div className="border border-gray-200 rounded-lg overflow-hidden">
        <table className="w-full">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Date</th>
              <th className="px-4 py-3 text-right text-xs font-medium text-gray-500 uppercase">Services</th>
              <th className="px-4 py-3 text-right text-xs font-medium text-gray-500 uppercase">Total Series</th>
              <th className="px-4 py-3 text-right text-xs font-medium text-gray-500 uppercase">Duration</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-200">
            {scans.map((scan) => (
              <tr
                key={scan.id}
                onClick={() => onNavigate({ page: 'services', scanId: scan.id })}
                className="hover:bg-gray-50 cursor-pointer"
              >
                <td className="px-4 py-3 text-sm text-gray-900">{formatDate(scan.collected_at)}</td>
                <td className="px-4 py-3 text-sm text-gray-600 text-right">{formatNumber(scan.total_services)}</td>
                <td className="px-4 py-3 text-sm text-gray-600 text-right">{formatNumber(scan.total_series)}</td>
                <td className="px-4 py-3 text-sm text-gray-500 text-right">{formatDuration(scan.duration_ms)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

// Services List Page
function ServicesPage({ scanId, onNavigate }: { scanId: number; onNavigate: (r: Route) => void }) {
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

  if (loading) return <div className="text-gray-500">Loading...</div>

  return (
    <div className="space-y-6">
      <Breadcrumb
        items={[
          { label: 'Scans', onClick: () => onNavigate({ page: 'scans' }) },
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

      <div className="border border-gray-200 rounded-lg overflow-hidden">
        <table className="w-full">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Service</th>
              <th className="px-4 py-3 text-right text-xs font-medium text-gray-500 uppercase">Series</th>
              <th className="px-4 py-3 text-right text-xs font-medium text-gray-500 uppercase">Metrics</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-200">
            {services.map((svc) => (
              <tr
                key={svc.id}
                onClick={() => onNavigate({ page: 'metrics', scanId, serviceName: svc.name })}
                className="hover:bg-gray-50 cursor-pointer"
              >
                <td className="px-4 py-3 text-sm font-mono text-gray-900">{svc.name}</td>
                <td className="px-4 py-3 text-sm text-gray-600 text-right">{formatNumber(svc.total_series)}</td>
                <td className="px-4 py-3 text-sm text-gray-500 text-right">{svc.metric_count}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

// Metrics List Page
function MetricsPage({
  scanId,
  serviceName,
  onNavigate,
}: {
  scanId: number
  serviceName: string
  onNavigate: (r: Route) => void
}) {
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

  if (loading) return <div className="text-gray-500">Loading...</div>

  return (
    <div className="space-y-6">
      <Breadcrumb
        items={[
          { label: 'Scans', onClick: () => onNavigate({ page: 'scans' }) },
          { label: scan ? formatDate(scan.collected_at) : `Scan #${scanId}`, onClick: () => onNavigate({ page: 'services', scanId }) },
          { label: serviceName },
        ]}
      />

      <div className="text-sm text-gray-500">
        {service ? formatNumber(service.total_series) : '–'} series &middot; {metrics.length} metrics
      </div>

      <div className="border border-gray-200 rounded-lg overflow-hidden">
        <table className="w-full">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Metric</th>
              <th className="px-4 py-3 text-right text-xs font-medium text-gray-500 uppercase">Series</th>
              <th className="px-4 py-3 text-right text-xs font-medium text-gray-500 uppercase">Labels</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-200">
            {metrics.map((m) => (
              <tr
                key={m.id}
                onClick={() => onNavigate({ page: 'labels', scanId, serviceName, metricName: m.name })}
                className="hover:bg-gray-50 cursor-pointer"
              >
                <td className="px-4 py-3 text-sm font-mono text-gray-900">{m.name}</td>
                <td className="px-4 py-3 text-sm text-gray-600 text-right">{formatNumber(m.series_count)}</td>
                <td className="px-4 py-3 text-sm text-gray-500 text-right">{m.label_count}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

// Labels List Page
function LabelsPage({
  scanId,
  serviceName,
  metricName,
  onNavigate,
}: {
  scanId: number
  serviceName: string
  metricName: string
  onNavigate: (r: Route) => void
}) {
  const [scan, setScan] = useState<Scan | null>(null)
  const [metric, setMetric] = useState<Metric | null>(null)
  const [labels, setLabels] = useState<Label[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    async function load() {
      setLoading(true)
      try {
        const [scanData, metricData, labelsData] = await Promise.all([
          api.getScan(scanId),
          api.getMetric(scanId, serviceName, metricName),
          api.getLabels(scanId, serviceName, metricName),
        ])
        setScan(scanData)
        setMetric(metricData)
        setLabels(labelsData || [])
      } catch (e) {
        console.error(e)
      }
      setLoading(false)
    }
    load()
  }, [scanId, serviceName, metricName])

  if (loading) return <div className="text-gray-500">Loading...</div>

  return (
    <div className="space-y-6">
      <Breadcrumb
        items={[
          { label: 'Scans', onClick: () => onNavigate({ page: 'scans' }) },
          { label: scan ? formatDate(scan.collected_at) : `Scan #${scanId}`, onClick: () => onNavigate({ page: 'services', scanId }) },
          { label: serviceName, onClick: () => onNavigate({ page: 'metrics', scanId, serviceName }) },
          { label: metricName },
        ]}
      />

      <div className="text-sm text-gray-500">
        {metric ? formatNumber(metric.series_count) : '–'} series &middot; {labels.length} labels
      </div>

      <div className="border border-gray-200 rounded-lg overflow-hidden">
        <table className="w-full">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Label</th>
              <th className="px-4 py-3 text-right text-xs font-medium text-gray-500 uppercase">Unique Values</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Sample Values</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-200">
            {labels.map((l) => (
              <tr key={l.id}>
                <td className="px-4 py-3 text-sm font-mono text-gray-900">{l.name}</td>
                <td className="px-4 py-3 text-sm text-gray-600 text-right">{formatNumber(l.unique_values)}</td>
                <td className="px-4 py-3 text-sm text-gray-500">
                  {l.sample_values && l.sample_values.length > 0 ? (
                    <div className="flex flex-wrap gap-1">
                      {l.sample_values.map((v, i) => (
                        <span key={i} className="px-2 py-0.5 bg-gray-100 rounded text-xs font-mono">
                          {v}
                        </span>
                      ))}
                    </div>
                  ) : (
                    <span className="text-gray-400">–</span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

// Breadcrumb component
function Breadcrumb({ items }: { items: Array<{ label: string; onClick?: () => void }> }) {
  return (
    <nav className="flex items-center gap-2 text-sm">
      {items.map((item, i) => (
        <span key={i} className="flex items-center gap-2">
          {i > 0 && <span className="text-gray-400">/</span>}
          {item.onClick ? (
            <button onClick={item.onClick} className="text-gray-500 hover:text-gray-900">
              {item.label}
            </button>
          ) : (
            <span className="text-gray-900 font-medium">{item.label}</span>
          )}
        </span>
      ))}
    </nav>
  )
}

export default App