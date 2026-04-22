package connection

import (
	"context"
	"sync"
	"sync/atomic"
)

// ChannelConnection implements the Connection interface for local channel-based communication
type ChannelConnection struct {
	address     string
	isConnected atomic.Bool
	dataChan    chan []byte
	doneChan    chan struct{}
	mu          sync.Mutex
}

// NewChannelConnection creates a new ChannelConnection
func NewChannelConnection(ctx context.Context, address string) (*Connection, error) {
	conn := &ChannelConnection{
		address:  address,
		dataChan: make(chan []byte, 100),
		doneChan: make(chan struct{}),
	}
	close(conn.doneChan)
	var c Connection = conn
	return &c, nil
}

// Connect establishes the channel connection
func (c *ChannelConnection) Connect(ctx context.Context) error {
	if c.isConnected.Swap(true) {
		return ErrAlreadyConnected
	}
	c.mu.Lock()
	c.dataChan = make(chan []byte, 100)
	c.doneChan = make(chan struct{})
	c.mu.Unlock()
	return nil
}

// Disconnect closes the connection
func (c *ChannelConnection) Disconnect() error {
	if !c.isConnected.Swap(false) {
		return ErrNotConnected
	}
	c.mu.Lock()
	close(c.doneChan)
	c.mu.Unlock()
	return nil
}

// IsConnected returns true if connected
func (c *ChannelConnection) IsConnected() bool {
	return c.isConnected.Load()
}

// Send sends data over the channel
func (c *ChannelConnection) Send(ctx context.Context, data []byte) error {
	if !c.IsConnected() {
		return ErrNotConnected
	}

	c.mu.Lock()
	doneChan := c.doneChan
	dataChan := c.dataChan
	c.mu.Unlock()

	select {
	case dataChan <- data:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-doneChan:
		return ErrNotConnected
	}
}

// Receive receives data from the channel
func (c *ChannelConnection) Receive(ctx context.Context) ([]byte, error) {
	if !c.IsConnected() {
		return nil, ErrNotConnected
	}

	c.mu.Lock()
	doneChan := c.doneChan
	dataChan := c.dataChan
	c.mu.Unlock()

	select {
	case data := <-dataChan:
		return data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-doneChan:
		return nil, ErrNotConnected
	}
}

// GetRemoteAddress returns the address of the connection
func (c *ChannelConnection) GetRemoteAddress() string {
	return c.address
}
