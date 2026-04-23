package node_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestBaseNodeEventNotification(t *testing.T) {
	const numNodes = 100
	const numEvents = 100
	var wg sync.WaitGroup

	type EventNode struct {
		*node.BaseNode
	}

	newEventNode := func(id string) *EventNode {
		return &EventNode{
			BaseNode: node.NewBaseNode(id),
		}
	}

	nodes := make([]*EventNode, numNodes)
	for i := 0; i < numNodes; i++ {
		node := newEventNode("node-" + fmt.Sprint(i))
		nodes[i] = node
		if err := node.Create(); err != nil {
			t.Fatalf("Node %d: Create() error = %v", i, err)
		}
	}

	// Subscribe each node to all other nodes
	for i := 0; i < numNodes; i++ {
		for j := 0; j < numNodes; j++ {
			if i != j {
				if err := nodes[i].Subscribe(nodes[j]); err != nil {
					t.Fatalf("Node %d: Subscribe() error = %v", i, err)
				}
			}
		}
	}

	// Set custom process method for EventNode
	for i := 0; i < numNodes; i++ {
		nodes[i].SetProcessFunc(func(input []byte) ([]byte, error) {
			// The event counting is now handled internally by BaseNode
			return input, nil
		})
	}

	// Notify all nodes concurrently
	for i := 0; i < numNodes; i++ {
		for j := 0; j < numEvents; j++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				event := []byte("event data")
				if err := nodes[i].Notify(event); err != nil {
					t.Errorf("Node %d: Notify() error = %v", i, err)
				}
			}(i)
		}
	}

	wg.Wait()

	// Verify that each node received the correct number of events
	for i := 0; i < numNodes; i++ {
		expectedEvents := uint64((numNodes - 1) * numEvents)
		if nodes[i].GetEventCount() != expectedEvents {
			t.Errorf("Node %d: eventsReceived = %d, want %d", i, nodes[i].GetEventCount(), expectedEvents)
		}
	}

	cleanupNodes(t, nodes)
}

// TestNotifyDoesNotDeadlockWhenProcessModifiesSubscriptions verifies that a
// subscriber can safely call Subscribe or Unsubscribe on the notifying node
// from within its Process method while Notify is in-flight.
// Under the old implementation (RLock held across wg.Wait), calling
// Subscribe/Unsubscribe from Process would deadlock because those methods
// require a write lock. A timeout is used to detect any regression.
func TestNotifyDoesNotDeadlockWhenProcessModifiesSubscriptions(t *testing.T) {
	publisher := node.NewBaseNode("publisher")
	if err := publisher.Create(); err != nil {
		t.Fatalf("publisher.Create() error = %v", err)
	}
	defer publisher.Delete()

	subscriber := node.NewBaseNode("subscriber")
	if err := subscriber.Create(); err != nil {
		t.Fatalf("subscriber.Create() error = %v", err)
	}
	defer subscriber.Delete()

	// The subscriber's Process calls Unsubscribe then Subscribe back on the
	// publisher. Under the old RLock-held-through-Wait implementation, this
	// would deadlock because Subscribe/Unsubscribe require a write lock.
	subscriber.SetProcessFunc(func(input []byte) ([]byte, error) {
		if err := publisher.Unsubscribe(subscriber); err != nil {
			return nil, err
		}
		if err := publisher.Subscribe(subscriber); err != nil {
			return nil, err
		}
		return input, nil
	})

	if err := publisher.Subscribe(subscriber); err != nil {
		t.Fatalf("publisher.Subscribe() error = %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := publisher.Notify([]byte("event")); err != nil {
			t.Errorf("Notify() error = %v", err)
		}
	}()

	select {
	case <-done:
		// Notify completed without deadlock.
	case <-time.After(5 * time.Second):
		t.Fatal("Notify deadlocked: subscriber's Process could not modify subscriptions while Notify was in-flight")
	}
}
