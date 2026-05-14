package event

import (
	"sync"
)

// EventStore defines the interface for storing and retrieving dispatched events.
type EventStore interface {
	// Store saves an event.
	Store(event Event) error

	// GetEvents retrieves all stored events of a specific type.
	// If eventType is empty, it returns all events.
	GetEvents(eventType string) ([]Event, error)

	// Clear removes all stored events.
	Clear() error
}

// InMemoryEventStore is a simple thread-safe, in-memory implementation of EventStore.
type InMemoryEventStore struct {
	mu     sync.RWMutex
	events []Event
}

// NewInMemoryEventStore creates a new InMemoryEventStore.
func NewInMemoryEventStore() *InMemoryEventStore {
	return &InMemoryEventStore{
		events: make([]Event, 0),
	}
}

// Store saves an event.
func (s *InMemoryEventStore) Store(event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}

// GetEvents retrieves all stored events of a specific type.
// If eventType is empty, it returns all events.
func (s *InMemoryEventStore) GetEvents(eventType string) ([]Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if eventType == "" {
		eventsCopy := make([]Event, len(s.events))
		copy(eventsCopy, s.events)
		return eventsCopy, nil
	}

	var filtered []Event
	for _, e := range s.events {
		if e.GetType() == eventType {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}

// Clear removes all stored events.
func (s *InMemoryEventStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = make([]Event, 0)
	return nil
}
