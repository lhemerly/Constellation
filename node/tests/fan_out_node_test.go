package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"testing"
	"time"
)

func TestFanOutNode_Success(t *testing.T) {
	fanOut := node.NewFanOutNode("fan-out", 500*time.Millisecond)
	defer cleanupNodes(t, []node.Node{fanOut})

	dest1 := node.NewBaseNode("dest1")
	dest2 := node.NewBaseNode("dest2")
	defer cleanupNodes(t, []node.Node{dest1, dest2})

	var count1, count2 int
	dest1.SetProcessFunc(func(input []byte) ([]byte, error) {
		count1++
		return input, nil
	})
	dest2.SetProcessFunc(func(input []byte) ([]byte, error) {
		count2++
		return input, nil
	})

	fanOut.AddDestination(dest1)
	fanOut.AddDestination(dest2)

	res, err := fanOut.Process([]byte("broadcast"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(res) != "broadcast" {
		t.Errorf("expected broadcast, got %s", string(res))
	}

	if count1 != 1 {
		t.Errorf("expected dest1 to receive 1 message, got %d", count1)
	}
	if count2 != 1 {
		t.Errorf("expected dest2 to receive 1 message, got %d", count2)
	}
}

func TestFanOutNode_Timeout(t *testing.T) {
	fanOut := node.NewFanOutNode("fan-out-timeout", 50*time.Millisecond)
	defer cleanupNodes(t, []node.Node{fanOut})

	dest1 := node.NewBaseNode("dest1")
	dest2 := node.NewBaseNode("dest2")
	defer cleanupNodes(t, []node.Node{dest1, dest2})

	dest1.SetProcessFunc(func(input []byte) ([]byte, error) {
		// Fast node
		return input, nil
	})

	blockCh := make(chan struct{})
	dest2.SetProcessFunc(func(input []byte) ([]byte, error) {
		// Slow node blocks until channel is closed
		<-blockCh
		return input, nil
	})
	defer close(blockCh) // release slow node on teardown

	fanOut.AddDestination(dest1)
	fanOut.AddDestination(dest2)

	_, err := fanOut.Process([]byte("broadcast"))
	if err == nil {
		t.Fatalf("expected timeout error, got nil")
	}

	if !errors.Is(err, node.ErrFanOutTimeout) {
		t.Errorf("expected ErrFanOutTimeout, got %v", err)
	}
}

func TestFanOutNode_NoDestinations(t *testing.T) {
	fanOut := node.NewFanOutNode("fan-out-empty", 500*time.Millisecond)
	defer cleanupNodes(t, []node.Node{fanOut})

	res, err := fanOut.Process([]byte("broadcast"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(res) != "broadcast" {
		t.Errorf("expected broadcast, got %s", string(res))
	}
}

func TestFanOutNode_DestinationError(t *testing.T) {
	fanOut := node.NewFanOutNode("fan-out-error", 500*time.Millisecond)
	defer cleanupNodes(t, []node.Node{fanOut})

	dest1 := node.NewBaseNode("dest1")
	defer cleanupNodes(t, []node.Node{dest1})

	expectedErr := errors.New("simulated error")
	dest1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, expectedErr
	})

	fanOut.AddDestination(dest1)

	_, err := fanOut.Process([]byte("broadcast"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf("expected %v to be in %v", expectedErr, err)
	}
}
