package node_test

import (
	"testing"

	"github.com/lhemerly/Constellation/node"
)

func cleanupNodes[T node.Node](t *testing.T, nodes []T) {
	t.Helper()
	for i := 0; i < len(nodes); i++ {
		if err := nodes[i].Delete(); err != nil {
			t.Fatalf("Node %d: Delete() error = %v", i, err)
		}
	}
}
