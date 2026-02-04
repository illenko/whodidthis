package api

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/illenko/whodidthis/api/handler"
)

type Server struct {
	httpServer *http.Server
}

type ServerConfig struct {
	Host         string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func NewServer(
	healthHandler *handler.HealthHandler,
	scansHandler *handler.ScansHandler,
	analysisHandler *handler.AnalysisHandler,
	servicesHandler *handler.ServicesHandler,
	metricsHandler *handler.MetricsHandler,
	labelsHandler *handler.LabelsHandler,
	cfg ServerConfig) *Server {
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 30 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 30 * time.Second
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", healthHandler.Health)

	mux.HandleFunc("POST /api/scan", scansHandler.Trigger)
	mux.HandleFunc("GET /api/scan/status", scansHandler.GetStatus)
	mux.HandleFunc("GET /api/scans", scansHandler.List)
	mux.HandleFunc("GET /api/scans/latest", scansHandler.GetLatest)
	mux.HandleFunc("GET /api/scans/{id}", scansHandler.Get)

	mux.HandleFunc("GET /api/scans/{id}/services", servicesHandler.List)
	mux.HandleFunc("GET /api/scans/{id}/services/{service}", servicesHandler.Get)

	mux.HandleFunc("GET /api/scans/{id}/services/{service}/metrics", metricsHandler.List)
	mux.HandleFunc("GET /api/scans/{id}/services/{service}/metrics/{metric}", metricsHandler.Get)

	mux.HandleFunc("GET /api/scans/{id}/services/{service}/metrics/{metric}/labels", labelsHandler.List)

	mux.HandleFunc("POST /api/analysis", analysisHandler.Start)
	mux.HandleFunc("GET /api/analysis", analysisHandler.Get)
	mux.HandleFunc("DELETE /api/analysis", analysisHandler.Delete)
	mux.HandleFunc("GET /api/analysis/status", analysisHandler.GetStatus)
	mux.HandleFunc("GET /api/scans/{id}/analyses", analysisHandler.ListBySnapshot)

	mux.Handle("/", staticHandler())

	return &Server{
		httpServer: &http.Server{
			Addr:         cfg.Host + ":" + strconv.Itoa(cfg.Port),
			Handler:      withMiddleware(mux),
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
		},
	}
}

func (s *Server) Start() error {
	slog.Info("starting HTTP server", "addr", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
