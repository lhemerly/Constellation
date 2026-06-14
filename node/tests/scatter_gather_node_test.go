package node_test

import (
	"errors"
	"github.com/lhemerly/Constellation/node"
	"testing"
	"time"
)

func TestScatterGatherNode_Success(t *testing.T) {
	n1 := node.NewBaseNode("t1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("A"), nil
	})

	n2 := node.NewBaseNode("t2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("B"), nil
	})

	sg := node.NewScatterGatherNode("sg", []node.Node{n1, n2}, 1*time.Second, 2)
	defer cleanupNodes(t, []node.Node{sg}) // SG Delete cleans targets

	if err := sg.Create(); err != nil {
		t.Fatalf("failed to create: %v", err)
	}

	res, err := sg.Process([]byte("test"))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	out := string(res)
	if out != "AB" && out != "BA" {
		t.Errorf("expected AB or BA, got %s", out)
	}
}

func TestScatterGatherNode_QuorumReachedEarly(t *testing.T) {
	n1 := node.NewBaseNode("t1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("A"), nil
	})

	n2 := node.NewBaseNode("t2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(500 * time.Millisecond) // slow
		return []byte("B"), nil
	})

	// Quorum is 1, so it shouldn't wait for n2
	sg := node.NewScatterGatherNode("sg-quorum", []node.Node{n1, n2}, 1*time.Second, 1)
	defer cleanupNodes(t, []node.Node{sg})

	if err := sg.Create(); err != nil {
		t.Fatalf("failed to create: %v", err)
	}

	start := time.Now()
	res, err := sg.Process([]byte("test"))
	dur := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if string(res) != "A" {
		t.Errorf("expected A, got %s", string(res))
	}
	if dur > 100*time.Millisecond {
		t.Errorf("expected fast return, took %v", dur)
	}
}

func TestScatterGatherNode_Timeout(t *testing.T) {
	n1 := node.NewBaseNode("t1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(200 * time.Millisecond)
		return []byte("A"), nil
	})

	sg := node.NewScatterGatherNode("sg-timeout", []node.Node{n1}, 50*time.Millisecond, 1)
	defer cleanupNodes(t, []node.Node{sg})

	if err := sg.Create(); err != nil {
		t.Fatalf("failed to create: %v", err)
	}

	_, err := sg.Process([]byte("test"))
	if !errors.Is(err, node.ErrScatterGatherTimeout) {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func TestScatterGatherNode_PartialSuccess(t *testing.T) {
	n1 := node.NewBaseNode("t1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("A"), nil
	})

	n2 := node.NewBaseNode("t2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("failed")
	})

	// Timeout should not be reached since both complete, but we only need quorum 1 or more.
	// Since 1 succeeds, it should return that output and not fail, even if n2 failed.
	// If quorum is 2, it won't be met, but all finish, so it will return what it got if successfulResponses > 0
	sg := node.NewScatterGatherNode("sg-partial", []node.Node{n1, n2}, 1*time.Second, 2)
	defer cleanupNodes(t, []node.Node{sg})

	if err := sg.Create(); err != nil {
		t.Fatalf("failed to create: %v", err)
	}

	res, err := sg.Process([]byte("test"))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if string(res) != "A" {
		t.Errorf("expected A, got %s", string(res))
	}
}

func TestScatterGatherNode_TotalFailure(t *testing.T) {
	n1 := node.NewBaseNode("t1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("err1")
	})

	n2 := node.NewBaseNode("t2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("err2")
	})

	sg := node.NewScatterGatherNode("sg-fail", []node.Node{n1, n2}, 1*time.Second, 2)
	defer cleanupNodes(t, []node.Node{sg})

	if err := sg.Create(); err != nil {
		t.Fatalf("failed to create: %v", err)
	}

	_, err := sg.Process([]byte("test"))
	if !errors.Is(err, node.ErrScatterGatherFailed) {
		t.Fatalf("expected ErrScatterGatherFailed, got %v", err)
	}
}
