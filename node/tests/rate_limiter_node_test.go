package node_test

import (
	"errors"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestRateLimiterNode_Burst(t *testing.T) {
	// Rate: 1 token/sec, Burst capacity: 3
	rl := node.NewRateLimiterNode("rl-node", 1.0, 3.0)
	err := rl.Create()
	if err != nil {
		t.Fatalf("Failed to create RateLimiterNode: %v", err)
	}
	defer cleanupNodes(t, []node.Node{rl})

	rl.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("processed "), input...), nil
	})

	// First 3 should pass due to burst capacity
	for i := 0; i < 3; i++ {
		res, err := rl.Process([]byte("data"))
		if err != nil {
			t.Fatalf("Expected process to succeed on attempt %d, got err: %v", i, err)
		}
		if string(res) != "processed data" {
			t.Errorf("Unexpected output: %s", string(res))
		}
	}

	// 4th should fail since capacity is exhausted and time hasn't elapsed
	_, err = rl.Process([]byte("data"))
	if !errors.Is(err, node.ErrRateLimitExceeded) {
		t.Errorf("Expected ErrRateLimitExceeded, got %v", err)
	}
}

func TestRateLimiterNode_Replenish(t *testing.T) {
	// Rate: 10 tokens/sec, Burst capacity: 1
	rl := node.NewRateLimiterNode("rl-node", 10.0, 1.0)
	err := rl.Create()
	if err != nil {
		t.Fatalf("Failed to create RateLimiterNode: %v", err)
	}
	defer cleanupNodes(t, []node.Node{rl})

	// Exhaust token
	_, err = rl.Process([]byte("data"))
	if err != nil {
		t.Fatalf("Expected initial process to succeed, got %v", err)
	}

	// Immediate next should fail
	_, err = rl.Process([]byte("data"))
	if !errors.Is(err, node.ErrRateLimitExceeded) {
		t.Fatalf("Expected ErrRateLimitExceeded, got %v", err)
	}

	// Wait enough time to replenish 1 token (rate 10/s = 100ms per token)
	time.Sleep(150 * time.Millisecond)

	// Should succeed now
	_, err = rl.Process([]byte("data"))
	if err != nil {
		t.Errorf("Expected process to succeed after replenishment, got err: %v", err)
	}
}
