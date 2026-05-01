package node

import (
	"errors"
	"sync"
)

var (
	// ErrNoConsensus is returned when the nodes cannot reach a majority agreement.
	ErrNoConsensus = errors.New("no consensus reached")
	// ErrAllNodesFailed is returned when all nodes fail to process the input.
	ErrAllNodesFailed = errors.New("all nodes failed to process")
)

// ConsensusNode represents a node that distributes processing to multiple identical nodes
// and returns the result agreed upon by the majority.
type ConsensusNode struct {
	*BaseNode
	nodesMutex sync.RWMutex
	nodes      []Node
}

// NewConsensusNode creates a new ConsensusNode with the given ID.
func NewConsensusNode(id string) *ConsensusNode {
	cn := &ConsensusNode{
		BaseNode: NewBaseNode(id),
		nodes:    make([]Node, 0),
	}
	cn.SetProcessFunc(cn.processConsensus)
	return cn
}

// AddNode adds a node to participate in the consensus.
func (cn *ConsensusNode) AddNode(node Node) {
	cn.nodesMutex.Lock()
	defer cn.nodesMutex.Unlock()
	cn.nodes = append(cn.nodes, node)
}

// RemoveNode removes a node from the consensus pool by its ID.
func (cn *ConsensusNode) RemoveNode(nodeID string) {
	cn.nodesMutex.Lock()
	defer cn.nodesMutex.Unlock()

	for i, node := range cn.nodes {
		if node.GetID() == nodeID {
			cn.nodes = append(cn.nodes[:i], cn.nodes[i+1:]...)
			break
		}
	}
}

// GetNodes returns the current list of nodes participating in consensus.
func (cn *ConsensusNode) GetNodes() []Node {
	cn.nodesMutex.RLock()
	defer cn.nodesMutex.RUnlock()

	nodesCopy := make([]Node, len(cn.nodes))
	copy(nodesCopy, cn.nodes)
	return nodesCopy
}

// processConsensus distributes the input to all nodes and determines the majority result.
func (cn *ConsensusNode) processConsensus(input []byte) ([]byte, error) {
	cn.nodesMutex.RLock()
	nodesCount := len(cn.nodes)
	if nodesCount == 0 {
		cn.nodesMutex.RUnlock()
		return nil, ErrNoNodes
	}
	nodesCopy := make([]Node, nodesCount)
	copy(nodesCopy, cn.nodes)
	cn.nodesMutex.RUnlock()

	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make([][]byte, 0, nodesCount)

	for _, n := range nodesCopy {
		wg.Add(1)
		go func(node Node) {
			defer wg.Done()

			// Clone input to prevent data races
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			res, err := node.Process(inputCopy)
			if err == nil {
				mu.Lock()
				results = append(results, res)
				mu.Unlock()
			}
		}(n)
	}

	wg.Wait()

	if len(results) == 0 {
		return nil, ErrAllNodesFailed
	}

	// Tally results to find the majority
	type tally struct {
		count int
		data  []byte
	}

	tallies := make(map[string]*tally)
	maxCount := 0
	var consensusData []byte

	for _, res := range results {
		key := string(res) // string cast works for byte slice hashing in maps
		if t, ok := tallies[key]; ok {
			t.count++
		} else {
			tallies[key] = &tally{count: 1, data: res}
		}

		if tallies[key].count > maxCount {
			maxCount = tallies[key].count
			consensusData = tallies[key].data
		}
	}

	// Check if the most common result achieves a strict majority of all configured nodes
	if maxCount > nodesCount/2 {
		return consensusData, nil
	}

	return nil, ErrNoConsensus
}
