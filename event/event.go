// Package event provides interfaces and implementations for creating, dispatching,
// and handling events and streaming events within the Constellation system.
//
// It is designed for high concurrency and efficient streaming between nodes.
//
// Main components:
//
//   - Event: Interface defining the basic structure of an event.
//   - StreamEvent: Interface extending Event for streaming data between nodes.
//   - BaseEvent: Concrete implementation of the Event interface.
//   - BaseStreamEvent: Concrete implementation of the StreamEvent interface.
//   - EventDispatcher: Manages event listeners and dispatches events to them
//     in a thread-safe, asynchronous manner.
//   - StreamEventDispatcher: Extends EventDispatcher to handle streaming events.
//
// Example usage:
//
//	dispatcher := event.NewEventDispatcher()
//	dispatcher.RegisterListener("greet", func(e event.Event) {
//	    fmt.Printf("Received event type=%s data=%s\n", e.GetType(), e.GetData())
//	})
//
//	evt := event.NewBaseEvent("greet", []byte("hello"))
//	dispatcher.Dispatch(evt)
//
//	// Streaming example:
//	streamDisp := event.NewStreamEventDispatcher()
//	streamDisp.RegisterStreamListener("sensor", func(e event.StreamEvent) {
//	    fmt.Printf("Stream chunk seq=%d data=%s\n", e.GetSequence(), e.GetData())
//	})
//
//	chunk := event.NewBaseStreamEvent("sensor", []byte("data-chunk"), 1, true)
//	streamDisp.DispatchStream(chunk)
package event

// Event defines the basic structure of an event.
type Event interface {
	// GetType returns the event type identifier.
	GetType() string

	// GetData returns the raw event payload.
	GetData() []byte
}

// StreamEvent extends Event with streaming metadata.
type StreamEvent interface {
	Event

	// GetSequence returns the sequence number of this chunk within a stream.
	GetSequence() uint64

	// IsLast reports whether this is the final chunk of the stream.
	IsLast() bool
}

// BaseEvent is a concrete, immutable implementation of Event.
type BaseEvent struct {
	eventType string
	data      []byte
}

// NewBaseEvent creates a new BaseEvent with the given type and data.
func NewBaseEvent(eventType string, data []byte) *BaseEvent {
	return &BaseEvent{
		eventType: eventType,
		data:      data,
	}
}

// GetType returns the event type.
func (e *BaseEvent) GetType() string {
	return e.eventType
}

// GetData returns the event data.
func (e *BaseEvent) GetData() []byte {
	return e.data
}

// BaseStreamEvent is a concrete, immutable implementation of StreamEvent.
type BaseStreamEvent struct {
	BaseEvent
	sequence uint64
	last     bool
}

// NewBaseStreamEvent creates a new BaseStreamEvent.
// sequence is the chunk's position in the stream; last indicates the final chunk.
func NewBaseStreamEvent(eventType string, data []byte, sequence uint64, last bool) *BaseStreamEvent {
	return &BaseStreamEvent{
		BaseEvent: BaseEvent{eventType: eventType, data: data},
		sequence:  sequence,
		last:      last,
	}
}

// GetSequence returns the sequence number of this stream chunk.
func (e *BaseStreamEvent) GetSequence() uint64 {
	return e.sequence
}

// IsLast reports whether this is the final chunk of the stream.
func (e *BaseStreamEvent) IsLast() bool {
	return e.last
}
