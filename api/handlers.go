package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/illenko/whodidthis/analyzer"
	"github.com/illenko/whodidthis/models"
	"github.com/illenko/whodidthis/prometheus"
	"github.com/illenko/whodidthis/scheduler"
	"github.com/illenko/whodidthis/storage"
)

type Handlers struct {
	snapshots  *storage.SnapshotsRepository
	services   *storage.ServicesRepository
	metrics    *storage.MetricsRepository
	labels     *storage.LabelsRepository
	scheduler  *scheduler.Scheduler
	db         *storage.DB
	promClient *prometheus.Client
	analyzer   *analyzer.Analyzer
}

type HandlersConfig struct {
	Snapshots  *storage.SnapshotsRepository
	Services   *storage.ServicesRepository
	Metrics    *storage.MetricsRepository
	Labels     *storage.LabelsRepository
	Scheduler  *scheduler.Scheduler
	DB         *storage.DB
	PromClient *prometheus.Client
	Analyzer   *analyzer.Analyzer
}

func NewHandlers(cfg HandlersConfig) *Handlers {
	return &Handlers{
		snapshots:  cfg.Snapshots,
		services:   cfg.Services,
		metrics:    cfg.Metrics,
		labels:     cfg.Labels,
		scheduler:  cfg.Scheduler,
		db:         cfg.DB,
		promClient: cfg.PromClient,
		analyzer:   cfg.Analyzer,
	}
}

func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	status := models.HealthStatus{
		Status:              "healthy",
		DatabaseOK:          true,
		PrometheusConnected: true,
	}

	if _, err := h.db.Stats(ctx); err != nil {
		status.Status = "unhealthy"
		status.DatabaseOK = false
	}

	if h.promClient != nil {
		if err := h.promClient.HealthCheck(ctx); err != nil {
			status.PrometheusConnected = false
			if status.Status == "healthy" {
				status.Status = "degraded"
			}
		}
	}

	latest, _ := h.snapshots.GetLatest(ctx)
	if latest != nil {
		status.LastScan = latest.CollectedAt
	}

	writeJSON(w, http.StatusOK, status)
}

func (h *Handlers) ListScans(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit := parseIntParam(r, "limit", 100)

	scans, err := h.snapshots.List(ctx, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if scans == nil {
		scans = []models.Snapshot{}
	}

	writeJSON(w, http.StatusOK, scans)
}

func (h *Handlers) GetLatestScan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	scan, err := h.snapshots.GetLatest(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if scan == nil {
		writeError(w, http.StatusNotFound, "no scans found")
		return
	}

	writeJSON(w, http.StatusOK, scan)
}

func (h *Handlers) GetScan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid scan id")
		return
	}

	scan, err := h.snapshots.GetByID(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if scan == nil {
		writeError(w, http.StatusNotFound, "scan not found")
		return
	}

	writeJSON(w, http.StatusOK, scan)
}

func (h *Handlers) ListServices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	scanID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid scan id")
		return
	}

	opts := storage.ServiceListOptions{
		Sort:   r.URL.Query().Get("sort"),
		Order:  r.URL.Query().Get("order"),
		Search: r.URL.Query().Get("search"),
	}

	services, err := h.services.List(ctx, scanID, opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if services == nil {
		services = []models.ServiceSnapshot{}
	}

	writeJSON(w, http.StatusOK, services)
}

func (h *Handlers) GetService(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	scanID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid scan id")
		return
	}

	serviceName := r.PathValue("service")

	service, err := h.services.GetByName(ctx, scanID, serviceName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if service == nil {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}

	writeJSON(w, http.StatusOK, service)
}

