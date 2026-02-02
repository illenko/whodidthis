import type { Route } from '../lib/router'
import { ScansPage } from '../pages/ScansPage'
import { ServicesPage } from '../pages/ServicesPage'
import { MetricsPage } from '../pages/MetricsPage'
import { LabelsPage } from '../pages/LabelsPage'

interface RouterProps {
  route: Route
}

export function Router({ route }: RouterProps) {
  switch (route.page) {
    case 'scans':
      return <ScansPage />
    case 'services':
      return <ServicesPage scanId={route.scanId} />
    case 'metrics':
      return <MetricsPage scanId={route.scanId} serviceName={route.serviceName} />
    case 'labels':
      return <LabelsPage scanId={route.scanId} serviceName={route.serviceName} metricName={route.metricName} />
    default:
      // TypeScript exhaustiveness check
      const _exhaustiveCheck: never = route
      return _exhaustiveCheck
  }
}
