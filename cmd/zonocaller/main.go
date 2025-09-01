package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Drakx/ZonoCaller/internal/config"
	"github.com/Drakx/ZonoCaller/internal/fetcher"
	"github.com/Drakx/ZonoCaller/internal/scheduler"
)

func main() {

	// Setup logging to stdout in JSON format
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
	}))

	// Load config
	cfg, err := config.New()
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Initialize fetcher
	f := fetcher.New(*cfg)

	// Check for interrupt signals
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Start health check server in background
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		if err := http.ListenAndServe(":8000", mux); err != nil {
			logger.Error("Failed to start health server", "error", err)
		}
	}()

	// Run scheduler
	s := scheduler.New(*cfg, f, logger)
	if err := s.Run(ctx); err != nil {
		logger.Error("Scheduler failed", "error", err)
		os.Exit(1)
	}

	logger.Info("Scheduler started", "schedule_time", cfg.ScheduleTime, "timezone", cfg.Timezone)
}
