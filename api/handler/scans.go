package handler

import (
	"net/http"
	"strconv"

	"github.com/illenko/whodidthis/api/helpers"
	"github.com/illenko/whodidthis/models"
	"github.com/illenko/whodidthis/scheduler"
	"github.com/illenko/whodidthis/storage"
)

type ScansHandler struct {
	repo      *storage.SnapshotsRepository
	scheduler *scheduler.Scheduler
}

func NewScansHandler(repo *storage.SnapshotsRepository, scheduler *scheduler.Scheduler) *ScansHandler {
	return &ScansHandler{
		repo:      repo,
		scheduler: scheduler,
	}
}

func (s *ScansHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit := helpers.ParseIntParam(r, "limit", 100)

	scans, err := s.repo.List(ctx, limit)
	if err != nil {
		helpers.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if scans == nil {
		scans = []models.Snapshot{}
	}

	helpers.WriteJSON(w, http.StatusOK, scans)
}

func (s *ScansHandler) GetLatest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	scan, err := s.repo.GetLatest(ctx)
	if err != nil {
		helpers.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if scan == nil {
		helpers.WriteError(w, http.StatusNotFound, "no scans found")
		return
	}

	helpers.WriteJSON(w, http.StatusOK, scan)
}

func (s *ScansHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		helpers.WriteError(w, http.StatusBadRequest, "invalid scan id")
		return
	}

	scan, err := s.repo.GetByID(ctx, id)
	if err != nil {
		helpers.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if scan == nil {
		helpers.WriteError(w, http.StatusNotFound, "scan not found")
		return
	}

	helpers.WriteJSON(w, http.StatusOK, scan)
}

func (s *ScansHandler) Trigger(w http.ResponseWriter, r *http.Request) {
	if s.scheduler == nil {
		helpers.WriteError(w, http.StatusServiceUnavailable, "scheduler not configured")
		return
	}

	err := s.scheduler.TriggerScan(r.Context())
	if err != nil {
		if err == scheduler.ErrScanAlreadyRunning {
			helpers.WriteError(w, http.StatusConflict, err.Error())
			return
		}
		helpers.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	helpers.WriteJSON(w, http.StatusAccepted, map[string]string{"status": "scan started"})
}

func (s *ScansHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	if s.scheduler == nil {
		helpers.WriteError(w, http.StatusServiceUnavailable, "scheduler not configured")
		return
	}

	status := s.scheduler.GetStatus()
	helpers.WriteJSON(w, http.StatusOK, status)
}
