package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/Rogercode97/scouter/internal/config"
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

	count := 0
	for sym, err := range db.GetAllSymbols(ctx) {
		if err != nil {
			log.Printf("error getting symbol: %v", err)
			continue
		}
		if strings.Contains(sym.Path, "study-claude") {
			count++
		}
	}

	fmt.Printf("Total symbols found for study-claude: %d\n", count)
}
