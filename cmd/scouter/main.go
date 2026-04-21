package main

import (
	"context"
	"os"

	"github.com/Rogercode97/scouter/internal/cli"
)

func main() {
	ctx := context.Background()
	os.Exit(cli.Run(ctx, os.Args))
}
