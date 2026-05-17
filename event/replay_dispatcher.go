package event

import (
	"sync"
)

// ReplayEventDispatcher wraps an EventDispatcher and stores the last N events per type.
// When a new listener registers, it immediately replays those historical events to the listener.
type ReplayEventDispatcher struct {
	*EventDispatcher
	historySize int
	historyMu   sync.RWMutex
	history     map[string][]Event
}

// NewReplayEventDispatcher creates a new ReplayEventDispatcher keeping up to historySize events.
func NewReplayEventDispatcher(historySize int) *ReplayEventDispatcher {
	return &ReplayEventDispatcher{
		EventDispatcher: NewEventDispatcher(),
		historySize:     historySize,
		history:         make(map[string][]Event),
	}
}

// RegisterListener registers a listener function and immediately replays historical events.
func (d *ReplayEventDispatcher) RegisterListener(eventType string, listener func(Event)) {
	// First register the listener
	d.EventDispatcher.RegisterListener(eventType, listener)

	// Replay history
	d.historyMu.RLock()
	events, ok := d.history[eventType]
	if !ok {
		d.historyMu.RUnlock()
		return
	}
	targets := make([]Event, len(events))
	copy(targets, events)
	d.historyMu.RUnlock()

	// Dispatch historical events synchronously to this single listener
	// to ensure they are processed before any new ones arrive.
	for _, e := range targets {
		listener(e)
	}
}

// Dispatch sends the event to all listeners and stores it in the history.
func (d *ReplayEventDispatcher) Dispatch(event Event) {
	d.historyMu.Lock()
	eventType := event.GetType()
	d.history[eventType] = append(d.history[eventType], event)
	if len(d.history[eventType]) > d.historySize {
		// remove oldest
		d.history[eventType] = d.history[eventType][1:]
	}
	d.historyMu.Unlock()

	d.EventDispatcher.Dispatch(event)
}
