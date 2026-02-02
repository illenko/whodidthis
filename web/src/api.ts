import { API_BASE_URL, DEFAULT_SCANS_LIMIT } from './lib/constants'

// Core types matching backend models
export interface Scan {
  id: number
  collected_at: string
  total_services: number
  total_series: number
  duration_ms: number
}

export interface Service {
  id: number
  snapshot_id: number
  name: string
  total_series: number
  metric_count: number
}

export interface Metric {
  id: number
  service_snapshot_id: number
  name: string
  series_count: number
  label_count: number
}

export interface Label {
  id: number
  metric_snapshot_id: number
  name: string
  unique_values: number
  sample_values: string[]
}

export interface ScanStatus {
  running: boolean
  progress: string
  last_scan_at: string
  last_duration: string
  last_error: string
  next_scan_at: string
  total_services: number
  total_series: number
}

export interface HealthStatus {
  status: string
  database_ok: boolean
  last_scan: string
}

async function fetchJSON<T>(url: string): Promise<T> {
  const res = await fetch(url)
  if (!res.ok) {
    throw new Error(`HTTP ${res.status}: ${res.statusText}`)
  }
  return res.json()
}

async function fetchJSONOrNull<T>(url: string): Promise<T | null> {
  const res = await fetch(url)
  if (!res.ok) {
    if (res.status === 404) return null
    throw new Error(`HTTP ${res.status}: ${res.statusText}`)
  }
  return res.json()
}

export const api = {
  // Health
  getHealth: () => fetchJSON<HealthStatus>(`${API_BASE_URL}/../health`),

  // Scan control
  getScanStatus: () => fetchJSON<ScanStatus>(`${API_BASE_URL}/scan/status`),
  triggerScan: () => fetch(`${API_BASE_URL}/scan`, { method: 'POST' }),

  // Scans (snapshots)
  getScans: (limit = DEFAULT_SCANS_LIMIT) => fetchJSON<Scan[]>(`${API_BASE_URL}/scans?limit=${limit}`),
  getLatestScan: () => fetchJSONOrNull<Scan>(`${API_BASE_URL}/scans/latest`),
  getScan: (id: number) => fetchJSON<Scan>(`${API_BASE_URL}/scans/${id}`),

  // Services (within a scan)
  getServices: (scanId: number, params?: { sort?: string; order?: string; search?: string }) => {
    const query = new URLSearchParams()
    if (params?.sort) query.set('sort', params.sort)
    if (params?.order) query.set('order', params.order)
    if (params?.search) query.set('search', params.search)
    const qs = query.toString()
    return fetchJSON<Service[]>(`${API_BASE_URL}/scans/${scanId}/services${qs ? '?' + qs : ''}`)
  },
  getService: (scanId: number, serviceName: string) =>
    fetchJSON<Service>(`${API_BASE_URL}/scans/${scanId}/services/${encodeURIComponent(serviceName)}`),

  // Metrics (within a service)
  getMetrics: (scanId: number, serviceName: string, params?: { sort?: string; order?: string }) => {
    const query = new URLSearchParams()
    if (params?.sort) query.set('sort', params.sort)
    if (params?.order) query.set('order', params.order)
    const qs = query.toString()
    return fetchJSON<Metric[]>(
      `${API_BASE_URL}/scans/${scanId}/services/${encodeURIComponent(serviceName)}/metrics${qs ? '?' + qs : ''}`
    )
  },
  getMetric: (scanId: number, serviceName: string, metricName: string) =>
    fetchJSON<Metric>(
      `${API_BASE_URL}/scans/${scanId}/services/${encodeURIComponent(serviceName)}/metrics/${encodeURIComponent(metricName)}`
    ),

  // Labels (within a metric)
  getLabels: (scanId: number, serviceName: string, metricName: string) =>
    fetchJSON<Label[]>(
      `${API_BASE_URL}/scans/${scanId}/services/${encodeURIComponent(serviceName)}/metrics/${encodeURIComponent(metricName)}/labels`
    ),
}
