import { useEffect, useState } from 'react'
import { api } from './api'
import type { ScanStatus } from './api'
import { type Route, parseRoute, navigate } from './lib/router'
import { ScansPage } from './pages/ScansPage'
import { ServicesPage } from './pages/ServicesPage'
import { MetricsPage } from './pages/MetricsPage'
import { LabelsPage } from './pages/LabelsPage'

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
      <Header scanStatus={scanStatus} onScan={handleScan} />
      <main className="max-w-6xl mx-auto px-6 py-8">
        <Router route={route} />
      </main>
    </div>
  )
}

function Header({ scanStatus, onScan }: { scanStatus: ScanStatus | null; onScan: () => void }) {
  return (
    <header className="border-b border-gray-200">
      <div className="max-w-6xl mx-auto px-6 py-4 flex items-center justify-between">
        <button
          onClick={() => navigate({ page: 'scans' })}
          className="text-xl font-semibold text-gray-900 hover:text-gray-700"
        >
          🤔WhoDidThis?
        </button>
        <div className="flex items-center gap-4">
          {scanStatus?.running && (
            <span className="text-sm text-gray-500 flex items-center gap-2">
              <span className="w-2 h-2 bg-green-500 rounded-full animate-pulse" />
              {scanStatus.progress || 'Scanning...'}
            </span>
          )}
          <button
            onClick={onScan}
            disabled={scanStatus?.running}
            className="px-4 py-2 text-sm bg-gray-900 text-white rounded-lg hover:bg-gray-800 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            Run Scan
          </button>
        </div>
      </div>
    </header>
  )
}

function Router({ route }: { route: Route }) {
  switch (route.page) {
    case 'scans':
      return <ScansPage />
    case 'services':
      return <ServicesPage scanId={route.scanId} />
    case 'metrics':
      return <MetricsPage scanId={route.scanId} serviceName={route.serviceName} />
    case 'labels':
      return <LabelsPage scanId={route.scanId} serviceName={route.serviceName} metricName={route.metricName} />
  }
}

export default App
