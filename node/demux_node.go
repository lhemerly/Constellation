package node

import (
	"errors"
	"sync"
)

// ErrNoDemuxRoutes is returned when no routes matched in a demultiplexer node.
var ErrNoDemuxRoutes = errors.New("no routes matched for demultiplexer")

// DemultiplexerRoute defines a condition for routing to a specific destination node.
// It is similar to RouterNode's Route, but used in the context of DemuxNode.
type DemultiplexerRoute struct {
	Condition   func([]byte) bool
	Destination Node
}

// DemuxNode extends BaseNode to conditionally route a single input to multiple
// destination nodes simultaneously, joining their outputs or errors.
type DemuxNode struct {
	*BaseNode
	routesMutex sync.RWMutex
	routes      []DemultiplexerRoute
}

// NewDemuxNode creates a new DemuxNode with the given ID.
func NewDemuxNode(id string) *DemuxNode {
	d := &DemuxNode{
		BaseNode: NewBaseNode(id),
		routes:   make([]DemultiplexerRoute, 0),
	}

	d.SetProcessFunc(d.demuxData)
	return d
}

// AddRoute adds a new conditional route.
func (d *DemuxNode) AddRoute(condition func([]byte) bool, destination Node) {
	d.routesMutex.Lock()
	defer d.routesMutex.Unlock()
	d.routes = append(d.routes, DemultiplexerRoute{Condition: condition, Destination: destination})
}

// demuxData evaluates the input against all registered routes and forwards it
// to *all* matching destination nodes concurrently. It aggregates and concatenates
// the results.
func (d *DemuxNode) demuxData(input []byte) ([]byte, error) {
	d.routesMutex.RLock()
	var targets []Node
	for _, route := range d.routes {
		if route.Condition(input) {
			targets = append(targets, route.Destination)
		}
	}
	d.routesMutex.RUnlock()

	if len(targets) == 0 {
		return nil, ErrNoDemuxRoutes
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error
	results := make([][]byte, len(targets))

	for i, target := range targets {
		wg.Add(1)
		go func(idx int, dest Node) {
			defer wg.Done()

			// Clone input to prevent data races if nodes modify the byte slice
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			res, err := dest.Process(inputCopy)

			if err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			} else {
				// Assign result by index to avoid lock contention
				results[idx] = res
			}
		}(i, target)
	}

	wg.Wait()

	if len(errs) > 0 {
		return nil, errors.Join(append([]error{errors.New("demux processing failed")}, errs...)...)
	}

	// Calculate total length for pre-allocation
	var totalLen int
	for _, res := range results {
		totalLen += len(res)
	}

	finalOutput := make([]byte, 0, totalLen)
	for _, res := range results {
		if res != nil {
			finalOutput = append(finalOutput, res...)
		}
	}

	return finalOutput, nil
}
