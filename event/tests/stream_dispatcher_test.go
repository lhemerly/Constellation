package event_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/event"
)

// TestStreamEventDispatcher_RegisterAndDispatchStream verifies that a single
// stream listener receives the dispatched stream event with the correct fields.
func TestStreamEventDispatcher_RegisterAndDispatchStream(t *testing.T) {
	d := event.NewStreamEventDispatcher()

	received := make(chan event.StreamEvent, 1)
	d.RegisterStreamListener("stream-ping", func(e event.StreamEvent) {
		received <- e
	})

	chunk := event.NewBaseStreamEvent("stream-ping", []byte("chunk-1"), 0, true)
	d.DispatchStream(chunk)

	select {
	case got := <-received:
		if got.GetType() != "stream-ping" {
			t.Errorf("GetType() = %q, want %q", got.GetType(), "stream-ping")
		}
		if string(got.GetData()) != "chunk-1" {
			t.Errorf("GetData() = %q, want %q", got.GetData(), "chunk-1")
		}
		if got.GetSequence() != 0 {
			t.Errorf("GetSequence() = %d, want 0", got.GetSequence())
		}
		if !got.IsLast() {
			t.Error("IsLast() = false, want true")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for stream event")
	}
}

// TestStreamEventDispatcher_NoListenerForType verifies that dispatching a
// stream event with no registered listeners does not panic.
func TestStreamEventDispatcher_NoListenerForType(t *testing.T) {
	d := event.NewStreamEventDispatcher()
	chunk := event.NewBaseStreamEvent("no-listeners", []byte("data"), 0, false)
	d.DispatchStream(chunk) // should not panic or block
}

// TestStreamEventDispatcher_MultipleListeners verifies that all stream
// listeners for a given type receive each dispatched chunk.
func TestStreamEventDispatcher_MultipleListeners(t *testing.T) {
	const numListeners = 5
	d := event.NewStreamEventDispatcher()

	var count atomic.Int64
	for i := 0; i < numListeners; i++ {
		d.RegisterStreamListener("multi", func(e event.StreamEvent) {
			count.Add(1)
		})
	}

	d.DispatchStream(event.NewBaseStreamEvent("multi", []byte("data"), 0, true))

	// Allow the asynchronous dispatch to complete.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if count.Load() == numListeners {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if got := count.Load(); got != numListeners {
		t.Errorf("listener call count = %d, want %d", got, numListeners)
	}
}

// TestStreamEventDispatcher_SequenceOrder dispatches multiple chunks and
// verifies that the listener receives them in sequence order.
func TestStreamEventDispatcher_SequenceOrder(t *testing.T) {
	const numChunks = 10
	d := event.NewStreamEventDispatcher()

	received := make(chan uint64, numChunks)
	d.RegisterStreamListener("ordered", func(e event.StreamEvent) {
		received <- e.GetSequence()
	})

	for i := 0; i < numChunks; i++ {
		last := i == numChunks-1
		d.DispatchStream(event.NewBaseStreamEvent("ordered", []byte("d"), uint64(i), last))
	}

	got := make([]uint64, 0, numChunks)
	deadline := time.After(2 * time.Second)
	for len(got) < numChunks {
		select {
		case seq := <-received:
			got = append(got, seq)
		case <-deadline:
			t.Fatalf("timed out: received %d of %d chunks", len(got), numChunks)
		}
	}

	for i, seq := range got {
		if seq != uint64(i) {
			t.Errorf("chunk[%d].GetSequence() = %d, want %d", i, seq, i)
		}
	}
}

// TestStreamEventDispatcher_ConcurrentDispatch exercises the dispatcher under
// high concurrency to detect data races (run with -race).
// DispatchStream is non-blocking by design: events may be dropped when the
// internal buffer is full under sustained heavy load. This test therefore only
// asserts that the dispatcher handles concurrent use without panics or data
// races, and that at least some events are delivered.
func TestStreamEventDispatcher_ConcurrentDispatch(t *testing.T) {
	const goroutines = 50
	const chunksPerGoroutine = 50

	d := event.NewStreamEventDispatcher()

	var count atomic.Int64
	d.RegisterStreamListener("concurrent", func(e event.StreamEvent) {
		count.Add(1)
	})

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for j := 0; j < chunksPerGoroutine; j++ {
				d.DispatchStream(event.NewBaseStreamEvent(
					"concurrent", []byte("data"), uint64(j), j == chunksPerGoroutine-1,
				))
			}
		}(i)
	}
	wg.Wait()

	// Allow the drain goroutine to flush buffered events.
	time.Sleep(200 * time.Millisecond)

	// At least some events must have been delivered.
	if count.Load() == 0 {
		t.Error("no stream events were delivered")
	}
}

func TestStreamEventDispatcher_FilteredStreamListener(t *testing.T) {
	d := event.NewStreamEventDispatcher()

	var receivedCount atomic.Int64
	d.RegisterFilteredStreamListener("filter_stream", func(e event.StreamEvent) bool {
		// Only allow events where sequence is even
		return e.GetSequence()%2 == 0
	}, func(e event.StreamEvent) {
		receivedCount.Add(1)
	})

	d.DispatchStream(event.NewBaseStreamEvent("filter_stream", []byte("d"), 1, false)) // Blocked
	d.DispatchStream(event.NewBaseStreamEvent("filter_stream", []byte("d"), 2, false)) // Allowed
	d.DispatchStream(event.NewBaseStreamEvent("filter_stream", []byte("d"), 3, false)) // Blocked
	d.DispatchStream(event.NewBaseStreamEvent("filter_stream", []byte("d"), 4, true))  // Allowed

	// Allow the asynchronous dispatch to complete.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if receivedCount.Load() == 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if receivedCount.Load() != 2 {
		t.Errorf("receivedCount = %d, want 2", receivedCount.Load())
	}
}

// TestStreamEventDispatcher_InheritsEventDispatcher verifies that the
// StreamEventDispatcher also works as a standard EventDispatcher.
func TestStreamEventDispatcher_InheritsEventDispatcher(t *testing.T) {
	d := event.NewStreamEventDispatcher()

	var count atomic.Int64
	d.RegisterListener("plain", func(e event.Event) {
		count.Add(1)
	})

	d.Dispatch(event.NewBaseEvent("plain", []byte("hello")))

	if count.Load() != 1 {
		t.Errorf("listener call count = %d, want 1", count.Load())
	}
}
