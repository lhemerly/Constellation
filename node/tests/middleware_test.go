package node_test

import (
	"strings"
	"testing"

	"github.com/lhemerly/Constellation/node"
)

func TestMiddleware(t *testing.T) {
	n := node.NewBaseNode("node-with-middleware")
	cleanupNodes(t, []node.Node{n})

	// Middleware 1: Convert input to uppercase
	uppercaseMiddleware := func(next func([]byte) ([]byte, error)) func([]byte) ([]byte, error) {
		return func(input []byte) ([]byte, error) {
			upperInput := []byte(strings.ToUpper(string(input)))
			return next(upperInput)
		}
	}

	// Middleware 2: Append a suffix to output
	suffixMiddleware := func(next func([]byte) ([]byte, error)) func([]byte) ([]byte, error) {
		return func(input []byte) ([]byte, error) {
			output, err := next(input)
			if err != nil {
				return nil, err
			}
			return append(output, []byte("-suffix")...), nil
		}
	}

	n.Use(uppercaseMiddleware, suffixMiddleware)

	// Set custom process func
	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append([]byte("processed: "), input...), nil
	})

	input := []byte("hello middleware")
	output, err := n.Process(input)
	if err != nil {
		t.Fatalf("Failed to process: %v", err)
	}

	expectedOutput := "processed: HELLO MIDDLEWARE-suffix"
	if string(output) != expectedOutput {
		t.Errorf("Expected output '%s', got '%s'", expectedOutput, string(output))
	}
}
