package scheduler

import (
	"testing"
	"time"

	"github.com/Drakx/ZonoCaller/internal/config"
	"github.com/Drakx/ZonoCaller/internal/fetcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_Success(t *testing.T) {
	cfg := config.Config{
		Timezone:     "UTC",
		ScheduleTime: "12:00",
	}
	f := fetcher.New(config.Config{})

	s, err := New(cfg, f)
	require.NoError(t, err)

	assert.NotNil(t, s)
	assert.NotNil(t, s.scheduler)
	assert.NotNil(t, s.logger)

	jobs := s.scheduler.Jobs()
	assert.Len(t, jobs, 1)

	// Start the scheduler to ensure NextRun returns a valid time
	s.scheduler.Start()
	defer s.scheduler.Shutdown()

	// Allow some time for the scheduler to initialize
	time.Sleep(100 * time.Millisecond)

	loc, _ := time.LoadLocation("UTC")
	now := time.Now().In(loc)
	hour := 12
	minute := 0
	today := now.Truncate(24 * time.Hour)
	scheduled := today.Add(time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute)
	if scheduled.Before(now) {
		scheduled = scheduled.Add(24 * time.Hour)
	}

	// Handle both return values from NextRun()
	nextRun, err := jobs[0].NextRun()
	require.NoError(t, err, "NextRun should not return an error")

	// Compare with some tolerance to account for minor timing differences
	assert.WithinDuration(t, scheduled, nextRun, time.Second, "Next run time should match scheduled time")
}

func TestNew_InvalidTimezone(t *testing.T) {
	cfg := config.Config{
		Timezone:     "Invalid",
		ScheduleTime: "12:00",
	}
	f := fetcher.New(config.Config{})

	_, err := New(cfg, f)
	assert.ErrorContains(t, err, "failed to load timezone")
}

func TestNew_InvalidScheduleTimeFormat(t *testing.T) {
	cfg := config.Config{
		Timezone:     "UTC",
		ScheduleTime: "12",
	}
	f := fetcher.New(config.Config{})

	_, err := New(cfg, f)
	assert.ErrorContains(t, err, "invalid schedule time format")
}

func TestNew_InvalidHour(t *testing.T) {
	cfg := config.Config{
		Timezone:     "UTC",
		ScheduleTime: "24:00",
	}
	f := fetcher.New(config.Config{})

	_, err := New(cfg, f)
	assert.ErrorContains(t, err, "failed to schedule job")
	assert.ErrorContains(t, err, "atTimes hours must be between 0 and 23 inclusive")
}

func TestNew_InvalidMinute(t *testing.T) {
	cfg := config.Config{
		Timezone:     "UTC",
		ScheduleTime: "12:60",
	}
	f := fetcher.New(config.Config{})

	_, err := New(cfg, f)
	assert.ErrorContains(t, err, "failed to schedule job")
	assert.ErrorContains(t, err, "atTimes minutes and seconds must be between 0 and 59 inclusive")
}

func TestStartAndShutdown(t *testing.T) {
	cfg := config.Config{
		Timezone:     "UTC",
		ScheduleTime: "12:00",
	}
	f := fetcher.New(config.Config{})

	s, err := New(cfg, f)
	require.NoError(t, err)

	// Start the scheduler in a goroutine
	go s.Start()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Verify that jobs are scheduled (indicating the scheduler is active)
	jobs := s.scheduler.Jobs()
	assert.Len(t, jobs, 1, "Scheduler should have one job scheduled")

	// Call Shutdown
	s.Shutdown()

	// Verify that no jobs are scheduled after shutdown
	time.Sleep(100 * time.Millisecond) // Allow shutdown to complete
	jobs = s.scheduler.Jobs()
	assert.Len(t, jobs, 0, "Scheduler should have no jobs after shutdown")
}
