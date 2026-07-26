package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/chengyixu/cheat-engine-cli/internal/cli"
)

func main() {
	contextWithSignal, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(cli.Run(contextWithSignal, os.Args[1:], os.Stdout, os.Stderr))
}
