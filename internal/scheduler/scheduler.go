package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/Drakx/ZonoCaller/internal/config"
	"github.com/Drakx/ZonoCaller/internal/fetcher"
	"github.com/go-co-op/gocron/v2"
)

// Scheduler manages the periodic IP fetching and DNS updates
type Scheduler struct {
	config    config.Config
	fetcher   fetcher.FetcherInterface
	logger    *slog.Logger
	scheduler gocron.Scheduler
}

// New creates a new Scheduler instance
func New(cfg config.Config, f fetcher.FetcherInterface, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		config:  cfg,
		fetcher: f,
		logger:  logger,
	}
}

// Run starts the scheduler
func (s *Scheduler) Run(ctx context.Context) error {

	if s.config.RunOnce {
		s.logger.Info("Running fetcher once")
		return s.fetcher.FetchIP(ctx)
	}

	s.logger.Info("Starting scheduler", "timezone", s.config.Timezone, "schedule", s.config.ScheduleTime)

	loc, err := time.LoadLocation(s.config.Timezone)
	if err != nil {
		return fmt.Errorf("failed to load timezone %s: %w", s.config.Timezone, err)
	}

	// Parse ScheduleTime (e.g., "23:59") into hour and minute
	parts := strings.Split(s.config.ScheduleTime, ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid SCHEDULE_TIME format: %s, expected HH:MM", s.config.ScheduleTime)
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("invalid SCHEDULE_TIME hour: %w", err)
	}

	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("invalid SCHEDULE_TIME minute: %w", err)
	}

	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return fmt.Errorf("invalid SCHEDULE_TIME: hour must be 0-23, minute must be 0-59, got %d:%d", hour, minute)
	}

	scheduler, err := gocron.NewScheduler(gocron.WithLocation(loc))
	if err != nil {
		return fmt.Errorf("failed to create scheduler: %w", err)
	}

	s.scheduler = scheduler

	_, err = scheduler.NewJob(
		gocron.DailyJob(
			1,
			gocron.NewAtTimes(
				gocron.NewAtTime(uint(hour), uint(minute), 0),
			),
		),
		gocron.NewTask(
			func() {
				s.logger.Info("Running scheduled IP fetch")
				if err := s.fetcher.FetchIP(ctx); err != nil {
					s.logger.Error("Failed to fetch IP", "error", err)
				}
			},
		),
	)
	if err != nil {
		return fmt.Errorf("failed to schedule job: %w", err)
	}

	scheduler.Start()
	<-ctx.Done()
	if err := scheduler.Shutdown(); err != nil {
		s.logger.Error("Failed to shutdown scheduler", "error", err)
	}
	return nil
}
