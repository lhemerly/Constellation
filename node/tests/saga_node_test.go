package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"testing"
)

func TestSagaNode_Success(t *testing.T) {
	n1 := node.NewBaseNode("f1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-n1")...), nil
	})

	n2 := node.NewBaseNode("f2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-n2")...), nil
	})

	saga := node.NewSagaNode("saga1", []node.SagaStep{
		{Forward: n1, Rollback: nil},
		{Forward: n2, Rollback: nil},
	})
	if err := saga.Create(); err != nil {
		t.Fatalf("Failed to create SagaNode: %v", err)
	}
	defer cleanupNodes(t, []node.Node{saga})

	res, err := saga.Process([]byte("init"))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if string(res) != "init-n1-n2" {
		t.Errorf("Expected 'init-n1-n2', got '%s'", string(res))
	}
}

func TestSagaNode_Rollback(t *testing.T) {
	var rollbackOrder []string

	f1 := node.NewBaseNode("f1")
	f1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-f1")...), nil
	})
	r1 := node.NewBaseNode("r1")
	r1.SetProcessFunc(func(input []byte) ([]byte, error) {
		rollbackOrder = append(rollbackOrder, "r1-"+string(input))
		return nil, nil
	})

	f2 := node.NewBaseNode("f2")
	f2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-f2")...), nil
	})
	r2 := node.NewBaseNode("r2")
	r2.SetProcessFunc(func(input []byte) ([]byte, error) {
		rollbackOrder = append(rollbackOrder, "r2-"+string(input))
		return nil, nil
	})

	f3 := node.NewBaseNode("f3")
	f3.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("f3 failed")
	})
	r3 := node.NewBaseNode("r3")
	r3.SetProcessFunc(func(input []byte) ([]byte, error) {
		rollbackOrder = append(rollbackOrder, "r3-"+string(input))
		return nil, nil
	})

	saga := node.NewSagaNode("saga2", []node.SagaStep{
		{Forward: f1, Rollback: r1},
		{Forward: f2, Rollback: r2},
		{Forward: f3, Rollback: r3},
	})
	if err := saga.Create(); err != nil {
		t.Fatalf("Failed to create SagaNode: %v", err)
	}
	defer cleanupNodes(t, []node.Node{saga})

	_, err := saga.Process([]byte("start"))
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}

	if !errors.Is(err, node.ErrSagaForwardFailed) {
		t.Errorf("Expected ErrSagaForwardFailed in error chain")
	}

	if len(rollbackOrder) != 2 {
		t.Fatalf("Expected 2 rollbacks, got %d", len(rollbackOrder))
	}
	if rollbackOrder[0] != "r2-start-f1" {
		t.Errorf("Expected first rollback to be r2 with input 'start-f1', got %s", rollbackOrder[0])
	}
	if rollbackOrder[1] != "r1-start" {
		t.Errorf("Expected second rollback to be r1 with input 'start', got %s", rollbackOrder[1])
	}
}
