package telemetry

import (
	"context"
	"time"
)

// TimedExecution tracks execution duration and delegates to Tracker.
type TimedExecution struct {
	tracker   *Tracker
	startTime time.Time
}

// Start creates a new TimedExecution.
func Start(tracker *Tracker) *TimedExecution {
	return &TimedExecution{
		tracker:   tracker,
		startTime: time.Now(),
	}
}

// Track records a filtered command with elapsed duration.
func (te *TimedExecution) Track(ctx context.Context, originalCmd, scouterCmd string, inputTokens, outputTokens int) error {
	if te.tracker == nil {
		return nil
	}
	ms := time.Since(te.startTime).Milliseconds()
	return te.tracker.Track(ctx, originalCmd, scouterCmd, inputTokens, outputTokens, ms)
}

// TrackPassthrough records a passthrough command with elapsed duration.
func (te *TimedExecution) TrackPassthrough(ctx context.Context, cmd string, tokens int) error {
	if te.tracker == nil {
		return nil
	}
	ms := time.Since(te.startTime).Milliseconds()
	return te.tracker.TrackPassthrough(ctx, cmd, tokens, ms)
}
