package node_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestScatterGatherNode_Success(t *testing.T) {
	w1 := node.NewBaseNode("w1")
	w1.SetProcessFunc(func(in []byte) ([]byte, error) { return []byte("w1:" + string(in)), nil })

	w2 := node.NewBaseNode("w2")
	w2.SetProcessFunc(func(in []byte) ([]byte, error) { return []byte(",w2:" + string(in)), nil })

	sg := node.NewScatterGatherNode("sg", []node.Node{w1, w2}, 5*time.Second, false)
	cleanupNodes(t, []node.Node{sg})
	sg.Create()

	res, err := sg.Process([]byte("data"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	resStr := string(res)
	if resStr != "w1:data,w2:data" {
		t.Errorf("expected w1:data,w2:data, got %v", resStr)
	}
}

func TestScatterGatherNode_Failure(t *testing.T) {
	w1 := node.NewBaseNode("w1")
	w1.SetProcessFunc(func(in []byte) ([]byte, error) { return []byte("w1:" + string(in)), nil })

	w2 := node.NewBaseNode("w2")
	w2.SetProcessFunc(func(in []byte) ([]byte, error) { return nil, errors.New("worker 2 error") })

	sg := node.NewScatterGatherNode("sg", []node.Node{w1, w2}, 5*time.Second, false)
	cleanupNodes(t, []node.Node{sg})
	sg.Create()

	res, err := sg.Process([]byte("data"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if res != nil {
		t.Errorf("expected res to be nil, got %v", res)
	}

	if !strings.Contains(err.Error(), "worker 2 error") {
		t.Errorf("expected error to contain worker 2 error, got %v", err)
	}
}

func TestScatterGatherNode_FailureIgnored(t *testing.T) {
	w1 := node.NewBaseNode("w1")
	w1.SetProcessFunc(func(in []byte) ([]byte, error) { return []byte("w1:" + string(in)), nil })

	w2 := node.NewBaseNode("w2")
	w2.SetProcessFunc(func(in []byte) ([]byte, error) { return nil, errors.New("worker 2 error") })

	sg := node.NewScatterGatherNode("sg", []node.Node{w1, w2}, 5*time.Second, true)
	cleanupNodes(t, []node.Node{sg})
	sg.Create()

	res, err := sg.Process([]byte("data"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	resStr := string(res)
	if resStr != "w1:data" {
		t.Errorf("expected w1:data, got %v", resStr)
	}
}

func TestScatterGatherNode_Timeout(t *testing.T) {
	w1 := node.NewBaseNode("w1")
	w1.SetProcessFunc(func(in []byte) ([]byte, error) { return []byte("w1:" + string(in)), nil })

	w2 := node.NewBaseNode("w2")
	w2.SetProcessFunc(func(in []byte) ([]byte, error) {
		time.Sleep(100 * time.Millisecond)
		return []byte(",w2:" + string(in)), nil
	})

	sg := node.NewScatterGatherNode("sg", []node.Node{w1, w2}, 10*time.Millisecond, false)
	cleanupNodes(t, []node.Node{sg})
	sg.Create()

	_, err := sg.Process([]byte("data"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !strings.Contains(err.Error(), node.ErrScatterGatherTimeout.Error()) {
		t.Errorf("expected timeout error, got %v", err)
	}
}

func TestScatterGatherNode_PanicsOnEmptyWorkers(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on empty workers")
		}
	}()

	node.NewScatterGatherNode("sg", []node.Node{}, 5*time.Second, false)
}
