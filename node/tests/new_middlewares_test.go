package node_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestRateLimitMiddleware(t *testing.T) {
	n := node.NewBaseNode("n1")
	// Limit to 2 requests per 50ms window
	n.Use(node.RateLimitMiddleware(2, 50*time.Millisecond))

	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		return input, nil
	})

	// Request 1: allowed
	_, err := n.Process([]byte("test"))
	if err != nil {
		t.Fatalf("Request 1 failed: %v", err)
	}

	// Request 2: allowed
	_, err = n.Process([]byte("test"))
	if err != nil {
		t.Fatalf("Request 2 failed: %v", err)
	}

	// Request 3: should fail
	_, err = n.Process([]byte("test"))
	if err == nil || err != node.ErrRateLimitExceeded {
		t.Errorf("Expected ErrRateLimitExceeded, got %v", err)
	}

	// Wait for window to pass
	time.Sleep(60 * time.Millisecond)

	// Request 4: should be allowed again
	_, err = n.Process([]byte("test"))
	if err != nil {
		t.Fatalf("Request 4 failed after window reset: %v", err)
	}
}

func TestGzipMiddleware(t *testing.T) {
	n := node.NewBaseNode("n1")
	n.Use(node.GzipMiddleware())

	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		return input, nil
	})

	originalPayload := []byte("hello gzip middleware")
	compressedPayload, err := n.Process(originalPayload)
	if err != nil {
		t.Fatalf("Failed to process: %v", err)
	}

	// Verify the payload is actually gzipped
	reader, err := gzip.NewReader(bytes.NewReader(compressedPayload))
	if err != nil {
		t.Fatalf("Failed to create gzip reader: %v", err)
	}
	defer reader.Close()

	decompressedPayload, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("Failed to read decompressed payload: %v", err)
	}

	if !bytes.Equal(decompressedPayload, originalPayload) {
		t.Errorf("Expected decompressed payload %q, got %q", string(originalPayload), string(decompressedPayload))
	}
}
