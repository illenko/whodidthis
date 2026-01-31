package analyzer

import (
	"context"
	"fmt"
	"time"

	"github.com/illenko/metriccost/models"
	"github.com/illenko/metriccost/storage"
)

type RecommendationsEngine struct {
	metricsRepo    *storage.MetricsRepository
	dashboardsRepo *storage.DashboardsRepository
	recsRepo       *storage.RecommendationsRepository

	highCardinalityThreshold int
	highLabelValueThreshold  int
	minCardinalityImpact     int
}

type RecommendationsConfig struct {
	HighCardinalityThreshold int
	HighLabelValueThreshold  int
	MinCardinalityImpact     int
}

func NewRecommendationsEngine(
	metricsRepo *storage.MetricsRepository,
	dashboardsRepo *storage.DashboardsRepository,
	recsRepo *storage.RecommendationsRepository,
	cfg RecommendationsConfig,
) *RecommendationsEngine {
	if cfg.HighCardinalityThreshold == 0 {
		cfg.HighCardinalityThreshold = 10000
	}
	if cfg.HighLabelValueThreshold == 0 {
		cfg.HighLabelValueThreshold = 100
	}
	if cfg.MinCardinalityImpact == 0 {
		cfg.MinCardinalityImpact = 1000
	}

	return &RecommendationsEngine{
		metricsRepo:              metricsRepo,
		dashboardsRepo:           dashboardsRepo,
		recsRepo:                 recsRepo,
		highCardinalityThreshold: cfg.HighCardinalityThreshold,
		highLabelValueThreshold:  cfg.HighLabelValueThreshold,
		minCardinalityImpact:     cfg.MinCardinalityImpact,
	}
}

type AnalyzeResult struct {
	TotalRecommendations int
	HighPriority         int
	MediumPriority       int
	LowPriority          int
	TotalCardinality     int64
}

func (e *RecommendationsEngine) Analyze(ctx context.Context) (*AnalyzeResult, error) {
	if err := e.recsRepo.Clear(ctx); err != nil {
		return nil, fmt.Errorf("failed to clear old recommendations: %w", err)
	}

	latestTime, err := e.metricsRepo.GetLatestCollectionTime(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest collection time: %w", err)
	}
	if latestTime.IsZero() {
		return &AnalyzeResult{}, nil
	}

	metrics, err := e.metricsRepo.List(ctx, latestTime, storage.ListOptions{Limit: 100000})
	if err != nil {
		return nil, fmt.Errorf("failed to list metrics: %w", err)
	}

	totalCardinality, err := e.metricsRepo.GetTotalCardinality(ctx, latestTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get total cardinality: %w", err)
	}

	usedMetrics, err := e.dashboardsRepo.GetAllMetricsUsed(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get used metrics: %w", err)
	}

	var recommendations []*models.Recommendation
	now := time.Now()

	for _, m := range metrics {
		recs := e.analyzeMetric(m, usedMetrics, totalCardinality, now)
		recommendations = append(recommendations, recs...)
	}

	if len(recommendations) > 0 {
		if err := e.recsRepo.SaveBatch(ctx, recommendations); err != nil {
			return nil, fmt.Errorf("failed to save recommendations: %w", err)
		}
	}

	result := &AnalyzeResult{
		TotalRecommendations: len(recommendations),
		TotalCardinality:     totalCardinality,
	}

	for _, r := range recommendations {
		switch r.Priority {
		case models.PriorityHigh:
			result.HighPriority++
		case models.PriorityMedium:
			result.MediumPriority++
		case models.PriorityLow:
			result.LowPriority++
		}
	}

	return result, nil
}

func (e *RecommendationsEngine) analyzeMetric(m *models.MetricSnapshot, usedMetrics map[string]struct{}, totalCardinality int64, now time.Time) []*models.Recommendation {
	var recs []*models.Recommendation

	_, isUsed := usedMetrics[m.MetricName]

	if rec := e.checkUnused(m, isUsed, totalCardinality, now); rec != nil {
		recs = append(recs, rec)
	}

	if rec := e.checkHighCardinality(m, isUsed, totalCardinality, now); rec != nil {
		recs = append(recs, rec)
	}

	if labelRecs := e.checkHighCardinalityLabels(m, totalCardinality, now); len(labelRecs) > 0 {
		recs = append(recs, labelRecs...)
	}

	return recs
}

