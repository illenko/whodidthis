package collector

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/illenko/whodidthis/config"
	"github.com/illenko/whodidthis/models"
	"github.com/illenko/whodidthis/prometheus"
	"github.com/illenko/whodidthis/storage"
)

type Collector struct {
	client       *prometheus.Client
	snapshots    *storage.SnapshotsRepository
	services     *storage.ServicesRepository
	metrics      *storage.MetricsRepository
	labels       *storage.LabelsRepository
	serviceLabel string
	sampleLimit  int
	logger       *slog.Logger
}

func NewCollector(
	client *prometheus.Client,
	snapshots *storage.SnapshotsRepository,
	services *storage.ServicesRepository,
	metrics *storage.MetricsRepository,
	labels *storage.LabelsRepository,
	cfg *config.Config,
) *Collector {
	return &Collector{
		client:       client,
		snapshots:    snapshots,
		services:     services,
		metrics:      metrics,
		labels:       labels,
		serviceLabel: cfg.Discovery.ServiceLabel,
		sampleLimit:  cfg.Scan.SampleValuesLimit,
		logger:       slog.Default(),
	}
}

type CollectResult struct {
	SnapshotID    int64
	TotalServices int
	TotalSeries   int64
	Duration      time.Duration
}

// ProgressCallback is called to report scan progress
type ProgressCallback func(phase string, current, total int, detail string)

func (c *Collector) Collect(ctx context.Context, scanID int64, progress ProgressCallback) (*CollectResult, error) {
	logger := c.logger.With("scan_id", scanID)
	start := time.Now()
	collectedAt := start.Truncate(time.Second)

	if progress == nil {
		progress = func(string, int, int, string) {}
	}

	logger.Info("starting service discovery", "label", c.serviceLabel)
	progress("discovering", 0, 0, "Discovering services...")

	// Step 1: Create snapshot
	snapshot := &models.Snapshot{
		CollectedAt: collectedAt,
	}
	snapshotID, err := c.snapshots.Create(ctx, snapshot)
	if err != nil {
		return nil, err
	}
	snapshot.ID = snapshotID

	// Step 2: Discover services
	serviceInfos, err := c.client.DiscoverServices(ctx, c.serviceLabel)
	if err != nil {
		return nil, err
	}

	logger.Info("discovered services", "count", len(serviceInfos))

	var totalSeries int64

	// Step 3: For each service, collect metrics and labels
	for i, svc := range serviceInfos {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		progress("scanning", i+1, len(serviceInfos), svc.Name)
		logger.Debug("scanning service", "name", svc.Name, "progress", i+1, "total", len(serviceInfos))

		serviceSnapshot, err := c.collectService(ctx, snapshotID, svc)
		if err != nil {
			logger.Error("failed to collect service", "name", svc.Name, "error", err)
			continue
		}

		totalSeries += int64(serviceSnapshot.TotalSeries)
	}

	// Step 4: Update snapshot with totals
	snapshot.TotalServices = len(serviceInfos)
	snapshot.TotalSeries = totalSeries
	snapshot.ScanDurationMs = int(time.Since(start).Milliseconds())

	if err := c.snapshots.Update(ctx, snapshot); err != nil {
		return nil, err
	}

	duration := time.Since(start)
	logger.Info("collection complete",
		"services", len(serviceInfos),
		"total_series", totalSeries,
		"duration", duration,
	)

	return &CollectResult{
		SnapshotID:    snapshotID,
		TotalServices: len(serviceInfos),
		TotalSeries:   totalSeries,
		Duration:      duration,
	}, nil
}

func (c *Collector) collectService(ctx context.Context, snapshotID int64, svc prometheus.ServiceInfo) (*models.ServiceSnapshot, error) {
	// Get metrics for this service
	metricInfos, err := c.client.GetMetricsForService(ctx, c.serviceLabel, svc.Name)
	if err != nil {
		return nil, err
	}

	c.logger.Debug("found metrics for service",
		"service", svc.Name,
		"metrics", len(metricInfos),
		"series", svc.SeriesCount,
	)

	// Create service snapshot
	serviceSnapshot := &models.ServiceSnapshot{
		SnapshotID:  snapshotID,
		ServiceName: svc.Name,
		TotalSeries: svc.SeriesCount,
		MetricCount: len(metricInfos),
	}

	serviceSnapshotID, err := c.services.Create(ctx, serviceSnapshot)
	if err != nil {
		return nil, err
	}
	serviceSnapshot.ID = serviceSnapshotID

	// Collect each metric
	for i, metric := range metricInfos {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		c.logger.Debug("collecting metric",
			"service", svc.Name,
			"metric", metric.Name,
			"progress", fmt.Sprintf("%d/%d", i+1, len(metricInfos)),
			"series", metric.SeriesCount,
		)

		if err := c.collectMetric(ctx, serviceSnapshotID, svc.Name, metric); err != nil {
			c.logger.Debug("failed to collect metric", "service", svc.Name, "metric", metric.Name, "error", err)
			continue
		}
	}

	return serviceSnapshot, nil
}

func (c *Collector) collectMetric(ctx context.Context, serviceSnapshotID int64, serviceName string, metric prometheus.MetricInfo) error {
	// Get labels for this metric
	labelInfos, err := c.client.GetLabelsForMetric(ctx, c.serviceLabel, serviceName, metric.Name, c.sampleLimit)
	if err != nil {
		// Log but don't fail - we can still store the metric without labels
		c.logger.Debug("failed to get labels", "metric", metric.Name, "error", err)
		labelInfos = nil
	} else {
		c.logger.Debug("collected labels",
			"metric", metric.Name,
			"labels", len(labelInfos),
		)
	}

	// Create metric snapshot
	metricSnapshot := &models.MetricSnapshot{
		ServiceSnapshotID: serviceSnapshotID,
		MetricName:        metric.Name,
		SeriesCount:       metric.SeriesCount,
		LabelCount:        len(labelInfos),
	}

	metricSnapshotID, err := c.metrics.Create(ctx, metricSnapshot)
	if err != nil {
		return err
	}

	if len(labelInfos) > 0 {
		labelSnapshots := make([]*models.LabelSnapshot, 0, len(labelInfos))
		for _, label := range labelInfos {
			labelSnapshots = append(labelSnapshots, &models.LabelSnapshot{
				MetricSnapshotID:  metricSnapshotID,
				LabelName:         label.Name,
				UniqueValuesCount: label.UniqueValues,
				SampleValues:      label.SampleValues,
			})
		}

		if err := c.labels.CreateBatch(ctx, labelSnapshots); err != nil {
			c.logger.Debug("failed to batch store labels", "metric", metric.Name, "error", err)
		}
	}

	return nil
}
