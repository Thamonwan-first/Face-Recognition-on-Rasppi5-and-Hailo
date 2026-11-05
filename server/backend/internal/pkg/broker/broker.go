package broker

import (
	"sync"
)

// Event represents an event broadcasted to connected clients
type Event struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
}

// EventBroker coordinates real-time event subscriptions and broadcasting
type EventBroker struct {
	mu          sync.RWMutex
	subscribers map[chan Event]struct{}
}

// NewEventBroker creates an initialized event broker
func NewEventBroker() *EventBroker {
	return &EventBroker{
		subscribers: make(map[chan Event]struct{}),
	}
}

// Subscribe registers a new subscriber channel
func (b *EventBroker) Subscribe() chan Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan Event, 32)
	b.subscribers[ch] = struct{}{}
	return ch
}

// Unsubscribe removes and closes a subscriber channel
func (b *EventBroker) Unsubscribe(ch chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.subscribers[ch]; ok {
		delete(b.subscribers, ch)
		close(ch)
	}
}

// Publish sends an event to all active subscribers without blocking
func (b *EventBroker) Publish(evt Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subscribers {
		select {
		case ch <- evt:
		default:
			// Buffer full, drop to prevent blocking the publisher
		}
	}
}
