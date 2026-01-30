# Observability Cost Optimizer - MVP Requirements

## Project Overview
**Type:** Open source tool for observability cost optimization  
**Goal:** Tool for analyzing and optimizing Prometheus/VictoriaMetrics/Grafana costs  
**First Use Case:** Dogfooding on production environment

### Core Idea
Create a simple single-binary tool that:
- Scans Prometheus/VictoriaMetrics metrics
- Analyzes Grafana usage (which dashboards/metrics are actually used)
- Calculates real costs in USD
- Generates optimization recommendations
- Shows trends and team breakdown

### Target Audience (future)
- Mid-size tech companies (50-200 devs)
- Self-hosted observability stacks
- Teams without dedicated FinOps
- $3k-15k/month observability spend

---

## Technology Stack

### Backend
- **Language:** Go
- **Database:** SQLite (embedded)
- **Why Go:** Single binary, cross-platform, easy deployment
- **Scheduler:** Built-in Go scheduler for periodic scans

### Frontend
- **Framework:** React + TypeScript
- **Styling:** Tailwind CSS
- **Charts:** Recharts or Chart.js
- **Build:** Embedded in Go binary (Go 1.16+ embed)

### Deployment
- Single binary executable
- Docker image (optional)
- No external dependencies (embedded SQLite)

---

## Functional Requirements

### 1. Data Collection

#### Prometheus Integration
**Required functionality:**
- Connect to Prometheus API (`http://prometheus-url:9090`)
- Query all metrics: `{__name__=~".+"}`
- For each metric calculate:
    - **Cardinality:** number of unique time series  
      Query: `count({__name__="metric_name"})`
    - **Sample count:** how many samples per day
    - **Label combinations:** which labels create cardinality
    - **Retention period:** from config or default 30 days

**Team attribution:**
- Distribute metrics by teams via regex patterns
- Example:
  ```yaml
  teams:
    backend-core:
      - "jvm_.*"
      - "http_server_.*"
    integrations:
      - "integration_.*"
      - "payment_.*"
  ```

**Scheduled Collection:**
- Configurable interval (default: 24h)
- Background job without blocking
- Store only aggregated data (not raw samples)

#### VictoriaMetrics Support
- Optional for MVP (same API as Prometheus)
- Can add later if needed

#### Grafana Integration
**Required functionality:**
- Connect to Grafana API
- List all dashboards (`/api/search`)
- For each dashboard:
    - Extract queries from panels
    - Determine which metrics are used
    - Get `last_viewed_at` timestamp
- Identify unused dashboards (not viewed for >90 days)

**Authentication:**
- API token (Grafana service account)
- Basic auth (optional)

---

### 2. Analysis Engine

#### Cost Calculator

**Formula:**
```
cost_per_metric = cardinality × samples_per_day × retention_days × bytes_per_sample × storage_cost_per_gb
```

**Parameters (configurable):**
- `bytes_per_sample`: 2 bytes (Prometheus default)
- `storage_cost_per_gb`: $0.10 USD (configurable)
- `retention_days`: 30 days (default)
- `samples_per_day`: based on scrape_interval (default 15s = 5760 samples/day)

**Team breakdown:**
- Calculate cost per team
- Percentage of total
- Trend (comparison with last week)

#### Metrics Classification

**Categories:**

1. **High-cardinality metrics**
    - Threshold: >10,000 time series
    - Impact: high storage cost

2. **Unused metrics**
    - Criteria: 0 queries in last 30 days
    - Not used in dashboards
    - Not used in alerts/recording rules

3. **Over-retention metrics**
    - Retention period > query window
    - Example: retention 90d but queries only last 7d

4. **Redundant labels**
    - Labels with many unique values
    - Can be aggregated or dropped

#### Recommendations Generator

**Priority HIGH:**
- Metric has cardinality >10K AND usage <10 queries/day
- Dashboard not opened for >90 days
- Metric not used at all

**Priority MEDIUM:**
- Retention 90d but queries only for 7d
- Labels with 100+ unique values
- Duplicate or similar metrics

**Priority LOW:**
- Aggregation opportunities
- Optimization suggestions

**Recommendation format:**
```json
{
  "metric_name": "http_server_requests_bucket",
  "type": "high_cardinality",
  "priority": "high",
  "current_cost_monthly": 450.00,
  "potential_savings": 200.00,
  "description": "Metric has 45,000 time series. Recommend removing label 'user_id'",
  "action": "Add relabel config to drop label 'user_id'"
}
```

