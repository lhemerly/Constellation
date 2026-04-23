package node

import (
	"errors"
	"sync"
)

// ErrNoNodesFailover is returned when both primary and fallback nodes are missing.
var ErrNoNodesFailover = errors.New("no primary or fallback nodes configured")

// ErrFailoverFailed is returned when both primary and fallback processing fail.
var ErrFailoverFailed = errors.New("failover completely failed")

// FailoverNode extends BaseNode to provide fault tolerance by trying a secondary node if the primary fails.
type FailoverNode struct {
	*BaseNode
	nodesMutex sync.RWMutex
	primary    Node
	fallback   Node
}

// NewFailoverNode creates a new FailoverNode with the given ID.
func NewFailoverNode(id string) *FailoverNode {
	f := &FailoverNode{
		BaseNode: NewBaseNode(id),
	}

	f.SetProcessFunc(f.processFailover)
	return f
}

// SetPrimary sets the primary node.
func (f *FailoverNode) SetPrimary(node Node) {
	f.nodesMutex.Lock()
	defer f.nodesMutex.Unlock()
	f.primary = node
}

// SetFallback sets the secondary/fallback node.
func (f *FailoverNode) SetFallback(node Node) {
	f.nodesMutex.Lock()
	defer f.nodesMutex.Unlock()
	f.fallback = node
}

// processFailover attempts to process using primary, then fallback on error.
func (f *FailoverNode) processFailover(input []byte) ([]byte, error) {
	f.nodesMutex.RLock()
	primaryNode := f.primary
	fallbackNode := f.fallback
	f.nodesMutex.RUnlock()

	if primaryNode == nil && fallbackNode == nil {
		return nil, ErrNoNodesFailover
	}

	var primaryErr error
	if primaryNode != nil {
		res, err := primaryNode.Process(input)
		if err == nil {
			return res, nil
		}
		primaryErr = err
	}

	if fallbackNode != nil {
		res, err := fallbackNode.Process(input)
		if err == nil {
			return res, nil
		}
		// Return wrapped error or just failover error, we can combine them.
		return nil, errors.Join(ErrFailoverFailed, primaryErr, err)
	}

	return nil, errors.Join(ErrFailoverFailed, primaryErr)
}
