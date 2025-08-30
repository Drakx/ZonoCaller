package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/Drakx/ZonoCaller/internal/config"
	"github.com/Drakx/ZonoCaller/internal/fetcher"
	"github.com/Drakx/ZonoCaller/internal/scheduler"
)

func main() {
	// Get a logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
	}))

	// Load config
	cfg, err := config.Load()
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Initialize fetcher
	f := fetcher.New(cfg)

	// Check for run-once mode for testing
	if os.Getenv("RUN_ONCE") == "true" {
		logger.Info("Running in run-once mode for testing")
		if err := f.FetchIP(context.Background()); err != nil {
			logger.Error("Failed to run fetch IP", "error", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Start health check server in background
	go func() {
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		if err := http.ListenAndServe(":8000", nil); err != nil {
			logger.Error("Failed to start health server", "error", err)
		}
	}()

	// Initialize and start the scheduler
	s, err := scheduler.New(cfg, f)
	if err != nil {
		logger.Error("Failed to initialize scheduler", "error", err)
		os.Exit(1)
	}
	defer s.Shutdown()

	logger.Info("Scheduler start", "schedule_time", cfg.ScheduleTime, "timezone", cfg.Timezone)
	s.Start()
}
