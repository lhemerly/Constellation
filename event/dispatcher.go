package event

import (
	"sync"
)

// EventDispatcher manages event listeners and dispatches events to them.
// It is safe for concurrent use by multiple goroutines.
// Events are dispatched asynchronously: each registered listener is invoked
// in its own goroutine so that slow listeners cannot block the dispatcher.
type EventDispatcher struct {
	mu        sync.RWMutex
	listeners map[string][]func(Event)
}

// NewEventDispatcher creates and returns a new, empty EventDispatcher.
func NewEventDispatcher() *EventDispatcher {
	return &EventDispatcher{
		listeners: make(map[string][]func(Event)),
	}
}

// RegisterListener registers a listener function for the given event type.
// Multiple listeners may be registered for the same type; they are all called
// when a matching event is dispatched.
func (d *EventDispatcher) RegisterListener(eventType string, listener func(Event)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.listeners[eventType] = append(d.listeners[eventType], listener)
}

// RegisterFilteredListener registers a listener function for the given event type
// that will only be invoked if the provided filter function returns true for the event.
func (d *EventDispatcher) RegisterFilteredListener(eventType string, filter func(Event) bool, listener func(Event)) {
	filteredListener := func(event Event) {
		if filter(event) {
			listener(event)
		}
	}
	d.RegisterListener(eventType, filteredListener)
}

// Dispatch sends event to every listener registered for event.GetType().
// Each listener is called in a separate goroutine so that slow listeners
// cannot block each other. Dispatch waits for all listener goroutines to
// finish before returning, giving callers a clear synchronisation point.
func (d *EventDispatcher) Dispatch(event Event) {
	d.mu.RLock()
	listeners := d.listeners[event.GetType()]
	// Copy the slice under the read-lock so we can release the lock before
	// spawning goroutines, keeping the critical section short.
	targets := make([]func(Event), len(listeners))
	copy(targets, listeners)
	d.mu.RUnlock()

	var wg sync.WaitGroup
	for _, l := range targets {
		wg.Add(1)
		go func(fn func(Event)) {
			defer wg.Done()
			fn(event)
		}(l)
	}
	wg.Wait()
}