func (e *RecommendationsEngine) checkUnused(m *models.MetricSnapshot, isUsed bool, totalCardinality int64, now time.Time) *models.Recommendation {
	if isUsed {
		return nil
	}

	if m.Cardinality < e.minCardinalityImpact {
		return nil
	}

	percentage := 0.0
	if totalCardinality > 0 {
		percentage = float64(m.Cardinality) / float64(totalCardinality) * 100
	}

	return &models.Recommendation{
		CreatedAt:           now,
		MetricName:          m.MetricName,
		Type:                models.RecommendationUnused,
		Priority:            models.PriorityHigh,
		CurrentCardinality:  m.Cardinality,
		PotentialReduction:  m.Cardinality,
		ReductionPercentage: percentage,
		Description:         fmt.Sprintf("Metric '%s' is not used in any Grafana dashboard (%.2f%% of total cardinality)", m.MetricName, percentage),
		SuggestedAction:     "Consider dropping this metric if it's not needed for alerting or other purposes",
	}
}

func (e *RecommendationsEngine) checkHighCardinality(m *models.MetricSnapshot, isUsed bool, totalCardinality int64, now time.Time) *models.Recommendation {
	if m.Cardinality < e.highCardinalityThreshold {
		return nil
	}

	priority := models.PriorityMedium
	if !isUsed || m.Cardinality > e.highCardinalityThreshold*10 {
		priority = models.PriorityHigh
	}

	var potentialReduction int
	if !isUsed {
		potentialReduction = m.Cardinality
	} else {
		potentialReduction = m.Cardinality - e.highCardinalityThreshold
	}

	percentage := 0.0
	if totalCardinality > 0 {
		percentage = float64(potentialReduction) / float64(totalCardinality) * 100
	}

	return &models.Recommendation{
		CreatedAt:           now,
		MetricName:          m.MetricName,
		Type:                models.RecommendationHighCardinality,
		Priority:            priority,
		CurrentCardinality:  m.Cardinality,
		PotentialReduction:  potentialReduction,
		ReductionPercentage: percentage,
		Description:         fmt.Sprintf("Metric '%s' has high cardinality (%d time series, %.2f%% of total)", m.MetricName, m.Cardinality, float64(m.Cardinality)/float64(totalCardinality)*100),
		SuggestedAction:     "Review labels for high-cardinality values (user IDs, request IDs, etc.) and consider using recording rules for aggregation",
	}
}

func (e *RecommendationsEngine) checkHighCardinalityLabels(m *models.MetricSnapshot, totalCardinality int64, now time.Time) []*models.Recommendation {
	if len(m.Labels) == 0 {
		return nil
	}

	var recs []*models.Recommendation

	for labelName, uniqueCount := range m.Labels {
		if uniqueCount < e.highLabelValueThreshold {
			continue
		}

		reducedCardinality := m.Cardinality / uniqueCount
		if reducedCardinality < 1 {
			reducedCardinality = 1
		}
		potentialReduction := m.Cardinality - reducedCardinality

		if potentialReduction < e.minCardinalityImpact {
			continue
		}

		percentage := 0.0
		if totalCardinality > 0 {
			percentage = float64(potentialReduction) / float64(totalCardinality) * 100
		}

		recs = append(recs, &models.Recommendation{
			CreatedAt:           now,
			MetricName:          m.MetricName,
			Type:                models.RecommendationRedundantLabels,
			Priority:            models.PriorityMedium,
			CurrentCardinality:  m.Cardinality,
			PotentialReduction:  potentialReduction,
			ReductionPercentage: percentage,
			Description:         fmt.Sprintf("Label '%s' on metric '%s' has %d unique values", labelName, m.MetricName, uniqueCount),
			SuggestedAction:     fmt.Sprintf("Consider removing or aggregating the '%s' label if it contains high-cardinality values like IDs", labelName),
		})
	}

	return recs
}
