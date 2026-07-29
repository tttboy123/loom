package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"loom-pi-rebuild/internal/credentials"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--credential-helper" {
		if credentials.RunProductKeychainHelper() == nil {
			os.Exit(0)
		}
		os.Exit(4)
	}
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr, nil))
}
