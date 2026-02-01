package events

import (
	"sync"

	"k2MarketingAi/internal/storage"
)

// Event describes a status update for a listing.
type Event struct {
	ListingID string         `json:"listing_id"`
	OwnerID   string         `json:"owner_id,omitempty"`
	Status    storage.Status `json:"status"`
}

// Broker manages SSE subscribers.
type Broker struct {
	mu          sync.RWMutex
	subscribers map[chan Event]struct{}
}

// NewBroker constructs a broker instance.
func NewBroker() *Broker {
	return &Broker{
		subscribers: make(map[chan Event]struct{}),
	}
}

// Subscribe returns a channel that receives events.
func (b *Broker) Subscribe() chan Event {
	ch := make(chan Event, 8)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes the channel from the broker.
func (b *Broker) Unsubscribe(ch chan Event) {
	b.mu.Lock()
	delete(b.subscribers, ch)
	b.mu.Unlock()
	close(ch)
}

// Publish fan-outs the event to all subscribers.
func (b *Broker) Publish(evt Event) {
	b.mu.RLock()
	for ch := range b.subscribers {
		select {
		case ch <- evt:
		default:
			// drop if subscriber is slow
		}
	}
	b.mu.RUnlock()
}
