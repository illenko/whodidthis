package handler

import (
	"net/http"

	"github.com/illenko/whodidthis/api/helpers"
	"github.com/illenko/whodidthis/models"
	"github.com/illenko/whodidthis/prometheus"
	"github.com/illenko/whodidthis/storage"
)

type HealthHandler struct {
	snapshots  *storage.SnapshotsRepository
	db         *storage.DB
	promClient *prometheus.Client
}

func NewHealthHandler(snapshots *storage.SnapshotsRepository,
	db *storage.DB,
	promClient *prometheus.Client) *HealthHandler {
	return &HealthHandler{
		snapshots:  snapshots,
		db:         db,
		promClient: promClient,
	}
}

func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
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

	helpers.WriteJSON(w, http.StatusOK, status)
}
