package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/RoboConOxfordshire/shepherd/internal/config"
	"github.com/RoboConOxfordshire/shepherd/internal/server"
)

func main() {
	cfg := config.Load()

	srv, err := server.New(cfg, staticFS)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	if err := srv.Run(ctx); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
