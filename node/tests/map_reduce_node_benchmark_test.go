package node_test

import (
	"github.com/lhemerly/Constellation/node"
	"testing"
)

func BenchmarkMapReduceNodeProcess(b *testing.B) {
	mappers := make([]node.Node, 10)
	for i := 0; i < 10; i++ {
		m := node.NewBaseNode("mapper")
		m.SetProcessFunc(func(input []byte) ([]byte, error) {
			return input, nil
		})
		mappers[i] = m
	}

	reducer := node.NewBaseNode("reducer")
	reducer.SetProcessFunc(func(input []byte) ([]byte, error) {
		return input, nil
	})

	mrNode := node.NewMapReduceNode("mr-node", mappers, reducer)
	mrNode.Create()
	defer mrNode.Delete()

	input := make([]byte, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mrNode.Process(input)
		if err != nil {
			b.Fatal(err)
		}
	}
}
