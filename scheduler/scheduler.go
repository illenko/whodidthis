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
	stopOnce  sync.Once
	status    *ScanStatus
	mu        sync.RWMutex
	scanIDSeq atomic.Int64
	logger    *slog.Logger
	parentCtx context.Context // set by Start, used for triggered scans
	scanWg    sync.WaitGroup  // tracks async triggered scans
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
	s.parentCtx = ctx
	s.logger.Info("starting scheduler", "interval", s.interval)

	// Run initial scan
	s.executeScan(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		s.mu.Lock()
		s.status.NextScanAt = time.Now().Add(s.interval)
		s.mu.Unlock()

		select {
		case <-ctx.Done():
			s.scanWg.Wait()
			s.logger.Info("scheduler stopped")
			return
		case <-s.stopCh:
			s.scanWg.Wait()
			s.logger.Info("scheduler stopped")
			return
		case <-ticker.C:
			s.executeScan(ctx)
		}
	}
}

func (s *Scheduler) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
}

func (s *Scheduler) TriggerScan() error {
	s.mu.Lock()
	if s.status.Running {
		s.mu.Unlock()
		return ErrScanAlreadyRunning
	}
	s.status.Running = true
	s.status.LastError = ""
	s.status.Progress = &ScanProgress{Phase: "starting"}
	ctx := s.parentCtx
	s.mu.Unlock()

	if ctx == nil {
		ctx = context.Background()
	}

	s.scanWg.Add(1)
	go func() {
		defer s.scanWg.Done()
		s.doScan(ctx)
	}()
	return nil
}

func (s *Scheduler) GetStatus() ScanStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return *s.status
}

// executeScan acquires the running lock and runs a scan synchronously.
func (s *Scheduler) executeScan(ctx context.Context) {
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

// doScan runs the actual scan. Caller must have already set status.Running = true.
func (s *Scheduler) doScan(ctx context.Context) {
	scanID := s.scanIDSeq.Add(1)
	start := time.Now()

	logger := s.logger.With("scan_id", scanID)
	logger.Info("starting scan")

	var result *collector.CollectResult
	var scanErr error

	defer func() {
		s.mu.Lock()
		s.status.Running = false
		s.status.Progress = nil
		s.status.LastScanAt = start
		s.status.LastDuration = time.Since(start).String()
		if scanErr != nil {
			s.status.LastError = scanErr.Error()
		} else if result != nil {
			s.status.TotalServices = result.TotalServices
			s.status.TotalSeries = result.TotalSeries
		}
		s.mu.Unlock()
	}()

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

	result, scanErr = s.collector.Collect(ctx, scanID, progress)
	if scanErr != nil {
		logger.Error("collection failed", "error", scanErr)
		return
	}

	logger.Info("scan complete",
		"services", result.TotalServices,
		"series", result.TotalSeries,
		"duration", time.Since(start),
	)

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
