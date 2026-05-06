package connection

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
)

// TCPConnection implements the Connection interface for standard TCP communication.
type TCPConnection struct {
	address     string
	conn        net.Conn
	isConnected atomic.Bool
	mu          sync.Mutex
}

// NewTCPConnection creates a new TCPConnection.
func NewTCPConnection(ctx context.Context, address string) (*Connection, error) {
	tcpConn := &TCPConnection{
		address: address,
	}
	var c Connection = tcpConn
	return &c, nil
}

// Connect establishes the TCP connection.
func (t *TCPConnection) Connect(ctx context.Context) error {
	if t.isConnected.Swap(true) {
		return ErrAlreadyConnected
	}

	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", t.address)
	if err != nil {
		t.isConnected.Store(false)
		return err
	}

	t.mu.Lock()
	t.conn = conn
	t.mu.Unlock()

	return nil
}

// Disconnect closes the connection.
func (t *TCPConnection) Disconnect() error {
	if !t.isConnected.Swap(false) {
		return ErrNotConnected
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if t.conn != nil {
		err := t.conn.Close()
		t.conn = nil
		return err
	}
	return nil
}

// IsConnected returns true if connected.
func (t *TCPConnection) IsConnected() bool {
	return t.isConnected.Load()
}

// Send sends data over the TCP connection.
func (t *TCPConnection) Send(ctx context.Context, data []byte) error {
	if !t.IsConnected() {
		return ErrNotConnected
	}

	t.mu.Lock()
	conn := t.conn
	t.mu.Unlock()

	if conn == nil {
		return ErrNotConnected
	}

	// For simple implementation, we just write. A real-world implementation might add framing (e.g. length prefix)
	_, err := conn.Write(data)
	return err
}

// Receive receives data from the TCP connection.
func (t *TCPConnection) Receive(ctx context.Context) ([]byte, error) {
	if !t.IsConnected() {
		return nil, ErrNotConnected
	}

	t.mu.Lock()
	conn := t.conn
	t.mu.Unlock()

	if conn == nil {
		return nil, ErrNotConnected
	}

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}

	return buf[:n], nil
}

// GetRemoteAddress returns the address of the connection.
func (t *TCPConnection) GetRemoteAddress() string {
	return t.address
}
