package node_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
	"github.com/stretchr/testify/assert"
)

func TestFanOutNode_Success(t *testing.T) {
	n1 := node.NewBaseNode("dest-1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("res1"), nil
	})

	n2 := node.NewBaseNode("dest-2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("res2"), nil
	})

	destinations := []node.Node{n1, n2}
	fanout := node.NewFanOutNode("fanout-1", destinations, 0)

	res, err := fanout.Process([]byte("input"))
	assert.NoError(t, err)

	resStr := string(res)
	assert.True(t, strings.Contains(resStr, "res1"))
	assert.True(t, strings.Contains(resStr, "res2"))
	assert.Equal(t, 8, len(resStr))
}

func TestFanOutNode_Timeout(t *testing.T) {
	n1 := node.NewBaseNode("dest-1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("res1"), nil
	})

	n2 := node.NewBaseNode("dest-2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		time.Sleep(200 * time.Millisecond) // Slow node
		return []byte("res2"), nil
	})

	destinations := []node.Node{n1, n2}
	fanout := node.NewFanOutNode("fanout-2", destinations, 100*time.Millisecond)

	res, err := fanout.Process([]byte("input"))
	assert.Error(t, err)
	assert.Nil(t, res)
	assert.True(t, strings.Contains(err.Error(), "process timed out"))
}

func TestFanOutNode_NoDestinations(t *testing.T) {
	fanout := node.NewFanOutNode("fanout-3", []node.Node{}, 0)

	res, err := fanout.Process([]byte("input"))
	assert.Error(t, err)
	assert.Nil(t, res)
	assert.Equal(t, "no destination nodes", err.Error())
}

func TestFanOutNode_PartialError(t *testing.T) {
	n1 := node.NewBaseNode("dest-1")
	n1.SetProcessFunc(func(input []byte) ([]byte, error) {
		return []byte("res1"), nil
	})

	n2 := node.NewBaseNode("dest-2")
	n2.SetProcessFunc(func(input []byte) ([]byte, error) {
		return nil, errors.New("node 2 failed")
	})

	destinations := []node.Node{n1, n2}
	fanout := node.NewFanOutNode("fanout-4", destinations, 0)

	res, err := fanout.Process([]byte("input"))
	assert.Error(t, err)
	assert.Nil(t, res)
	assert.True(t, strings.Contains(err.Error(), "node 2 failed"))
}
