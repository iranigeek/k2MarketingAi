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
