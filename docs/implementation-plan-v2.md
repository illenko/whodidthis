# Service-Level Metrics Audit Tool - Implementation Plan v2

## Overview

This plan implements the **pivot** from a metric-centric audit tool to a **service-centric audit tool** as defined in `concept.md`. The new architecture discovers services via Prometheus labels and provides drill-down visibility: **Services → Metrics → Labels**.

### Key Changes from Previous Implementation

| Aspect | Before (v1) | After (v2) |
|--------|-------------|------------|
| Top-level grouping | Metrics globally | Services discovered via label |
| Team attribution | Regex patterns on metric names | Service label (e.g., `app`, `job`) |
| Hierarchy | Metric → Labels | Service → Metrics → Labels → Samples |
| Discovery | All metrics | Services first, then their metrics |
| Storage | Flat `metric_snapshots` | Normalized hierarchy with FK cascades |

---

## Phase 1: Database Schema Migration

### 1.1 Create New Migration

**File:** `storage/migrations/002_service_hierarchy.sql`

**Tasks:**
- [ ] Create new schema matching concept.md:
  ```sql
  -- Snapshot metadata (one per daily scan)
  CREATE TABLE snapshots (
      id INTEGER PRIMARY KEY,
      collected_at TIMESTAMP NOT NULL,
      scan_duration_ms INTEGER,
      total_services INTEGER,
      total_series BIGINT
  );

  -- Service level (top of hierarchy)
  CREATE TABLE service_snapshots (
      id INTEGER PRIMARY KEY,
      snapshot_id INTEGER REFERENCES snapshots(id) ON DELETE CASCADE,
      service_name TEXT NOT NULL,
      total_series INTEGER NOT NULL,
      metric_count INTEGER NOT NULL,
      UNIQUE(snapshot_id, service_name)
  );

  -- Metric level (per service)
  CREATE TABLE metric_snapshots (
      id INTEGER PRIMARY KEY,
      service_snapshot_id INTEGER REFERENCES service_snapshots(id) ON DELETE CASCADE,
      metric_name TEXT NOT NULL,
      series_count INTEGER NOT NULL,
      label_count INTEGER NOT NULL,
      UNIQUE(service_snapshot_id, metric_name)
  );

  -- Label level (per metric)
  CREATE TABLE label_snapshots (
      id INTEGER PRIMARY KEY,
      metric_snapshot_id INTEGER REFERENCES metric_snapshots(id) ON DELETE CASCADE,
      label_name TEXT NOT NULL,
      unique_values_count INTEGER NOT NULL,
      sample_values TEXT, -- JSON array of top N sample values
      UNIQUE(metric_snapshot_id, label_name)
  );
  ```
- [ ] Add all required indexes from concept.md
- [ ] Drop old tables: `metric_snapshots` (old), `recommendations`, `dashboard_stats`

**Deliverable:** New database schema with hierarchical structure

---

### 1.2 Update Storage Layer

**Files to modify:**
- `storage/sqlite.go` - Update migration loading
- `storage/snapshots_repo.go` - Rewrite for new schema

**Files to create:**
- `storage/services_repo.go` - Service-level queries
- `storage/labels_repo.go` - Label-level queries

**Tasks:**
- [ ] Update `sqlite.go` to run migration 002
- [ ] Implement `SnapshotsRepository`:
  ```go
  type SnapshotsRepository interface {
      Create(ctx context.Context, snapshot *Snapshot) error
      GetLatest(ctx context.Context) (*Snapshot, error)
      List(ctx context.Context) ([]Snapshot, error)
      GetByDate(ctx context.Context, date time.Time) (*Snapshot, error)
      DeleteOlderThan(ctx context.Context, days int) error
  }
  ```
- [ ] Implement `ServicesRepository`:
  ```go
  type ServicesRepository interface {
      Create(ctx context.Context, serviceSnapshot *ServiceSnapshot) error
      GetBySnapshotID(ctx context.Context, snapshotID int64) ([]ServiceSnapshot, error)
      GetByName(ctx context.Context, snapshotID int64, name string) (*ServiceSnapshot, error)
      GetWithDeltas(ctx context.Context, name string) (*ServiceWithDeltas, error)
      GetTrend(ctx context.Context, name string, days int) ([]TrendPoint, error)
  }
  ```
