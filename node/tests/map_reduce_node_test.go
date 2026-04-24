package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"strings"
	"testing"
)

func TestMapReduceNode_Success(t *testing.T) {
	// Create Mappers
	mapper1 := node.NewBaseNode("mapper-1")
	mapper1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte(strings.ToUpper(string(input))), nil
	})

	mapper2 := node.NewBaseNode("mapper-2")
	mapper2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte(strings.ToLower(string(input))), nil
	})

	// Create Reducer
	reducer := node.NewBaseNode("reducer")
	reducer.SetProcessFunc(func(input []byte) ([]byte, error) {
		// Just returning length as a simple aggregation
		return []byte(string(input)), nil
	})

	mrNode := node.NewMapReduceNode("mr-node", []node.Node{mapper1, mapper2}, reducer)
	defer cleanupNodes(t, []node.Node{mrNode}) // cleanups will delete the mappers and reducers internally as well if Create was called

	if err := mrNode.Create(); err != nil {
		t.Fatalf("unexpected error on create: %v", err)
	}

	res, err := mrNode.Process([]byte("TeSt"))
	if err != nil {
		t.Fatalf("unexpected error on process: %v", err)
	}

	resultStr := string(res)
	// Because we preallocate based on array index, the order is deterministic now
	if resultStr != "TESTtest" {
		t.Errorf("unexpected output: %s", resultStr)
	}
}

func TestMapReduceNode_MapError(t *testing.T) {
	mapper1 := node.NewBaseNode("mapper-1")
	mapper1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("mapper error")
	})

	reducer := node.NewBaseNode("reducer")

	mrNode := node.NewMapReduceNode("mr-node", []node.Node{mapper1}, reducer)
	defer cleanupNodes(t, []node.Node{mrNode})

	_, err := mrNode.Process([]byte("data"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "map phase failed") {
		t.Errorf("expected map phase failure error, got: %v", err)
	}
}

func TestMapReduceNode_PanicsOnNilMappersOrReducer(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on nil mappers")
		}
	}()
	reducer := node.NewBaseNode("reducer")
	node.NewMapReduceNode("mr", nil, reducer)
}

func TestMapReduceNode_PanicsOnNilReducer(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on nil reducer")
		}
	}()
	mapper := node.NewBaseNode("mapper")
	node.NewMapReduceNode("mr", []node.Node{mapper}, nil)
}
