export type Route =
  | { page: 'scans' }
  | { page: 'services'; scanId: number }
  | { page: 'metrics'; scanId: number; serviceName: string }
  | { page: 'labels'; scanId: number; serviceName: string; metricName: string }

export function parseRoute(): Route {
  const hash = window.location.hash.slice(1)
  if (!hash || hash === '/') return { page: 'scans' }

  const parts = hash.split('/').filter(Boolean)

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

export function navigate(route: Route) {
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
