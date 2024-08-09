package node_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/lhemerly/Constellation/node"
)

func TestBaseNodeInitializationAndCleanup(t *testing.T) {
	const numNodes = 100
	var wg sync.WaitGroup
	wg.Add(numNodes)

	nodes := make([]*node.BaseNode, numNodes)

	for i := 0; i < numNodes; i++ {
		go func(i int) {
			defer wg.Done()
			node := node.NewBaseNode("node-" + fmt.Sprint(i))
			nodes[i] = node
			if err := node.Create(); err != nil {
				t.Errorf("Node %d: Create() error = %v", i, err)
			}
		}(i)
	}

	wg.Wait()

	// Verify all nodes were created successfully
	for i, node := range nodes {
		if node == nil {
			t.Errorf("Node %d: Create() failed", i)
		}
	}

	wg.Add(numNodes)

	for i := 0; i < numNodes; i++ {
		go func(i int) {
			defer wg.Done()
			if err := nodes[i].Delete(); err != nil {
				t.Errorf("Node %d: Delete() error = %v", i, err)
			}
		}(i)
	}

}
