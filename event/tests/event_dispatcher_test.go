package event_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/lhemerly/Constellation/event"
)

// TestEventDispatcher_NilData verifies that an event with nil data does not cause panics
// and is correctly received by listeners.
func TestEventDispatcher_NilData(t *testing.T) {
	d := event.NewEventDispatcher()

	var received event.Event
	d.RegisterListener("niltest", func(e event.Event) {
		received = e
	})

	evt := event.NewBaseEvent("niltest", nil)
	d.Dispatch(evt)

	if received == nil {
		t.Fatal("listener was not called")
	}
	if received.GetData() != nil {
		t.Errorf("GetData() = %q, want nil", received.GetData())
	}
}

func TestEventDispatcher_PatternMatching(t *testing.T) {
	dispatcher := event.NewEventDispatcher()
	var (
		mu           sync.Mutex
		matchedTypes []string
	)

	// Register a listener with a pattern
	dispatcher.RegisterListener("sensor.*.temperature", func(e event.Event) {
		mu.Lock()
		matchedTypes = append(matchedTypes, e.GetType())
		mu.Unlock()
	})

	dispatcher.RegisterListener("sensor.kitchen.*", func(e event.Event) {
		mu.Lock()
		matchedTypes = append(matchedTypes, e.GetType())
		mu.Unlock()
	})

	dispatcher.RegisterListener("*", func(e event.Event) {
		mu.Lock()
		matchedTypes = append(matchedTypes, "wildcard:"+e.GetType())
		mu.Unlock()
	})

	dispatcher.RegisterListener("sensor", func(e event.Event) {
		mu.Lock()
		matchedTypes = append(matchedTypes, "exact:"+e.GetType())
		mu.Unlock()
	})

	// Dispatch an event matching "sensor.*.temperature"
	dispatcher.Dispatch(event.NewBaseEvent("sensor.livingroom.temperature", nil))

	// Dispatch an event matching "sensor.kitchen.*"
	dispatcher.Dispatch(event.NewBaseEvent("sensor.kitchen.humidity", nil))

	// Dispatch an exact event
	dispatcher.Dispatch(event.NewBaseEvent("sensor", nil))

	mu.Lock()
	defer mu.Unlock()

	// "sensor.livingroom.temperature" should match "sensor.*.temperature" and "*"
	// "sensor.kitchen.humidity" should match "sensor.kitchen.*" and "*"
	// "sensor" should match "sensor" and "*"

	if len(matchedTypes) != 6 {
		t.Fatalf("expected 6 matches, got %d. Matches: %v", len(matchedTypes), matchedTypes)
	}

	expectedMatches := map[string]bool{
		"sensor.livingroom.temperature":          false,
		"wildcard:sensor.livingroom.temperature": false,
		"sensor.kitchen.humidity":                false,
		"wildcard:sensor.kitchen.humidity":       false,
		"exact:sensor":                           false,
		"wildcard:sensor":                        false,
	}

	for _, m := range matchedTypes {
		expectedMatches[m] = true
	}

	for k, v := range expectedMatches {
		if !v {
			t.Errorf("expected match %s was not triggered", k)
		}
	}
}

// TestEventDispatcher_RegisterAndDispatch verifies that a single listener
// receives the dispatched event with the correct type and data.
func TestEventDispatcher_RegisterAndDispatch(t *testing.T) {
	d := event.NewEventDispatcher()

	var received event.Event
	d.RegisterListener("ping", func(e event.Event) {
		received = e
	})

	evt := event.NewBaseEvent("ping", []byte("pong"))
	d.Dispatch(evt)

	if received == nil {
		t.Fatal("listener was not called")
	}
	if received.GetType() != "ping" {
		t.Errorf("GetType() = %q, want %q", received.GetType(), "ping")
	}
	if string(received.GetData()) != "pong" {
		t.Errorf("GetData() = %q, want %q", received.GetData(), "pong")
	}
}

// TestEventDispatcher_NoListenerForType verifies that dispatching an event
// with no registered listeners does not panic or block.
func TestEventDispatcher_NoListenerForType(t *testing.T) {
	d := event.NewEventDispatcher()
	evt := event.NewBaseEvent("unknown", []byte("data"))
	d.Dispatch(evt) // should not panic
}

// TestEventDispatcher_MultipleListeners verifies that all registered listeners
// for an event type are invoked.
func TestEventDispatcher_MultipleListeners(t *testing.T) {
	const n = 10
	d := event.NewEventDispatcher()

	var count atomic.Int64
	for i := 0; i < n; i++ {
		d.RegisterListener("tick", func(e event.Event) {
			count.Add(1)
		})
	}

	d.Dispatch(event.NewBaseEvent("tick", nil))

	if got := count.Load(); got != n {
		t.Errorf("listener call count = %d, want %d", got, n)
	}
}

// TestEventDispatcher_TypeIsolation verifies that a listener registered for
// one event type does not receive events of a different type.
func TestEventDispatcher_TypeIsolation(t *testing.T) {
	d := event.NewEventDispatcher()

	var pingCount, pongCount atomic.Int64
	d.RegisterListener("ping", func(e event.Event) { pingCount.Add(1) })
	d.RegisterListener("pong", func(e event.Event) { pongCount.Add(1) })

	d.Dispatch(event.NewBaseEvent("ping", nil))
	d.Dispatch(event.NewBaseEvent("ping", nil))
	d.Dispatch(event.NewBaseEvent("pong", nil))

	if pingCount.Load() != 2 {
		t.Errorf("pingCount = %d, want 2", pingCount.Load())
	}
	if pongCount.Load() != 1 {
		t.Errorf("pongCount = %d, want 1", pongCount.Load())
	}
}

// TestEventDispatcher_ConcurrentDispatch verifies that the dispatcher handles
// many concurrent Dispatch calls without data races.
func TestEventDispatcher_ConcurrentDispatch(t *testing.T) {
	const goroutines = 100
	const eventsPerGoroutine = 100

	d := event.NewEventDispatcher()

	var count atomic.Int64
	d.RegisterListener("load", func(e event.Event) {
		count.Add(1)
	})

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				d.Dispatch(event.NewBaseEvent("load", []byte("data")))
			}
		}()
	}
	wg.Wait()

	expected := int64(goroutines * eventsPerGoroutine)
	if got := count.Load(); got != expected {
		t.Errorf("count = %d, want %d", got, expected)
	}
}

// TestEventDispatcher_ConcurrentRegisterAndDispatch verifies that listeners
// can be registered concurrently while events are being dispatched.
func TestEventDispatcher_ConcurrentRegisterAndDispatch(t *testing.T) {
	d := event.NewEventDispatcher()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.RegisterListener("concurrent", func(e event.Event) {})
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.Dispatch(event.NewBaseEvent("concurrent", nil))
		}()
	}
	wg.Wait()
}