- [ ] Implement `MetricsRepository` (new version):
  ```go
  type MetricsRepository interface {
      Create(ctx context.Context, metricSnapshot *MetricSnapshot) error
      GetByServiceSnapshotID(ctx context.Context, serviceSnapshotID int64) ([]MetricSnapshot, error)
      GetByName(ctx context.Context, serviceSnapshotID int64, name string) (*MetricSnapshot, error)
      GetWithDeltas(ctx context.Context, serviceName, metricName string) (*MetricWithDeltas, error)
      GetTrend(ctx context.Context, serviceName, metricName string, days int) ([]TrendPoint, error)
  }
  ```
- [ ] Implement `LabelsRepository`:
  ```go
  type LabelsRepository interface {
      Create(ctx context.Context, labelSnapshot *LabelSnapshot) error
      GetByMetricSnapshotID(ctx context.Context, metricSnapshotID int64) ([]LabelSnapshot, error)
      GetWithDeltas(ctx context.Context, serviceName, metricName, labelName string) (*LabelWithDeltas, error)
  }
  ```
- [ ] Remove old repositories: `metrics_repo.go` (old version), `recommendations_repo.go`, `dashboards_repo.go`

**Deliverable:** Complete storage layer for hierarchical data

---

## Phase 2: Models Update

### 2.1 Update Data Models

**File:** `models/models.go`

**Tasks:**
- [ ] Remove old models: `MetricListItem`, `Recommendation`, `DashboardStats`, team-related structs
- [ ] Add new models matching concept.md:

```go
// Snapshot metadata
type Snapshot struct {
    ID             int64     `json:"id"`
    CollectedAt    time.Time `json:"collected_at"`
    ScanDurationMs int       `json:"duration_ms"`
    TotalServices  int       `json:"total_services"`
    TotalSeries    int64     `json:"total_series"`
}

// Service level
type ServiceSnapshot struct {
    ID           int64  `json:"id"`
    SnapshotID   int64  `json:"snapshot_id"`
    ServiceName  string `json:"service_name"`
    TotalSeries  int    `json:"total_series"`
    MetricCount  int    `json:"metric_count"`
}

// Service with delta calculations
type ServiceWithDeltas struct {
    Name         string  `json:"name"`
    TotalSeries  int     `json:"total_series"`
    MetricCount  int     `json:"metric_count"`
    Delta1d      int     `json:"delta_1d"`
    Delta7d      int     `json:"delta_7d"`
    Delta30d     int     `json:"delta_30d"`
    GrowthPct7d  float64 `json:"growth_pct_7d"`
}

// Metric level
type MetricSnapshot struct {
    ID                int64  `json:"id"`
    ServiceSnapshotID int64  `json:"service_snapshot_id"`
    MetricName        string `json:"metric_name"`
    SeriesCount       int    `json:"series_count"`
    LabelCount        int    `json:"label_count"`
}

// Metric with delta calculations
type MetricWithDeltas struct {
    Name        string `json:"name"`
    SeriesCount int    `json:"series_count"`
    LabelCount  int    `json:"label_count"`
    Delta1d     int    `json:"delta_1d"`
    Delta7d     int    `json:"delta_7d"`
    Delta30d    int    `json:"delta_30d"`
}

// Label level
type LabelSnapshot struct {
    ID                int64    `json:"id"`
    MetricSnapshotID  int64    `json:"metric_snapshot_id"`
    LabelName         string   `json:"label_name"`
    UniqueValuesCount int      `json:"unique_values"`
    SampleValues      []string `json:"sample_values"` // Top N samples
}

// Label with delta
type LabelWithDeltas struct {
    Name         string   `json:"name"`
    UniqueValues int      `json:"unique_values"`
    Delta1d      int      `json:"delta_1d"`
    SampleValues []string `json:"sample_values"`
}

// Trend data point
type TrendPoint struct {
    Date   string `json:"date"`
    Series int    `json:"series"`
}

// Overview response
type Overview struct {
    LatestScan     time.Time `json:"latest_scan"`
    TotalServices  int       `json:"total_services"`
    TotalSeries    int64     `json:"total_series"`
    SeriesDelta1d  int64     `json:"series_delta_1d"`
    SeriesDelta7d  int64     `json:"series_delta_7d"`
    SeriesDelta30d int64     `json:"series_delta_30d"`
}

// Scan status
type ScanStatus struct {
    Running      bool      `json:"running"`
    Progress     string    `json:"progress,omitempty"`
    LastScanAt   time.Time `json:"last_scan_at,omitempty"`
    LastDuration string    `json:"last_duration,omitempty"`
}
```

