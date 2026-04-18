package node_test

import (
	"testing"
)

type deletableNode interface {
	Delete() error
}

func cleanupNodes[T deletableNode](t *testing.T, nodes []T) {
	t.Helper()
	for i := range nodes {
		if err := nodes[i].Delete(); err != nil {
			t.Fatalf("Node %d: Delete() error = %v", i, err)
		}
	}
}
