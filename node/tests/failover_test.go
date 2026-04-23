package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"testing"
)

func TestFailoverNode_PrimarySuccess(t *testing.T) {
	f := node.NewFailoverNode("failover-1")
	primary := node.NewBaseNode("primary")
	fallback := node.NewBaseNode("fallback")

	var nodes []node.Node
	nodes = append(nodes, f, primary, fallback)
	defer cleanupNodes(t, nodes)

	primary.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("primary-success"), nil
	})
	fallback.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("fallback-success"), nil
	})

	f.SetPrimary(primary)
	f.SetFallback(fallback)

	res, err := f.Process([]byte("input"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(res) != "primary-success" {
		t.Errorf("expected primary-success, got %s", string(res))
	}
}

func TestFailoverNode_FallbackSuccess(t *testing.T) {
	f := node.NewFailoverNode("failover-2")
	primary := node.NewBaseNode("primary")
	fallback := node.NewBaseNode("fallback")

	var nodes []node.Node
	nodes = append(nodes, f, primary, fallback)
	defer cleanupNodes(t, nodes)

	primaryErr := errors.New("primary failed")
	primary.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, primaryErr
	})
	fallback.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("fallback-success"), nil
	})

	f.SetPrimary(primary)
	f.SetFallback(fallback)

	res, err := f.Process([]byte("input"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(res) != "fallback-success" {
		t.Errorf("expected fallback-success, got %s", string(res))
	}
}

func TestFailoverNode_BothFail(t *testing.T) {
	f := node.NewFailoverNode("failover-3")
	primary := node.NewBaseNode("primary")
	fallback := node.NewBaseNode("fallback")

	var nodes []node.Node
	nodes = append(nodes, f, primary, fallback)
	defer cleanupNodes(t, nodes)

	primaryErr := errors.New("primary failed")
	fallbackErr := errors.New("fallback failed")

	primary.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, primaryErr
	})
	fallback.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, fallbackErr
	})

	f.SetPrimary(primary)
	f.SetFallback(fallback)

	_, err := f.Process([]byte("input"))
	if !errors.Is(err, node.ErrFailoverFailed) {
		t.Errorf("expected ErrFailoverFailed, got %v", err)
	}
	if !errors.Is(err, primaryErr) {
		t.Errorf("expected error to wrap primary error")
	}
	if !errors.Is(err, fallbackErr) {
		t.Errorf("expected error to wrap fallback error")
	}
}

func TestFailoverNode_NoNodes(t *testing.T) {
	f := node.NewFailoverNode("failover-empty")
	defer cleanupNodes(t, []node.Node{f})

	_, err := f.Process([]byte("input"))
	if !errors.Is(err, node.ErrNoNodesFailover) {
		t.Errorf("expected ErrNoNodesFailover, got %v", err)
	}
}
