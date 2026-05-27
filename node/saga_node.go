package node

import (
	"errors"
	"sync"
)

// SagaStep defines a single step in a saga transaction.
type SagaStep struct {
	Action       Node
	Compensation Node
}

// SagaNode manages a sequence of steps. If a step fails, it executes the
// compensation actions for all previously successful steps in reverse order.
type SagaNode struct {
	*BaseNode
	stepsMutex sync.RWMutex
	steps      []SagaStep
}

// NewSagaNode creates a new SagaNode with the given ID.
func NewSagaNode(id string) *SagaNode {
	s := &SagaNode{
		BaseNode: NewBaseNode(id),
		steps:    make([]SagaStep, 0),
	}
	s.SetProcessFunc(s.processSaga)
	return s
}

// AddStep appends a new step to the saga.
func (s *SagaNode) AddStep(action, compensation Node) {
	s.stepsMutex.Lock()
	defer s.stepsMutex.Unlock()
	s.steps = append(s.steps, SagaStep{Action: action, Compensation: compensation})
}

func (s *SagaNode) processSaga(input []byte) ([]byte, error) {
	s.stepsMutex.RLock()
	if len(s.steps) == 0 {
		s.stepsMutex.RUnlock()
		return input, nil
	}
	stepsCopy := make([]SagaStep, len(s.steps))
	copy(stepsCopy, s.steps)
	s.stepsMutex.RUnlock()

	currentData := input
	var err error

	// Keep track of inputs passed to each action so we can use them for compensation
	inputs := make([][]byte, len(stepsCopy))

	for i, step := range stepsCopy {
		inputs[i] = currentData
		currentData, err = step.Action.Process(currentData)
		if err != nil {
			// Step failed, trigger compensations for previous steps in reverse order
			var compErrs []error
			compErrs = append(compErrs, err)

			for j := i - 1; j >= 0; j-- {
				if stepsCopy[j].Compensation != nil {
					// Use the input that was originally passed to the action
					_, compErr := stepsCopy[j].Compensation.Process(inputs[j])
					if compErr != nil {
						compErrs = append(compErrs, compErr)
					}
				}
			}
			return nil, errors.Join(append([]error{errors.New("saga failed, compensations executed")}, compErrs...)...)
		}
	}

	return currentData, nil
}

// Create initializes the saga node and its dependencies.
func (s *SagaNode) Create() error {
	if err := s.BaseNode.Create(); err != nil {
		return err
	}
	s.stepsMutex.RLock()
	defer s.stepsMutex.RUnlock()

	for _, step := range s.steps {
		if step.Action != nil {
			if err := step.Action.Create(); err != nil {
				return err
			}
		}
		if step.Compensation != nil {
			if err := step.Compensation.Create(); err != nil {
				return err
			}
		}
	}
	return nil
}

// Delete cleans up the saga node and its dependencies.
func (s *SagaNode) Delete() error {
	var errs []error
	if err := s.BaseNode.Delete(); err != nil {
		errs = append(errs, err)
	}

	s.stepsMutex.RLock()
	defer s.stepsMutex.RUnlock()

	for _, step := range s.steps {
		if step.Action != nil {
			if err := step.Action.Delete(); err != nil {
				errs = append(errs, err)
			}
		}
		if step.Compensation != nil {
			if err := step.Compensation.Delete(); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
