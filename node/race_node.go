package node

import (
	"errors"
	"sync"
)

var (
	// ErrNoNodesRace is returned when RaceNode is created with no nodes
	ErrNoNodesRace = errors.New("RaceNode requires at least one node")
	// ErrAllNodesFailed is returned when all nodes in a RaceNode fail to process
	ErrAllNodesFailed = errors.New("all nodes failed in race")
)

// RaceNode represents a node that forwards input to multiple competitor nodes concurrently.
// It returns the result of the first node that completes successfully, ignoring the rest.
type RaceNode struct {
	*BaseNode
	competitors []Node
}

// NewRaceNode creates a new RaceNode.
func NewRaceNode(id string, competitors []Node) *RaceNode {
	if len(competitors) == 0 {
		panic(ErrNoNodesRace.Error())
	}

	raceNode := &RaceNode{
		BaseNode:    NewBaseNode(id),
		competitors: competitors,
	}

	raceNode.SetProcessFunc(raceNode.raceProcess)

	return raceNode
}

// raceProcess handles the race lifecycle.
func (rn *RaceNode) raceProcess(input []byte) ([]byte, error) {
	resultChan := make(chan []byte, 1)
	errChan := make(chan error, len(rn.competitors))

	var wg sync.WaitGroup

	for _, competitor := range rn.competitors {
		wg.Add(1)
		go func(c Node) {
			defer wg.Done()

			// Clone input to prevent data races
			inputCopy := make([]byte, len(input))
			copy(inputCopy, input)

			res, err := c.Process(inputCopy)
			if err != nil {
				errChan <- err
				return
			}

			// Try to send result to resultChan. First one wins.
			select {
			case resultChan <- res:
			default:
				// resultChan is already full, meaning another node won the race.
			}
		}(competitor)
	}

	// Wait for all to finish in a separate goroutine
	go func() {
		wg.Wait()
		close(errChan)
	}()

	var errs []error
	for {
		select {
		case res := <-resultChan:
			return res, nil
		case err, ok := <-errChan:
			if !ok {
				// All goroutines finished and closed errChan, which means no one succeeded.
				return nil, errors.Join(append([]error{ErrAllNodesFailed}, errs...)...)
			}
			errs = append(errs, err)
		}
	}
}

// Create initializes the race node and its dependencies.
func (rn *RaceNode) Create() error {
	if err := rn.BaseNode.Create(); err != nil {
		return err
	}
	for _, c := range rn.competitors {
		if err := c.Create(); err != nil {
			return err
		}
	}
	return nil
}

// Delete cleans up the race node and its dependencies.
func (rn *RaceNode) Delete() error {
	var errs []error
	if err := rn.BaseNode.Delete(); err != nil {
		errs = append(errs, err)
	}
	for _, c := range rn.competitors {
		if err := c.Delete(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
