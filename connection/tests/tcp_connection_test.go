package connection_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/lhemerly/Constellation/connection"
)

func startEchoTCPServer(t *testing.T) (string, func()) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 4096)
				for {
					n, err := c.Read(buf)
					if err != nil {
						return
					}
					_, _ = c.Write(buf[:n])
				}
			}(conn)
		}
	}()

	return listener.Addr().String(), func() { listener.Close() }
}

func TestTCPConnection_Lifecycle(t *testing.T) {
	addr, stop := startEchoTCPServer(t)
	defer stop()

	factory := connection.NewConnectionFactory()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	connPtr, err := factory.NewConnection(ctx, "tcp", addr)
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
		t.Errorf("expected connected")
	}

	err = conn.Send(ctx, []byte("hello tcp"))
	if err != nil {
		t.Fatalf("failed to send: %v", err)
	}

	data, err := conn.Receive(ctx)
	if err != nil {
		t.Fatalf("failed to receive: %v", err)
	}

	if string(data) != "hello tcp" {
		t.Errorf("expected 'hello tcp', got '%s'", string(data))
	}

	err = conn.Disconnect()
	if err != nil {
		t.Fatalf("failed to disconnect: %v", err)
	}

	if conn.IsConnected() {
		t.Errorf("expected not connected after disconnect")
	}
}

func TestTCPConnection_ConnectFail(t *testing.T) {
	factory := connection.NewConnectionFactory()
	ctx := context.Background()

	// Connect to a port that's likely closed
	connPtr, err := factory.NewConnection(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create connection: %v", err)
	}
	conn := *connPtr

	err = conn.Connect(ctx)
	if err == nil {
		t.Errorf("expected connection failure")
	}

	if conn.IsConnected() {
		t.Errorf("expected not connected after failure")
	}
}
