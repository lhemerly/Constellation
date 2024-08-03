package network_test

import (
	"sync"
	"testing"
	"fmt"

	"github.com/lhemerly/Constellation/network"
)

func TestBaseNodeEventNotification(t *testing.T) {
	const numNodes = 100
	const numEvents = 100
	var wg sync.WaitGroup

	type EventNode struct {
		*network.BaseNode
		eventsReceived int
		eventMutex     sync.Mutex
	}

	newEventNode := func(id string) *EventNode {
		return &EventNode{
			BaseNode:       network.NewBaseNode(id),
			eventsReceived: 0,
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

	// Process method for EventNode
	process := func(n *EventNode, input []byte) ([]byte, error) {
		n.eventMutex.Lock()
		n.eventsReceived++
		n.eventMutex.Unlock()
		return input, nil
	}

	// Set custom process method for EventNode
	for i := 0; i < numNodes; i++ {
		nodes[i].SetProcessFunc(func(input []byte) ([]byte, error) {
			return process(nodes[i], input)
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
		nodes[i].eventMutex.Lock()
		expectedEvents := (numNodes - 1) * numEvents
		if nodes[i].eventsReceived != expectedEvents {
			t.Errorf("Node %d: eventsReceived = %d, want %d", i, nodes[i].eventsReceived, expectedEvents)
		}
		nodes[i].eventMutex.Unlock()
	}

	for i := 0; i < numNodes; i++ {
		if err := nodes[i].Delete(); err != nil {
			t.Fatalf("Node %d: Delete() error = %v", i, err)
		}
	}
}
