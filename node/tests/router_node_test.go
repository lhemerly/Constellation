package node_test

import (
	"strings"
	"sync/atomic"
	"testing"

	"github.com/lhemerly/Constellation/node"
)

func TestRouterNode(t *testing.T) {
	router := node.NewRouterNode("router-1")

	nodeA := node.NewBaseNode("node-a")
	nodeB := node.NewBaseNode("node-b")

	var countA, countB int32

	nodeA.SetProcessFunc(func(input []byte) ([]byte, error) {
		atomic.AddInt32(&countA, 1)
		return input, nil
	})

	nodeB.SetProcessFunc(func(input []byte) ([]byte, error) {
		atomic.AddInt32(&countB, 1)
		return input, nil
	})

	// Add routes
	router.AddRoute(func(event []byte) bool {
		return strings.HasPrefix(string(event), "A:")
	}, nodeA)

	router.AddRoute(func(event []byte) bool {
		return strings.HasPrefix(string(event), "B:")
	}, nodeB)

	// Send event meant for A
	router.Notify([]byte("A: hello A"))
	// Wait a bit since Notify is async in BaseNode, though RouterNode waits inside Notify
	// However the child Process is async within Notify. Wait for the sync.WaitGroup in Notify to finish.
	// Oh, Notify in RouterNode waits using wg.Wait(), so no need to sleep if it waits before returning.

	if atomic.LoadInt32(&countA) != 1 {
		t.Errorf("Expected node A to receive 1 event, got %d", countA)
	}
	if atomic.LoadInt32(&countB) != 0 {
		t.Errorf("Expected node B to receive 0 events, got %d", countB)
	}

	// Send event meant for B
	router.Notify([]byte("B: hello B"))

	if atomic.LoadInt32(&countA) != 1 {
		t.Errorf("Expected node A to receive 1 event, got %d", countA)
	}
	if atomic.LoadInt32(&countB) != 1 {
		t.Errorf("Expected node B to receive 1 event, got %d", countB)
	}

	// Send event meant for both or neither (none should match)
	router.Notify([]byte("C: hello C"))

	if atomic.LoadInt32(&countA) != 1 {
		t.Errorf("Expected node A to still have 1 event, got %d", countA)
	}
	if atomic.LoadInt32(&countB) != 1 {
		t.Errorf("Expected node B to still have 1 event, got %d", countB)
	}

	// Clear routes
	router.ClearRoutes()

	// Send again, should not reach anyone
	router.Notify([]byte("A: hello A again"))
	if atomic.LoadInt32(&countA) != 1 {
		t.Errorf("Expected node A to still have 1 event, got %d", countA)
	}
}

func TestRouterNodeCleanup(t *testing.T) {
	router := node.NewRouterNode("router-2")
	cleanupNodes(t, []node.Node{router})
}
