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
	mu        sync.Mutex
	connected atomic.Bool
	dataChan  chan []byte // Channel to simulate data transfer
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
	// Recreate channel to reset state for a new session
	g.dataChan = make(chan []byte, 100)
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
	// Do not close dataChan here to prevent panics during concurrent Send/Receive
	// when Disconnect is called, and to avoid close-of-closed-channel on reconnects.
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
	if !g.IsConnected() {
		return ErrNotConnected
	}

	select {
	case g.dataChan <- data:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Receive receives data from the gRPC connection
func (g *GRPCConnection) Receive(ctx context.Context) ([]byte, error) {
	if !g.IsConnected() {
		return nil, ErrNotConnected
	}

	select {
	case data := <-g.dataChan:
		return data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// GetRemoteAddress returns the remote address of the gRPC connection
func (g *GRPCConnection) GetRemoteAddress() string {
	return g.address
}
