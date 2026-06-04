package node_test

import (
	"github.com/lhemerly/Constellation/node"
	"sync/atomic"
	"testing"
	"time"
)

func TestTickerNode_Basic(t *testing.T) {
	interval := 50 * time.Millisecond
	payload := []byte("tick")

	tn := node.NewTickerNode("ticker", interval, payload)
	defer cleanupNodes(t, []node.Node{tn})

	// Create a subscriber
	sub := node.NewBaseNode("sub")
	var count int32
	sub.SetProcessFunc(func(input []byte) ([]byte, error) {
		if string(input) != "tick" {
			t.Errorf("expected payload 'tick', got '%s'", string(input))
		}
		atomic.AddInt32(&count, 1)
		return nil, nil
	})

	if err := tn.Subscribe(sub); err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	if err := tn.Create(); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Wait for 3 ticks
	time.Sleep(175 * time.Millisecond)

	// Delete stops the ticker
	if err := tn.Delete(); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Wait a bit to ensure it actually stopped
	time.Sleep(100 * time.Millisecond)

	finalCount := atomic.LoadInt32(&count)
	if finalCount < 3 || finalCount > 4 {
		t.Errorf("expected 3 or 4 ticks, got %d", finalCount)
	}
}

func TestTickerNode_Passthrough(t *testing.T) {
	tn := node.NewTickerNode("ticker", 1*time.Second, []byte("tick"))
	defer cleanupNodes(t, []node.Node{tn})

	res, err := tn.Process([]byte("hello"))
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if string(res) != "hello" {
		t.Errorf("Process() expected 'hello', got '%s'", string(res))
	}
}

func TestTickerNode_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()

	_ = node.NewTickerNode("ticker", 0, []byte("tick"))
}
