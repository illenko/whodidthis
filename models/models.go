package models

import "time"

// Snapshot metadata (one per daily scan)
type Snapshot struct {
	ID             int64     `json:"id"`
	CollectedAt    time.Time `json:"collected_at"`
	ScanDurationMs int       `json:"duration_ms,omitempty"`
	TotalServices  int       `json:"total_services"`
	TotalSeries    int64     `json:"total_series"`
}

// ServiceSnapshot represents a service at a point in time
type ServiceSnapshot struct {
	ID          int64  `json:"id"`
	SnapshotID  int64  `json:"snapshot_id"`
	ServiceName string `json:"name"`
	TotalSeries int    `json:"total_series"`
	MetricCount int    `json:"metric_count"`
}

// MetricSnapshot represents a metric within a service at a point in time
type MetricSnapshot struct {
	ID                int64  `json:"id"`
	ServiceSnapshotID int64  `json:"service_snapshot_id"`
	MetricName        string `json:"name"`
	SeriesCount       int    `json:"series_count"`
	LabelCount        int    `json:"label_count"`
}

// LabelSnapshot represents a label within a metric at a point in time
type LabelSnapshot struct {
	ID                int64    `json:"id"`
	MetricSnapshotID  int64    `json:"metric_snapshot_id"`
	LabelName         string   `json:"name"`
	UniqueValuesCount int      `json:"unique_values"`
	SampleValues      []string `json:"sample_values,omitempty"`
}

// Overview response for the main dashboard
type Overview struct {
	LatestScan    time.Time `json:"latest_scan"`
	TotalServices int       `json:"total_services"`
	TotalSeries   int64     `json:"total_series"`
}

// ScanStatus represents the current scan state
type ScanStatus struct {
	Running      bool      `json:"running"`
	Progress     string    `json:"progress,omitempty"`
	LastScanAt   time.Time `json:"last_scan_at,omitempty"`
	LastDuration string    `json:"last_duration,omitempty"`
}

// HealthStatus for health check endpoint
type HealthStatus struct {
	Status              string    `json:"status"`
	PrometheusConnected bool      `json:"prometheus_connected"`
	DatabaseOK          bool      `json:"database_ok"`
	LastScan            time.Time `json:"last_scan,omitempty"`
}
