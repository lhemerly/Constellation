package event

import (
	"sync"
)

// StreamEventDispatcher extends EventDispatcher to handle streaming events.
// In addition to the standard event listener functionality it supports
// registering stream listeners and dispatching StreamEvent values to them.
//
// Stream events are dispatched through a per-type buffered channel so that
// producers are never blocked by slow consumers (non-blocking I/O).
// A background goroutine per event type drains the channel and forwards
// each chunk to all registered stream listeners.
type StreamEventDispatcher struct {
	EventDispatcher

	streamMu        sync.RWMutex
	streamListeners map[string][]func(StreamEvent)
	channels        map[string]chan StreamEvent
}

const defaultStreamBufferSize = 256

// NewStreamEventDispatcher creates and returns a new StreamEventDispatcher.
func NewStreamEventDispatcher() *StreamEventDispatcher {
	return &StreamEventDispatcher{
		EventDispatcher: EventDispatcher{
			listeners: make(map[string][]func(Event)),
		},
		streamListeners: make(map[string][]func(StreamEvent)),
		channels:        make(map[string]chan StreamEvent),
	}
}

// RegisterStreamListener registers a listener for streaming events of the
// given type. It is safe to call concurrently with DispatchStream.
func (d *StreamEventDispatcher) RegisterStreamListener(eventType string, listener func(StreamEvent)) {
	d.streamMu.Lock()
	defer d.streamMu.Unlock()

	d.streamListeners[eventType] = append(d.streamListeners[eventType], listener)

	// Lazily create the dispatch channel and its drain goroutine the first time
	// a listener is registered for this event type.
	if _, ok := d.channels[eventType]; !ok {
		ch := make(chan StreamEvent, defaultStreamBufferSize)
		d.channels[eventType] = ch
		go d.drainStream(eventType, ch)
	}
}

// DispatchStream sends a StreamEvent to all registered stream listeners for
// its event type. The send is non-blocking: if the internal buffer is full
// the event is dropped rather than blocking the caller.
func (d *StreamEventDispatcher) DispatchStream(event StreamEvent) {
	d.streamMu.RLock()
	ch, ok := d.channels[event.GetType()]
	d.streamMu.RUnlock()

	if !ok {
		// No listeners registered for this event type; nothing to do.
		return
	}

	// Non-blocking send: drop the event if the buffer is full to avoid
	// blocking the producer.
	select {
	case ch <- event:
	default:
	}
}

// drainStream reads from ch and forwards each StreamEvent to all currently
// registered listeners for eventType. It runs until ch is closed.
func (d *StreamEventDispatcher) drainStream(eventType string, ch <-chan StreamEvent) {
	for event := range ch {
		d.streamMu.RLock()
		listeners := d.streamListeners[eventType]
		targets := make([]func(StreamEvent), len(listeners))
		copy(targets, listeners)
		d.streamMu.RUnlock()

		var wg sync.WaitGroup
		for _, l := range targets {
			wg.Add(1)
			go func(fn func(StreamEvent)) {
				defer wg.Done()
				fn(event)
			}(l)
		}
		wg.Wait()
	}
}
