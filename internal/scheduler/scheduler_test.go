package scheduler

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Drakx/ZonoCaller/internal/config"
	"github.com/go-co-op/gocron/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockFetcher struct {
	fetchCalled bool
	fetchError  error
}

func (m *mockFetcher) FetchIP(_ context.Context) error {
	m.fetchCalled = true
	return m.fetchError
}

func TestNew(t *testing.T) {
	cfg := config.Config{
		Timezone:     "UTC",
		ScheduleTime: "23:59",
	}
	f := &mockFetcher{}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	s := New(cfg, f, logger)

	assert.Equal(t, cfg, s.config)
	assert.Equal(t, f, s.fetcher)
	assert.Equal(t, logger, s.logger)
	assert.Nil(t, s.scheduler)
}

func TestRun_RunOnce(t *testing.T) {
	cfg := config.Config{
		RunOnce: true,
	}
	f := &mockFetcher{}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	s := New(cfg, f, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := s.Run(ctx)
	require.NoError(t, err)
	assert.True(t, f.fetchCalled, "FetchIP should be called in run-once mode")
}

func TestRun_Scheduled(t *testing.T) {
	// Use a temporary directory for OUTPUT_FILE
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "ip_log.txt")

	cfg := config.Config{
		Timezone:     "UTC",
		ScheduleTime: time.Now().UTC().Format("15:04"), // Set to current time
		ZonomiHosts:  []string{"test.host"},
		ZonomiAPIKey: "test-key",
		OutputFile:   outputFile,
	}

	f := &mockFetcher{}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	s := New(cfg, f, logger)

	// Override scheduler for testing to run every second
	s.scheduler, _ = gocron.NewScheduler(gocron.WithLocation(time.UTC))
	_, err := s.scheduler.NewJob(
		gocron.DurationJob(1*time.Second),
		gocron.NewTask(func() {
			s.logger.Info("Running scheduled IP fetch")
			if err := s.fetcher.FetchIP(context.Background()); err != nil {
				s.logger.Error("Failed to fetch IP", "error", err)
			}
		}),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		s.scheduler.Start()
		<-ctx.Done()
		s.scheduler.Shutdown()
		wg.Done()
	}()

	// Wait for the scheduler to trigger
	time.Sleep(3 * time.Second)
	cancel()
	wg.Wait()

	assert.True(t, f.fetchCalled, "FetchIP should be called by scheduler")
}

func TestRun_InvalidTimezone(t *testing.T) {
	cfg := config.Config{
		Timezone:     "Invalid/Timezone",
		ScheduleTime: "23:59",
		ZonomiHosts:  []string{"test.host"},
		ZonomiAPIKey: "test-key",
	}
	f := &mockFetcher{}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	s := New(cfg, f, logger)

	err := s.Run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load timezone")
}

func TestRun_InvalidScheduleTime(t *testing.T) {
	tests := []struct {
		scheduleTime string
		expectedErr  string
	}{
		{
			scheduleTime: "25:00",
			expectedErr:  "invalid SCHEDULE_TIME: hour must be 0-23",
		},
		{
			scheduleTime: "23:60",
			expectedErr:  "invalid SCHEDULE_TIME: hour must be 0-23, minute must be 0-59",
		},
		{
			scheduleTime: "invalid",
			expectedErr:  "invalid SCHEDULE_TIME format",
		},
		{
			scheduleTime: "23:invalid",
			expectedErr:  "invalid SCHEDULE_TIME minute",
		},
	}

	for _, tt := range tests {
		t.Run(tt.scheduleTime, func(t *testing.T) {
			cfg := config.Config{
				Timezone:     "UTC",
				ScheduleTime: tt.scheduleTime,
				ZonomiHosts:  []string{"test.host"},
				ZonomiAPIKey: "test-key",
			}
			f := &mockFetcher{}
			logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
			s := New(cfg, f, logger)

			err := s.Run(context.Background())
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}
