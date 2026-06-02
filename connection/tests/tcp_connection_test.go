package connection_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/connection"
)

func TestTCPConnection_ConnectAndDisconnect(t *testing.T) {
	// Start a dummy TCP server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Close()

	address := listener.Addr().String()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	factory := connection.NewConnectionFactory()
	ctx := context.Background()

	connPtr, err := factory.NewConnection(ctx, "tcp", address)
	if err != nil {
		t.Fatalf("failed to create connection: %v", err)
	}
	conn := *connPtr

	if conn.IsConnected() {
		t.Errorf("expected not connected initially")
	}

	err = conn.Connect(ctx)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	if !conn.IsConnected() {
		t.Errorf("expected to be connected")
	}

	// Connect again should return ErrAlreadyConnected
	err = conn.Connect(ctx)
	if !errors.Is(err, connection.ErrAlreadyConnected) {
		t.Errorf("expected ErrAlreadyConnected, got %v", err)
	}

	err = conn.Disconnect()
	if err != nil {
		t.Fatalf("failed to disconnect: %v", err)
	}

	if conn.IsConnected() {
		t.Errorf("expected not connected after disconnect")
	}

	// Disconnect again should return ErrNotConnected
	err = conn.Disconnect()
	if !errors.Is(err, connection.ErrNotConnected) {
		t.Errorf("expected ErrNotConnected, got %v", err)
	}
}

func TestTCPConnection_SendAndReceive(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Close()

	address := listener.Addr().String()
	testData := []byte("hello tcp")

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		buf := make([]byte, 1024)
		n, _ := conn.Read(buf)
		if string(buf[:n]) == string(testData) {
			conn.Write([]byte("world tcp"))
		}
	}()

	factory := connection.NewConnectionFactory()
	ctx := context.Background()

	connPtr, err := factory.NewConnection(ctx, "tcp", address)
	if err != nil {
		t.Fatalf("failed to create connection: %v", err)
	}
	conn := *connPtr

	err = conn.Connect(ctx)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Disconnect()

	err = conn.Send(ctx, testData)
	if err != nil {
		t.Fatalf("failed to send data: %v", err)
	}

	res, err := conn.Receive(ctx)
	if err != nil {
		t.Fatalf("failed to receive data: %v", err)
	}

	if string(res) != "world tcp" {
		t.Errorf("expected 'world tcp', got '%s'", string(res))
	}
}

func TestTCPConnection_NotConnectedOperations(t *testing.T) {
	factory := connection.NewConnectionFactory()
	ctx := context.Background()

	connPtr, _ := factory.NewConnection(ctx, "tcp", "127.0.0.1:12345")
	conn := *connPtr

	err := conn.Send(ctx, []byte("test"))
	if !errors.Is(err, connection.ErrNotConnected) {
		t.Errorf("expected ErrNotConnected on Send, got %v", err)
	}

	_, err = conn.Receive(ctx)
	if !errors.Is(err, connection.ErrNotConnected) {
		t.Errorf("expected ErrNotConnected on Receive, got %v", err)
	}

	if conn.GetRemoteAddress() != "127.0.0.1:12345" {
		t.Errorf("expected '127.0.0.1:12345', got '%s'", conn.GetRemoteAddress())
	}
}

func TestTCPConnection_ContextCancellation(t *testing.T) {
	factory := connection.NewConnectionFactory()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// Connect to a non-existent port (should timeout/fail)
	connPtr, _ := factory.NewConnection(ctx, "tcp", "10.255.255.1:12345")
	conn := *connPtr

	err := conn.Connect(ctx)
	if err == nil {
		t.Errorf("expected connection failure or context deadline exceeded, got nil")
	}
}