**Deliverable:** Updated models for hierarchical data

---

## Phase 3: Configuration Update

### 3.1 Simplify Configuration

**File:** `config/config.go`

**Tasks:**
- [ ] Remove `Teams` config with `MetricsPatterns`
- [ ] Add `Discovery` config:
  ```go
  type DiscoveryConfig struct {
      ServiceLabel string `mapstructure:"service_label"` // e.g., "app", "service", "job"
  }
  ```
- [ ] Add `Scan` config:
  ```go
  type ScanConfig struct {
      Schedule           string `mapstructure:"schedule"` // Cron format
      ServiceConcurrency int    `mapstructure:"service_concurrency"`
      MetricConcurrency  int    `mapstructure:"metric_concurrency"`
      RequestDelayMs     int    `mapstructure:"request_delay_ms"`
      SampleValuesLimit  int    `mapstructure:"sample_values_limit"`
  }
  ```
- [ ] Update defaults:
  ```go
  func DefaultConfig() *Config {
      return &Config{
          Discovery: DiscoveryConfig{
              ServiceLabel: "app",
          },
          Scan: ScanConfig{
              Schedule:           "0 2 * * *",
              ServiceConcurrency: 5,
              MetricConcurrency:  10,
              RequestDelayMs:     50,
              SampleValuesLimit:  10,
          },
          Storage: StorageConfig{
              Path:          "./data/metrics-audit.db",
              RetentionDays: 90,
          },
          Server: ServerConfig{
              Port: 8080,
              Host: "0.0.0.0",
          },
      }
  }
  ```
- [ ] Update `config.yaml` example

**Deliverable:** Simplified configuration focused on service discovery

---

## Phase 4: Prometheus Client Update

### 4.1 Add Service Discovery Queries

**File:** `prometheus/client.go`

**Tasks:**
- [ ] Add new query methods for service discovery:
  ```go
  // DiscoverServices returns all unique service values for the configured label
  // Query: count({service_label}!="") by ({service_label})
  func (c *Client) DiscoverServices(ctx context.Context, serviceLabel string) ([]ServiceCardinality, error)

  // GetMetricsForService returns all metrics for a specific service
  // Query: count({service_label}="X") by (__name__)
  func (c *Client) GetMetricsForService(ctx context.Context, serviceLabel, serviceName string) ([]MetricCardinality, error)

  // GetLabelsForMetric returns all label names and their cardinality for a metric within a service
  func (c *Client) GetLabelsForMetric(ctx context.Context, serviceLabel, serviceName, metricName string) ([]LabelInfo, error)

  // GetLabelSampleValues returns top N sample values for a specific label
  // Query: topk(N, count({...}) by (label_name))
  func (c *Client) GetLabelSampleValues(ctx context.Context, serviceLabel, serviceName, metricName, labelName string, limit int) ([]string, error)
  ```
- [ ] Add supporting types:
  ```go
  type ServiceCardinality struct {
      ServiceName string
      SeriesCount int
  }

  type MetricCardinality struct {
      MetricName  string
      SeriesCount int
  }

  type LabelInfo struct {
      LabelName    string
      UniqueValues int
      SampleValues []string
  }
  ```

**Deliverable:** Prometheus client with service discovery capabilities

---

## Phase 5: Collector Rewrite