func (h *Handlers) ListMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	scanID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid scan id")
		return
	}

	serviceName := r.PathValue("service")

	service, err := h.services.GetByName(ctx, scanID, serviceName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if service == nil {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}

	opts := storage.MetricListOptions{
		Sort:  r.URL.Query().Get("sort"),
		Order: r.URL.Query().Get("order"),
	}

	metrics, err := h.metrics.List(ctx, service.ID, opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if metrics == nil {
		metrics = []models.MetricSnapshot{}
	}

	writeJSON(w, http.StatusOK, metrics)
}

func (h *Handlers) GetMetric(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	scanID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid scan id")
		return
	}

	serviceName := r.PathValue("service")
	metricName := r.PathValue("metric")

	service, err := h.services.GetByName(ctx, scanID, serviceName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if service == nil {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}

	metric, err := h.metrics.GetByName(ctx, service.ID, metricName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if metric == nil {
		writeError(w, http.StatusNotFound, "metric not found")
		return
	}

	writeJSON(w, http.StatusOK, metric)
}

func (h *Handlers) ListLabels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	scanID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid scan id")
		return
	}

	serviceName := r.PathValue("service")
	metricName := r.PathValue("metric")

	service, err := h.services.GetByName(ctx, scanID, serviceName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if service == nil {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}

	metric, err := h.metrics.GetByName(ctx, service.ID, metricName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if metric == nil {
		writeError(w, http.StatusNotFound, "metric not found")
		return
	}

	labels, err := h.labels.List(ctx, metric.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if labels == nil {
		labels = []models.LabelSnapshot{}
	}

	writeJSON(w, http.StatusOK, labels)
}

func (h *Handlers) TriggerScan(w http.ResponseWriter, r *http.Request) {
	if h.scheduler == nil {
		writeError(w, http.StatusServiceUnavailable, "scheduler not configured")
		return
	}

	err := h.scheduler.TriggerScan(r.Context())
	if err != nil {
		if err == scheduler.ErrScanAlreadyRunning {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "scan started"})
}

func (h *Handlers) GetScanStatus(w http.ResponseWriter, r *http.Request) {
	if h.scheduler == nil {
		writeError(w, http.StatusServiceUnavailable, "scheduler not configured")
		return
	}

	status := h.scheduler.GetStatus()
	writeJSON(w, http.StatusOK, status)
}

func (h *Handlers) StartAnalysis(w http.ResponseWriter, r *http.Request) {
	if h.analyzer == nil {
		writeError(w, http.StatusServiceUnavailable, "analysis not configured (missing Gemini API key)")
		return
	}

	var req struct {
		CurrentSnapshotID  int64 `json:"current_snapshot_id"`
		PreviousSnapshotID int64 `json:"previous_snapshot_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.CurrentSnapshotID == 0 || req.PreviousSnapshotID == 0 {
		writeError(w, http.StatusBadRequest, "current_snapshot_id and previous_snapshot_id are required")
		return
	}

	analysis, err := h.analyzer.StartAnalysis(r.Context(), req.CurrentSnapshotID, req.PreviousSnapshotID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, analysis)
}

func (h *Handlers) GetAnalysis(w http.ResponseWriter, r *http.Request) {
	if h.analyzer == nil {
		writeError(w, http.StatusServiceUnavailable, "analysis not configured (missing Gemini API key)")
		return
	}

	currentID, err := strconv.ParseInt(r.URL.Query().Get("current"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid current parameter")
		return
	}
	previousID, err := strconv.ParseInt(r.URL.Query().Get("previous"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid previous parameter")
		return
	}

	analysis, err := h.analyzer.GetAnalysis(r.Context(), currentID, previousID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if analysis == nil {
		writeError(w, http.StatusNotFound, "analysis not found")
		return
	}

	writeJSON(w, http.StatusOK, analysis)
}

func (h *Handlers) ListAnalysesBySnapshot(w http.ResponseWriter, r *http.Request) {
	if h.analyzer == nil {
		writeError(w, http.StatusServiceUnavailable, "analysis not configured (missing Gemini API key)")
		return
	}

	snapshotID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid scan id")
		return
	}

	analyses, err := h.analyzer.ListAnalyses(r.Context(), snapshotID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if analyses == nil {
		analyses = []models.SnapshotAnalysis{}
	}

	writeJSON(w, http.StatusOK, analyses)
}

func (h *Handlers) DeleteAnalysis(w http.ResponseWriter, r *http.Request) {
	if h.analyzer == nil {
		writeError(w, http.StatusServiceUnavailable, "analysis not configured (missing Gemini API key)")
		return
	}

	currentID, err := strconv.ParseInt(r.URL.Query().Get("current"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid current parameter")
		return
	}
	previousID, err := strconv.ParseInt(r.URL.Query().Get("previous"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid previous parameter")
		return
	}

	if err := h.analyzer.DeleteAnalysis(r.Context(), currentID, previousID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handlers) GetAnalysisStatus(w http.ResponseWriter, r *http.Request) {
	if h.analyzer == nil {
		writeError(w, http.StatusServiceUnavailable, "analysis not configured (missing Gemini API key)")
		return
	}

	status := h.analyzer.GetGlobalStatus()
	writeJSON(w, http.StatusOK, status)
}

func parseIntParam(r *http.Request, name string, defaultVal int) int {
	val := r.URL.Query().Get(name)
	if val == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return n
}
