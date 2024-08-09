package node_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/lhemerly/Constellation/node"
)

func TestBaseNodeDataProcessing(t *testing.T) {
	const numNodes = 100
	const numRequests = 100
	var wg sync.WaitGroup

	nodes := make([]*node.BaseNode, numNodes)

	for i := 0; i < numNodes; i++ {
		node := node.NewBaseNode("node-" + fmt.Sprint(i))
		nodes[i] = node
		if err := node.Create(); err != nil {
			t.Fatalf("Node %d: Create() error = %v", i, err)
		}
	}

	for i := 0; i < numNodes; i++ {
		for j := 0; j < numRequests; j++ {
			wg.Add(1)
			go func(i, j int) {
				defer wg.Done()
				input := []byte("input data")
				output, err := nodes[i].Process(input)
				if err != nil {
					t.Errorf("Node %d: Process() error = %v", i, err)
				}
				if string(output) != string(input) {
					t.Errorf("Node %d: Process() output = %v, want %v", i, string(output), string(input))
				}
			}(i, j)
		}
	}

	wg.Wait()

	for i := 0; i < numNodes; i++ {
		if err := nodes[i].Delete(); err != nil {
			t.Fatalf("Node %d: Delete() error = %v", i, err)
		}
	}
}