---

### 3. Data Storage (SQLite)

#### Database Schema

**metric_snapshots** - daily snapshot per metric (aggregated)
```sql
CREATE TABLE metric_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    collected_at TIMESTAMP NOT NULL,
    metric_name TEXT NOT NULL,
    cardinality INTEGER NOT NULL,
    estimated_size_mb REAL NOT NULL,
    sample_count INTEGER,
    team TEXT,
    UNIQUE(metric_name, collected_at)
);

CREATE INDEX idx_snapshots_date ON metric_snapshots(collected_at);
CREATE INDEX idx_snapshots_metric ON metric_snapshots(metric_name);
```

**recommendations** - generated recommendations
```sql
CREATE TABLE recommendations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP NOT NULL,
    metric_name TEXT NOT NULL,
    type TEXT NOT NULL, -- 'high_cardinality', 'unused', 'retention'
    priority TEXT NOT NULL, -- 'high', 'medium', 'low'
    potential_savings_usd REAL,
    current_cost_usd REAL,
    description TEXT,
    suggested_action TEXT
);

CREATE INDEX idx_recommendations_priority ON recommendations(priority);
```

**dashboard_stats** - Grafana dashboard usage
```sql
CREATE TABLE dashboard_stats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    collected_at TIMESTAMP NOT NULL,
    dashboard_uid TEXT NOT NULL,
    dashboard_name TEXT NOT NULL,
    last_viewed_at TIMESTAMP,
    query_count INTEGER,
    metrics_used TEXT, -- JSON array
    UNIQUE(dashboard_uid, collected_at)
);
```

**cost_snapshots** - total costs history
```sql
CREATE TABLE cost_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    collected_at TIMESTAMP NOT NULL,
    total_metrics INTEGER,
    total_cardinality BIGINT,
    total_cost_usd REAL,
    team_breakdown TEXT -- JSON: {"backend-core": 1200.50, "integrations": 800.30}
);
```

#### Data Retention
- SQLite stores data for last 90 days (configurable)
- Auto-cleanup of old records
- Periodic database compaction

---

### 4. REST API

#### Endpoints

**GET /api/overview**
```json
{
  "total_metrics": 1247,
  "total_cardinality": 127000,
  "total_cost_monthly": 2340.50,
  "trend_percentage": 15.2,
  "last_scan": "2026-01-30T10:00:00Z",
  "team_breakdown": {
    "backend-core": 1400.30,
    "integrations": 940.20
  }
}
```

**GET /api/metrics**  
Query params: `?sort=cost&limit=20&team=backend-core`
```json
[
  {
    "name": "http_server_requests_bucket",
    "cardinality": 45000,
    "cost_monthly": 450.00,
    "team": "backend-core",
    "trend": 12.5
  }
]
```

**GET /api/recommendations**  
Query params: `?priority=high`
```json
[
  {
    "id": 1,
    "metric_name": "http_server_requests_bucket",
    "type": "high_cardinality",
    "priority": "high",
    "potential_savings": 200.00,
    "current_cost_monthly": 450.00,
    "description": "Metric has 45,000 time series due to high-cardinality label 'user_id'",
    "suggested_action": "Add metric_relabel_configs to drop label 'user_id'"
  }
]
```

**GET /api/trends**  
Query params: `?period=30d`
```json
{
  "data_points": [
    {
      "date": "2026-01-01",
      "total_cost": 2100.00,
      "total_metrics": 1150
    },
    {
      "date": "2026-01-30",
      "total_cost": 2340.50,
      "total_metrics": 1247
    }
  ]
}
```

**GET /api/dashboards/unused**
```json
[
  {
    "uid": "abc123",
    "name": "Old Payment Dashboard",
    "last_viewed": "2025-09-15T10:00:00Z",
    "url": "http://grafana.url/d/abc123"
  }
]
```

**GET /health**
```json
{
  "status": "healthy",
  "prometheus_connected": true,
  "grafana_connected": true,
  "last_scan": "2026-01-30T10:00:00Z"
}
```

---

### 5. Web UI

#### Dashboard Page (/)

