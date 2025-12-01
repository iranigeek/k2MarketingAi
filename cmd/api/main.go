package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"k2MarketingAi/internal/config"
	"k2MarketingAi/internal/listings"
	"k2MarketingAi/internal/server"
	"k2MarketingAi/internal/storage"
)

func main() {
	cfg := config.FromEnv()

	ctx := context.Background()
	store, err := storage.NewStore(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to init store: %v", err)
	}
	defer store.Close()

	listingHandler := listings.Handler{Store: store}

	staticFS := http.FileServer(http.Dir("web"))
	srv := server.New(cfg.Port, listingHandler, staticFS)

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-shutdownChan
		log.Println("shutting down server...")
		if err := srv.Close(); err != nil {
			log.Printf("server close error: %v", err)
		}
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}
}
