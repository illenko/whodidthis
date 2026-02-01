import { useEffect, useState, useMemo } from 'react'
import { api } from '../api'
import type { Scan, Metric, Label } from '../api'
import { navigate } from '../lib/router'
import { formatNumber, formatDate } from '../lib/format'
import { DataTable, type Column } from '../components/DataTable'
import { Breadcrumb } from '../components/Breadcrumb'
import { Loading } from '../components/Loading'

interface LabelsPageProps {
  scanId: number
  serviceName: string
  metricName: string
}

export function LabelsPage({ scanId, serviceName, metricName }: LabelsPageProps) {
  const [scans, setScans] = useState<Scan[]>([])
  const [selectedScanId, setSelectedScanId] = useState<number>(scanId)
  const [metric, setMetric] = useState<Metric | null>(null)
  const [labels, setLabels] = useState<Label[]>([])
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

  // Load metric and labels when scan changes
  useEffect(() => {
    async function load() {
      setLoading(true)
      try {
        const [metricData, labelsData] = await Promise.all([
          api.getMetric(selectedScanId, serviceName, metricName),
          api.getLabels(selectedScanId, serviceName, metricName),
        ])
        setMetric(metricData)
        setLabels(labelsData || [])
      } catch (e) {
        console.error(e)
      }
      setLoading(false)
    }
    load()
  }, [selectedScanId, serviceName, metricName])

  // Update URL when snapshot changes
  useEffect(() => {
    if (selectedScanId !== scanId) {
      navigate({ page: 'labels', scanId: selectedScanId, serviceName, metricName })
    }
  }, [selectedScanId, scanId, serviceName, metricName])

  // Filter labels by search
  const filteredLabels = useMemo(() => {
    if (!search) return labels
    const lower = search.toLowerCase()
    return labels.filter(l => l.name.toLowerCase().includes(lower))
  }, [labels, search])

  if (loading) return <Loading />

  const columns: Column<Label>[] = [
    {
      key: 'name',
      header: 'Label',
      render: (l) => <span className="font-mono text-gray-900">{l.name}</span>,
    },
    {
      key: 'unique',
      header: 'Unique Values',
      align: 'right',
      render: (l) => <span className="text-gray-600">{formatNumber(l.unique_values)}</span>,
    },
    {
      key: 'samples',
      header: 'Sample Values',
      render: (l) => <SampleValues values={l.sample_values} />,
    },
  ]

  return (
    <div className="space-y-6">
      <Breadcrumb
        items={[
          { label: 'Services', onClick: () => navigate({ page: 'scans' }) },
          { label: serviceName, onClick: () => navigate({ page: 'metrics', scanId: selectedScanId, serviceName }) },
          { label: metricName },
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
            {metric ? formatNumber(metric.series_count) : '–'} series · {formatNumber(filteredLabels.length)} labels
          </span>
        </div>
        <input
          type="text"
          placeholder="Search labels..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="px-4 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-gray-200 w-64"
        />
      </div>

      <DataTable
        columns={columns}
        data={filteredLabels}
        keyExtractor={(l) => l.id}
      />
    </div>
  )
}

function SampleValues({ values }: { values?: string[] }) {
  if (!values || values.length === 0) {
    return <span className="text-gray-400">–</span>
  }

  return (
    <div className="flex flex-wrap gap-1">
      {values.map((v, i) => (
        <span key={i} className="px-2 py-0.5 bg-gray-100 rounded text-xs font-mono">
          {v}
        </span>
      ))}
    </div>
  )
}
