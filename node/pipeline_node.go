package node

import (
	"errors"
	"sync"
)

// ErrEmptyPipeline is returned when the PipelineNode has no stages.
var ErrEmptyPipeline = errors.New("empty pipeline")

// PipelineNode extends BaseNode to process data sequentially through multiple chained nodes.
type PipelineNode struct {
	*BaseNode
	stagesMutex sync.RWMutex
	stages      []Node
}

// NewPipelineNode creates a new PipelineNode with the given ID.
func NewPipelineNode(id string) *PipelineNode {
	p := &PipelineNode{
		BaseNode: NewBaseNode(id),
		stages:   make([]Node, 0),
	}

	p.SetProcessFunc(p.processPipeline)
	return p
}

// AddStage appends a node to the pipeline sequence.
func (p *PipelineNode) AddStage(node Node) {
	p.stagesMutex.Lock()
	defer p.stagesMutex.Unlock()
	p.stages = append(p.stages, node)
}

// GetStages returns the nodes composing the pipeline.
func (p *PipelineNode) GetStages() []Node {
	p.stagesMutex.RLock()
	defer p.stagesMutex.RUnlock()

	stagesCopy := make([]Node, len(p.stages))
	copy(stagesCopy, p.stages)
	return stagesCopy
}

// processPipeline sequentially passes the input through all registered stages.
// The output of one stage becomes the input of the next.
func (p *PipelineNode) processPipeline(input []byte) ([]byte, error) {
	p.stagesMutex.RLock()

	if len(p.stages) == 0 {
		p.stagesMutex.RUnlock()
		return nil, ErrEmptyPipeline
	}

	stagesCopy := make([]Node, len(p.stages))
	copy(stagesCopy, p.stages)
	p.stagesMutex.RUnlock()

	currentData := input
	var err error

	for _, stage := range stagesCopy {
		currentData, err = stage.Process(currentData)
		if err != nil {
			return nil, err
		}
	}

	return currentData, nil
}
