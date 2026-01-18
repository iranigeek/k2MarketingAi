package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"strings"

	"k2MarketingAi/internal/config"
	"k2MarketingAi/internal/dataset"
	"k2MarketingAi/internal/storage"
)

func main() {
	var (
		configPath   = flag.String("config", "config.json", "Path to config.json (must include database_url)")
		outputPath   = flag.String("out", "dataset.jsonl", "Where to write the JSONL dataset")
		styleFilter  = flag.String("style-profile", "", "Optional style_profile_id to filter on")
		minSections  = flag.Int("min-sections", 3, "Minimum number of populated sections")
		minWords     = flag.Int("min-words", 120, "Minimum number of words in the final copy")
		includeEmpty = flag.Bool("include-empty-text", false, "Include listings without generated copy")
	)
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if cfg.DatabaseURL == "" {
		log.Fatal("database_url is required in config.json to export datasets")
	}

	ctx := context.Background()
	store, err := storage.NewStore(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect store: %v", err)
	}
	defer store.Close()

	listings, err := store.ListAllListings(ctx)
	if err != nil {
		log.Fatalf("fetch listings: %v", err)
	}
	if len(listings) == 0 {
		log.Fatal("no listings found to export")
	}

	if err := attachStyleProfiles(ctx, store, listings); err != nil {
		log.Fatalf("attach style profiles: %v", err)
	}

	filtered := listings
	if trimmed := strings.TrimSpace(*styleFilter); trimmed != "" {
		filtered = filterByStyle(listings, trimmed)
		if len(filtered) == 0 {
			log.Fatalf("no listings matched style profile %s", trimmed)
		}
	}

	opts := dataset.Options{MinSections: *minSections, MinWords: *minWords}
	examples, err := dataset.BuildExamples(filtered, opts)
	if err != nil {
		log.Fatalf("build dataset: %v", err)
	}
	if len(examples) == 0 && !*includeEmpty {
		log.Fatal("no examples matched the provided filters")
	}

	if len(examples) == 0 && *includeEmpty {
		log.Println("warning: dataset is empty but include-empty-text flag allowed continuation")
	}

	if err := dataset.WriteJSONL(*outputPath, examples); err != nil {
		log.Fatalf("write dataset: %v", err)
	}
	log.Printf("exported %d examples to %s", len(examples), *outputPath)
}

func attachStyleProfiles(ctx context.Context, store storage.Store, listings []storage.Listing) error {
	cache := make(map[string]*storage.StyleProfile)
	for i := range listings {
		styleID := strings.TrimSpace(listings[i].Details.Meta.StyleProfileID)
		if styleID == "" {
			continue
		}
		if cached, ok := cache[styleID]; ok {
			listings[i].StyleProfile = cached
			continue
		}
		profile, err := store.GetStyleProfile(ctx, styleID)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				log.Printf("style profile %s missing, skipping listing %s", styleID, listings[i].ID)
				continue
			}
			return fmt.Errorf("fetch style profile %s: %w", styleID, err)
		}
		profileCopy := profile
		cache[styleID] = &profileCopy
		listings[i].StyleProfile = &profileCopy
	}
	return nil
}

func filterByStyle(listings []storage.Listing, styleID string) []storage.Listing {
	var filtered []storage.Listing
	for _, listing := range listings {
		if strings.EqualFold(strings.TrimSpace(listing.Details.Meta.StyleProfileID), styleID) {
			filtered = append(filtered, listing)
		}
	}
	return filtered
}
