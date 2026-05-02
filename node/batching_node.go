package node

import (
	"sync"
	"time"
)

// BatchingNode extends BaseNode to collect incoming requests and flush them as a single batch.
type BatchingNode struct {
	*BaseNode
	batchSize     int
	flushInterval time.Duration
	buffer        [][]byte
	bufferMutex   sync.Mutex
	stopChan      chan struct{}
	flushwg       sync.WaitGroup
}

// NewBatchingNode creates a new BatchingNode with the given batch size and flush interval.
func NewBatchingNode(id string, batchSize int, flushInterval time.Duration) *BatchingNode {
	b := &BatchingNode{
		BaseNode:      NewBaseNode(id),
		batchSize:     batchSize,
		flushInterval: flushInterval,
		buffer:        make([][]byte, 0, batchSize),
		stopChan:      make(chan struct{}),
	}
	b.SetProcessFunc(b.processBatching)
	return b
}

// Create initializes the batching node and starts the background flush goroutine.
func (b *BatchingNode) Create() error {
	if err := b.BaseNode.Create(); err != nil {
		return err
	}
	b.flushwg.Add(1)
	go b.flushLoop()
	return nil
}

// Delete cleans up the batching node and stops the background flush goroutine.
func (b *BatchingNode) Delete() error {
	close(b.stopChan)
	b.flushwg.Wait()
	return b.BaseNode.Delete()
}

// processBatching adds the input to the buffer and flushes if the batch size is reached.
func (b *BatchingNode) processBatching(input []byte) ([]byte, error) {
	b.bufferMutex.Lock()

	// Clone input to avoid data corruption if the caller modifies it later
	inputCopy := make([]byte, len(input))
	copy(inputCopy, input)

	b.buffer = append(b.buffer, inputCopy)

	var batch [][]byte
	if len(b.buffer) >= b.batchSize {
		batch = b.buffer
		b.buffer = make([][]byte, 0, b.batchSize)
	}
	b.bufferMutex.Unlock()

	if len(batch) > 0 {
		b.flushBatch(batch)
	}

	return nil, nil // Return nil as output since this is an async node
}

// flushLoop periodically flushes the buffer if the flush interval has elapsed.
func (b *BatchingNode) flushLoop() {
	defer b.flushwg.Done()
	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			b.bufferMutex.Lock()
			var batch [][]byte
			if len(b.buffer) > 0 {
				batch = b.buffer
				b.buffer = make([][]byte, 0, b.batchSize)
			}
			b.bufferMutex.Unlock()

			if len(batch) > 0 {
				b.flushBatch(batch)
			}
		case <-b.stopChan:
			// Flush remaining items before shutting down
			b.bufferMutex.Lock()
			var batch [][]byte
			if len(b.buffer) > 0 {
				batch = b.buffer
				b.buffer = nil
			}
			b.bufferMutex.Unlock()

			if len(batch) > 0 {
				b.flushBatch(batch)
			}
			return
		}
	}
}

// flushBatch flattens the batch and notifies subscribers.
func (b *BatchingNode) flushBatch(batch [][]byte) {
	if len(batch) == 0 {
		return
	}

	// Flatten the batch (simple concatenation for this implementation)
	var totalLen int
	for _, data := range batch {
		totalLen += len(data)
	}

	flattened := make([]byte, 0, totalLen)
	for _, data := range batch {
		flattened = append(flattened, data...)
	}

	// The original process returned nil, so we send the aggregated data to subscribers
	b.Notify(flattened)
}
