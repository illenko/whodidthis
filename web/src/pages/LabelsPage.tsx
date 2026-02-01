import { useEffect, useState } from 'react'
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
          { label: 'Scans', onClick: () => navigate({ page: 'scans' }) },
          { label: scan ? formatDate(scan.collected_at) : `Scan #${scanId}`, onClick: () => navigate({ page: 'services', scanId }) },
          { label: serviceName, onClick: () => navigate({ page: 'metrics', scanId, serviceName }) },
          { label: metricName },
        ]}
      />

      <div className="text-sm text-gray-500">
        {metric ? formatNumber(metric.series_count) : '–'} series &middot; {labels.length} labels
      </div>

      <DataTable
        columns={columns}
        data={labels}
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
