package node

import (
	"bytes"
	"errors"
	"sync"
)

// DAGNode allows executing a Directed Acyclic Graph of nodes.
// Each node can have dependencies, and it will only be executed after
// all its dependencies have successfully executed. The outputs of the dependencies
// are concatenated and passed as input to the node. If a node has no dependencies,
// it receives the initial input passed to the DAGNode.
type DAGNode struct {
	*BaseNode
	nodes        map[string]Node
	dependencies map[string][]string
}

// NewDAGNode creates a new empty DAGNode.
func NewDAGNode(id string) *DAGNode {
	d := &DAGNode{
		BaseNode:     NewBaseNode(id),
		nodes:        make(map[string]Node),
		dependencies: make(map[string][]string),
	}
	d.SetProcessFunc(d.processDAG)
	return d
}

// AddNode adds a node to the DAG with its dependencies.
func (d *DAGNode) AddNode(node Node, deps ...string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.nodes[node.GetID()] = node
	d.dependencies[node.GetID()] = deps
}

// Create initializes all nodes in the DAG.
func (d *DAGNode) Create() error {
	if err := d.BaseNode.Create(); err != nil {
		return err
	}
	for _, n := range d.nodes {
		if err := n.Create(); err != nil {
			return err
		}
	}
	return nil
}

// Delete cleans up all nodes in the DAG.
func (d *DAGNode) Delete() error {
	var errs []error
	if err := d.BaseNode.Delete(); err != nil {
		errs = append(errs, err)
	}
	for _, n := range d.nodes {
		if err := n.Delete(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (d *DAGNode) processDAG(input []byte) ([]byte, error) {
	d.mutex.RLock()
	nodesCount := len(d.nodes)
	nodesMap := make(map[string]Node, nodesCount)
	depsMap := make(map[string][]string, nodesCount)

	for k, v := range d.nodes {
		nodesMap[k] = v
	}
	for k, v := range d.dependencies {
		depsMap[k] = v
	}
	d.mutex.RUnlock()

	// Track results and errors for each node
	var (
		resultsMutex sync.Mutex
		results      = make(map[string][]byte)
		nodeErrs     []error
		doneChans    = make(map[string]chan struct{})
	)

	for id := range nodesMap {
		doneChans[id] = make(chan struct{})
	}

	var wg sync.WaitGroup

	for id, n := range nodesMap {
		wg.Add(1)
		go func(nodeID string, node Node) {
			defer wg.Done()
			defer close(doneChans[nodeID])

			deps := depsMap[nodeID]

			// Wait for dependencies to finish
			for _, depID := range deps {
				if depChan, exists := doneChans[depID]; exists {
					<-depChan
				}
			}

			resultsMutex.Lock()
			hasErr := len(nodeErrs) > 0
			resultsMutex.Unlock()

			// If any dependency (or any other node) failed, short-circuit
			if hasErr {
				return
			}

			// Gather inputs
			var nodeInput []byte
			if len(deps) == 0 {
				nodeInput = make([]byte, len(input))
				copy(nodeInput, input)
			} else {
				var buf bytes.Buffer
				resultsMutex.Lock()
				for _, depID := range deps {
					buf.Write(results[depID])
				}
				resultsMutex.Unlock()
				nodeInput = buf.Bytes()
			}

			out, err := node.Process(nodeInput)

			resultsMutex.Lock()
			if err != nil {
				nodeErrs = append(nodeErrs, err)
			} else {
				results[nodeID] = out
			}
			resultsMutex.Unlock()
		}(id, n)
	}

	wg.Wait()

	if len(nodeErrs) > 0 {
		return nil, errors.Join(append([]error{errors.New("DAG execution failed")}, nodeErrs...)...)
	}

	// Determine leaf nodes (nodes that are not dependencies for any other node)
	isDep := make(map[string]bool)
	for _, deps := range depsMap {
		for _, dep := range deps {
			isDep[dep] = true
		}
	}

	var finalOutput bytes.Buffer
	for id := range nodesMap {
		if !isDep[id] {
			finalOutput.Write(results[id])
		}
	}

	return finalOutput.Bytes(), nil
}
