package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/illenko/whodidthis/collector"
	"github.com/illenko/whodidthis/storage"
)

type Scheduler struct {
	collector *collector.Collector
	db        *storage.DB
	interval  time.Duration
	retention time.Duration
	stopCh    chan struct{}
	status    *ScanStatus
	mu        sync.RWMutex
	scanIDSeq atomic.Int64
	logger    *slog.Logger
}

type ScanProgress struct {
	Phase   string `json:"phase"`
	Current int    `json:"current"`
	Total   int    `json:"total"`
	Detail  string `json:"detail"`
}

type ScanStatus struct {
	Running       bool          `json:"running"`
	Progress      *ScanProgress `json:"progress,omitempty"`
	LastScanAt    time.Time     `json:"last_scan_at,omitempty"`
	LastDuration  string        `json:"last_duration,omitempty"`
	LastError     string        `json:"last_error,omitempty"`
	NextScanAt    time.Time     `json:"next_scan_at,omitempty"`
	TotalServices int           `json:"total_services,omitempty"`
	TotalSeries   int64         `json:"total_series,omitempty"`
}

type Config struct {
	Interval  time.Duration
	Retention time.Duration
	DB        *storage.DB
}

func New(collector *collector.Collector, cfg Config) *Scheduler {
	if cfg.Interval == 0 {
		cfg.Interval = 24 * time.Hour
	}
	if cfg.Retention == 0 {
		cfg.Retention = 90 * 24 * time.Hour // 90 days default
	}

	return &Scheduler{
		collector: collector,
		db:        cfg.DB,
		interval:  cfg.Interval,
		retention: cfg.Retention,
		stopCh:    make(chan struct{}),
		status:    &ScanStatus{},
		logger:    slog.Default(),
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	slog.Info("starting scheduler", "interval", s.interval)

	// Run initial scan
	s.runScan(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		s.mu.Lock()
		s.status.NextScanAt = time.Now().Add(s.interval)
		s.mu.Unlock()

		select {
		case <-ctx.Done():
			slog.Info("scheduler stopped")
			return
		case <-s.stopCh:
			slog.Info("scheduler stopped")
			return
		case <-ticker.C:
			s.runScan(ctx)
		}
	}
}

func (s *Scheduler) Stop() {
	close(s.stopCh)
}

func (s *Scheduler) TriggerScan(_ context.Context) error {
	s.mu.Lock()
	if s.status.Running {
		s.mu.Unlock()
		return ErrScanAlreadyRunning
	}
	s.status.Running = true
	s.status.LastError = ""
	s.status.Progress = &ScanProgress{Phase: "starting"}
	s.mu.Unlock()

	go s.runScanAlreadyLocked(context.Background())
	return nil
}

func (s *Scheduler) GetStatus() ScanStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return *s.status
}

func (s *Scheduler) runScan(ctx context.Context) {
	s.mu.Lock()
	if s.status.Running {
		s.mu.Unlock()
		return
	}
	s.status.Running = true
	s.status.LastError = ""
	s.status.Progress = &ScanProgress{Phase: "starting"}
	s.mu.Unlock()

	s.doScan(ctx)
}

func (s *Scheduler) runScanAlreadyLocked(ctx context.Context) {
	s.doScan(ctx)
}

func (s *Scheduler) doScan(ctx context.Context) {
	scanID := s.scanIDSeq.Add(1)
	start := time.Now()

	logger := s.logger.With("scan_id", scanID)
	logger.Info("starting scan")

	var scanErr error
	defer func() {
		s.mu.Lock()
		s.status.Running = false
		s.status.Progress = nil
		s.status.LastScanAt = start
		s.status.LastDuration = time.Since(start).String()
		if scanErr != nil {
			s.status.LastError = scanErr.Error()
		}
		s.mu.Unlock()
	}()

	// Progress callback
	progress := func(phase string, current, total int, detail string) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.status.Progress = &ScanProgress{
			Phase:   phase,
			Current: current,
			Total:   total,
			Detail:  detail,
		}
	}

	// Run collection
	result, err := s.collector.Collect(ctx, scanID, progress)
	if err != nil {
		logger.Error("collection failed", "error", err)
		scanErr = err
		return
	}

	s.mu.Lock()
	s.status.TotalServices = result.TotalServices
	s.status.TotalSeries = result.TotalSeries
	s.mu.Unlock()

	logger.Info("scan complete",
		"services", result.TotalServices,
		"series", result.TotalSeries,
		"duration", time.Since(start),
	)

	// Run cleanup after successful scan
	s.runCleanup(ctx, scanID)
}

func (s *Scheduler) runCleanup(ctx context.Context, scanID int64) {
	if s.db == nil || s.retention == 0 {
		return
	}

	deleted, err := s.db.Cleanup(ctx, s.retention)
	if err != nil {
		s.logger.Error("cleanup failed", "scan_id", scanID, "error", err)
		return
	}

	if deleted > 0 {
		s.logger.Info("cleanup completed", "scan_id", scanID, "deleted_snapshots", deleted)
	}
}

type scanError string

func (e scanError) Error() string { return string(e) }

const ErrScanAlreadyRunning = scanError("scan already running")
