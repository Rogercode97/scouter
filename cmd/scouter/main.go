package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/Rogercode97/scouter/cmd/scouter/scoutercmd"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	exitCode := scoutercmd.Execute(ctx, os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}
