package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"testing"
	"time"
)

func TestRaceNode_Success(t *testing.T) {
	node1 := node.NewBaseNode("node-1")
	node1.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(50 * time.Millisecond) // slower node
		return []byte("slow"), nil
	})

	node2 := node.NewBaseNode("node-2")
	node2.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(10 * time.Millisecond) // faster node
		return []byte("fast"), nil
	})

	raceNode := node.NewRaceNode("race", []node.Node{node1, node2})
	defer cleanupNodes(t, []node.Node{raceNode})

	if err := raceNode.Create(); err != nil {
		t.Fatalf("unexpected error on create: %v", err)
	}

	res, err := raceNode.Process([]byte("input"))
	if err != nil {
		t.Fatalf("unexpected error on process: %v", err)
	}

	if string(res) != "fast" {
		t.Errorf("expected 'fast', got '%s'", string(res))
	}
}

func TestRaceNode_OneFailsOthersSucceed(t *testing.T) {
	node1 := node.NewBaseNode("node-1")
	node1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("failed")
	})

	node2 := node.NewBaseNode("node-2")
	node2.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(10 * time.Millisecond)
		return []byte("success"), nil
	})

	raceNode := node.NewRaceNode("race", []node.Node{node1, node2})
	defer cleanupNodes(t, []node.Node{raceNode})

	res, err := raceNode.Process([]byte("input"))
	if err != nil {
		t.Fatalf("unexpected error on process: %v", err)
	}

	if string(res) != "success" {
		t.Errorf("expected 'success', got '%s'", string(res))
	}
}

func TestRaceNode_AllFail(t *testing.T) {
	node1 := node.NewBaseNode("node-1")
	node1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("err1")
	})

	node2 := node.NewBaseNode("node-2")
	node2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("err2")
	})

	raceNode := node.NewRaceNode("race", []node.Node{node1, node2})
	defer cleanupNodes(t, []node.Node{raceNode})

	_, err := raceNode.Process([]byte("input"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, node.ErrAllNodesFailed) {
		t.Errorf("expected ErrAllNodesFailed, got %v", err)
	}
}

func TestRaceNode_PanicsOnNoNodes(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on no nodes")
		} else if err, ok := r.(string); ok && err != node.ErrNoNodesRace.Error() {
			t.Errorf("expected ErrNoNodesRace panic, got %v", r)
		}
	}()

	node.NewRaceNode("race", nil)
}
