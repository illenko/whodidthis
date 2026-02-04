package handler

import (
	"net/http"
	"strconv"

	"github.com/illenko/whodidthis/api/helpers"
	"github.com/illenko/whodidthis/models"
	"github.com/illenko/whodidthis/storage"
)

type MetricsHandler struct {
	servicesRepo *storage.ServicesRepository
	metricsRepo  *storage.MetricsRepository
}

func NewMetricsHandler(servicesRepo *storage.ServicesRepository, metricsRepo *storage.MetricsRepository) *MetricsHandler {
	return &MetricsHandler{
		servicesRepo: servicesRepo,
		metricsRepo:  metricsRepo,
	}
}

func (m *MetricsHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	scanID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		helpers.WriteError(w, http.StatusBadRequest, "invalid scan id")
		return
	}

	serviceName := r.PathValue("service")

	service, err := m.servicesRepo.GetByName(ctx, scanID, serviceName)
	if err != nil {
		helpers.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if service == nil {
		helpers.WriteError(w, http.StatusNotFound, "service not found")
		return
	}

	opts := storage.MetricListOptions{
		Sort:  r.URL.Query().Get("sort"),
		Order: r.URL.Query().Get("order"),
	}

	metrics, err := m.metricsRepo.List(ctx, service.ID, opts)
	if err != nil {
		helpers.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if metrics == nil {
		metrics = []models.MetricSnapshot{}
	}

	helpers.WriteJSON(w, http.StatusOK, metrics)
}

func (m *MetricsHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	scanID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		helpers.WriteError(w, http.StatusBadRequest, "invalid scan id")
		return
	}

	serviceName := r.PathValue("service")
	metricName := r.PathValue("metric")

	service, err := m.servicesRepo.GetByName(ctx, scanID, serviceName)
	if err != nil {
		helpers.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if service == nil {
		helpers.WriteError(w, http.StatusNotFound, "service not found")
		return
	}

	metric, err := m.metricsRepo.GetByName(ctx, service.ID, metricName)
	if err != nil {
		helpers.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if metric == nil {
		helpers.WriteError(w, http.StatusNotFound, "metric not found")
		return
	}

	helpers.WriteJSON(w, http.StatusOK, metric)
}
