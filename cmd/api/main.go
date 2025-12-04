package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"k2MarketingAi/internal/config"
	"k2MarketingAi/internal/events"
	"k2MarketingAi/internal/generation"
	"k2MarketingAi/internal/geodata"
	"k2MarketingAi/internal/listings"
	"k2MarketingAi/internal/llm"
	"k2MarketingAi/internal/media"
	"k2MarketingAi/internal/server"
	"k2MarketingAi/internal/storage"
)

func main() {
	cfg, err := config.Load("config.json")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ctx := context.Background()
	store, err := storage.NewStore(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to init store: %v", err)
	}
	defer store.Close()

	var uploader media.Uploader
	if cfg.Media.Bucket != "" && cfg.Media.Region != "" {
		uploader, err = media.NewUploader(ctx, media.Config{
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
	} else {
		uploader, err = media.NewLocalUploader("")
		if err != nil {
			log.Fatalf("failed to init local media storage: %v", err)
		}
		log.Println("media uploader: using local temp storage (S3 config missing)")
	}

	geoProvider := geodata.NewProvider(geodata.Config{
		GooglePlacesAPIKey: cfg.Geodata.GooglePlacesAPIKey,
		TrafficAPIKey:      cfg.Geodata.TrafficAPIKey,
		CacheTTL:           time.Duration(cfg.Geodata.CacheTTLMinutes) * time.Minute,
	})

	var generator generation.Generator
	if strings.EqualFold(cfg.AI.Provider, "openai") && cfg.AI.OpenAI.APIKey != "" {
		openAIClient := llm.NewOpenAIClient(cfg.AI.OpenAI.APIKey, cfg.AI.OpenAI.Model)
		generator = generation.NewOpenAI(openAIClient)
		log.Println("generator ready: OpenAI")
	} else {
		generator = generation.NewHeuristic()
		log.Println("generator ready: heuristic fallback")
	}

	eventBroker := events.NewBroker()

	listingHandler := listings.Handler{
		Store:       store,
		Uploader:    uploader,
		GeoProvider: geoProvider,
		Generator:   generator,
		Events:      eventBroker,
	}

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
