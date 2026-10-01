package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/ernat-soltanbekov/swap-sort/internal/ai"
	"github.com/ernat-soltanbekov/swap-sort/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	// Interrupt also unblocks an interactive read, not only an HTTP request.
	go func() { <-ctx.Done(); _ = os.Stdin.Close() }()
	code := cli.AICoach(ctx, os.Args[1:], ai.FromEnvironment(), os.Stdin, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}
