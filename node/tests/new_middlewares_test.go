package node_test

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/node"
)

func TestRateLimitMiddleware(t *testing.T) {
	n := node.NewBaseNode("rate-limiter")
	defer cleanupNodes(t, []*node.BaseNode{n})

	n.Use(node.RateLimitMiddleware(2, 50*time.Millisecond))

	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		return input, nil
	})

	// First two should succeed
	_, err := n.Process([]byte("test1"))
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	_, err = n.Process([]byte("test2"))
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	// Third should fail
	_, err = n.Process([]byte("test3"))
	if !errors.Is(err, node.ErrRateLimitExceeded) {
		t.Errorf("expected ErrRateLimitExceeded, got %v", err)
	}

	// Wait for window to pass
	time.Sleep(60 * time.Millisecond)

	// Should succeed again
	_, err = n.Process([]byte("test4"))
	if err != nil {
		t.Errorf("expected success after wait, got %v", err)
	}
}

func TestCompressMiddleware(t *testing.T) {
	n := node.NewBaseNode("compressor")
	defer cleanupNodes(t, []*node.BaseNode{n})

	n.Use(node.CompressMiddleware())

	original := []byte("hello world hello world hello world")
	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		return input, nil
	})

	compressed, err := n.Process(original)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	if bytes.Equal(compressed, original) {
		t.Errorf("expected output to be compressed")
	}

	// Decompress and verify
	r, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("expected valid gzip, got %v", err)
	}
	defer r.Close()

	decompressed, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("expected decompression success, got %v", err)
	}

	if !bytes.Equal(decompressed, original) {
		t.Errorf("expected %s, got %s", string(original), string(decompressed))
	}
}

func TestDecompressMiddleware(t *testing.T) {
	n := node.NewBaseNode("decompressor")
	defer cleanupNodes(t, []*node.BaseNode{n})

	n.Use(node.DecompressMiddleware())

	var processed []byte
	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		processed = input
		return input, nil
	})

	original := []byte("hello world hello world hello world")

	// Compress the input
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	_, _ = w.Write(original)
	_ = w.Close()

	_, err := n.Process(b.Bytes())
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	if !bytes.Equal(processed, original) {
		t.Errorf("expected node to process %s, got %s", string(original), string(processed))
	}
}

func TestDecompressCompressPipeline(t *testing.T) {
	n := node.NewBaseNode("pipeline")
	defer cleanupNodes(t, []*node.BaseNode{n})

	// Decompress input, run process, compress output
	// Order matters: outer middleware is applied first to input, and last to output.
	// Since we Use(a, b), b is wrapped by a.
	// We want: Decompress -> Process -> Compress
	// Actually, Compress needs to run AFTER Process finishes.
	// Decompress needs to run BEFORE Process starts.
	// DecompressMiddleware:
	//   decompressed = gzip.Unzip(input)
	//   return next(decompressed)
	// CompressMiddleware:
	//   output = next(input)
	//   return gzip.Zip(output)
	//
	// If we use Compress, Decompress:
	//   Compress runs: output = next(input); zip(output)
	//   where `next` is Decompress.
	//   Decompress runs: decompressed = unzip(input); return process(decompressed)
	// So Compress wraps Decompress. Input to Compress goes to Decompress. Wait, if input is not compressed, Decompress will fail?
	// Oh, if we send compressed data in, Decompress must wrap the Process.
	// Let's just do them individually to avoid confusing nesting.
	// Actually `n.Use(node.DecompressMiddleware(), node.CompressMiddleware())`
	// Middlewares are applied backwards: `processFunc = middlewares[i](processFunc)`
	// So Decompress wraps Compress wraps Process.
	// Input -> Decompress -> Compress -> Process
	// Wait, Compress passes input through directly: `output = next(input)`
	// Then it compresses the output.
	// Decompress passes `decompressed` to next: `return next(decompressed)`.
	// Let's test this order.

	n.Use(node.DecompressMiddleware(), node.CompressMiddleware())

	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		// Appends "-processed" to decompressed input
		return append(input, []byte("-processed")...), nil
	})

	original := []byte("hello world")
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	_, _ = w.Write(original)
	_ = w.Close()

	res, err := n.Process(b.Bytes())
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	r, err := gzip.NewReader(bytes.NewReader(res))
	if err != nil {
		t.Fatalf("expected valid gzip, got %v", err)
	}
	defer r.Close()

	decompressedRes, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("expected decompression success, got %v", err)
	}

	expected := "hello world-processed"
	if string(decompressedRes) != expected {
		t.Errorf("expected %q, got %q", expected, string(decompressedRes))
	}
}
