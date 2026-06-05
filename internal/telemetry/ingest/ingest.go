package ingest

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	"github.com/Rogercode97/scouter/internal/store"
)

// Ingest reads JSON-Lines from reader, parses them into UsageRecords, and saves to store.
func Ingest(ctx context.Context, reader io.Reader, env string, st store.Store) error {
	scanner := bufio.NewScanner(reader)
	var usages []store.UsageRecord

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var record store.UsageRecord
		if err := json.Unmarshal(line, &record); err != nil {
			slog.Warn("skipping malformed telemetry line", "line", lineNum, "error", err)
			continue
		}

		// Basic validation
		if record.SymbolName == "" || record.SymbolPath == "" {
			slog.Warn("skipping telemetry line with missing fields", "line", lineNum)
			continue
		}

		if record.HitCount <= 0 {
			record.HitCount = 1
		}

		usages = append(usages, record)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading telemetry stream: %w", err)
	}

	if len(usages) > 0 {
		if err := st.RecordSymbolUsage(ctx, env, usages); err != nil {
			return fmt.Errorf("failed to record symbol usages: %w", err)
		}
	}

	return nil
}
