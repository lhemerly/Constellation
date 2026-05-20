package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"strings"
	"testing"
)

func TestDemuxNode_Success(t *testing.T) {
	node1 := node.NewBaseNode("dest-1")
	node1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("N1:"), input...), nil
	})

	node2 := node.NewBaseNode("dest-2")
	node2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("N2:"), input...), nil
	})

	node3 := node.NewBaseNode("dest-3")
	node3.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("N3:"), input...), nil
	})

	demuxNode := node.NewDemuxNode("demux-node")
	defer cleanupNodes(t, []node.Node{demuxNode, node1, node2, node3})

	// Add routes
	// Route 1 matches everything
	demuxNode.AddRoute(func(b []byte) bool { return true }, node1)
	// Route 2 matches "test"
	demuxNode.AddRoute(func(b []byte) bool { return string(b) == "test" }, node2)
	// Route 3 matches "other"
	demuxNode.AddRoute(func(b []byte) bool { return string(b) == "other" }, node3)

	if err := demuxNode.Create(); err != nil {
		t.Fatalf("unexpected error on create: %v", err)
	}

	res, err := demuxNode.Process([]byte("test"))
	if err != nil {
		t.Fatalf("unexpected error on process: %v", err)
	}

	resultStr := string(res)

	// Since node1 and node2 match "test", the output should be "N1:test" and "N2:test" concatenated in order.
	if resultStr != "N1:testN2:test" {
		t.Errorf("unexpected output: %s", resultStr)
	}
}

func TestDemuxNode_NoMatch(t *testing.T) {
	demuxNode := node.NewDemuxNode("demux-node")
	defer cleanupNodes(t, []node.Node{demuxNode})

	// Add route that never matches
	demuxNode.AddRoute(func(b []byte) bool { return false }, node.NewBaseNode("never"))

	if err := demuxNode.Create(); err != nil {
		t.Fatalf("unexpected error on create: %v", err)
	}

	_, err := demuxNode.Process([]byte("test"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !errors.Is(err, node.ErrNoDemuxRoutes) {
		t.Errorf("expected ErrNoDemuxRoutes, got: %v", err)
	}
}

func TestDemuxNode_PartialError(t *testing.T) {
	node1 := node.NewBaseNode("dest-1")
	node1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("N1:"), input...), nil
	})

	node2 := node.NewBaseNode("dest-2")
	node2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("node2 error")
	})

	demuxNode := node.NewDemuxNode("demux-node")
	defer cleanupNodes(t, []node.Node{demuxNode, node1, node2})

	demuxNode.AddRoute(func(b []byte) bool { return true }, node1)
	demuxNode.AddRoute(func(b []byte) bool { return true }, node2)

	if err := demuxNode.Create(); err != nil {
		t.Fatalf("unexpected error on create: %v", err)
	}

	_, err := demuxNode.Process([]byte("test"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "demux processing failed") || !strings.Contains(err.Error(), "node2 error") {
		t.Errorf("expected demux processing failed error with node2 error, got: %v", err)
	}
}
