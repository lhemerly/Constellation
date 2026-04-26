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

	numMappers := len(mr.mappers)
	// Pre-allocate result and error slices to avoid lock contention
	results := make([][]byte, numMappers)
	errs := make([]error, numMappers)

	// Step 1: Map
	// The input is broadcasted to all mappers.
	// For a more advanced implementation, the input could be split into chunks.
	for i, mapper := range mr.mappers {
		wg.Add(1)
		go func(idx int, m Node) {
			defer wg.Done()

			// Clone input to prevent data races
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			res, err := m.Process(inputCopy)

			// Assign values by index instead of using sync.Mutex and append
			if err != nil {
				errs[idx] = err
			} else {
				results[idx] = res
			}
		}(i, mapper)
	}

	wg.Wait()

	var finalErrs []error
	for _, err := range errs {
		if err != nil {
			finalErrs = append(finalErrs, err)
		}
	}

	if len(finalErrs) > 0 {
		return nil, errors.Join(append([]error{errors.New("map phase failed")}, finalErrs...)...)
	}

	// Flatten results into a single byte slice for the reducer
	// Format: simple concatenation for this basic implementation.
	var totalLen int
	for _, res := range results {
		totalLen += len(res)
	}

	reducedInput := make([]byte, 0, totalLen)
	for _, res := range results {
		reducedInput = append(reducedInput, res...)
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
