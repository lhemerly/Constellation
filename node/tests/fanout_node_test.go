package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"strings"
	"testing"
)

func TestFanOutNode_Success(t *testing.T) {
	n1 := node.NewBaseNode("dest1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("n1:"), input...), nil
	})

	n2 := node.NewBaseNode("dest2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("n2:"), input...), nil
	})

	destinations := []node.Node{n1, n2}

	aggregator := func(results [][]byte) ([]byte, error) {
		var combined []string
		for _, res := range results {
			combined = append(combined, string(res))
		}
		return []byte(strings.Join(combined, ",")), nil
	}

	fanOutNode := node.NewFanOutNode("fanout", destinations, aggregator)
	defer cleanupNodes(t, []node.Node{fanOutNode})

	if err := fanOutNode.Create(); err != nil {
		t.Fatalf("unexpected error on Create: %v", err)
	}

	res, err := fanOutNode.Process([]byte("data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedStr := "n1:data,n2:data"
	if string(res) != expectedStr {
		t.Errorf("expected %s, got %s", expectedStr, string(res))
	}
}

func TestFanOutNode_Failure(t *testing.T) {
	n1 := node.NewBaseNode("dest1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("n1:"), input...), nil
	})

	n2 := node.NewBaseNode("dest2")
	expectedErr := errors.New("n2 failed")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, expectedErr
	})

	destinations := []node.Node{n1, n2}

	aggregator := func(results [][]byte) ([]byte, error) {
		return []byte("should not reach here"), nil
	}

	fanOutNode := node.NewFanOutNode("fanout-fail", destinations, aggregator)
	defer cleanupNodes(t, []node.Node{fanOutNode})

	if err := fanOutNode.Create(); err != nil {
		t.Fatalf("unexpected error on Create: %v", err)
	}

	_, err := fanOutNode.Process([]byte("data"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "fan-out phase failed") {
		t.Errorf("expected error to contain 'fan-out phase failed', got %v", err)
	}
	if !strings.Contains(err.Error(), expectedErr.Error()) {
		t.Errorf("expected error to contain '%v', got %v", expectedErr, err)
	}
}

func TestFanOutNode_PanicConditions(t *testing.T) {
	// 1. Empty destinations
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected panic for empty destinations")
			}
		}()
		node.NewFanOutNode("empty", []node.Node{}, func(results [][]byte) ([]byte, error) { return nil, nil })
	}()

	// 2. Nil aggregator
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected panic for nil aggregator")
			}
		}()
		node.NewFanOutNode("nil-agg", []node.Node{node.NewBaseNode("n1")}, nil)
	}()
}
