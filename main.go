package main

import (
	"chatwoot-sync-go/internal/config"
	"chatwoot-sync-go/internal/sync"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	syncService := sync.NewService(cfg)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	done := make(chan bool)
	go func() {
		if err := syncService.Start(); err != nil {
			log.Fatalf("Sync service failed: %v", err)
		}
		done <- true
	}()

	select {
	case <-sigChan:
		log.Println("Received interrupt signal, shutting down...")
		syncService.Stop()
	case <-done:
		log.Println("Sync completed successfully")
	}
}

