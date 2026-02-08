# whodidthis

Prometheus metrics cardinality auditor. Scans your Prometheus-compatible endpoint, discovers all services and their metrics, and tracks series cardinality over time — so you can find out who blew up your TSDB.

## Features

- **Service discovery** — automatically discovers services via a configurable label (e.g. `job`)
- **Cardinality scanning** — collects per-metric series counts, label counts, and sample label values
- **Snapshot history** — stores scan results in SQLite, tracks cardinality changes over time
- **AI-powered analysis** — compares snapshots using Gemini to explain what changed and why (optional)
- **Scheduled scans** — runs scans on a configurable interval with manual trigger support
- **Built-in web UI** — React dashboard with drill-down from services to metrics to labels
- **Single binary** — frontend is embedded in the Go binary, no separate web server needed

## Roadmap

- [ ] Top cardinality offenders view — landing page with top metrics by series count and growth rate
- [ ] Cardinality diff without AI — structured comparison table between any two snapshots
- [ ] Label explosion detection — auto-flag labels with high unique value counts (e.g. `user_id`, `trace_id`)
- [ ] Cardinality trend charts — sparklines showing series count over time per service/metric
- [ ] Optimization suggestions — "dropping label X from metric Y would reduce cardinality by Z%"
- [ ] Webhook/Slack notifications — alert when cardinality grows by X% or exceeds a threshold