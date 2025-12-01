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
	"k2MarketingAi/internal/media"
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

	uploader, err := media.NewUploader(ctx, media.Config{
		Bucket:         cfg.Media.Bucket,
		Region:         cfg.Media.Region,
		Endpoint:       cfg.Media.Endpoint,
		PublicURL:      cfg.Media.PublicURL,
		KeyPrefix:      cfg.Media.KeyPrefix,
		ForcePathStyle: cfg.Media.ForcePathStyle,
	})
	if err != nil {
		log.Fatalf("failed to init media uploader: %v", err)
	}

	listingHandler := listings.Handler{Store: store, Uploader: uploader}

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
