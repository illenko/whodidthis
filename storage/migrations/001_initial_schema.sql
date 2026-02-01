-- Snapshot metadata (one per daily scan)
CREATE TABLE IF NOT EXISTS snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    collected_at TIMESTAMP NOT NULL UNIQUE,
    scan_duration_ms INTEGER,
    total_services INTEGER NOT NULL DEFAULT 0,
    total_series INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_snapshots_time ON snapshots(collected_at DESC);

-- Service level (top of hierarchy)
CREATE TABLE IF NOT EXISTS service_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    snapshot_id INTEGER NOT NULL REFERENCES snapshots(id) ON DELETE CASCADE,
    service_name TEXT NOT NULL,
    total_series INTEGER NOT NULL DEFAULT 0,
    metric_count INTEGER NOT NULL DEFAULT 0,
    UNIQUE(snapshot_id, service_name)
);
CREATE INDEX IF NOT EXISTS idx_service_snapshots_lookup ON service_snapshots(snapshot_id, service_name);
CREATE INDEX IF NOT EXISTS idx_service_snapshots_name ON service_snapshots(service_name);

-- Metric level (per service)
CREATE TABLE IF NOT EXISTS metric_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    service_snapshot_id INTEGER NOT NULL REFERENCES service_snapshots(id) ON DELETE CASCADE,
    metric_name TEXT NOT NULL,
    series_count INTEGER NOT NULL DEFAULT 0,
    label_count INTEGER NOT NULL DEFAULT 0,
    UNIQUE(service_snapshot_id, metric_name)
);
CREATE INDEX IF NOT EXISTS idx_metric_snapshots_lookup ON metric_snapshots(service_snapshot_id);
CREATE INDEX IF NOT EXISTS idx_metric_snapshots_name ON metric_snapshots(metric_name);

-- Label level (per metric)
CREATE TABLE IF NOT EXISTS label_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    metric_snapshot_id INTEGER NOT NULL REFERENCES metric_snapshots(id) ON DELETE CASCADE,
    label_name TEXT NOT NULL,
    unique_values_count INTEGER NOT NULL DEFAULT 0,
    sample_values TEXT,
    UNIQUE(metric_snapshot_id, label_name)
);
CREATE INDEX IF NOT EXISTS idx_label_snapshots_lookup ON label_snapshots(metric_snapshot_id);