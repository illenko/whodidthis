# Metrics Audit Tool - Concept

## Problem Statement

As SRE team member I need to:
- **Regularly audit** which services produce the most metrics
- **Catch growth trends** before Prometheus/VictoriaMetrics gets overloaded
- **Quickly investigate** what changed when incidents happen
- **Hold teams accountable** for their metrics footprint

Current tools (VictoriaMetrics Cardinality Explorer, Prometheus TSDB Status) show point-in-time data but don't track history or provide service-level aggregation.

## Solution

A **Service-Level Metrics Audit Tool** that:
1. Runs daily automated scans (configurable schedule)
2. Discovers services via configured label (e.g., `app`, `service`, `job`)
3. Stores hierarchical snapshots in SQLite
4. Provides drill-down UI: Services → Metrics → Labels
5. Shows comparisons: Δ 1d, Δ 7d, Δ 30d
6. Retains 90 days of history

## Data Hierarchy

```
Snapshot (daily scan)
└── Service (discovered via label)
    └── Metric (belongs to service)
        └── Label (belongs to metric)
            └── Sample values (top N for debugging)
```

## Database Schema

```sql
-- Snapshot metadata (one per daily scan)
CREATE TABLE snapshots (
    id INTEGER PRIMARY KEY,
    collected_at TIMESTAMP NOT NULL,
    scan_duration_ms INTEGER,
    total_services INTEGER,
    total_series BIGINT
);
CREATE INDEX idx_snapshots_time ON snapshots(collected_at DESC);

-- Service level (top of hierarchy)
CREATE TABLE service_snapshots (
    id INTEGER PRIMARY KEY,
    snapshot_id INTEGER REFERENCES snapshots(id) ON DELETE CASCADE,
    service_name TEXT NOT NULL,
    total_series INTEGER NOT NULL,
    metric_count INTEGER NOT NULL,
    UNIQUE(snapshot_id, service_name)
);
CREATE INDEX idx_service_snapshots_lookup ON service_snapshots(snapshot_id, service_name);

-- Metric level (per service)
CREATE TABLE metric_snapshots (
    id INTEGER PRIMARY KEY,
    service_snapshot_id INTEGER REFERENCES service_snapshots(id) ON DELETE CASCADE,
    metric_name TEXT NOT NULL,
    series_count INTEGER NOT NULL,
    label_count INTEGER NOT NULL,
    UNIQUE(service_snapshot_id, metric_name)
);
CREATE INDEX idx_metric_snapshots_lookup ON metric_snapshots(service_snapshot_id);

-- Label level (per metric)
CREATE TABLE label_snapshots (
    id INTEGER PRIMARY KEY,
    metric_snapshot_id INTEGER REFERENCES metric_snapshots(id) ON DELETE CASCADE,
    label_name TEXT NOT NULL,
    unique_values_count INTEGER NOT NULL,
    sample_values TEXT, -- JSON array of top N sample values
    UNIQUE(metric_snapshot_id, label_name)
);
CREATE INDEX idx_label_snapshots_lookup ON label_snapshots(metric_snapshot_id);
```

### Schema Rationale

- **Normalized hierarchy** via foreign keys (snapshot → service → metric → label)
- **CASCADE deletes** for easy retention cleanup
- **Composite unique constraints** prevent duplicates per snapshot
- **Indexes** optimized for drill-down queries

### Estimated Data Volume (90 days retention)

| Table | Rows per snapshot | 90 days |
|-------|-------------------|---------|
| snapshots | 1 | 90 |
| service_snapshots | ~100 | 9,000 |
| metric_snapshots | ~50,000 | 4,500,000 |
| label_snapshots | ~250,000 | 22,500,000 |

SQLite handles this fine with proper indexes.

## API Endpoints

### Overview & Scans

```
GET /api/overview
Response: {
  latest_scan: timestamp,
  total_services: int,
  total_series: int,
  series_delta_1d: int,
  series_delta_7d: int,
  series_delta_30d: int
}

GET /api/scans
Response: [{
  id: int,
  collected_at: timestamp,
  total_services: int,
  total_series: int,
  duration_ms: int
}]

POST /api/scan
Response: { status: "started" }

GET /api/scan/status
Response: {
  running: bool,
  progress: string,
  last_scan_at: timestamp,
  last_duration: string
}
```

### Services (Level 1)

```
GET /api/services
Query params: ?sort=series|growth_1d|growth_7d|name&order=asc|desc
Response: [{
  name: string,
  total_series: int,
  metric_count: int,
  delta_1d: int,
  delta_7d: int,
  delta_30d: int,
  growth_pct_7d: float
}]

GET /api/services/{name}
Response: {
  name: string,
  total_series: int,
  metric_count: int,
  delta_1d: int,
  delta_7d: int,
  delta_30d: int,
  trend: [{ date: string, series: int }]  // 30 days
}
```

### Metrics (Level 2)

```
GET /api/services/{service}/metrics
Query params: ?sort=series|growth_1d|name&order=asc|desc
Response: [{
  name: string,
  series_count: int,
  label_count: int,
  delta_1d: int,
  delta_7d: int,
  delta_30d: int
}]

GET /api/services/{service}/metrics/{metric}
Response: {
  name: string,
  series_count: int,
  label_count: int,
  delta_1d: int,
  delta_7d: int,
  delta_30d: int,
  trend: [{ date: string, series: int }]
}
```

### Labels (Level 3)

```
GET /api/services/{service}/metrics/{metric}/labels
Response: [{
  name: string,
  unique_values: int,
  delta_1d: int,
  sample_values: [string]  // top N samples
}]
```

## UI Structure