**Layout:**
```
┌─────────────────────────────────────────────┐
│  Overview                                   │
│  ┌──────────┐ ┌──────────┐ ┌─────────────┐ │
│  │ 1,247    │ │ 127K     │ │ $2,340/mo   │ │
│  │ Metrics  │ │ Series   │ │ Est. Cost   │ │
│  │ +15%     │ │ +12%     │ │ +8%         │ │
│  └──────────┘ └──────────┘ └─────────────┘ │
│                                             │
│  Cost Trend (30 days)                       │
│  [Line chart showing cost over time]        │
│                                             │
│  Top 10 Expensive Metrics                   │
│  ┌────────────────┬──────┬─────────┬──────┐│
│  │ Metric         │ Card │ Cost/mo │ Team ││
│  ├────────────────┼──────┼─────────┼──────┤│
│  │ http_requests* │ 45K  │ $450    │ BE   ││
│  │ jvm_memory_*   │ 12K  │ $120    │ BE   ││
│  └────────────────┴──────┴─────────┴──────┘│
│                                             │
│  Top 5 Recommendations (Quick Wins)         │
│  • Drop user_id label from http_* (save $200)│
│  • Archive 3 unused dashboards              │
└─────────────────────────────────────────────┘
```

**Components:**
- Hero metrics cards (total metrics, cardinality, cost, trends)
- Cost trend chart (last 30 days line chart)
- Top expensive metrics table (sortable)
- Quick wins recommendations list

#### Metrics Page (/metrics)

**Features:**
- Searchable table (search by metric name)
- Filterable by team
- Sortable columns: Name, Cardinality, Cost, Team, Trend
- Pagination (20 items per page)
- Click metric name → detail view modal/page

**Table columns:**
- Metric Name
- Cardinality
- Cost/month ($)
- Team
- Trend (% change from last week)

#### Recommendations Page (/recommendations)

**Features:**
- Tabs for priority: High / Medium / Low
- Card-based layout
- Each card shows:
    - Metric name (clickable to metrics page)
    - Issue type badge (high-cardinality, unused, etc)
    - Current monthly cost
    - Potential savings (highlighted)
    - Description
    - Suggested action (code snippet if applicable)

**Note:** For MVP, no status tracking (acknowledged/ignored). Just display recommendations.

#### Dashboards Page (/dashboards)

**Features:**
- List of unused Grafana dashboards
- Table with columns:
    - Dashboard name (link to Grafana)
    - Last viewed date
    - Days since last view
    - Metrics count (how many metrics used)

---

### 6. Configuration

**Config file format:** YAML

**Example config.yaml:**
```yaml
prometheus:
  url: http://localhost:9090
  # Optional basic auth
  username: ""
  password: ""

grafana:
  url: http://localhost:3000
  api_token: "glsa_xxxxxxxxxxxxx"

collection:
  interval: 24h  # How often to scan
  retention: 90d # How long to keep history in SQLite

cost_model:
  storage_cost_per_gb: 0.10  # USD per GB per month
  bytes_per_sample: 2         # Prometheus default
  default_retention_days: 30
  scrape_interval: 15s        # Used to calculate samples/day
  
teams:
  backend-core:
    metrics_patterns:
      - "jvm_.*"
      - "http_server_.*"
      - "spring_.*"
  integrations:
    metrics_patterns:
      - "integration_.*"
      - "payment_.*"
      - "provider_.*"

# Thresholds for recommendations
recommendations:
  high_cardinality_threshold: 10000
  unused_days_threshold: 30
  min_potential_savings: 50.0  # Don't show recommendations under $50/month

server:
  port: 8080
  host: 0.0.0.0
  
# Optional: Authentication (can be added post-MVP)
# auth:
#   enabled: false
#   username: admin
#   password: secret
```

---

### 7. CLI Interface

**Commands:**

```bash
# Initialize config file
metriccost init
# Creates config.yaml with defaults

# One-time scan
metriccost scan
# Scans Prometheus/Grafana and stores results

# Print report to console
metriccost report --format=table
metriccost report --format=json > report.json
metriccost report --top=20 --sort=cost

# Show specific metric details
metriccost metric http_server_requests_total
# Shows: cardinality, cost, labels, trend

# Start web server (continuous mode)
metriccost serve --config=config.yaml --port=8080

# Export data
metriccost export --format=csv --output=metrics.csv

# Database management
metriccost db cleanup --older-than=90d
metriccost db stats
```

**CLI Output Examples:**

