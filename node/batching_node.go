package node

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrBatchingNodeNoTarget = errors.New("no target node configured for batching")

// BatchingNode groups multiple incoming requests into a single batch and
// flushes them to a target node based on a batch size or a timeout.
type BatchingNode struct {
	*BaseNode
	targetMutex sync.RWMutex
	target      Node

	batchSize    int
	batchTimeout time.Duration

	mu      sync.Mutex
	batch   [][]byte
	waiters []chan struct {
		res []byte
		err error
	}
	timer *time.Timer

	ctx    context.Context
	cancel context.CancelFunc
}

// NewBatchingNode creates a new BatchingNode.
func NewBatchingNode(id string, batchSize int, batchTimeout time.Duration) *BatchingNode {
	ctx, cancel := context.WithCancel(context.Background())
	bn := &BatchingNode{
		BaseNode:     NewBaseNode(id),
		batchSize:    batchSize,
		batchTimeout: batchTimeout,
		ctx:          ctx,
		cancel:       cancel,
	}

	bn.SetProcessFunc(bn.addToBatch)
	return bn
}

// SetTarget sets the destination node where batches are flushed.
func (bn *BatchingNode) SetTarget(node Node) {
	bn.targetMutex.Lock()
	defer bn.targetMutex.Unlock()
	bn.target = node
}

// Delete cleans up the node and stops background flush timers.
func (bn *BatchingNode) Delete() error {
	bn.cancel()

	bn.mu.Lock()
	if bn.timer != nil {
		bn.timer.Stop()
	}

	// flush remaining items with an error, or just return an error to waiting channels
	for _, ch := range bn.waiters {
		ch <- struct {
			res []byte
			err error
		}{nil, errors.New("node deleted before batch could be processed")}
	}
	bn.waiters = nil
	bn.batch = nil
	bn.mu.Unlock()

	return bn.BaseNode.Delete()
}

func (bn *BatchingNode) addToBatch(input []byte) ([]byte, error) {
	bn.targetMutex.RLock()
	target := bn.target
	bn.targetMutex.RUnlock()

	if target == nil {
		return nil, ErrBatchingNodeNoTarget
	}

	resChan := make(chan struct {
		res []byte
		err error
	}, 1)

	bn.mu.Lock()

	// Clone the input to prevent modification while waiting in the batch
	inputCopy := make([]byte, len(input))
	copy(inputCopy, input)

	bn.batch = append(bn.batch, inputCopy)
	bn.waiters = append(bn.waiters, resChan)

	if len(bn.batch) == 1 {
		// First item in a new batch, start the timer
		bn.timer = time.AfterFunc(bn.batchTimeout, func() {
			bn.flush()
		})
	}

	shouldFlush := len(bn.batch) >= bn.batchSize
	bn.mu.Unlock()

	if shouldFlush {
		bn.flush()
	}

	select {
	case <-bn.ctx.Done():
		return nil, errors.New("batching node stopped")
	case res := <-resChan:
		return res.res, res.err
	}
}

func (bn *BatchingNode) flush() {
	bn.mu.Lock()
	if len(bn.batch) == 0 {
		bn.mu.Unlock()
		return
	}

	if bn.timer != nil {
		bn.timer.Stop()
	}

	batch := bn.batch
	waiters := bn.waiters

	bn.batch = nil
	bn.waiters = nil
	bn.mu.Unlock()

	bn.targetMutex.RLock()
	target := bn.target
	bn.targetMutex.RUnlock()

	if target == nil {
		for _, ch := range waiters {
			ch <- struct {
				res []byte
				err error
			}{nil, ErrBatchingNodeNoTarget}
		}
		return
	}

	// For simplicity, we just concatenate all items in the batch
	// and send it as a single request to the target node.
	var totalLen int
	for _, item := range batch {
		totalLen += len(item)
	}

	combined := make([]byte, 0, totalLen)
	for _, item := range batch {
		combined = append(combined, item...)
	}

	res, err := target.Process(combined)

	// In a more complex scenario, the target node might return a list of results
	// corresponding to the batch items. Here, we just return the same combined result
	// and error to all waiters.
	for _, ch := range waiters {
		ch <- struct {
			res []byte
			err error
		}{res, err}
	}
}