### Page 1: Services List (Home)

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
│ order-processor      │ 156,889  │ 124     │ +0.2% │ +1%  │ +2%  │
│ ...                  │          │         │       │      │      │
└─────────────────────────────────────────────────────────────────┘
```

### Page 2: Service Detail (Metrics List)

```
┌─────────────────────────────────────────────────────────────────┐
│ ← Services / payment-gateway                                    │
├─────────────────────────────────────────────────────────────────┤
│ 245,231 series │ 89 metrics │ +45% (30d)                       │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │ [Simple 30-day trend chart]                                 │ │
│ └─────────────────────────────────────────────────────────────┘ │
├─────────────────────────────────────────────────────────────────┤
│ Metric                        │ Series  │ Labels │ Δ1d  │ Δ7d  │
│───────────────────────────────│─────────│────────│──────│──────│
│ http_request_duration_seconds │ 45,231  │ 8      │ +1%  │ +5%  │
│ http_requests_total           │ 38,992  │ 6      │ +2%  │ +8%  │
│ grpc_server_handled_total     │ 28,445  │ 7      │ 0%   │ +1%  │
│ ...                           │         │        │      │      │
└─────────────────────────────────────────────────────────────────┘
```

### Page 3: Metric Detail (Labels Breakdown)

```
┌─────────────────────────────────────────────────────────────────┐
│ ← payment-gateway / http_request_duration_seconds               │
├─────────────────────────────────────────────────────────────────┤
│ 45,231 series │ 8 labels │ +5% (7d)                            │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │ [Simple 30-day trend chart]                                 │ │
│ └─────────────────────────────────────────────────────────────┘ │
├─────────────────────────────────────────────────────────────────┤
│ Label          │ Unique Values │ Δ1d  │ Sample Values          │
│────────────────│───────────────│──────│────────────────────────│
│ endpoint       │ 1,234         │ +12  │ /api/v1/pay, /api/v1/… │
│ status_code    │ 8             │ 0    │ 200, 201, 400, 404, …  │
│ method         │ 4             │ 0    │ GET, POST, PUT, DELETE │
│ instance       │ 45            │ +2   │ pod-abc123, pod-def456 │
│ ...            │               │      │                        │
└─────────────────────────────────────────────────────────────────┘
```

## Scanning Strategy

### Discovery Flow

```
1. Discover services
   Query: count({service_label}!="") by ({service_label})
   Result: [{service: "payment-gateway", count: 245231}, ...]

2. For each service (parallel, rate-limited):
   Query: count({service_label}="X") by (__name__)
   Result: [{__name__: "http_requests_total", count: 38992}, ...]

3. For each metric (parallel, rate-limited):
   Query: count({service_label}="X", __name__="Y"}) by (label_name)
   Note: Need to query label names first, then count each

4. For high-cardinality labels (>100 values):
   Query: topk(100, count({...}) by (label_name))
   Store sample values for debugging
```

### Rate Limiting

```yaml
scan:
  service_concurrency: 5      # parallel service scans
  metric_concurrency: 10      # parallel metric scans per service
  request_delay_ms: 50        # delay between requests
  timeout_per_service: 60s    # max time per service
```

### Estimated Scan Time

- 100 services × ~500 metrics = 50,000 queries
- At 50ms delay = ~42 minutes
- With parallelization (5 services × 10 metrics) = ~8 minutes

## Configuration

```yaml
# config.yaml
prometheus:
  url: http://localhost:9090
  # For VictoriaMetrics, use the same URL

discovery:
  service_label: app           # Single label that identifies services

scan:
  schedule: "0 2 * * *"        # Cron format
  concurrency: 5               # Parallel service scans
  timeout: 30m                 # Max scan duration
  sample_values_limit: 10      # Max sample values to store per label

storage:
  path: ./data/metrics-audit.db
  retention_days: 90

server:
  port: 8080
  host: 0.0.0.0
```

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Service label | Single configurable | Simpler, one source of truth |
| Sample values | 10 max (configurable) | Enough for debugging, not too much data |
| Schedule format | Cron | Flexible, familiar to ops |
| Delta display | Percentage only | Cleaner UI, easier to compare |
| Comparison windows | 1d, 7d, 30d | Standard audit periods |

## Use Cases

### 1. Weekly Audit
> "Show me the top 10 services by cardinality growth this week"

- Go to Services list
- Sort by Δ7d descending
- Review top offenders

### 2. Incident Investigation
> "Our Prometheus is slow since yesterday. What changed?"

- Go to Services list
- Sort by Δ1d descending
- Find services with abnormal growth
- Drill down to see which metrics/labels exploded

### 3. Capacity Planning
> "payment-gateway grew 45% in the last month"

- Click on payment-gateway
- View 30-day trend chart
- Drill into metrics to find growth drivers
- Check labels for high-cardinality culprits (e.g., user_id in labels)

### 4. Team Accountability
> "Your service is using 10% of our total cardinality budget"

- Services list shows relative contribution
- Share link to service detail page
- Historical data proves growth pattern

## Out of Scope (MVP)

- ❌ Real-time alerts/monitoring
- ❌ Cost calculations
- ❌ Recommendations engine
- ❌ Grafana integration
- ❌ Team/owner attribution
- ❌ CLI interface
- ❌ Anomaly detection
- ❌ Multiple Prometheus sources
- ❌ Authentication/authorization

## Future Enhancements (v2+)

- Slack/email notifications for abnormal growth
- Team attribution via config mapping
- Compare arbitrary snapshots
- Export to CSV/JSON
- Recording rule suggestions
- Multi-cluster support

## Tech Stack

- **Backend**: Go (keep existing)
- **Frontend**: React + Tailwind (keep existing)
- **Database**: SQLite (keep existing)
- **Prometheus queries**: PromQL via HTTP API