```
$ metriccost report --top=10

Observability Cost Report
Generated: 2026-01-30 10:00:00

Overview:
  Total Metrics:     1,247
  Total Cardinality: 127,000
  Monthly Cost:      $2,340.50
  Trend:            ↑ 15.2%

Top 10 Expensive Metrics:
┌────────────────────────────┬──────────┬───────────┬──────────────┐
│ Metric                     │ Card     │ Cost/mo   │ Team         │
├────────────────────────────┼──────────┼───────────┼──────────────┤
│ http_server_requests_*     │ 45,000   │ $450.00   │ backend-core │
│ jvm_memory_used_bytes      │ 12,000   │ $120.00   │ backend-core │
│ integration_api_calls_*    │ 8,500    │ $85.00    │ integrations │
└────────────────────────────┴──────────┴───────────┴──────────────┘

High Priority Recommendations (3):
  1. Drop 'user_id' label from http_server_requests_* - Save $200/mo
  2. Reduce retention for old_metrics_* from 90d to 7d - Save $150/mo
  3. Archive 5 unused dashboards - Cleanup
```

---

## Non-Functional Requirements

### Performance
- Scan 100K+ time series in <5 minutes
- UI page load <1 second
- API response time <500ms for most queries
- Memory usage <200MB during normal operation
- Binary size <50MB

### Reliability
- Graceful handling of Prometheus/Grafana API failures
- Retry logic with exponential backoff
- Health check endpoint for monitoring
- Detailed error logging
- Continue operation if one data source fails

### Security
- **Read-only access** to Prometheus and Grafana (no write operations)
- API token/password stored in config file (file permissions 600)
- Optional: Basic auth for web UI (post-MVP)
- No sensitive data in logs

### Maintainability
- Clean, idiomatic Go code
- Unit tests for core business logic (cost calculation, recommendations)
- Clear code structure:
  ```
  /cmd          - CLI commands
  /internal     - Business logic
  /pkg          - Reusable packages
  /web          - React frontend
  /configs      - Example configs
  ```
- Godoc comments for exported functions
- README with setup instructions

### Usability
- Clear error messages
- Sensible defaults (can run with minimal config)
- Example config file included
- Simple deployment (just copy binary)

---

## Out of Scope for MVP

**Do NOT implement in MVP:**
- ❌ Multi-cluster support
- ❌ Recommendation status tracking (acknowledge/apply/ignore)
- ❌ Alerting on cost spikes
- ❌ PostgreSQL support
- ❌ User authentication & RBAC
- ❌ API for automation/CI/CD
- ❌ Slack/Teams notifications
- ❌ Thanos/Mimir/Cortex support
- ❌ Historical data >90 days
- ❌ Budget tracking & forecasting
- ❌ Auto-apply recommendations
- ❌ Export to external systems (S3, etc)
- ❌ Multi-language support
- ❌ Mobile-responsive UI (desktop-first is fine)

**Can be added post-MVP if needed**

---

## Success Criteria

### Technical
- ✅ Successfully scans 100K+ time series
- ✅ UI loads and renders charts correctly
- ✅ All API endpoints return correct data
- ✅ SQLite stores and retrieves data correctly
- ✅ Binary runs on Linux/macOS/Windows

### Functional
- ✅ Identifies top 20 most expensive metrics
- ✅ Detects unused metrics (not queried in 30 days)
- ✅ Generates actionable recommendations
- ✅ Shows cost in USD with configurable rates
- ✅ Team breakdown working (Backend Core vs Integrations)
- ✅ 30-day historical trends displayed

### Business
- ✅ Finds at least 3-5 quick wins
- ✅ Estimates realistic potential savings
- ✅ Used for 2+ weeks without major issues
- ✅ Team finds it useful

---

## Development Phases

### Week 1-2: Core Backend
**Tasks:**
- [ ] Go project setup (modules, structure)
- [ ] Prometheus API client implementation
- [ ] Basic metrics collection (list all metrics)
- [ ] Cardinality calculation per metric
- [ ] SQLite schema creation & migrations
- [ ] Cost estimation logic
- [ ] CLI: `metriccost scan` command

**Deliverable:** Can scan Prometheus and store results in SQLite

### Week 3-4: Analysis Engine + API
**Tasks:**
- [ ] Recommendations engine implementation
- [ ] Team attribution logic (regex matching)
- [ ] Unused metrics detection
- [ ] High-cardinality detection
- [ ] Historical trends calculation
- [ ] REST API endpoints (/api/overview, /api/metrics, etc)
- [ ] CLI: `metriccost report` command
- [ ] Scheduled collection job (background)

