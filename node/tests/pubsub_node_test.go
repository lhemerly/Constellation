package node_test

import (
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/lhemerly/Constellation/node"
)

func TestPubSubNode_Publish(t *testing.T) {
	psn := node.NewPubSubNode("pubsub")

	var sub1Received int32
	sub1 := node.NewBaseNode("sub1")
	sub1.SetProcessFunc(func(input []byte) ([]byte, error) {
		atomic.AddInt32(&sub1Received, 1)
		return nil, nil
	})

	var sub2Received int32
	sub2 := node.NewBaseNode("sub2")
	sub2.SetProcessFunc(func(input []byte) ([]byte, error) {
		atomic.AddInt32(&sub2Received, 1)
		return nil, nil
	})

	err := psn.SubscribeTopic("news", sub1)
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	err = psn.SubscribeTopic("sports", sub2)
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	err = psn.SubscribeTopic("news", sub2) // sub2 listens to news and sports
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	err = psn.Publish("news", []byte("breaking news"))
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	if atomic.LoadInt32(&sub1Received) != 1 {
		t.Errorf("Expected sub1 to receive 1 news event, got %d", sub1Received)
	}
	if atomic.LoadInt32(&sub2Received) != 1 {
		t.Errorf("Expected sub2 to receive 1 news event, got %d", sub2Received)
	}

	err = psn.Publish("sports", []byte("game over"))
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	if atomic.LoadInt32(&sub1Received) != 1 {
		t.Errorf("Expected sub1 to still have 1 event, got %d", sub1Received)
	}
	if atomic.LoadInt32(&sub2Received) != 2 {
		t.Errorf("Expected sub2 to have 2 events total, got %d", sub2Received)
	}

	err = psn.UnsubscribeTopic("news", sub2)
	if err != nil {
		t.Fatalf("Failed to unsubscribe: %v", err)
	}

	err = psn.Publish("news", []byte("more news"))
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	if atomic.LoadInt32(&sub1Received) != 2 {
		t.Errorf("Expected sub1 to have 2 events, got %d", sub1Received)
	}
	if atomic.LoadInt32(&sub2Received) != 2 {
		t.Errorf("Expected sub2 to still have 2 events, got %d", sub2Received)
	}
}

func TestPubSubNode_InvalidMessage(t *testing.T) {
	psn := node.NewPubSubNode("pubsub")
	_, err := psn.Process([]byte("not valid json"))
	if err == nil {
		t.Fatal("Expected error on invalid json")
	}
	if !strings.Contains(err.Error(), "invalid pubsub message format") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestPubSubNode_Process(t *testing.T) {
	psn := node.NewPubSubNode("pubsub")

	var sub1Received int32
	sub1 := node.NewBaseNode("sub1")
	sub1.SetProcessFunc(func(input []byte) ([]byte, error) {
		atomic.AddInt32(&sub1Received, 1)
		return nil, nil
	})

	_ = psn.SubscribeTopic("events", sub1)

	msg := node.PubSubMessage{
		Topic:   "events",
		Payload: []byte("payload"),
	}
	data, _ := json.Marshal(msg)

	_, err := psn.Process(data)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if atomic.LoadInt32(&sub1Received) != 1 {
		t.Errorf("Expected sub1 to receive 1 event via Process, got %d", sub1Received)
	}
}
