package engram

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Rogercode97/scouter/internal/domain/memory"
	"github.com/Rogercode97/scouter/internal/utils"
)

/**
 * ⚔️ HAKAISHIN ENGRAM ADAPTER (WAVE 7)
 * Persists distilled summaries via the 'engram' CLI.
 */
type EngramRepository struct {
	// dryRun mode for safe auditing
	dryRun bool
}

func NewEngramRepository(dryRun bool) *EngramRepository {
	return &EngramRepository{
		dryRun: dryRun,
	}
}

func (r *EngramRepository) SaveSummary(ctx context.Context, project string, summary memory.Summary) error {
	today := time.Now().Format("2006-01-02")
	title := fmt.Sprintf("Daily Distillation: %s", today)
	topicKey := fmt.Sprintf("distillation/daily/%s", today)

	markdown := r.formatMarkdown(summary)

	if r.dryRun {
		fmt.Printf("\n--- [DRY-RUN] Distilled Summary for %s ---\n", project)
		fmt.Println(markdown)
		fmt.Println("--- [DRY-RUN] End Summary ---")
		return nil
	}

	cmd, err := utils.SafeCommand(ctx, "engram", "save",
		title,
		markdown,
		"--project", project,
		"--type", "architecture",
		"--topic", topicKey,
	)
	if err != nil {
		return err
	}

	cmd.Stdin = strings.NewReader(markdown)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("engram CLI failed: %v\nStderr: %s", err, stderr.String())
	}

	return nil
}

func (r *EngramRepository) formatMarkdown(s memory.Summary) string {
	var sb strings.Builder
	sb.WriteString("# Engram Distillation Summary\n\n")

	sb.WriteString("## Architectural Decisions\n")
	if len(s.ADRs) == 0 {
		sb.WriteString("- No significant architectural decisions detected.\n")
	} else {
		for _, adr := range s.ADRs {
			sb.WriteString("- ")
			sb.WriteString(adr)
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n## Root Cause Bug Fixes\n")
	if len(s.BugFixes) == 0 {
		sb.WriteString("- No root cause bug fixes identified.\n")
	} else {
		for _, bf := range s.BugFixes {
			sb.WriteString("- ")
			sb.WriteString(bf)
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n## Established Patterns\n")
	if len(s.Patterns) == 0 {
		sb.WriteString("- No new patterns or conventions found.\n")
	} else {
		for _, p := range s.Patterns {
			sb.WriteString("- ")
			sb.WriteString(p)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
