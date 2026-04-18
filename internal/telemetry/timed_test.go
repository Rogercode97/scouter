//go:build !lite

package telemetry

import (
	"context"
	"testing"
)

func TestTimedExecution(t *testing.T) {
	tracker := newTestTracker(t)

	timed := Start(tracker)
	err := timed.Track(context.Background(), "git log", "scouter git log", 500, 100)
	if err != nil {
		t.Fatalf("timed track: %v", err)
	}

	summary, err := tracker.GetSummary(context.Background())
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.TotalCommands != 1 {
		t.Errorf("total commands = %d", summary.TotalCommands)
	}
}

func TestTimedExecutionNilTracker(t *testing.T) {
	timed := Start(nil)
	err := timed.Track(context.Background(), "cmd", "scouter cmd", 100, 50)
	if err != nil {
		t.Fatalf("expected nil tracker to be no-op: %v", err)
	}
	err = timed.TrackPassthrough(context.Background(), "cmd", 100)
	if err != nil {
		t.Fatalf("expected nil tracker passthrough to be no-op: %v", err)
	}
}
