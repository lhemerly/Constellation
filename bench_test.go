package main

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

var input = []byte("some data to hash")

func BenchmarkHashOld(b *testing.B) {
	for i := 0; i < b.N; i++ {
		hasher := sha256.New()
		hasher.Write(input)
		_ = hex.EncodeToString(hasher.Sum(nil))
	}
}

func BenchmarkHashNew(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = sha256.Sum256(input)
	}
}
