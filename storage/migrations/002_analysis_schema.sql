-- Snapshot analysis results (AI-powered comparison between two snapshots)
CREATE TABLE IF NOT EXISTS snapshot_analyses (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    current_snapshot_id INTEGER NOT NULL,
    previous_snapshot_id INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    result TEXT,
    tool_calls TEXT,
    error TEXT,
    created_at TEXT NOT NULL,
    completed_at TEXT,
    UNIQUE(current_snapshot_id, previous_snapshot_id),
    FOREIGN KEY (current_snapshot_id) REFERENCES snapshots(id) ON DELETE CASCADE,
    FOREIGN KEY (previous_snapshot_id) REFERENCES snapshots(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_analyses_current ON snapshot_analyses(current_snapshot_id);
CREATE INDEX IF NOT EXISTS idx_analyses_previous ON snapshot_analyses(previous_snapshot_id);
CREATE INDEX IF NOT EXISTS idx_analyses_status ON snapshot_analyses(status);
