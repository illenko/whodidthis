package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

type Server struct {
	httpServer *http.Server
	handlers   *Handlers
}

type ServerConfig struct {
	Host         string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func NewServer(handlers *Handlers, cfg ServerConfig) *Server {
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 30 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 30 * time.Second
	}

	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /health", handlers.Health)

	// Scan control
	mux.HandleFunc("POST /api/scan", handlers.TriggerScan)
	mux.HandleFunc("GET /api/scan/status", handlers.GetScanStatus)

	// Scans (snapshots)
	mux.HandleFunc("GET /api/scans", handlers.ListScans)
	mux.HandleFunc("GET /api/scans/latest", handlers.GetLatestScan)
	mux.HandleFunc("GET /api/scans/{id}", handlers.GetScan)

	// Services (within a scan)
	mux.HandleFunc("GET /api/scans/{id}/services", handlers.ListServices)
	mux.HandleFunc("GET /api/scans/{id}/services/{service}", handlers.GetService)

	// Metrics (within a service)
	mux.HandleFunc("GET /api/scans/{id}/services/{service}/metrics", handlers.ListMetrics)
	mux.HandleFunc("GET /api/scans/{id}/services/{service}/metrics/{metric}", handlers.GetMetric)

	// Labels (within a metric)
	mux.HandleFunc("GET /api/scans/{id}/services/{service}/metrics/{metric}/labels", handlers.ListLabels)

	// Analysis
	mux.HandleFunc("POST /api/analysis", handlers.StartAnalysis)
	mux.HandleFunc("GET /api/analysis", handlers.GetAnalysis)
	mux.HandleFunc("DELETE /api/analysis", handlers.DeleteAnalysis)
	mux.HandleFunc("GET /api/analysis/status", handlers.GetAnalysisStatus)
	mux.HandleFunc("GET /api/scans/{id}/analyses", handlers.ListAnalysesBySnapshot)

	// Static files (frontend)
	mux.Handle("/", staticHandler())

	handler := withMiddleware(mux)

	return &Server{
		httpServer: &http.Server{
			Addr:         cfg.Host + ":" + strconv.Itoa(cfg.Port),
			Handler:      handler,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
		},
		handlers: handlers,
	}
}

func (s *Server) Start() error {
	slog.Info("starting HTTP server", "addr", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

const requestTimeout = 30 * time.Second

// statusWriter wraps http.ResponseWriter to capture the status code
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		// Recovery
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered", "error", err)
				writeError(sw, http.StatusInternalServerError, "internal server error")
			}
		}()

		// CORS
		sw.Header().Set("Access-Control-Allow-Origin", "*")
		sw.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		sw.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			sw.WriteHeader(http.StatusOK)
			return
		}

		// Request timeout
		ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
		defer cancel()

		next.ServeHTTP(sw, r.WithContext(ctx))

		slog.Debug("request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration", time.Since(start),
		)
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
