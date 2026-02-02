import { useEffect, useState, useMemo } from 'react'
import { api } from '../api'
import type { Scan, Metric, Label } from '../api'
import { navigate } from '../lib/router'
import { formatNumber, formatDate } from '../lib/format'
import { useDebounce } from '../hooks/useDebounce'
import { DataTable, type Column } from '../components/DataTable'
import { Breadcrumb } from '../components/Breadcrumb'
import { Loading } from '../components/Loading'
import { Input } from '../components/Input'
import { Select } from '../components/Select'

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
      } catch (err) {
        console.error('Failed to load labels:', err)
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
    if (!debouncedSearch) return labels
    const lower = debouncedSearch.toLowerCase()
    return labels.filter(l => l.name.toLowerCase().includes(lower))
  }, [labels, debouncedSearch])

  if (loading) return <Loading />

  const columns: Column<Label>[] = [
    {
      key: 'name',
      header: 'Label',
      render: (l) => <span className="font-mono text-gray-900 dark:text-gray-100">{l.name}</span>,
    },
    {
      key: 'unique',
      header: 'Unique Values',
      align: 'right',
      render: (l) => <span className="text-gray-600 dark:text-gray-400">{formatNumber(l.unique_values)}</span>,
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
            {metric ? formatNumber(metric.series_count) : '–'} series · {formatNumber(filteredLabels.length)} labels
          </span>
        </div>
        <Input
          type="text"
          placeholder="Search labels..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="w-64"
          aria-label="Search labels"
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
    return <span className="text-gray-400 dark:text-gray-600">–</span>
  }

  return (
    <div className="flex flex-wrap gap-1">
      {values.map((v, i) => (
        <span key={i} className="px-2 py-0.5 bg-gray-100 dark:bg-gray-800 text-gray-900 dark:text-gray-100 rounded text-xs font-mono">
          {v}
        </span>
      ))}
    </div>
  )
}
