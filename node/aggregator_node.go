package node

import (
	"bytes"
	"sync"
	"time"
)

// AggregatorNode collects incoming messages into batches.
// It flushes a batch by aggregating it with an aggregator function and then
// notifying subscribers, when either the batch size is met or the flush interval triggers.
type AggregatorNode struct {
	*BaseNode
	batchSize     int
	flushInterval time.Duration
	aggregator    func([][]byte) []byte

	mu       sync.Mutex
	batch    [][]byte
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewAggregatorNode creates a new AggregatorNode.
func NewAggregatorNode(id string, batchSize int, flushInterval time.Duration, aggregator func([][]byte) []byte) *AggregatorNode {
	an := &AggregatorNode{
		BaseNode:      NewBaseNode(id),
		batchSize:     batchSize,
		flushInterval: flushInterval,
		aggregator:    aggregator,
		batch:         make([][]byte, 0, batchSize),
		stopChan:      make(chan struct{}),
	}

	an.SetProcessFunc(an.process)
	return an
}

// Create starts the background ticker for time-based flushing.
func (an *AggregatorNode) Create() error {
	if err := an.BaseNode.Create(); err != nil {
		return err
	}

	an.wg.Add(1)
	go an.flushLoop()
	return nil
}

// Delete stops the background ticker and releases resources.
func (an *AggregatorNode) Delete() error {
	close(an.stopChan)
	an.wg.Wait()
	return an.BaseNode.Delete()
}

func (an *AggregatorNode) process(input []byte) ([]byte, error) {
	an.mu.Lock()

	// Create a copy to prevent external mutation
	inputCopy := make([]byte, len(input))
	copy(inputCopy, input)

	an.batch = append(an.batch, inputCopy)

	if len(an.batch) >= an.batchSize {
		batchToFlush := an.batch
		an.batch = make([][]byte, 0, an.batchSize)
		an.mu.Unlock()

		an.flush(batchToFlush)
		return nil, nil // Return immediately after processing
	}

	an.mu.Unlock()
	return nil, nil
}

func (an *AggregatorNode) flushLoop() {
	defer an.wg.Done()
	ticker := time.NewTicker(an.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			an.mu.Lock()
			if len(an.batch) > 0 {
				batchToFlush := an.batch
				an.batch = make([][]byte, 0, an.batchSize)
				an.mu.Unlock()
				an.flush(batchToFlush)
			} else {
				an.mu.Unlock()
			}
		case <-an.stopChan:
			// Flush any remaining items before stopping
			an.mu.Lock()
			if len(an.batch) > 0 {
				batchToFlush := an.batch
				an.batch = nil
				an.mu.Unlock()
				an.flush(batchToFlush)
			} else {
				an.mu.Unlock()
			}
			return
		}
	}
}

func (an *AggregatorNode) flush(batch [][]byte) {
	if len(batch) == 0 {
		return
	}
	aggregatedData := an.aggregator(batch)
	_ = an.Notify(aggregatedData)
}

// DefaultAggregator is a simple aggregator that joins bytes with a newline.
func DefaultAggregator(batch [][]byte) []byte {
	return bytes.Join(batch, []byte("\n"))
}
