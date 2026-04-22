package node_test

import (
	"bytes"
	"testing"

	"github.com/lhemerly/Constellation/node"
)

func TestBaseNodeMiddleware(t *testing.T) {
	n := node.NewBaseNode("node-middleware-test")
	if err := n.Create(); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer n.Delete()

	// Add a middleware to prepend "MW1:" to the input
	mw1 := func(next func([]byte) ([]byte, error)) func([]byte) ([]byte, error) {
		return func(input []byte) ([]byte, error) {
			newInput := append([]byte("MW1:"), input...)
			return next(newInput)
		}
	}

	// Add another middleware to append ":MW2" to the output
	mw2 := func(next func([]byte) ([]byte, error)) func([]byte) ([]byte, error) {
		return func(input []byte) ([]byte, error) {
			out, err := next(input)
			if err != nil {
				return nil, err
			}
			return append(out, []byte(":MW2")...), nil
		}
	}

	n.Use(mw1, mw2)

	// Custom process func
	n.SetProcessFunc(func(input []byte) ([]byte, error) {
		return append(input, []byte("-processed")...), nil
	})

	input := []byte("data")
	output, err := n.Process(input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	// Expected: mw1 modifies input -> "MW1:data"
	// Process func appends -> "MW1:data-processed"
	// mw2 appends to output -> "MW1:data-processed:MW2"
	expected := []byte("MW1:data-processed:MW2")

	if !bytes.Equal(output, expected) {
		t.Errorf("Process() output = %v, want %v", string(output), string(expected))
	}

	// Test adding another middleware after process func is already set
	mw3 := func(next func([]byte) ([]byte, error)) func([]byte) ([]byte, error) {
		return func(input []byte) ([]byte, error) {
			return next(append([]byte("MW3:"), input...))
		}
	}
	n.Use(mw3)

	output, err = n.Process(input)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	// Order of middlewares: mw1(mw2(mw3(processFunc)))
	// MW1 prepends "MW1:" -> "MW1:data"
	// MW3 prepends "MW3:" -> "MW3:MW1:data"
	// Process func appends "-processed" -> "MW3:MW1:data-processed"
	// MW2 appends to output ":MW2" -> "MW3:MW1:data-processed:MW2"

	// Wait, the order of use is Use(mw1, mw2), Use(mw3)
	// So middlewares = [mw1, mw2, mw3]
	// Rebuild:
	// fn = mw3(base)
	// fn = mw2(fn)
	// fn = mw1(fn)
	// Output: fn(input)
	// mw1 runs: prepend "MW1:" to input, calls next(mw2)
	// mw2 runs: calls next(mw3), appends ":MW2" to output
	// mw3 runs: prepends "MW3:" to input, calls next(base)
	// base runs: appends "-processed" to input

	// Let's trace input "data"
	// mw1: newInput = "MW1:data", calls mw2("MW1:data")
	// mw2: calls mw3("MW1:data")
	// mw3: newInput = "MW3:MW1:data", calls base("MW3:MW1:data")
	// base: returns "MW3:MW1:data-processed"
	// mw2: appends ":MW2", returns "MW3:MW1:data-processed:MW2"
	// mw1: returns "MW3:MW1:data-processed:MW2"

	expected2 := []byte("MW3:MW1:data-processed:MW2")
	if !bytes.Equal(output, expected2) {
		t.Errorf("Process() output = %v, want %v", string(output), string(expected2))
	}
}
