package connection_test

import (
	"context"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/connection"
)

func TestChannelConnection(t *testing.T) {
	ctx := context.Background()
	factory := connection.NewConnectionFactory()

	// Test creation
	conn, err := factory.NewConnection(ctx, "local", "local-addr")
	if err != nil {
		t.Fatalf("Failed to create local connection: %v", err)
	}

	if (*conn).GetRemoteAddress() != "local-addr" {
		t.Errorf("Expected address 'local-addr', got '%s'", (*conn).GetRemoteAddress())
	}

	// Test connection state
	if (*conn).IsConnected() {
		t.Error("Connection should not be connected initially")
	}

	// Test Send and Receive before connect
	if err := (*conn).Send(ctx, []byte("data")); err != connection.ErrNotConnected {
		t.Errorf("Expected ErrNotConnected, got %v", err)
	}
	if _, err := (*conn).Receive(ctx); err != connection.ErrNotConnected {
		t.Errorf("Expected ErrNotConnected, got %v", err)
	}

	// Connect
	if err := (*conn).Connect(ctx); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	if !(*conn).IsConnected() {
		t.Error("Connection should be connected")
	}

	// Double connect
	if err := (*conn).Connect(ctx); err != connection.ErrAlreadyConnected {
		t.Errorf("Expected ErrAlreadyConnected, got %v", err)
	}

	// Send and Receive
	testData := []byte("hello channel")
	if err := (*conn).Send(ctx, testData); err != nil {
		t.Fatalf("Failed to send: %v", err)
	}

	// Receive with timeout
	ctxTimeout, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	received, err := (*conn).Receive(ctxTimeout)
	if err != nil {
		t.Fatalf("Failed to receive: %v", err)
	}

	if string(received) != string(testData) {
		t.Errorf("Expected %s, got %s", string(testData), string(received))
	}

	// Disconnect
	if err := (*conn).Disconnect(); err != nil {
		t.Fatalf("Failed to disconnect: %v", err)
	}

	if (*conn).IsConnected() {
		t.Error("Connection should not be connected after disconnect")
	}

	// Double disconnect
	if err := (*conn).Disconnect(); err != connection.ErrNotConnected {
		t.Errorf("Expected ErrNotConnected, got %v", err)
	}
}
