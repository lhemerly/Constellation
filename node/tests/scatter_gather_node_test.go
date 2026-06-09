package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"testing"
	"time"
)

func TestScatterGatherNode_Success(t *testing.T) {
	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("A"), nil
	})

	n2 := node.NewBaseNode("n2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("B"), nil
	})

	sg := node.NewScatterGatherNode("sg", 1*time.Second)
	sg.AddDestination(n1)
	sg.AddDestination(n2)

	defer cleanupNodes(t, []node.Node{n1, n2, sg})

	out, err := sg.Process([]byte("test"))
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	strOut := string(out)
	if strOut != "AB" && strOut != "BA" {
		t.Fatalf("expected output to be AB or BA, got: %s", strOut)
	}
}

func TestScatterGatherNode_Timeout(t *testing.T) {
	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(200 * time.Millisecond)
		return []byte("A"), nil
	})

	sg := node.NewScatterGatherNode("sg", 50*time.Millisecond)
	sg.AddDestination(n1)

	defer cleanupNodes(t, []node.Node{n1, sg})

	_, err := sg.Process([]byte("test"))
	if err != node.ErrScatterGatherTimeout {
		t.Fatalf("expected timeout error, got: %v", err)
	}
}

func TestScatterGatherNode_AllFailed(t *testing.T) {
	n1 := node.NewBaseNode("n1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("fail 1")
	})

	n2 := node.NewBaseNode("n2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("fail 2")
	})

	sg := node.NewScatterGatherNode("sg", 1*time.Second)
	sg.AddDestination(n1)
	sg.AddDestination(n2)

	defer cleanupNodes(t, []node.Node{n1, n2, sg})

	_, err := sg.Process([]byte("test"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !errors.Is(err, node.ErrScatterGatherFailed) {
		t.Fatalf("expected ErrScatterGatherFailed in error chain, got: %v", err)
	}
}

func TestScatterGatherNode_NoDestinations(t *testing.T) {
	sg := node.NewScatterGatherNode("sg", 1*time.Second)
	defer cleanupNodes(t, []node.Node{sg})

	_, err := sg.Process([]byte("test"))
	if err == nil || err.Error() != "no destinations configured" {
		t.Fatalf("expected 'no destinations configured' error, got: %v", err)
	}
}
