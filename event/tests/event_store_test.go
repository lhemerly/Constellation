package event_test

import (
	"bytes"
	"testing"

	"github.com/lhemerly/Constellation/event"
)

func TestInMemoryEventStore(t *testing.T) {
	store := event.NewInMemoryEventStore()

	e1 := event.NewBaseEvent("typeA", []byte("data1"))
	e2 := event.NewBaseEvent("typeB", []byte("data2"))
	e3 := event.NewBaseEvent("typeA", []byte("data3"))

	if err := store.Store(e1); err != nil {
		t.Fatalf("Store() error = %v", err)
	}
	if err := store.Store(e2); err != nil {
		t.Fatalf("Store() error = %v", err)
	}
	if err := store.Store(e3); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	// Test GetEvents without filter
	allEvents, err := store.GetEvents("")
	if err != nil {
		t.Fatalf("GetEvents() error = %v", err)
	}
	if len(allEvents) != 3 {
		t.Errorf("GetEvents() returned %d events, want 3", len(allEvents))
	}

	// Test GetEvents with filter
	typeAEvents, err := store.GetEvents("typeA")
	if err != nil {
		t.Fatalf("GetEvents() error = %v", err)
	}
	if len(typeAEvents) != 2 {
		t.Errorf("GetEvents() returned %d events, want 2", len(typeAEvents))
	}
	if !bytes.Equal(typeAEvents[0].GetData(), []byte("data1")) {
		t.Errorf("Event data mismatch: got %v, want data1", string(typeAEvents[0].GetData()))
	}
	if !bytes.Equal(typeAEvents[1].GetData(), []byte("data3")) {
		t.Errorf("Event data mismatch: got %v, want data3", string(typeAEvents[1].GetData()))
	}

	// Test Clear
	if err := store.Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	allEvents, _ = store.GetEvents("")
	if len(allEvents) != 0 {
		t.Errorf("GetEvents() returned %d events after clear, want 0", len(allEvents))
	}
}
