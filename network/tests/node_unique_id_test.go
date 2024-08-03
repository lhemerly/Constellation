package network_test

import (
	"fmt"
	"testing"

	"github.com/lhemerly/Constellation/network"
)

func TestBaseNodeUniqueIdentification(t *testing.T) {
	const numNodes = 100

	nodes := make([]*network.BaseNode, numNodes)
	ids := make(map[string]bool)

	for i := 0; i < numNodes; i++ {
		nodeID := "node-" + fmt.Sprint(i)
		node := network.NewBaseNode(nodeID)
		nodes[i] = node
		if err := node.Create(); err != nil {
			t.Fatalf("Node %d: Create() error = %v", i, err)
		}
		if ids[node.GetID()] {
			t.Errorf("Duplicate ID found: %s", node.GetID())
		}
		ids[node.GetID()] = true
	}

	for i := 0; i < numNodes; i++ {
		if nodes[i].GetID() != "node-"+fmt.Sprint(i) {
			t.Errorf("Node %d: GetID() = %v, want %v", i, nodes[i].GetID(), "node-"+fmt.Sprint(i))
		}
		if err := nodes[i].Delete(); err != nil {
			t.Fatalf("Node %d: Delete() error = %v", i, err)
		}
	}
}
