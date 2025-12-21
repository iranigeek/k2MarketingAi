package storage

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// InMemoryStore is a thread-safe store used when a database is not configured.
type InMemoryStore struct {
	mu       sync.RWMutex
	listings []Listing
}

// NewInMemoryStore constructs an empty in-memory store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{listings: make([]Listing, 0)}
}

// CreateListing appends a listing to the in-memory slice.
func (s *InMemoryStore) CreateListing(_ context.Context, input Listing) (Listing, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if input.ID == "" {
		input.ID = uuid.NewString()
	}
	if input.CreatedAt.IsZero() {
		input.CreatedAt = time.Now()
	}
	if input.Sections == nil {
		input.Sections = []Section{}
	}
	if input.FullCopy == "" {
		input.FullCopy = ""
	}
	if input.History == nil {
		input.History = History{}
	}
	if input.Status == (Status{}) {
		input.Status = Status{}
	}

	s.listings = append([]Listing{input}, s.listings...)
	if len(s.listings) > 50 {
		s.listings = s.listings[:50]
	}

	return input, nil
}

// ListListings returns a snapshot of stored listings.
func (s *InMemoryStore) ListListings(_ context.Context) ([]Listing, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := make([]Listing, len(s.listings))
	copy(snapshot, s.listings)
	return snapshot, nil
}

// Close satisfies the Store interface.
func (s *InMemoryStore) Close() {}

// GetListing returns a listing by ID.
func (s *InMemoryStore) GetListing(_ context.Context, id string) (Listing, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, l := range s.listings {
		if l.ID == id {
			return l, nil
		}
	}
	return Listing{}, ErrNotFound
}

// UpdateListingSections replaces the sections on a listing.
func (s *InMemoryStore) UpdateListingSections(_ context.Context, id string, sections []Section, fullCopy string, history History, status Status) (Listing, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for idx, l := range s.listings {
		if l.ID == id {
			s.listings[idx].Sections = sections
			s.listings[idx].FullCopy = fullCopy
			s.listings[idx].History = history
			s.listings[idx].Status = status
			return s.listings[idx], nil
		}
	}
	return Listing{}, ErrNotFound
}

// UpdateListingDetails updates the details JSON and cover image.
func (s *InMemoryStore) UpdateListingDetails(_ context.Context, id string, details Details, imageURL string) (Listing, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for idx, l := range s.listings {
		if l.ID == id {
			s.listings[idx].Details = details
			s.listings[idx].ImageURL = imageURL
			return s.listings[idx], nil
		}
	}
	return Listing{}, ErrNotFound
}

// UpdateInsights stores refreshed insights and status for a listing.
func (s *InMemoryStore) UpdateInsights(_ context.Context, id string, insights Insights, status Status) (Listing, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for idx, l := range s.listings {
		if l.ID == id {
			s.listings[idx].Insights = insights
			s.listings[idx].Status = status
			return s.listings[idx], nil
		}
	}
	return Listing{}, ErrNotFound
}

// DeleteListing removes a listing by ID.
func (s *InMemoryStore) DeleteListing(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for idx, l := range s.listings {
		if l.ID == id {
			s.listings = append(s.listings[:idx], s.listings[idx+1:]...)
			return nil
		}
	}
	return ErrNotFound
}

// UpdateStatus sets only the pipeline status for a listing.
func (s *InMemoryStore) UpdateStatus(_ context.Context, id string, status Status) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for idx, l := range s.listings {
		if l.ID == id {
			s.listings[idx].Status = status
			return nil
		}
	}
	return ErrNotFound
}
