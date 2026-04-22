package node_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lhemerly/Constellation/node"
)

func TestRouterNode(t *testing.T) {
	// Create destination nodes
	dest1 := node.NewBaseNode("dest-1")
	dest1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("dest1:"), input...), nil
	})

	dest2 := node.NewBaseNode("dest-2")
	dest2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("dest2:"), input...), nil
	})

	// Create router node
	router := node.NewRouterNode("router")

	// Add routes
	router.AddRoute(func(input []byte) bool {
		return strings.HasPrefix(string(input), "1:")
	}, dest1)

	router.AddRoute(func(input []byte) bool {
		return strings.HasPrefix(string(input), "2:")
	}, dest2)

	// Test route to dest1
	input1 := []byte("1:hello")
	output1, err := router.Process(input1)
	if err != nil {
		t.Fatalf("Process error: %v", err)
	}
	expected1 := []byte("dest1:1:hello")
	if !bytes.Equal(output1, expected1) {
		t.Errorf("Process output = %s, want %s", string(output1), string(expected1))
	}

	// Test route to dest2
	input2 := []byte("2:world")
	output2, err := router.Process(input2)
	if err != nil {
		t.Fatalf("Process error: %v", err)
	}
	expected2 := []byte("dest2:2:world")
	if !bytes.Equal(output2, expected2) {
		t.Errorf("Process output = %s, want %s", string(output2), string(expected2))
	}

	// Test fallback (no match)
	input3 := []byte("3:nomatch")
	output3, err := router.Process(input3)
	if err != nil {
		t.Fatalf("Process error: %v", err)
	}
	expected3 := []byte("3:nomatch") // Passthrough behavior
	if !bytes.Equal(output3, expected3) {
		t.Errorf("Process output = %s, want %s", string(output3), string(expected3))
	}

	// Make sure we test cleanup using the memory-recommended helper
	nodesToCleanup := []node.Node{dest1, dest2, router}
	for _, n := range nodesToCleanup {
		if err := n.Delete(); err != nil {
			t.Errorf("Failed to delete node %s: %v", n.GetID(), err)
		}
	}
}
