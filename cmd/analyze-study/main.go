package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/Rogercode97/scouter/internal/config"
	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/store"
)

func main() {
	ctx := context.Background()
	cfg, err := config.Load(ctx)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	db, err := store.New(ctx, cfg.Tracking.DBPath)
	if err != nil {
		log.Fatalf("failed to open store: %v", err)
	}
	defer db.Close()

	analyzer := engine.NewAnalysisEngine(db)
	_ = analyzer.ResolveCentrality(ctx)
	
	symbols, err := analyzer.GetCriticalSymbols(ctx, 100) // Increase limit to find study-claude symbols
	if err != nil {
		log.Fatalf("failed to get critical symbols: %v", err)
	}

	fmt.Println("Critical Symbols in study-claude:")
	count := 0
	for _, s := range symbols {
		if strings.Contains(s.Path, "study-claude") {
			fmt.Printf("- %s (File: %s, Centrality: %d, Fragility: %d)\n", s.Name, s.Path, s.Centrality, s.Fragility)
			count++
			if count >= 10 {
				break
			}
		}
	}
}
