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

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"k2MarketingAi/internal/auth"
	"k2MarketingAi/internal/config"
	"k2MarketingAi/internal/events"
	"k2MarketingAi/internal/generation"
	"k2MarketingAi/internal/geodata"
	"k2MarketingAi/internal/listings"
	"k2MarketingAi/internal/llm"
	"k2MarketingAi/internal/media"
	"k2MarketingAi/internal/server"
	"k2MarketingAi/internal/storage"
	"k2MarketingAi/internal/vision"
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
			Bucket:          cfg.Media.Bucket,
			Region:          cfg.Media.Region,
			Endpoint:        cfg.Media.Endpoint,
			PublicURL:       cfg.Media.PublicURL,
			KeyPrefix:       cfg.Media.KeyPrefix,
			ForcePathStyle:  cfg.Media.ForcePathStyle,
			AccessKeyID:     cfg.Media.AccessKeyID,
			SecretAccessKey: cfg.Media.SecretAccessKey,
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

	var (
		generator      generation.Generator
		visionAnalyzer vision.Analyzer
		visionDesigner vision.Designer
		visionRenderer vision.ImageGenerator
		imagenRenderer vision.ImagenClient
	)
	var geminiTokenSource oauth2.TokenSource
	if tokenBytes, err := loadServiceAccountJSON(cfg.AI.Gemini.ServiceAccount, cfg.AI.Gemini.ServiceAccountJSON); err != nil {
		log.Fatalf("failed to load gemini service account: %v", err)
	} else if len(tokenBytes) > 0 {
		creds, err := google.CredentialsFromJSON(ctx, tokenBytes, "https://www.googleapis.com/auth/generative-language")
		if err != nil {
			log.Fatalf("failed to parse gemini service account: %v", err)
		}
		geminiTokenSource = creds.TokenSource
	}

	switch {
	case strings.EqualFold(cfg.AI.Provider, "gemini") && (cfg.AI.Gemini.APIKey != "" || geminiTokenSource != nil):
		timeout := time.Duration(cfg.AI.Gemini.TimeoutSeconds) * time.Second
		geminiClient := llm.NewGeminiClient(cfg.AI.Gemini.APIKey, cfg.AI.Gemini.Model, timeout, geminiTokenSource)
		generator = generation.NewLLM(geminiClient)
		visionAnalyzer = vision.NewGeminiAnalyzer(cfg.AI.Gemini.APIKey, cfg.AI.Gemini.VisionModel, timeout)
		visionDesigner = vision.NewGeminiDesigner(geminiClient)
		visionRenderer = vision.NewGeminiImageGenerator(cfg.AI.Gemini.APIKey, cfg.AI.Gemini.ImageModel, timeout)
		log.Println("generator ready: Gemini")
	default:
		generator = generation.NewHeuristic()
		log.Println("generator ready: heuristic fallback")
	}
	if cfg.AI.Imagen.Enabled && cfg.AI.Imagen.ProjectID != "" {
		imagenRenderer = vision.NewVertexImagen(vision.VertexImagenConfig{
			ProjectID:          cfg.AI.Imagen.ProjectID,
			Location:           cfg.AI.Imagen.Location,
			Model:              cfg.AI.Imagen.Model,
			APIKey:             cfg.AI.Gemini.APIKey,
			ServiceAccount:     cfg.AI.Imagen.ServiceAccount,
			ServiceAccountJSON: cfg.AI.Imagen.ServiceAccountJSON,
		}, uploader)
	} else if cfg.AI.Imagen.Enabled {
		log.Println("imagen renderer disabled: missing project id")
	} else {
		log.Println("imagen renderer disabled via config")
	}

	eventBroker := events.NewBroker()
	sessionManager := auth.SessionManager{
		Secret:       []byte(cfg.Auth.Secret),
		Duration:     time.Duration(cfg.Auth.SessionHours) * time.Hour,
		CookieName:   cfg.Auth.CookieName,
		SecureCookie: cfg.Auth.SecureCookie,
	}
	authHandler := auth.Handler{
		Store:    store,
		Sessions: sessionManager,
	}
	authMiddleware := auth.Middleware{
		Store:    store,
		Sessions: sessionManager,
	}

	listingHandler := listings.Handler{
		Store:       store,
		Uploader:    uploader,
		GeoProvider: geoProvider,
		Generator:   generator,
		Vision:      visionAnalyzer,
		Events:      eventBroker,
	}

	staticFS := http.FileServer(http.Dir("web"))
	visionHandler := vision.Handler{
		Analyzer: visionAnalyzer,
		Designer: visionDesigner,
		Renderer: visionRenderer,
		Imagen:   imagenRenderer,
	}
	srv := server.New(cfg.Port, authHandler, authMiddleware, listingHandler, visionHandler, staticFS)

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

func loadServiceAccountJSON(path, inline string) ([]byte, error) {
	trimmed := strings.TrimSpace(inline)
	if trimmed != "" {
		return []byte(trimmed), nil
	}
	p := strings.TrimSpace(path)
	if p == "" {
		return nil, nil
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	return data, nil
}
