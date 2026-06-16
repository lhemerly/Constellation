package node_test

import (
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestChaosMiddleware_Panic(t *testing.T) {
	n := node.NewBaseNode("panic-node")
	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		return input, nil
	})

	n.Use(node.RecoveryMiddleware) // Wrap with Recovery to catch the panic and turn it into an error
	n.Use(node.ChaosMiddleware(node.ChaosConfig{
		PanicProb: 1.0, // Always panic
	}))

	_, err := n.Process([]byte("test"))
	if err == nil {
		t.Fatal("Expected panic to be caught as an error, got nil")
	}

	if err.Error() != "chaos injected panic" {
		t.Fatalf("Expected error 'chaos injected panic', got: %v", err)
	}
}

func TestChaosMiddleware_Error(t *testing.T) {
	n := node.NewBaseNode("error-node")
	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		return input, nil
	})

	n.Use(node.ChaosMiddleware(node.ChaosConfig{
		ErrorProb: 1.0, // Always error
	}))

	_, err := n.Process([]byte("test"))
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if err != node.ErrChaosInjected {
		t.Fatalf("Expected ErrChaosInjected, got: %v", err)
	}
}

func TestChaosMiddleware_Latency(t *testing.T) {
	n := node.NewBaseNode("latency-node")
	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		return input, nil
	})

	maxLatency := 50 * time.Millisecond
	n.Use(node.ChaosMiddleware(node.ChaosConfig{
		LatencyProb: 1.0, // Always add latency
		MaxLatency:  maxLatency,
	}))

	start := time.Now()
	_, err := n.Process([]byte("test"))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	duration := time.Since(start)
	// We expect *some* latency, but rand.Int63n can return 0.
	// To reliably test this, we should just ensure it doesn't exceed MaxLatency significantly or panic.
	// Since we use 1.0 prob, it will sleep for some time in [0, 50ms).
	if duration > 2*maxLatency {
		t.Fatalf("Process took too long: %v (max expected ~%v)", duration, maxLatency)
	}
}

func TestChaosMiddleware_NoChaos(t *testing.T) {
	n := node.NewBaseNode("no-chaos-node")
	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		return input, nil
	})

	n.Use(node.ChaosMiddleware(node.ChaosConfig{
		PanicProb:   0.0,
		ErrorProb:   0.0,
		LatencyProb: 0.0,
	}))

	res, err := n.Process([]byte("test"))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if string(res) != "test" {
		t.Fatalf("Expected 'test', got: %s", string(res))
	}
}
