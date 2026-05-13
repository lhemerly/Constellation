package node_test

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

// oldCacheMiddleware is the old implementation to compare against
func oldCacheMiddleware(ttl time.Duration) node.Middleware {
	type cacheEntry struct {
		output    []byte
		expiresAt time.Time
	}

	var (
		mu    sync.RWMutex
		cache = make(map[string]cacheEntry)
	)

	return func(next func([]byte) ([]byte, error)) func([]byte) ([]byte, error) {
		return func(input []byte) ([]byte, error) {
			hasher := sha256.New()
			hasher.Write(input)
			key := hex.EncodeToString(hasher.Sum(nil))

			mu.RLock()
			entry, exists := cache[key]
			mu.RUnlock()

			if exists && time.Now().Before(entry.expiresAt) {
				outputCopy := make([]byte, len(entry.output))
				copy(outputCopy, entry.output)
				return outputCopy, nil
			}

			output, err := next(input)
			if err == nil {
				outputCopy := make([]byte, len(output))
				copy(outputCopy, output)

				mu.Lock()
				cache[key] = cacheEntry{
					output:    outputCopy,
					expiresAt: time.Now().Add(ttl),
				}
				mu.Unlock()
			}

			return output, err
		}
	}
}

func BenchmarkCacheMiddleware(b *testing.B) {
	input := []byte("this is a test payload for the cache middleware benchmark")
	ttl := 1 * time.Minute

	b.Run("Old_HexKey", func(b *testing.B) {
		middleware := oldCacheMiddleware(ttl)
		processFunc := middleware(func(in []byte) ([]byte, error) {
			return in, nil
		})

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, _ = processFunc(input)
		}
	})

	b.Run("New_ArrayKey", func(b *testing.B) {
		middleware := node.CacheMiddleware(ttl)
		processFunc := middleware(func(in []byte) ([]byte, error) {
			return in, nil
		})

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, _ = processFunc(input)
		}
	})
}