### 5.1 Service-Centric Collection

**File:** `collector/prometheus_collector.go` (rewrite)

**Tasks:**
- [ ] Implement new collection flow per concept.md:
  ```
  1. Discover services
     Query: count({service_label}!="") by ({service_label})
     Result: [{service: "payment-gateway", count: 245231}, ...]

  2. For each service (parallel, rate-limited):
     Query: count({service_label}="X") by (__name__)
     Result: [{__name__: "http_requests_total", count: 38992}, ...]

  3. For each metric (parallel, rate-limited):
     Get label names and counts
     For high-cardinality labels (>100 values):
       Store top N sample values

  4. Store snapshot with full hierarchy
  ```
- [ ] Implement progress tracking with callback:
  ```go
  type ScanProgress struct {
      Phase           string // "discovering_services", "scanning_service", "scanning_metric"
      CurrentService  string
      ServicesTotal   int
      ServicesScanned int
      MetricsTotal    int
      MetricsScanned  int
  }

  type ProgressCallback func(progress ScanProgress)
  ```
- [ ] Add rate limiting per config:
  - Service concurrency (default: 5)
  - Metric concurrency (default: 10)
  - Request delay (default: 50ms)
- [ ] Implement transaction-based storage (all-or-nothing per snapshot)

**Deliverable:** Service-centric collection with progress tracking

---

### 5.2 Remove Unused Collectors

**Tasks:**
- [ ] Remove `collector/grafana_collector.go` (out of scope for MVP per concept.md)
- [ ] Remove `analyzer/team_matcher.go` (replaced by service discovery)
- [ ] Remove `analyzer/recommendations.go` (out of scope for MVP)

**Deliverable:** Cleaned up collector package

---

## Phase 6: API Refactoring

### 6.1 New API Endpoints

**File:** `api/handlers.go` (rewrite)

**Implement endpoints from concept.md:**

```
Overview & Scans:
  GET  /api/overview        → Overview stats with deltas
  GET  /api/scans           → List of all scans
  POST /api/scan            → Trigger manual scan
  GET  /api/scan/status     → Current scan status

Services (Level 1):
  GET  /api/services                → List all services with deltas
  GET  /api/services/{name}         → Service detail with 30-day trend

Metrics (Level 2):
  GET  /api/services/{service}/metrics          → List metrics for service
  GET  /api/services/{service}/metrics/{metric} → Metric detail with trend

Labels (Level 3):
  GET  /api/services/{service}/metrics/{metric}/labels → List labels with samples
```

**Tasks:**
- [ ] Implement `GET /api/overview`:
  ```go
  // Response
  {
      "latest_scan": "2024-01-15T02:00:00Z",
      "total_services": 127,
      "total_series": 2400000,
      "series_delta_1d": 50000,
      "series_delta_7d": 125000,
      "series_delta_30d": 300000
  }
  ```

- [ ] Implement `GET /api/scans`:
  ```go
  // Response
  [{
      "id": 1,
      "collected_at": "2024-01-15T02:00:00Z",
      "total_services": 127,
      "total_series": 2400000,
      "duration_ms": 754000
  }]
  ```

- [ ] Implement `GET /api/services`:
  ```go
  // Query params: ?sort=series|growth_1d|growth_7d|name&order=asc|desc
  // Response
  [{
      "name": "payment-gateway",
      "total_series": 245231,
      "metric_count": 89,
      "delta_1d": 5000,
      "delta_7d": 30000,
      "delta_30d": 110000,
      "growth_pct_7d": 12.0
  }]
  ```

- [ ] Implement `GET /api/services/{name}`:
  ```go
  // Response
  {
      "name": "payment-gateway",
      "total_series": 245231,
      "metric_count": 89,
      "delta_1d": 5000,
      "delta_7d": 30000,
      "delta_30d": 110000,
      "trend": [
          {"date": "2024-01-14", "series": 240231},
          {"date": "2024-01-15", "series": 245231}
      ]
  }
  ```

