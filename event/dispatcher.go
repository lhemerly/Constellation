package event

import (
	"strings"
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

// matchTopic determines if a dispatched topic matches a registered pattern.
// Supported wildcard: '*' matches a single segment (or rest of string if last).
func matchTopic(pattern, topic string) bool {
	if pattern == "*" {
		return true
	}

	pParts := strings.Split(pattern, ".")
	tParts := strings.Split(topic, ".")

	for i, p := range pParts {
		if p == "*" {
			// If '*' is the last part of the pattern, it matches everything remaining.
			if i == len(pParts)-1 {
				return true
			}
			continue
		}
		if i >= len(tParts) || p != tParts[i] {
			return false
		}
	}

	return len(pParts) == len(tParts)
}

// Dispatch sends event to every listener whose registered pattern matches event.GetType().
// Each listener is called in a separate goroutine so that slow listeners
// cannot block each other. Dispatch waits for all listener goroutines to
// finish before returning, giving callers a clear synchronisation point.
func (d *EventDispatcher) Dispatch(event Event) {
	evtType := event.GetType()

	d.mu.RLock()
	var targets []func(Event)
	for pattern, funcs := range d.listeners {
		if pattern == evtType || matchTopic(pattern, evtType) {
			targets = append(targets, funcs...)
		}
	}
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
