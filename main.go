package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/illenko/whodidthis/analyzer"
	"github.com/illenko/whodidthis/api"
	"github.com/illenko/whodidthis/collector"
	"github.com/illenko/whodidthis/config"
	"github.com/illenko/whodidthis/prometheus"
	"github.com/illenko/whodidthis/scheduler"
	"github.com/illenko/whodidthis/storage"
)

func main() {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logLevel := cfg.LogLevel()
	if os.Getenv("DEBUG") == "true" {
		logLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})))

	db, err := storage.New(cfg.Storage.Path)
	if err != nil {
		slog.Error("failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	snapshotsRepo := storage.NewSnapshotsRepository(db)
	servicesRepo := storage.NewServicesRepository(db)
	metricsRepo := storage.NewMetricsRepository(db)
	labelsRepo := storage.NewLabelsRepository(db)

	promClient, err := prometheus.NewClient(prometheus.Config{
		URL:      cfg.Prometheus.URL,
		Username: cfg.Prometheus.Username,
		Password: cfg.Prometheus.Password,
	})
	if err != nil {
		slog.Error("failed to create prometheus client", "error", err)
		os.Exit(1)
	}

	coll := collector.NewCollector(
		promClient,
		snapshotsRepo,
		servicesRepo,
		metricsRepo,
		labelsRepo,
		cfg,
	)

	sched := scheduler.New(coll, scheduler.Config{
		Interval:  cfg.Scan.Interval,
		Retention: cfg.RetentionDuration(),
		DB:        db,
	})

	analysisRepo := storage.NewAnalysisRepository(db)

	var snapshotAnalyzer *analyzer.Analyzer
	if cfg.Gemini.APIKey != "" {
		toolExecutor := analyzer.NewToolExecutor(servicesRepo, metricsRepo, labelsRepo)
		var err error
		snapshotAnalyzer, err = analyzer.New(context.Background(), analyzer.Config{
			APIKey:       cfg.Gemini.APIKey,
			Model:        cfg.Gemini.Model,
			ToolExecutor: toolExecutor,
			AnalysisRepo: analysisRepo,
			Snapshots:    snapshotsRepo,
			Services:     servicesRepo,
		})
		if err != nil {
			slog.Error("failed to create analyzer", "error", err)
			os.Exit(1)
		}
		slog.Info("AI analysis enabled", "model", cfg.Gemini.Model)
	} else {
		slog.Warn("AI analysis disabled: WDT_GEMINI_API_KEY not set")
	}

	handlers := api.NewHandlers(api.HandlersConfig{
		Snapshots:  snapshotsRepo,
		Services:   servicesRepo,
		Metrics:    metricsRepo,
		Labels:     labelsRepo,
		Scheduler:  sched,
		DB:         db,
		PromClient: promClient,
		Analyzer:   snapshotAnalyzer,
	})

	server := api.NewServer(handlers, api.ServerConfig{
		Host: cfg.Server.Host,
		Port: cfg.Server.Port,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go sched.Start(ctx)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		slog.Info("shutting down...")
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("server shutdown error", "error", err)
		}
	}()

	if err := server.Start(); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