- [ ] Implement `GET /api/services/{service}/metrics`:
  ```go
  // Query params: ?sort=series|growth_1d|name&order=asc|desc
  // Response
  [{
      "name": "http_request_duration_seconds",
      "series_count": 45231,
      "label_count": 8,
      "delta_1d": 500,
      "delta_7d": 2000,
      "delta_30d": 5000
  }]
  ```

- [ ] Implement `GET /api/services/{service}/metrics/{metric}`:
  ```go
  // Response
  {
      "name": "http_request_duration_seconds",
      "series_count": 45231,
      "label_count": 8,
      "delta_1d": 500,
      "delta_7d": 2000,
      "delta_30d": 5000,
      "trend": [...]
  }
  ```

- [ ] Implement `GET /api/services/{service}/metrics/{metric}/labels`:
  ```go
  // Response
  [{
      "name": "endpoint",
      "unique_values": 1234,
      "delta_1d": 12,
      "sample_values": ["/api/v1/pay", "/api/v1/refund", ...]
  }]
  ```

- [ ] Update `POST /api/scan` and `GET /api/scan/status` for new flow

**Deliverable:** Complete API matching concept.md spec

---

### 6.2 Update Router

**File:** `api/server.go`

**Tasks:**
- [ ] Update route registration:
  ```go
  mux.HandleFunc("GET /api/overview", h.GetOverview)
  mux.HandleFunc("GET /api/scans", h.ListScans)
  mux.HandleFunc("POST /api/scan", h.TriggerScan)
  mux.HandleFunc("GET /api/scan/status", h.GetScanStatus)

  mux.HandleFunc("GET /api/services", h.ListServices)
  mux.HandleFunc("GET /api/services/{name}", h.GetService)
  mux.HandleFunc("GET /api/services/{service}/metrics", h.ListMetrics)
  mux.HandleFunc("GET /api/services/{service}/metrics/{metric}", h.GetMetric)
  mux.HandleFunc("GET /api/services/{service}/metrics/{metric}/labels", h.ListLabels)
  ```
- [ ] Remove unused endpoints: `/api/recommendations`, `/api/dashboards/unused`, `/api/trends`

**Deliverable:** Updated router with hierarchical endpoints

---

## Phase 7: Frontend Rewrite

### 7.1 Update TypeScript Types

**File:** `web/src/api.ts`

**Tasks:**
- [ ] Remove old types: `Metric`, `Recommendation`, `TeamMetrics`
- [ ] Add new types:
  ```typescript
  interface Overview {
      latest_scan: string;
      total_services: number;
      total_series: number;
      series_delta_1d: number;
      series_delta_7d: number;
      series_delta_30d: number;
  }

  interface Scan {
      id: number;
      collected_at: string;
      total_services: number;
      total_series: number;
      duration_ms: number;
  }

  interface Service {
      name: string;
      total_series: number;
      metric_count: number;
      delta_1d: number;
      delta_7d: number;
      delta_30d: number;
      growth_pct_7d: number;
  }

  interface ServiceDetail extends Service {
      trend: TrendPoint[];
  }

  interface Metric {
      name: string;
      series_count: number;
      label_count: number;
      delta_1d: number;
      delta_7d: number;
      delta_30d: number;
  }

  interface MetricDetail extends Metric {
      trend: TrendPoint[];
  }

  interface Label {
      name: string;
      unique_values: number;
      delta_1d: number;
      sample_values: string[];
  }

  interface TrendPoint {
      date: string;
      series: number;
  }
  ```
- [ ] Update API client functions:
  ```typescript
  export const fetchOverview = (): Promise<Overview> => ...
  export const fetchScans = (): Promise<Scan[]> => ...
  export const fetchServices = (sort?: string, order?: string): Promise<Service[]> => ...
  export const fetchService = (name: string): Promise<ServiceDetail> => ...
  export const fetchMetrics = (service: string, sort?: string): Promise<Metric[]> => ...
  export const fetchMetric = (service: string, metric: string): Promise<MetricDetail> => ...
  export const fetchLabels = (service: string, metric: string): Promise<Label[]> => ...
  ```

**Deliverable:** Updated TypeScript API client

---

