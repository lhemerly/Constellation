package node

import (
	"errors"
)

var (
	ErrSagaForwardFailed  = errors.New("saga forward phase failed")
	ErrSagaRollbackFailed = errors.New("saga rollback phase failed")
)

// SagaStep defines a single step in a Saga distributed transaction.
// Forward is executed first. If any Forward fails, Rollback nodes are
// executed in reverse order.
type SagaStep struct {
	Forward  Node
	Rollback Node
}

// SagaNode manages a sequence of dependent operations, executing compensating
// operations automatically if an error occurs along the chain.
type SagaNode struct {
	*BaseNode
	steps []SagaStep
}

// NewSagaNode creates a new SagaNode with the given steps.
func NewSagaNode(id string, steps []SagaStep) *SagaNode {
	sNode := &SagaNode{
		BaseNode: NewBaseNode(id),
		steps:    steps,
	}

	sNode.SetProcessFunc(sNode.sagaProcess)
	return sNode
}

// sagaProcess executes the saga steps sequentially.
// If a step's Forward node returns an error, it initiates the rollback process
// using the stored inputs for the already successful steps.
func (sn *SagaNode) sagaProcess(input []byte) ([]byte, error) {
	if len(sn.steps) == 0 {
		return input, nil
	}

	var currentInput []byte = input
	// Keep track of the inputs supplied to each step to use during rollback
	inputs := make([][]byte, len(sn.steps))

	for i, step := range sn.steps {
		inputs[i] = make([]byte, len(currentInput))
		copy(inputs[i], currentInput)

		output, err := step.Forward.Process(currentInput)
		if err != nil {
			// Rollback phase
			rollbackErrs := sn.executeRollback(i-1, inputs)
			if len(rollbackErrs) > 0 {
				return nil, errors.Join(ErrSagaForwardFailed, err, ErrSagaRollbackFailed, errors.Join(rollbackErrs...))
			}
			return nil, errors.Join(ErrSagaForwardFailed, err)
		}
		currentInput = output
	}

	return currentInput, nil
}

// executeRollback executes the rollback nodes for the completed steps in reverse order.
func (sn *SagaNode) executeRollback(lastCompletedIndex int, inputs [][]byte) []error {
	var errs []error
	for i := lastCompletedIndex; i >= 0; i-- {
		if sn.steps[i].Rollback != nil {
			// Pass the original input that was given to the Forward node
			_, err := sn.steps[i].Rollback.Process(inputs[i])
			if err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errs
}

// Create initializes all forward and rollback nodes in the saga.
func (sn *SagaNode) Create() error {
	if err := sn.BaseNode.Create(); err != nil {
		return err
	}

	for _, step := range sn.steps {
		if step.Forward != nil {
			if err := step.Forward.Create(); err != nil {
				return err
			}
		}
		if step.Rollback != nil {
			if err := step.Rollback.Create(); err != nil {
				return err
			}
		}
	}
	return nil
}

// Delete tears down all forward and rollback nodes in the saga.
func (sn *SagaNode) Delete() error {
	var errs []error

	if err := sn.BaseNode.Delete(); err != nil {
		errs = append(errs, err)
	}

	for _, step := range sn.steps {
		if step.Forward != nil {
			if err := step.Forward.Delete(); err != nil {
				errs = append(errs, err)
			}
		}
		if step.Rollback != nil {
			if err := step.Rollback.Delete(); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
