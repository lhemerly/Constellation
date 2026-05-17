package event_test

import (
	"sync"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/event"
)

func TestReplayEventDispatcher(t *testing.T) {
	dispatcher := event.NewReplayEventDispatcher(2)

	var mu sync.Mutex
	var received []string

	// Dispatch 3 events before anyone is listening
	dispatcher.Dispatch(event.NewBaseEvent("test", []byte("1")))
	dispatcher.Dispatch(event.NewBaseEvent("test", []byte("2")))
	dispatcher.Dispatch(event.NewBaseEvent("test", []byte("3")))

	// Register listener (should replay last 2 events)
	dispatcher.RegisterListener("test", func(e event.Event) {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, string(e.GetData()))
	})

	// Wait for listener to be invoked (in ReplayEventDispatcher it is invoked synchronously during RegisterListener)
	// But let's sleep just in case
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if len(received) != 2 {
		t.Fatalf("Expected 2 replays, got %d", len(received))
	}
	if received[0] != "2" || received[1] != "3" {
		t.Errorf("Expected replays '2', '3', got %v", received)
	}
	mu.Unlock()

	// Dispatch a new event
	dispatcher.Dispatch(event.NewBaseEvent("test", []byte("4")))

	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if len(received) != 3 {
		t.Fatalf("Expected 3 total received events, got %d", len(received))
	}
	if received[2] != "4" {
		t.Errorf("Expected '4' as last received event, got %v", received[2])
	}
	mu.Unlock()
}