### 7.2 Services List Page (Home)

**File:** `web/src/App.tsx` (or new component files)

**Tasks:**
- [ ] Create Services List as home page matching wireframe:
  ```
  ┌─────────────────────────────────────────────────────────────────┐
  │ Metrics Audit                              [Run Scan] [Status]  │
  ├─────────────────────────────────────────────────────────────────┤
  │ Overview: 127 services │ 2.4M series │ +5.2% (7d)              │
  │ Last scan: 2 hours ago (took 12m 34s)                          │
  ├─────────────────────────────────────────────────────────────────┤
  │ [Search services...]                        Sort: [Series ▼]   │
  ├─────────────────────────────────────────────────────────────────┤
  │ Service              │ Series   │ Metrics │ Δ1d   │ Δ7d  │ Δ30d │
  │──────────────────────│──────────│─────────│───────│──────│──────│
  │ payment-gateway      │ 245,231  │ 89      │ +2.1% │+12%  │ +45% │
  │ user-service         │ 189,442  │ 67      │ -0.5% │ +3%  │ +8%  │
  │ ...                  │          │         │       │      │      │
  └─────────────────────────────────────────────────────────────────┘
  ```
- [ ] Implement search filter
- [ ] Implement column sorting (Series, Δ1d, Δ7d, Name)
- [ ] Make service names clickable (navigate to detail)
- [ ] Color-code growth percentages (green positive, red negative)

**Deliverable:** Services list page

---

### 7.3 Service Detail Page

**File:** New component

**Tasks:**
- [ ] Create Service Detail page:
  ```
  ┌─────────────────────────────────────────────────────────────────┐
  │ ← Services / payment-gateway                                    │
  ├─────────────────────────────────────────────────────────────────┤
  │ 245,231 series │ 89 metrics │ +45% (30d)                       │
  │ ┌─────────────────────────────────────────────────────────────┐ │
  │ │ [30-day trend chart]                                        │ │
  │ └─────────────────────────────────────────────────────────────┘ │
  ├─────────────────────────────────────────────────────────────────┤
  │ Metric                        │ Series  │ Labels │ Δ1d  │ Δ7d  │
  │───────────────────────────────│─────────│────────│──────│──────│
  │ http_request_duration_seconds │ 45,231  │ 8      │ +1%  │ +5%  │
  │ http_requests_total           │ 38,992  │ 6      │ +2%  │ +8%  │
  │ ...                           │         │        │      │      │
  └─────────────────────────────────────────────────────────────────┘
  ```
- [ ] Add back navigation
- [ ] Add simple 30-day trend chart (use recharts or simple SVG)
- [ ] Make metric names clickable (navigate to metric detail)

**Deliverable:** Service detail page with metrics list

---

### 7.4 Metric Detail Page

**File:** New component

**Tasks:**
- [ ] Create Metric Detail page:
  ```
  ┌─────────────────────────────────────────────────────────────────┐
  │ ← payment-gateway / http_request_duration_seconds               │
  ├─────────────────────────────────────────────────────────────────┤
  │ 45,231 series │ 8 labels │ +5% (7d)                            │
  │ ┌─────────────────────────────────────────────────────────────┐ │
  │ │ [30-day trend chart]                                        │ │
  │ └─────────────────────────────────────────────────────────────┘ │
  ├─────────────────────────────────────────────────────────────────┤
  │ Label          │ Unique Values │ Δ1d  │ Sample Values          │
  │────────────────│───────────────│──────│────────────────────────│
  │ endpoint       │ 1,234         │ +12  │ /api/v1/pay, /api/v1/… │
  │ status_code    │ 8             │ 0    │ 200, 201, 400, 404, …  │
  │ method         │ 4             │ 0    │ GET, POST, PUT, DELETE │
  │ ...            │               │      │                        │
  └─────────────────────────────────────────────────────────────────┘
  ```
- [ ] Add breadcrumb navigation
- [ ] Show trend chart
- [ ] Display labels with sample values (truncated with tooltip)

**Deliverable:** Metric detail page with labels

---

### 7.5 Navigation & Routing

