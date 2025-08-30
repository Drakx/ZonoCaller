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
	assert.Equal(t, scheduled, nextRun)
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
