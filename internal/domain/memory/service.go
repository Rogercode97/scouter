package memory

import (
	"context"
	"fmt"
)

/**
 * ⚔️ HAKAISHIN DOMAIN SERVICE (WAVE 7)
 * Orchestrates the Kairos Memory Distillation flow.
 */
type AppService struct {
	memory    MemoryProvider
	distiller Distiller
}

func NewAppService(memory MemoryProvider, distiller Distiller) *AppService {
	return &AppService{
		memory:    memory,
		distiller: distiller,
	}
}

func (s *AppService) SetDistiller(distiller Distiller) {
	s.distiller = distiller
}

/**
 * DistillAndSave: Core logic for memory purification.
 * Decoupled from CLI flags, SQLite, and Gemini SDK.
 */
func (s *AppService) DistillAndSave(ctx context.Context, project string, hours int) error {
	// (1) Extract
	observations, err := s.memory.GetRecentObservations(ctx, project, hours)
	if err != nil {
		return fmt.Errorf("[HAKAI] Failed to extract observations: %w", err)
	}

	if len(observations) == 0 {
		return fmt.Errorf("[HAKAI] No recent observations found for project '%s'", project)
	}

	// (2) Distill
	summary, err := s.distiller.Distill(ctx, observations)
	if err != nil {
		return fmt.Errorf("[HAKAI] Distillation failed: %w", err)
	}

	// (3) Persist
	err = s.memory.SaveSummary(ctx, project, summary)
	if err != nil {
		return fmt.Errorf("[HAKAI] Failed to save distilled summary: %w", err)
	}

	return nil
}

/**
 * PassiveDistill: Background distillation of session highlights.
 * Triggered automatically at turn ends.
 */
func (s *AppService) PassiveDistill(ctx context.Context, project string, transcript []Message) error {
	// (1) Distill Transcript
	memories, err := s.distiller.DistillTranscript(ctx, transcript)
	if err != nil {
		return fmt.Errorf("[HAKAI] Passive distillation failed: %w", err)
	}

	if len(memories) == 0 {
		return nil
	}

	// (2) Deduplication Check
	// Fetch observations from the last 24 hours to avoid duplicates
	recent, err := s.memory.GetRecentObservations(ctx, project, 24)
	if err != nil {
		// Log error but continue (fail-safe)
		fmt.Printf("[HAKAI] Warning: Failed to fetch recent observations for deduplication: %v\n", err)
	}

	recentMap := make(map[string]bool)
	for _, obs := range recent {
		recentMap[obs.Content] = true
	}

	// (3) Save new memories
	for _, mem := range memories {
		if recentMap[mem.Content] {
			continue // Skip duplicate
		}

		err := s.memory.SaveObservation(ctx, project, mem)
		if err != nil {
			fmt.Printf("[HAKAI] Warning: Failed to save passive observation: %v\n", err)
		}
	}

	return nil
}
