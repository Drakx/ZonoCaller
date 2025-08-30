package scheduler

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/Drakx/ZonoCaller/internal/config"
	"github.com/Drakx/ZonoCaller/internal/fetcher"
	"github.com/go-co-op/gocron/v2"
)

// Scheduler wraps the gocron scheduler
type Scheduler struct {
	scheduler gocron.Scheduler
	logger    *slog.Logger
}

// New initializes a new Scheduler with the given config and fetcher
func New(cfg config.Config, fetcher *fetcher.Fetcher) (*Scheduler, error) {

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
	}))

	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return nil, fmt.Errorf("failed to load timezone: %w", err)
	}

	scheduler, err := gocron.NewScheduler(gocron.WithLocation(loc))
	if err != nil {
		return nil, fmt.Errorf("failed to create scheduler: %w", err)
	}

	// Parse schedule time
	var hour, minute int
	if _, err := fmt.Sscanf(cfg.ScheduleTime, "%d:%d", &hour, &minute); err != nil {
		return nil, fmt.Errorf("invalid schedule time format: %w", err)
	}

	_, err = scheduler.NewJob(
		//		gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(hour, minute, 0))),
		gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(uint(hour), uint(minute), 0))),
		gocron.NewTask(fetcher.FetchIP),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to schedule job: %w", err)
	}

	return &Scheduler{scheduler: scheduler, logger: logger}, nil
}

// Start begins the scheduler
func (s *Scheduler) Start() {
	s.scheduler.Start()
	select {}
}

// Shutdown stops the scheduler gracefully
func (s *Scheduler) Shutdown() {
	if err := s.scheduler.Shutdown(); err != nil {
		s.logger.Error("Failed to shutdown scheduler", "error", err)
	}
}