**Deliverable:** Complete backend with API

### Week 5-6: Web UI
**Tasks:**
- [ ] React project setup (Vite or Create React App)
- [ ] Tailwind CSS configuration
- [ ] Dashboard page (hero cards + chart)
- [ ] Metrics table page (sortable, filterable)
- [ ] Recommendations page (priority tabs + cards)
- [ ] Dashboards page (unused dashboards list)
- [ ] API integration (fetch data from backend)
- [ ] Basic charts (Recharts integration)

**Deliverable:** Working web UI

### Week 7-8: Grafana Integration + Polish
**Tasks:**
- [ ] Grafana API client
- [ ] Dashboard usage tracking
- [ ] Unused dashboard detection
- [ ] Embed React build into Go binary
- [ ] Configuration file support (YAML parsing)
- [ ] Docker image creation
- [ ] README & documentation
- [ ] Basic testing
- [ ] Deploy to internal infrastructure

**Deliverable:** Complete MVP ready for dogfooding

### Week 9-10: Dogfooding & Iteration
**Tasks:**
- [ ] Deploy on production
- [ ] Test with real data (10-15M transactions/month)
- [ ] Collect feedback from Backend Core team
- [ ] Bug fixes
- [ ] Performance tuning
- [ ] UX improvements based on feedback

**Deliverable:** Stable version ready for open source release

---

## Deployment Architecture

### Internal Deployment

**Infrastructure:**
- Docker container on internal infrastructure
- **Read-only** access to Prometheus: `http://prometheus.internal:9090`
- **Read-only** access to Grafana: `http://grafana.internal:3000`
- SQLite database mounted on persistent volume

**Access:**
- Available to Backend Core team
- Available to Integrations team
- Optional: DevOps team

**Schedule:**
- Weekly scans (can increase if needed)
- Manual scans on-demand via CLI

**Monitoring:**
- Health check endpoint monitored
- Resource usage tracked
- Errors logged to centralized logging

### Open Source Release (post-dogfooding)

**When:** After 2-4 weeks of successful dogfooding

**Steps:**
1. Clean up code, remove specific configs
2. MIT License
3. GitHub repository with:
    - Good README with screenshots
    - Installation instructions
    - Example config files
    - Contributing guidelines
4. Docker Hub image: `metriccost/metriccost:latest`
5. GitHub Releases with binaries for:
    - Linux (amd64, arm64)
    - macOS (amd64, arm64)
    - Windows (amd64)
6. Announcement:
    - Prometheus community Slack
    - VictoriaMetrics Slack/GitHub
    - Reddit r/devops, r/prometheus
    - Hacker News (maybe)

---

## Testing Strategy

### Unit Tests
**Priority:** Medium (basic coverage)
- Cost calculation logic
- Team attribution regex matching
- Recommendations priority scoring
- Cardinality calculation

### Integration Tests
**Priority:** Low for MVP
- Prometheus API client (can use test Prometheus instance)
- SQLite operations

### Manual Testing
**Priority:** High
- Full end-to-end workflow on production
- UI usability testing
- Different screen sizes (desktop only for MVP)

### Performance Testing
- Test with 100K+ metrics
- Check memory usage under load
- API response times

---

## Documentation Requirements

### README.md
**Must include:**
- Project description & goals
- Quick start guide (5 minutes to running)
- Installation instructions
- Configuration file explanation
- Screenshots of UI
- Example output
- Architecture diagram (optional but nice)
- Contributing guidelines
- License

### Example:
```markdown
# metriccost - Observability Cost Optimizer

Simple tool to analyze and optimize your Prometheus/VictoriaMetrics costs.

## Quick Start

1. Download binary
2. Create config.yaml
3. Run: `metriccost serve`
4. Open http://localhost:8080

## Features
- 💰 Cost estimation in USD
- 📊 Historical trends
- 🎯 Actionable recommendations
- 👥 Team attribution
- 🚀 Single binary deployment
```

### Config File Documentation
- Inline comments in example config.yaml
- Explanation of each parameter
- Recommended values

### API Documentation
- OpenAPI/Swagger spec (nice to have)
- Or simple markdown with examples

---

## Risk Mitigation

