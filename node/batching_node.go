package node

import (
	"sync"
	"time"
)

// BatchingNode acts as a node that accumulates incoming inputs until a certain
// batch size or timeout is reached, at which point it processes them together
// and resets the batch. To do this, it calls an underlying node or process function
// with the concatenated batch.
type BatchingNode struct {
	*BaseNode
	batchSize    int
	batchTimeout time.Duration
	batch        [][]byte
	mu           sync.Mutex
	timer        *time.Timer
	targetNode   Node
	stopChan     chan struct{}
	wg           sync.WaitGroup
}

// NewBatchingNode creates a new BatchingNode.
// TargetNode is the node that will receive the concatenated batch when flush triggers.
func NewBatchingNode(id string, batchSize int, batchTimeout time.Duration, targetNode Node) *BatchingNode {
	if batchSize <= 0 {
		panic("batchSize must be greater than 0")
	}

	bn := &BatchingNode{
		BaseNode:     NewBaseNode(id),
		batchSize:    batchSize,
		batchTimeout: batchTimeout,
		targetNode:   targetNode,
		stopChan:     make(chan struct{}),
	}

	bn.SetProcessFunc(bn.batchProcess)

	return bn
}

func (bn *BatchingNode) Create() error {
	if err := bn.BaseNode.Create(); err != nil {
		return err
	}
	if bn.targetNode != nil {
		if err := bn.targetNode.Create(); err != nil {
			return err
		}
	}

	bn.mu.Lock()
	bn.timer = time.AfterFunc(bn.batchTimeout, bn.flushTimeout)
	bn.mu.Unlock()

	return nil
}

func (bn *BatchingNode) flushTimeout() {
	bn.mu.Lock()
	if bn.timer == nil {
		bn.mu.Unlock()
		return
	}

	if len(bn.batch) > 0 {
		batchToProcess := bn.batch
		bn.batch = nil
		bn.wg.Add(1)
		bn.mu.Unlock()
		bn.processBatch(batchToProcess)
	} else {
		bn.mu.Unlock()
	}

	bn.mu.Lock()
	if bn.timer != nil {
		bn.timer.Reset(bn.batchTimeout)
	}
	bn.mu.Unlock()
}

func (bn *BatchingNode) processBatch(batch [][]byte) {
	defer bn.wg.Done()

	if bn.targetNode == nil {
		return
	}

	var totalLen int
	for _, b := range batch {
		totalLen += len(b)
	}

	flattened := make([]byte, 0, totalLen)
	for _, b := range batch {
		flattened = append(flattened, b...)
	}

	// We run it asynchronously or synchronously, depending on usage.
	// The problem describes process returning ([]byte, error),
	// but a batching node typically doesn't return anything meaningful to the caller of Process()
	// because Process() just enqueues it.
	_, _ = bn.targetNode.Process(flattened)
}

func (bn *BatchingNode) batchProcess(input []byte) ([]byte, error) {
	bn.mu.Lock()

	// Copy input so it's not modified externally
	inputCopy := make([]byte, len(input))
	copy(inputCopy, input)

	bn.batch = append(bn.batch, inputCopy)

	if len(bn.batch) >= bn.batchSize {
		batchToProcess := bn.batch
		bn.batch = nil

		if bn.timer != nil {
			if !bn.timer.Stop() {
				// drain channel if stopped
				select {
				case <-bn.timer.C:
				default:
				}
			}
			bn.timer.Reset(bn.batchTimeout)
		}

		bn.wg.Add(1)
		bn.mu.Unlock()

		// Run processing asynchronously to not block the caller
		go bn.processBatch(batchToProcess)

		return nil, nil // Enqueued successfully
	}

	bn.mu.Unlock()
	return nil, nil // Enqueued successfully
}

// Delete stops the batching node and flushes any remaining data
func (bn *BatchingNode) Delete() error {
	bn.mu.Lock()
	if bn.timer != nil {
		bn.timer.Stop()
		bn.timer = nil
	}

	batchToProcess := bn.batch
	bn.batch = nil
	bn.mu.Unlock()

	if len(batchToProcess) > 0 {
		bn.wg.Add(1)
		bn.processBatch(batchToProcess)
	}

	close(bn.stopChan)
	bn.wg.Wait() // wait for any background processing to finish

	if bn.targetNode != nil {
		if err := bn.targetNode.Delete(); err != nil {
			return err
		}
	}

	return bn.BaseNode.Delete()
}
