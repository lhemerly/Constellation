package connection

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
)

// TCPConnection implements the Connection interface for raw TCP sockets.
type TCPConnection struct {
	address     string
	isConnected atomic.Bool
	conn        net.Conn
	mu          sync.Mutex
}

// NewTCPConnection creates a new TCPConnection
func NewTCPConnection(ctx context.Context, address string) (*Connection, error) {
	tc := &TCPConnection{
		address: address,
	}
	var c Connection = tc
	return &c, nil
}

// Connect establishes the TCP connection.
func (c *TCPConnection) Connect(ctx context.Context) error {
	if c.isConnected.Load() {
		return ErrAlreadyConnected
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Recheck under lock
	if c.isConnected.Load() {
		return ErrAlreadyConnected
	}

	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", c.address)
	if err != nil {
		return err
	}

	c.conn = conn
	c.isConnected.Store(true)
	return nil
}

// Disconnect closes the TCP connection.
func (c *TCPConnection) Disconnect() error {
	if !c.isConnected.Swap(false) {
		return ErrNotConnected
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

// IsConnected returns true if connected.
func (c *TCPConnection) IsConnected() bool {
	return c.isConnected.Load()
}

// Send sends data over the TCP connection.
func (c *TCPConnection) Send(ctx context.Context, data []byte) error {
	if !c.IsConnected() {
		return ErrNotConnected
	}

	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return ErrNotConnected
	}

	// Use a goroutine to support context cancellation during Write
	errChan := make(chan error, 1)
	go func() {
		_, err := conn.Write(data)
		errChan <- err
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Receive receives data from the TCP connection.
func (c *TCPConnection) Receive(ctx context.Context) ([]byte, error) {
	if !c.IsConnected() {
		return nil, ErrNotConnected
	}

	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return nil, ErrNotConnected
	}

	// Use a goroutine to support context cancellation during Read
	type result struct {
		data []byte
		err  error
	}
	resChan := make(chan result, 1)

	go func() {
		buf := make([]byte, 4096)
		n, err := conn.Read(buf)
		if err != nil {
			resChan <- result{nil, err}
			return
		}
		resChan <- result{buf[:n], nil}
	}()

	select {
	case res := <-resChan:
		return res.data, res.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// GetRemoteAddress returns the address of the connection.
func (c *TCPConnection) GetRemoteAddress() string {
	return c.address
}
