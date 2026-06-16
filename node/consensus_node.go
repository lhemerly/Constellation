package node

import (
	"crypto/sha256"
	"errors"
	"sync"
)

// ErrConsensusFailed is returned when the node fails to reach a quorum among its sub-nodes.
var ErrConsensusFailed = errors.New("failed to reach consensus quorum")

// ConsensusNode dispatches input to a set of redundant nodes concurrently
// and returns the output that appears most frequently (quorum/majority),
// ignoring faults and outliers.
type ConsensusNode struct {
	*BaseNode
	nodes []Node
}

// NewConsensusNode creates a new ConsensusNode.
func NewConsensusNode(id string, nodes []Node) *ConsensusNode {
	if len(nodes) == 0 {
		panic("ConsensusNode requires at least one node")
	}

	cn := &ConsensusNode{
		BaseNode: NewBaseNode(id),
		nodes:    nodes,
	}

	cn.SetProcessFunc(cn.processConsensus)
	return cn
}

// processConsensus dispatches input concurrently and finds the quorum result.
func (cn *ConsensusNode) processConsensus(input []byte) ([]byte, error) {
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results = make([][]byte, 0, len(cn.nodes))
	)

	// Dispatch to all nodes concurrently
	for _, n := range cn.nodes {
		wg.Add(1)
		go func(targetNode Node) {
			defer wg.Done()

			// Clone input to prevent data races if nodes modify it in-place
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			res, err := targetNode.Process(inputCopy)
			if err == nil {
				mu.Lock()
				results = append(results, res)
				mu.Unlock()
			}
		}(n)
	}

	wg.Wait()

	if len(results) == 0 {
		return nil, errors.Join(ErrConsensusFailed, errors.New("all nodes failed"))
	}

	// Count occurrences of each result
	type resultData struct {
		count int
		data  []byte
	}
	counts := make(map[[32]byte]*resultData)

	maxCount := 0
	var quorumData []byte

	for _, res := range results {
		hash := sha256.Sum256(res)
		if entry, exists := counts[hash]; exists {
			entry.count++
			if entry.count > maxCount {
				maxCount = entry.count
				quorumData = entry.data
			}
		} else {
			counts[hash] = &resultData{count: 1, data: res}
			if maxCount == 0 { // First entry
				maxCount = 1
				quorumData = res
			}
		}
	}

	// Quorum condition: strictly greater than half of the *total configured nodes*,
	// not just the successful ones, to ensure true consensus.
	requiredQuorum := (len(cn.nodes) / 2) + 1
	if maxCount >= requiredQuorum {
		return quorumData, nil
	}

	return nil, ErrConsensusFailed
}

// Create initializes the consensus node and its dependencies.
func (cn *ConsensusNode) Create() error {
	if err := cn.BaseNode.Create(); err != nil {
		return err
	}
	for _, n := range cn.nodes {
		if err := n.Create(); err != nil {
			return err
		}
	}
	return nil
}

// Delete cleans up the consensus node and its dependencies.
func (cn *ConsensusNode) Delete() error {
	var errs []error
	if err := cn.BaseNode.Delete(); err != nil {
		errs = append(errs, err)
	}
	for _, n := range cn.nodes {
		if err := n.Delete(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
