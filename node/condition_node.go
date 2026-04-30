package node

// ConditionNode acts as a branching mechanism. It takes a ConditionFunc,
// a TrueNode, and a FalseNode. It routes the input synchronously to the
// respective node based on the condition's result.
type ConditionNode struct {
	BaseNode
	conditionFunc func([]byte) bool
	trueNode      Node
	falseNode     Node
}

// NewConditionNode creates a new ConditionNode with the specified ID, condition function,
// and branch nodes.
func NewConditionNode(id string, conditionFunc func([]byte) bool, trueNode Node, falseNode Node) *ConditionNode {
	if conditionFunc == nil {
		panic("conditionFunc cannot be nil")
	}
	if trueNode == nil {
		panic("trueNode cannot be nil")
	}
	if falseNode == nil {
		panic("falseNode cannot be nil")
	}

	n := &ConditionNode{
		BaseNode:      *NewBaseNode(id),
		conditionFunc: conditionFunc,
		trueNode:      trueNode,
		falseNode:     falseNode,
	}

	// We override Process later
	return n
}

// Create initializes the ConditionNode.
func (n *ConditionNode) Create() error {
	if err := n.BaseNode.Create(); err != nil {
		return err
	}
	return nil
}

// Process evaluates the condition against the input, and then synchronously
// routes it to either TrueNode or FalseNode, returning its output.
func (n *ConditionNode) Process(input []byte) ([]byte, error) {
	if n.conditionFunc(input) {
		return n.trueNode.Process(input)
	}
	return n.falseNode.Process(input)
}
