package connection

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"google.golang.org/grpc"
)

// GRPCConnection implements the Connection interface for gRPC
type GRPCConnection struct {
	address   string
	conn      *grpc.ClientConn
	opts      []grpc.DialOption
	mu        sync.RWMutex
	connected atomic.Bool
	dataChan  chan []byte    // Channel to simulate data transfer
	doneChan  chan struct{}  // Closed by Disconnect to unblock in-flight Send/Receive
}

// NewGRPCConnection creates a new GRPCConnection
func NewGRPCConnection(ctx context.Context, address string, opts ...interface{}) (*Connection, error) {
	grpcOpts := make([]grpc.DialOption, 0, len(opts))
	for _, opt := range opts {
		if grpcOpt, ok := opt.(grpc.DialOption); ok {
			grpcOpts = append(grpcOpts, grpcOpt)
		}
	}

	conn := &GRPCConnection{
		address:  address,
		opts:     grpcOpts,
		dataChan: make(chan []byte, 100), // Buffer size of 100
		doneChan: make(chan struct{}),
	}

	var c Connection = conn
	return &c, nil
}

// Connect establishes a gRPC connection
func (g *GRPCConnection) Connect(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.connected.Load() {
		return ErrAlreadyConnected
	}

	conn, err := grpc.DialContext(ctx, g.address, g.opts...)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	g.conn = conn
	// Recreate channels to reset state for a new session
	g.dataChan = make(chan []byte, 100)
	g.doneChan = make(chan struct{})
	g.connected.Store(true)
	return nil
}

// Disconnect closes the gRPC connection
func (g *GRPCConnection) Disconnect() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.connected.Load() {
		return ErrNotConnected
	}

	err := g.conn.Close()
	g.conn = nil
	g.connected.Store(false)
	close(g.doneChan) // Unblock any Send/Receive blocked on the channel
	return err
}

// IsConnected checks if the gRPC connection is established
func (g *GRPCConnection) IsConnected() bool {
	// Optimization: Use atomic.Bool to prevent lock contention in hot paths
	// since IsConnected is frequently called during Send/Receive loops.
	return g.connected.Load()
}

// Send sends data over the gRPC connection
func (g *GRPCConnection) Send(ctx context.Context, data []byte) error {
	g.mu.RLock()
	if !g.connected.Load() {
		g.mu.RUnlock()
		return ErrNotConnected
	}
	dataChan := g.dataChan
	doneChan := g.doneChan
	g.mu.RUnlock()

	select {
	case dataChan <- data:
		return nil
	case <-doneChan:
		return ErrNotConnected
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Receive receives data from the gRPC connection
func (g *GRPCConnection) Receive(ctx context.Context) ([]byte, error) {
	g.mu.RLock()
	if !g.connected.Load() {
		g.mu.RUnlock()
		return nil, ErrNotConnected
	}
	dataChan := g.dataChan
	doneChan := g.doneChan
	g.mu.RUnlock()

	select {
	case data := <-dataChan:
		return data, nil
	case <-doneChan:
		return nil, ErrNotConnected
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// GetRemoteAddress returns the remote address of the gRPC connection
func (g *GRPCConnection) GetRemoteAddress() string {
	return g.address
}
