package node_test

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func BenchmarkHashOld(b *testing.B) {
	input := []byte("hello world this is a test input for cache key generation")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hasher := sha256.New()
		hasher.Write(input)
		_ = hex.EncodeToString(hasher.Sum(nil))
	}
}

func BenchmarkHashNew(b *testing.B) {
	input := []byte("hello world this is a test input for cache key generation")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sha256.Sum256(input)
	}
}