**Tasks:**
- [ ] Implement hash-based routing:
  - `#/` → Services List
  - `#/services/{name}` → Service Detail
  - `#/services/{service}/metrics/{metric}` → Metric Detail
- [ ] Add breadcrumb component
- [ ] Update header with scan controls

**Deliverable:** Working navigation between pages

---

## Phase 8: Cleanup & Polish

### 8.1 Remove Unused Code

**Tasks:**
- [ ] Remove `analyzer/` directory (team_matcher, recommendations, trends)
- [ ] Remove `grafana/` directory (not in MVP scope)
- [ ] Remove old storage repositories
- [ ] Update main.go to remove grafana/recommendations initialization

**Deliverable:** Clean codebase

---

### 8.2 Update Configuration Example

**File:** `config.yaml`

**Tasks:**
- [ ] Update config.yaml to match new structure:
  ```yaml
  prometheus:
    url: http://localhost:9090
    # For VictoriaMetrics, use the same URL

  discovery:
    service_label: app  # Single label that identifies services

  scan:
    schedule: "0 2 * * *"  # Cron format (2am daily)
    service_concurrency: 5
    metric_concurrency: 10
    request_delay_ms: 50
    sample_values_limit: 10

  storage:
    path: ./data/metrics-audit.db
    retention_days: 90

  server:
    port: 8080
    host: 0.0.0.0
  ```

**Deliverable:** Updated example configuration

---

## Implementation Order

### Week 1: Database & Models
1. Phase 1.1: Database migration
2. Phase 2.1: Update models
3. Phase 1.2: Storage layer (snapshots + services repos)

### Week 2: Collection
4. Phase 3.1: Configuration update
5. Phase 4.1: Prometheus client update
6. Phase 5.1: Collector rewrite
7. Phase 5.2: Remove unused collectors

### Week 3: API
8. Phase 6.1: New API endpoints
9. Phase 6.2: Update router
10. Phase 1.2 (continued): Storage layer (metrics + labels repos)

### Week 4: Frontend
11. Phase 7.1: Update TypeScript types
12. Phase 7.2: Services List page
13. Phase 7.3: Service Detail page
14. Phase 7.4: Metric Detail page
15. Phase 7.5: Navigation & routing

### Week 5: Polish
16. Phase 8.1: Remove unused code
17. Phase 8.2: Update configuration

---

## Testing Checklist

### Manual Testing
- [ ] Run scan with real Prometheus/VictoriaMetrics
- [ ] Verify services discovered correctly
- [ ] Verify metrics per service correct
- [ ] Verify labels and sample values stored
- [ ] Verify delta calculations (need 2+ scans)
- [ ] Test all API endpoints with curl
- [ ] Navigate through all frontend pages
- [ ] Test sort/filter functionality

### Integration Points
- [ ] Prometheus service discovery query
- [ ] SQLite cascade deletes
- [ ] Frontend → API communication
- [ ] Scan progress reporting

---

## Out of Scope (per concept.md)

- Real-time alerts/monitoring
- Cost calculations
- Recommendations engine
- Grafana integration
- Team/owner attribution
- CLI interface (beyond serve)
- Anomaly detection
- Multiple Prometheus sources
- Authentication/authorization

---

## Files Summary

### To Create
- `storage/migrations/002_service_hierarchy.sql`
- `storage/services_repo.go`
- `storage/labels_repo.go`

### To Rewrite
- `storage/snapshots_repo.go`
- `storage/metrics_repo.go` (if exists, or create new)
- `collector/prometheus_collector.go`
- `api/handlers.go`
- `web/src/App.tsx`
- `web/src/api.ts`

### To Remove
- `storage/recommendations_repo.go`
- `storage/dashboards_repo.go`
- `analyzer/team_matcher.go`
- `analyzer/recommendations.go`
- `analyzer/trends.go`
- `collector/grafana_collector.go`
- `grafana/client.go`

### To Modify
- `config/config.go`
- `models/models.go`
- `prometheus/client.go`
- `api/server.go`
- `main.go`
- `config.yaml`