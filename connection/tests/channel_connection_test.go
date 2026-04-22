package connection_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lhemerly/Constellation/connection"
)

func TestChannelConnection(t *testing.T) {
	ctx := context.Background()
	conn, err := connection.NewChannelConnection(ctx, "local-address")
	if err != nil {
		t.Fatalf("Failed to create ChannelConnection: %v", err)
	}

	t.Run("Connect", func(t *testing.T) {
		err := (*conn).Connect(ctx)
		if err != nil {
			t.Errorf("Connect failed: %v", err)
		}
	})

	t.Run("IsConnected", func(t *testing.T) {
		if !(*conn).IsConnected() {
			t.Error("IsConnected returned false, expected true")
		}
	})

	t.Run("GetRemoteAddress", func(t *testing.T) {
		addr := (*conn).GetRemoteAddress()
		if addr != "local-address" {
			t.Errorf("GetRemoteAddress returned %s, expected local-address", addr)
		}
	})

	t.Run("Send and Receive", func(t *testing.T) {
		testData := []byte("test data")
		err := (*conn).Send(ctx, testData)
		if err != nil {
			t.Errorf("Send failed: %v", err)
		}

		receivedData, err := (*conn).Receive(ctx)
		if err != nil {
			t.Errorf("Receive failed: %v", err)
		}
		if string(receivedData) != string(testData) {
			t.Errorf("Received data %s, expected %s", string(receivedData), string(testData))
		}
	})

	t.Run("Disconnect", func(t *testing.T) {
		err := (*conn).Disconnect()
		if err != nil {
			t.Errorf("Disconnect failed: %v", err)
		}
		if (*conn).IsConnected() {
			t.Error("IsConnected returned true after Disconnect, expected false")
		}
	})

	t.Run("Sentinel Errors", func(t *testing.T) {
		// Test ErrNotConnected on Send
		err := (*conn).Send(ctx, []byte("data"))
		if !errors.Is(err, connection.ErrNotConnected) {
			t.Errorf("Expected ErrNotConnected, got %v", err)
		}

		// Test ErrNotConnected on Receive
		_, err = (*conn).Receive(ctx)
		if !errors.Is(err, connection.ErrNotConnected) {
			t.Errorf("Expected ErrNotConnected, got %v", err)
		}

		// Test ErrNotConnected on Disconnect
		err = (*conn).Disconnect()
		if !errors.Is(err, connection.ErrNotConnected) {
			t.Errorf("Expected ErrNotConnected, got %v", err)
		}

		// Reconnect for ErrAlreadyConnected test
		err = (*conn).Connect(ctx)
		if err != nil {
			t.Fatalf("Connect failed: %v", err)
		}
		t.Cleanup(func() {
			(*conn).Disconnect()
		})

		// Test ErrAlreadyConnected on Connect
		err = (*conn).Connect(ctx)
		if !errors.Is(err, connection.ErrAlreadyConnected) {
			t.Errorf("Expected ErrAlreadyConnected, got %v", err)
		}
	})

	t.Run("Concurrent Operations and Reconnect", func(t *testing.T) {
		err := (*conn).Disconnect()
		if err != nil && !errors.Is(err, connection.ErrNotConnected) {
			t.Errorf("Disconnect failed: %v", err)
		}

		err = (*conn).Connect(ctx)
		if err != nil {
			t.Fatalf("Connect failed: %v", err)
		}

		err = (*conn).Send(ctx, []byte("test"))
		if err != nil {
			t.Errorf("Send failed: %v", err)
		}

		err = (*conn).Disconnect()
		if err != nil {
			t.Errorf("Disconnect failed: %v", err)
		}

		err = (*conn).Connect(ctx)
		if err != nil {
			t.Fatalf("Connect failed: %v", err)
		}

		err = (*conn).Send(ctx, []byte("test2"))
		if err != nil {
			t.Errorf("Send failed: %v", err)
		}

		err = (*conn).Disconnect()
		if err != nil {
			t.Errorf("Disconnect failed: %v", err)
		}
	})
}
