package connection

import (
	"context"
	"sync/atomic"
)

// ChannelConnection implements the Connection interface for local channel-based communication
type ChannelConnection struct {
	address     string
	isConnected atomic.Bool
	dataChan    chan []byte
}

// NewChannelConnection creates a new ChannelConnection
func NewChannelConnection(ctx context.Context, address string) (*Connection, error) {
	conn := &ChannelConnection{
		address:  address,
		dataChan: make(chan []byte, 100),
	}
	var c Connection = conn
	return &c, nil
}

// Connect establishes the channel connection
func (c *ChannelConnection) Connect(ctx context.Context) error {
	if c.isConnected.Swap(true) {
		return ErrAlreadyConnected
	}
	// Note: We don't recreate the dataChan here, so if it was closed, it would stay closed.
	// But we don't close it on Disconnect.
	c.dataChan = make(chan []byte, 100)
	return nil
}

// Disconnect closes the connection
func (c *ChannelConnection) Disconnect() error {
	if !c.isConnected.Swap(false) {
		return ErrNotConnected
	}
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

	select {
	case c.dataChan <- data:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Receive receives data from the channel
func (c *ChannelConnection) Receive(ctx context.Context) ([]byte, error) {
	if !c.IsConnected() {
		return nil, ErrNotConnected
	}

	select {
	case data := <-c.dataChan:
		return data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// GetRemoteAddress returns the address of the connection
func (c *ChannelConnection) GetRemoteAddress() string {
	return c.address
}
