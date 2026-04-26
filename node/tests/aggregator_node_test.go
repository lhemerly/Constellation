package node_test

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestAggregatorNode_SizeFlush(t *testing.T) {
	// Simple join aggregator
	aggregator := func(batch [][]byte) []byte {
		return bytes.Join(batch, []byte(","))
	}

	aggNode := node.NewAggregatorNode("agg1", 3, 1*time.Hour, aggregator)
	_ = aggNode.Create()
	defer cleanupNodes(t, []*node.AggregatorNode{aggNode})

	// Subscriber node to capture output
	var mu sync.Mutex
	var received []byte
	subNode := node.NewBaseNode("sub1")
	subNode.SetProcessFunc(func(input []byte) ([]byte, error) {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, input...)
		return nil, nil
	})
	_ = aggNode.Subscribe(subNode)

	// Send 2 messages (should not flush)
	_, _ = aggNode.Process([]byte("A"))
	_, _ = aggNode.Process([]byte("B"))

	mu.Lock()
	if len(received) > 0 {
		t.Fatalf("Expected no flush yet, got %s", received)
	}
	mu.Unlock()

	// Send 3rd message (should flush)
	_, _ = aggNode.Process([]byte("C"))

	// Give time for async notify to finish
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if string(received) != "A,B,C" {
		t.Fatalf("Expected 'A,B,C', got '%s'", received)
	}
	mu.Unlock()
}

func TestAggregatorNode_TimeFlush(t *testing.T) {
	aggregator := func(batch [][]byte) []byte {
		return bytes.Join(batch, []byte("-"))
	}

	aggNode := node.NewAggregatorNode("agg2", 10, 100*time.Millisecond, aggregator)
	_ = aggNode.Create()
	defer cleanupNodes(t, []*node.AggregatorNode{aggNode})

	var mu sync.Mutex
	var received []byte
	subNode := node.NewBaseNode("sub2")
	subNode.SetProcessFunc(func(input []byte) ([]byte, error) {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, input...)
		return nil, nil
	})
	_ = aggNode.Subscribe(subNode)

	// Send messages under batch size
	_, _ = aggNode.Process([]byte("X"))
	_, _ = aggNode.Process([]byte("Y"))

	// Wait for ticker to flush
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	if string(received) != "X-Y" {
		t.Fatalf("Expected 'X-Y', got '%s'", received)
	}
	mu.Unlock()
}

func TestAggregatorNode_Concurrency(t *testing.T) {
	aggNode := node.NewAggregatorNode("agg3", 50, 1*time.Hour, node.DefaultAggregator)
	_ = aggNode.Create()
	defer cleanupNodes(t, []*node.AggregatorNode{aggNode})

	var mu sync.Mutex
	var received []byte
	subNode := node.NewBaseNode("sub3")
	subNode.SetProcessFunc(func(input []byte) ([]byte, error) {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, input...)
		received = append(received, []byte("\n")...)
		return nil, nil
	})
	_ = aggNode.Subscribe(subNode)

	var wg sync.WaitGroup
	// 50 goroutines sending 1 message each
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = aggNode.Process([]byte("M"))
		}()
	}

	wg.Wait()
	time.Sleep(50 * time.Millisecond) // Wait for notify

	mu.Lock()
	count := strings.Count(string(received), "M")
	if count != 50 {
		t.Fatalf("Expected 50 'M's, got %d", count)
	}
	mu.Unlock()
}
