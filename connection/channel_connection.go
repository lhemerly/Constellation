package connection

import (
	"context"
	"sync"
	"sync/atomic"
)

// ChannelConnection implements the Connection interface using Go channels
// for in-memory, local communication.
type ChannelConnection struct {
	address   string
	connected atomic.Bool
	mu        sync.Mutex
	dataChan  chan []byte
}

// NewChannelConnection creates a new ChannelConnection
func NewChannelConnection(ctx context.Context, address string) (*Connection, error) {
	conn := &ChannelConnection{
		address:  address,
		dataChan: make(chan []byte, 100), // Buffer size of 100
	}

	var c Connection = conn
	return &c, nil
}

// Connect establishes the local connection by setting the connected state to true
func (c *ChannelConnection) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected.Load() {
		return ErrAlreadyConnected
	}

	c.dataChan = make(chan []byte, 100)
	c.connected.Store(true)
	return nil
}

// Disconnect closes the local connection
func (c *ChannelConnection) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected.Load() {
		return ErrNotConnected
	}

	c.connected.Store(false)
	close(c.dataChan)
	return nil
}

// IsConnected checks if the local connection is established
func (c *ChannelConnection) IsConnected() bool {
	return c.connected.Load()
}

// Send sends data over the local connection
func (c *ChannelConnection) Send(ctx context.Context, data []byte) (err error) {
	if !c.IsConnected() {
		return ErrNotConnected
	}

	// Recover from sending on closed channel if Disconnect was called concurrently
	defer func() {
		if r := recover(); r != nil {
			err = ErrNotConnected
		}
	}()

	select {
	case c.dataChan <- data:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Receive receives data from the local connection
func (c *ChannelConnection) Receive(ctx context.Context) ([]byte, error) {
	if !c.IsConnected() {
		return nil, ErrNotConnected
	}

	select {
	case data, ok := <-c.dataChan:
		if !ok {
			return nil, ErrNotConnected
		}
		return data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// GetRemoteAddress returns the remote address of the local connection
func (c *ChannelConnection) GetRemoteAddress() string {
	return c.address
}
