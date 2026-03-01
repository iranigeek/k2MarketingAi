package storage

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// InMemoryStore is a thread-safe store used when a database is not configured.
type InMemoryStore struct {
	mu            sync.RWMutex
	listings      []Listing
	styleProfiles map[string]StyleProfile
	templates     map[string]Template
	users         map[string]User
	emailIndex    map[string]string
}

// NewInMemoryStore constructs an empty in-memory store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		listings:      make([]Listing, 0),
		styleProfiles: make(map[string]StyleProfile),
		templates:     make(map[string]Template),
		users:         make(map[string]User),
		emailIndex:    make(map[string]string),
	}
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

// ListListingsByOwner returns listings belonging to the provided owner.
func (s *InMemoryStore) ListListingsByOwner(_ context.Context, ownerID string) ([]Listing, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []Listing
	for _, l := range s.listings {
		if l.OwnerID == ownerID {
			results = append(results, l)
		}
	}
	return results, nil
}

// ListAllListings returns the same snapshot as ListListings for the in-memory store.
func (s *InMemoryStore) ListAllListings(ctx context.Context) ([]Listing, error) {
	return s.ListListings(ctx)
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

// SaveStyleProfile stores or updates a style profile in memory.
func (s *InMemoryStore) SaveStyleProfile(_ context.Context, profile StyleProfile) (StyleProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if profile.ID == "" {
		profile.ID = uuid.NewString()
		profile.CreatedAt = now
	} else if existing, ok := s.styleProfiles[profile.ID]; ok {
		if profile.CreatedAt.IsZero() {
			profile.CreatedAt = existing.CreatedAt
		}
	}
	if profile.CreatedAt.IsZero() {
		profile.CreatedAt = now
	}
	profile.UpdatedAt = now
	s.styleProfiles[profile.ID] = profile
	return profile, nil
}

// ListStyleProfiles returns all profiles.
func (s *InMemoryStore) ListStyleProfiles(_ context.Context) ([]StyleProfile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	profiles := make([]StyleProfile, 0, len(s.styleProfiles))
	for _, profile := range s.styleProfiles {
		profiles = append(profiles, profile)
	}
	return profiles, nil
}

// GetStyleProfile returns a profile by ID.
func (s *InMemoryStore) GetStyleProfile(_ context.Context, id string) (StyleProfile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	profile, ok := s.styleProfiles[id]
	if !ok {
		return StyleProfile{}, ErrNotFound
	}
	return profile, nil
}

// CreateUser stores a new user in memory.
func (s *InMemoryStore) CreateUser(_ context.Context, user User) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	email := strings.ToLower(strings.TrimSpace(user.Email))
	if email == "" {
		return User{}, fmt.Errorf("email is required")
	}
	if _, exists := s.emailIndex[email]; exists {
		return User{}, ErrUserExists
	}
	if user.ID == "" {
		user.ID = uuid.NewString()
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now()
	}
	user.Email = email
	user.Approved = true
	s.users[user.ID] = user
	s.emailIndex[email] = user.ID
	return user, nil
}

// GetUserByEmail fetches a user using their email.
func (s *InMemoryStore) GetUserByEmail(_ context.Context, email string) (User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, ok := s.emailIndex[strings.ToLower(strings.TrimSpace(email))]
	if !ok {
		return User{}, ErrNotFound
	}
	user := s.users[id]
	return user, nil
}

// GetUserByID fetches a user using their ID.
func (s *InMemoryStore) GetUserByID(_ context.Context, id string) (User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.users[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return user, nil
}

// ApproveUser updates approval flag.
func (s *InMemoryStore) ApproveUser(_ context.Context, id string, approved bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[id]
	if !ok {
		return ErrNotFound
	}
	user.Approved = approved
	s.users[id] = user
	return nil
}

// ListUsers returns all users.
func (s *InMemoryStore) ListUsers(_ context.Context) ([]User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	users := make([]User, 0, len(s.users))
	for _, u := range s.users {
		users = append(users, u)
	}
	return users, nil
}

// DeleteUser removes a user by id.
func (s *InMemoryStore) DeleteUser(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[id]; !ok {
		return ErrNotFound
	}
	delete(s.emailIndex, strings.ToLower(s.users[id].Email))
	delete(s.users, id)
	return nil
}

// UpdateUserStripe updates stripe-related fields on a user in memory.
func (s *InMemoryStore) UpdateUserStripe(_ context.Context, userID, stripeCustomerID, subscriptionID, subscriptionStatus, planID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[userID]
	if !ok {
		return ErrNotFound
	}
	user.StripeCustomerID = stripeCustomerID
	user.SubscriptionID = subscriptionID
	user.SubscriptionStatus = subscriptionStatus
	user.PlanID = planID
	s.users[userID] = user
	return nil
}

// GetUserByStripeCustomer fetches a user by Stripe customer ID in memory.
func (s *InMemoryStore) GetUserByStripeCustomer(_ context.Context, stripeCustomerID string) (User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.users {
		if u.StripeCustomerID == stripeCustomerID {
			return u, nil
		}
	}
	return User{}, ErrNotFound
}

// IncrementUsage atomically increments the usage counter in memory.
func (s *InMemoryStore) IncrementUsage(_ context.Context, userID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[userID]
	if !ok {
		return 0, ErrNotFound
	}
	user.UsageCount++
	s.users[userID] = user
	return user.UsageCount, nil
}

// ResetUsage sets the usage counter back to zero in memory.
func (s *InMemoryStore) ResetUsage(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[userID]
	if !ok {
		return ErrNotFound
	}
	user.UsageCount = 0
	s.users[userID] = user
	return nil
}

// ── Template CRUD (in-memory) ──

// SaveTemplate stores or updates a template in memory.
func (s *InMemoryStore) SaveTemplate(_ context.Context, tpl Template) (Template, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if tpl.ID == "" {
		tpl.ID = uuid.NewString()
		tpl.CreatedAt = now
	} else if existing, ok := s.templates[tpl.ID]; ok {
		if tpl.CreatedAt.IsZero() {
			tpl.CreatedAt = existing.CreatedAt
		}
	}
	if tpl.CreatedAt.IsZero() {
		tpl.CreatedAt = now
	}
	tpl.UpdatedAt = now
	s.templates[tpl.ID] = tpl
	return tpl, nil
}

// ListTemplatesByOwner returns templates for a given owner.
func (s *InMemoryStore) ListTemplatesByOwner(_ context.Context, ownerID string) ([]Template, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []Template
	for _, tpl := range s.templates {
		if tpl.OwnerID == ownerID {
			result = append(result, tpl)
		}
	}
	return result, nil
}

// GetTemplate returns a template by ID.
func (s *InMemoryStore) GetTemplate(_ context.Context, id string) (Template, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tpl, ok := s.templates[id]
	if !ok {
		return Template{}, ErrNotFound
	}
	return tpl, nil
}

// DeleteTemplate removes a template by ID.
func (s *InMemoryStore) DeleteTemplate(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.templates[id]; !ok {
		return ErrNotFound
	}
	delete(s.templates, id)
	return nil
}