### Risk 1: Go learning curve
**Probability:** Medium  
**Impact:** High  
**Mitigation:**
- Start with simple features
- Use existing Go libraries (Prometheus client, etc)
- Reference: https://github.com/prometheus/client_golang
- Can always rewrite in Java later if Go doesn't work out

### Risk 2: Embedded frontend complexity
**Probability:** Low  
**Impact:** Medium  
**Mitigation:**
- Go 1.16+ has built-in embed support
- Lots of examples available
- Fallback: Serve frontend separately

### Risk 3: Prometheus API rate limiting
**Probability:** Low  
**Impact:** Low  
**Mitigation:**
- Respect rate limits
- Cache results
- Scheduled scans (not continuous polling)

### Risk 4: Time commitment
**Probability:** Medium  
**Impact:** Medium  
**Mitigation:**
- Clear MVP scope (no feature creep)
- 2-3 hours evenings/weekends sustainable
- Can extend timeline if needed (not a hard deadline)

### Risk 5: Data accuracy
**Probability:** Medium  
**Impact:** Medium  
**Mitigation:**
- Compare results with actual Prometheus storage
- Manual validation of top expensive metrics
- Configurable cost model (easy to adjust)

---

## Future Enhancements (Post-MVP)

**v0.2 (Month 3-4):**
- VictoriaMetrics native support
- Thanos/Cortex/Mimir support
- Alerting on cost spikes
- Recommendation status tracking

**v0.3 (Month 5-6):**
- Multi-cluster support
- PostgreSQL backend option
- Budget tracking
- Team RBAC

**v1.0 (Month 7-12):**
- API for automation
- Slack/Teams integrations
- Auto-apply recommendations
- Enterprise features (SSO, etc)

---

## Notes for Implementation

### Prometheus Client Libraries
- Go: `github.com/prometheus/client_golang`
- API v1: `github.com/prometheus/client_golang/api/prometheus/v1`

### Example Prometheus Queries
```promql
# Get all metric names
{__name__=~".+"}

# Count time series for specific metric
count({__name__="http_server_requests_total"})

# Top metrics by cardinality
topk(20, count by (__name__)({__name__=~".+"}))
```

### Grafana API Endpoints
```
GET /api/search?type=dash-db
GET /api/dashboards/uid/:uid
GET /api/dashboards/:id/permissions
```

### SQLite in Go
- Library: `github.com/mattn/go-sqlite3` or `modernc.org/sqlite` (pure Go)
- ORM option: `gorm.io/gorm` with SQLite driver

### React Embedding
```go
//go:embed web/dist
var webFS embed.FS

http.Handle("/", http.FileServer(http.FS(webFS)))
```

### Cost Calculation Example
```
Metric: http_server_requests_bucket
Cardinality: 45,000 time series
Scrape interval: 15s → 5,760 samples/day per series
Total samples/day: 45,000 × 5,760 = 259,200,000
Storage per day: 259,200,000 × 2 bytes = 518.4 MB
Storage per month (30d): 15,552 MB ≈ 15.2 GB
Cost per month: 15.2 GB × $0.10/GB = $1.52

But wait - with retention 30d:
Total storage needed: 15.2 GB
Cost: 15.2 × $0.10 = $1.52/month for this metric
```

---

## Glossary

**Cardinality:** Number of unique time series for a metric (unique combinations of label values)

**Time Series:** A metric with a unique set of labels. Example: `http_requests_total{method="GET", endpoint="/api/payment"}` is one time series.

**Churn Rate:** How frequently new time series are created (e.g., when pods are recreated in Kubernetes)

**Scrape Interval:** How often Prometheus collects metrics from targets (default: 15s)

**Retention:** How long Prometheus/VictoriaMetrics stores data before deleting old data

**DPM:** Data Points per Minute - how many samples are collected per minute

**TSDB:** Time Series Database - how Prometheus stores data

---

## Final Checklist Before Starting Development

- [ ] Confirm Go is installed and working
- [ ] Confirm Node.js/npm for React development
- [ ] Access to Prometheus (read-only)
- [ ] Access to Grafana (read-only API token)
- [ ] SQLite3 CLI installed for debugging
- [ ] Code editor setup (VSCode recommended)
- [ ] Git repository created
- [ ] Initial project structure decided

---

## Contact & Feedback

This MVP is designed to be iterated on based on real usage. Feedback from teams during dogfooding will shape future development.

**Remember:** Perfect is the enemy of good. Ship MVP, gather feedback, iterate.