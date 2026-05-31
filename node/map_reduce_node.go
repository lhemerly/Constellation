package node

import (
	"errors"
	"sync"
)

// MapReduceNode represents a node that acts as a coordinator for map-reduce tasks.
// It uses multiple mapper nodes to process data concurrently, and a single reducer node
// to aggregate the results.
type MapReduceNode struct {
	*BaseNode
	mappers []Node
	reducer Node
}

// NewMapReduceNode creates a new MapReduceNode.
func NewMapReduceNode(id string, mappers []Node, reducer Node) *MapReduceNode {
	if len(mappers) == 0 {
		panic("MapReduceNode requires at least one mapper")
	}
	if reducer == nil {
		panic("MapReduceNode requires a reducer")
	}

	mrNode := &MapReduceNode{
		BaseNode: NewBaseNode(id),
		mappers:  mappers,
		reducer:  reducer,
	}

	mrNode.SetProcessFunc(mrNode.mapReduceProcess)

	return mrNode
}

// mapReduceProcess handles the map-reduce lifecycle.
func (mr *MapReduceNode) mapReduceProcess(input []byte) ([]byte, error) {
	var wg sync.WaitGroup
	mapperCount := len(mr.mappers)

	// Pre-allocate slices to avoid mutex lock contention
	errs := make([]error, mapperCount)
	results := make([][]byte, mapperCount)

	// Step 1: Map
	// The input is broadcasted to all mappers.
	// For a more advanced implementation, the input could be split into chunks.
	for i, mapper := range mr.mappers {
		wg.Add(1)
		go func(i int, m Node) {
			defer wg.Done()

			// Clone input to prevent data races
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			res, err := m.Process(inputCopy)

			// Assign by index to avoid lock contention
			if err != nil {
				errs[i] = err
			} else {
				results[i] = res
			}
		}(i, mapper)
	}

	wg.Wait()

	// Collect errors
	var finalErrs []error
	for _, err := range errs {
		if err != nil {
			finalErrs = append(finalErrs, err)
		}
	}

	if len(finalErrs) > 0 {
		return nil, errors.Join(append([]error{errors.New("map phase failed")}, finalErrs...)...)
	}

	// Calculate total capacity for flattened results to avoid reallocations
	totalLen := 0
	for _, res := range results {
		totalLen += len(res)
	}

	// Flatten results into a single byte slice for the reducer
	// Format: simple concatenation for this basic implementation.
	reducedInput := make([]byte, 0, totalLen)
	for _, res := range results {
		if len(res) > 0 {
			reducedInput = append(reducedInput, res...)
		}
	}

	// Step 2: Reduce
	return mr.reducer.Process(reducedInput)
}

// Create initializes the map-reduce node and its dependencies.
func (mr *MapReduceNode) Create() error {
	if err := mr.BaseNode.Create(); err != nil {
		return err
	}
	for _, m := range mr.mappers {
		if err := m.Create(); err != nil {
			return err
		}
	}
	return mr.reducer.Create()
}

// Delete cleans up the map-reduce node and its dependencies.
func (mr *MapReduceNode) Delete() error {
	var errs []error
	if err := mr.BaseNode.Delete(); err != nil {
		errs = append(errs, err)
	}
	for _, m := range mr.mappers {
		if err := m.Delete(); err != nil {
			errs = append(errs, err)
		}
	}
	if err := mr.reducer.Delete(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
