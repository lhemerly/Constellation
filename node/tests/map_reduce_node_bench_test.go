package node_test

import (
	"testing"
	"github.com/lhemerly/Constellation/node"
)

func BenchmarkMapReduceNodeProcess(b *testing.B) {
	mappers := make([]node.Node, 10)
	for i := 0; i < 10; i++ {
		m := node.NewBaseNode("mapper")
		m.SetProcessFunc(func(in []byte) ([]byte, error) {
			return in, nil
		})
		mappers[i] = m
	}

	reducer := node.NewBaseNode("reducer")
	reducer.SetProcessFunc(func(in []byte) ([]byte, error) {
		return in, nil
	})

	mrNode := node.NewMapReduceNode("mr", mappers, reducer)

	input := []byte("test data")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mrNode.Process(input)
		if err != nil {
			b.Fatal(err)
		}
	}
}
