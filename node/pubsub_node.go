package node

import (
	"encoding/json"
	"errors"
	"sync"
)

// PubSubMessage is the structure expected by the PubSubNode for processing.
type PubSubMessage struct {
	Topic   string `json:"topic"`
	Payload []byte `json:"payload"`
}

// PubSubNode allows nodes to subscribe to specific topics rather than receiving all notifications.
type PubSubNode struct {
	*BaseNode
	topicSubs map[string]map[string]Node
	mu        sync.RWMutex
}

// NewPubSubNode creates a new PubSubNode.
func NewPubSubNode(id string) *PubSubNode {
	psn := &PubSubNode{
		BaseNode:  NewBaseNode(id),
		topicSubs: make(map[string]map[string]Node),
	}
	psn.SetProcessFunc(psn.pubSubProcess)
	return psn
}

// SubscribeTopic adds a node to the subscription list for a specific topic.
func (psn *PubSubNode) SubscribeTopic(topic string, node Node) error {
	psn.mu.Lock()
	defer psn.mu.Unlock()

	if _, exists := psn.topicSubs[topic]; !exists {
		psn.topicSubs[topic] = make(map[string]Node)
	}
	psn.topicSubs[topic][node.GetID()] = node

	// Also subscribe to the BaseNode to keep general track, though we route by topic
	return psn.BaseNode.Subscribe(node)
}

// UnsubscribeTopic removes a node from the subscription list for a specific topic.
func (psn *PubSubNode) UnsubscribeTopic(topic string, node Node) error {
	psn.mu.Lock()
	defer psn.mu.Unlock()

	if subs, exists := psn.topicSubs[topic]; exists {
		delete(subs, node.GetID())
		if len(subs) == 0 {
			delete(psn.topicSubs, topic)
		}
	}

	return psn.BaseNode.Unsubscribe(node)
}

// Publish sends an event to all nodes subscribed to a specific topic.
func (psn *PubSubNode) Publish(topic string, payload []byte) error {
	msg := PubSubMessage{
		Topic:   topic,
		Payload: payload,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = psn.Process(data)
	return err
}

// pubSubProcess handles routing the input to the appropriate topic subscribers.
// The input must be a JSON-encoded PubSubMessage.
func (psn *PubSubNode) pubSubProcess(input []byte) ([]byte, error) {
	var msg PubSubMessage
	if err := json.Unmarshal(input, &msg); err != nil {
		return nil, errors.New("invalid pubsub message format")
	}

	psn.mu.RLock()
	subsForTopic, exists := psn.topicSubs[msg.Topic]
	if !exists {
		psn.mu.RUnlock()
		return nil, nil // No subscribers for this topic, drop silently or could return an error
	}

	// Copy subscribers to avoid holding lock during processing
	targets := make([]Node, 0, len(subsForTopic))
	for _, sub := range subsForTopic {
		targets = append(targets, sub)
	}
	psn.mu.RUnlock()

	var wg sync.WaitGroup
	var errs []error
	var errMu sync.Mutex

	for _, target := range targets {
		wg.Add(1)
		go func(t Node) {
			defer wg.Done()

			// Clone payload to prevent data races
			payloadCopy := make([]byte, len(msg.Payload))
			copy(payloadCopy, msg.Payload)

			_, err := t.Process(payloadCopy)
			if err != nil {
				errMu.Lock()
				errs = append(errs, err)
				errMu.Unlock()
			}
		}(target)
	}

	wg.Wait()

	if len(errs) > 0 {
		return nil, errors.Join(append([]error{errors.New("some topic subscribers failed")}, errs...)...)
	}

	return nil, nil
}
