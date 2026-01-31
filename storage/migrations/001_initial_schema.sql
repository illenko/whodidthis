-- metric_snapshots: daily snapshot per metric
CREATE TABLE IF NOT EXISTS metric_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    collected_at TIMESTAMP NOT NULL,
    metric_name TEXT NOT NULL,
    cardinality INTEGER NOT NULL,
    sample_count INTEGER,
    team TEXT,
    labels_json TEXT,
    UNIQUE(metric_name, collected_at)
);

CREATE INDEX IF NOT EXISTS idx_metric_snapshots_date ON metric_snapshots(collected_at);
CREATE INDEX IF NOT EXISTS idx_metric_snapshots_metric ON metric_snapshots(metric_name);
CREATE INDEX IF NOT EXISTS idx_metric_snapshots_team ON metric_snapshots(team);

-- recommendations: optimization recommendations
CREATE TABLE IF NOT EXISTS recommendations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP NOT NULL,
    metric_name TEXT NOT NULL,
    type TEXT NOT NULL,
    priority TEXT NOT NULL,
    current_cardinality INTEGER,
    potential_reduction INTEGER,
    reduction_percentage REAL,
    description TEXT,
    suggested_action TEXT
);

CREATE INDEX IF NOT EXISTS idx_recommendations_priority ON recommendations(priority);
CREATE INDEX IF NOT EXISTS idx_recommendations_type ON recommendations(type);

-- dashboard_stats: Grafana dashboard usage tracking
CREATE TABLE IF NOT EXISTS dashboard_stats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    collected_at TIMESTAMP NOT NULL,
    dashboard_uid TEXT NOT NULL,
    dashboard_name TEXT NOT NULL,
    folder_name TEXT,
    last_viewed_at TIMESTAMP,
    query_count INTEGER,
    metrics_used TEXT,
    UNIQUE(dashboard_uid, collected_at)
);

CREATE INDEX IF NOT EXISTS idx_dashboard_stats_uid ON dashboard_stats(dashboard_uid);
CREATE INDEX IF NOT EXISTS idx_dashboard_stats_date ON dashboard_stats(collected_at);

-- snapshots: overall system snapshots
CREATE TABLE IF NOT EXISTS snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    collected_at TIMESTAMP NOT NULL UNIQUE,
    total_metrics INTEGER NOT NULL,
    total_cardinality INTEGER NOT NULL,
    team_breakdown TEXT
);

CREATE INDEX IF NOT EXISTS idx_snapshots_date ON snapshots(collected_at);
