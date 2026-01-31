package collector

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/illenko/metriccost/analyzer"
	"github.com/illenko/metriccost/models"
	"github.com/illenko/metriccost/prometheus"
	"github.com/illenko/metriccost/storage"
)

type PrometheusCollector struct {
	client      *prometheus.Client
	metricsRepo *storage.MetricsRepository
	snapRepo    *storage.SnapshotsRepository
	teamMatcher *analyzer.TeamMatcher

	batchSize           int
	concurrency         int
	labelFetchThreshold int
}

type CollectorConfig struct {
	BatchSize           int
	Concurrency         int
	LabelFetchThreshold int
}

func NewPrometheusCollector(
	client *prometheus.Client,
	metricsRepo *storage.MetricsRepository,
	snapRepo *storage.SnapshotsRepository,
	teamMatcher *analyzer.TeamMatcher,
	cfg CollectorConfig,
) *PrometheusCollector {
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 100
	}
	if cfg.Concurrency == 0 {
		cfg.Concurrency = 5
	}
	if cfg.LabelFetchThreshold == 0 {
		cfg.LabelFetchThreshold = 10000
	}

	return &PrometheusCollector{
		client:              client,
		metricsRepo:         metricsRepo,
		snapRepo:            snapRepo,
		teamMatcher:         teamMatcher,
		batchSize:           cfg.BatchSize,
		concurrency:         cfg.Concurrency,
		labelFetchThreshold: cfg.LabelFetchThreshold,
	}
}

type CollectResult struct {
	TotalMetrics     int
	TotalCardinality int64
	TeamBreakdown    map[string]models.TeamMetrics
	Duration         time.Duration
	Errors           []error
}

func (c *PrometheusCollector) Collect(ctx context.Context) (*CollectResult, error) {
	start := time.Now()
	collectedAt := start.Truncate(time.Second)

	slog.Info("starting prometheus metrics collection")

	names, err := c.client.GetAllMetricNames(ctx)
	if err != nil {
		return nil, err
	}

	slog.Info("found metrics", "count", len(names))

	result := &CollectResult{
		TeamBreakdown: make(map[string]models.TeamMetrics),
	}

	var batch []*models.MetricSnapshot
	var mu sync.Mutex
	var wg sync.WaitGroup

	sem := make(chan struct{}, c.concurrency)

	saveBatch := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := c.metricsRepo.SaveBatch(ctx, batch); err != nil {
			return err
		}
		batch = batch[:0]
		return nil
	}

	for i, name := range names {
		if ctx.Err() != nil {
			break
		}

		if (i+1)%100 == 0 || i+1 == len(names) {
			slog.Info("processing metrics", "progress", i+1, "total", len(names))
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(metricName string) {
			defer wg.Done()
			defer func() { <-sem }()

			m, err := c.collectMetric(ctx, metricName, collectedAt)
			if err != nil {
				slog.Debug("failed to collect metric", "name", metricName, "error", err)
				mu.Lock()
				result.Errors = append(result.Errors, err)
				mu.Unlock()
				return
			}

			mu.Lock()
			batch = append(batch, m)
			result.TotalMetrics++
			result.TotalCardinality += int64(m.Cardinality)

			team := m.Team
			if team == "" {
				team = "unassigned"
			}
			tm := result.TeamBreakdown[team]
			tm.Cardinality += int64(m.Cardinality)
			tm.MetricCount++
			result.TeamBreakdown[team] = tm

			if len(batch) >= c.batchSize {
				if err := saveBatch(); err != nil {
					slog.Error("failed to save batch", "error", err)
				}
			}
			mu.Unlock()
		}(name)
	}

	wg.Wait()

	if err := saveBatch(); err != nil {
		return nil, err
	}

	// Calculate percentages for team breakdown
	for team, tm := range result.TeamBreakdown {
		if result.TotalCardinality > 0 {
			tm.Percentage = float64(tm.Cardinality) / float64(result.TotalCardinality) * 100
		}
		result.TeamBreakdown[team] = tm
	}

	snapshot := &models.Snapshot{
		CollectedAt:      collectedAt,
		TotalMetrics:     result.TotalMetrics,
		TotalCardinality: result.TotalCardinality,
		TeamBreakdown:    result.TeamBreakdown,
	}

	if err := c.snapRepo.Save(ctx, snapshot); err != nil {
		return nil, err
	}

	result.Duration = time.Since(start)

	slog.Info("collection complete",
		"metrics", result.TotalMetrics,
		"cardinality", result.TotalCardinality,
		"duration", result.Duration,
		"errors", len(result.Errors),
	)

	return result, nil
}

func (c *PrometheusCollector) collectMetric(ctx context.Context, name string, collectedAt time.Time) (*models.MetricSnapshot, error) {
	cardinality, err := c.client.GetMetricCardinality(ctx, name)
	if err != nil {
		return nil, err
	}

	team := "unassigned"
	if c.teamMatcher != nil {
		team = c.teamMatcher.GetTeam(name)
	}

	var labels map[string]int
	if cardinality > 0 && cardinality < c.labelFetchThreshold {
		labelInfo, err := c.client.GetMetricLabels(ctx, name)
		if err == nil {
			labels = make(map[string]int)
			for _, l := range labelInfo {
				labels[l.Name] = l.UniqueCount
			}
		}
	}

	return &models.MetricSnapshot{
		CollectedAt: collectedAt,
		MetricName:  name,
		Cardinality: cardinality,
		Team:        team,
		Labels:      labels,
	}, nil
}
