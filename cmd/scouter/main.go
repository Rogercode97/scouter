package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/Rogercode97/scouter/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Inject real dependencies into the testable Run function
	exitCode := cli.Run(ctx, os.Args, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}
