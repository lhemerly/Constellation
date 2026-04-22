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
	address     string
	conn        *grpc.ClientConn
	opts        []grpc.DialOption
	mu          sync.Mutex
	isConnected atomic.Bool
	dataChan    chan []byte   // Channel to simulate data transfer
	doneChan    chan struct{} // Channel to signal disconnect
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

	// Make sure the connection starts in disconnected state
	close(conn.doneChan)

	var c Connection = conn
	return &c, nil
}

// Connect establishes a gRPC connection
func (g *GRPCConnection) Connect(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.conn != nil {
		return ErrAlreadyConnected
	}

	conn, err := grpc.DialContext(ctx, g.address, g.opts...)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	g.conn = conn
	g.isConnected.Store(true)
	// Recreate channel to reset state for a new session
	g.dataChan = make(chan []byte, 100)
	g.doneChan = make(chan struct{})
	return nil
}

// Disconnect closes the gRPC connection
func (g *GRPCConnection) Disconnect() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.conn == nil {
		return ErrNotConnected
	}

	err := g.conn.Close()
	g.conn = nil
	g.isConnected.Store(false)
	close(g.doneChan)
	// DO NOT close the data channel to avoid 'close of closed channel' panic
	// when re-connecting or concurrent usages.
	return err
}

// IsConnected checks if the gRPC connection is established
func (g *GRPCConnection) IsConnected() bool {
	return g.isConnected.Load()
}

// Send sends data over the gRPC connection
func (g *GRPCConnection) Send(ctx context.Context, data []byte) error {
	if !g.IsConnected() {
		return ErrNotConnected
	}

	g.mu.Lock()
	doneChan := g.doneChan
	dataChan := g.dataChan
	g.mu.Unlock()

	select {
	case dataChan <- data:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-doneChan:
		return ErrNotConnected
	}
}

// Receive receives data from the gRPC connection
func (g *GRPCConnection) Receive(ctx context.Context) ([]byte, error) {
	if !g.IsConnected() {
		return nil, ErrNotConnected
	}

	g.mu.Lock()
	doneChan := g.doneChan
	dataChan := g.dataChan
	g.mu.Unlock()

	select {
	case data := <-dataChan:
		return data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-doneChan:
		return nil, ErrNotConnected
	}
}

// GetRemoteAddress returns the remote address of the gRPC connection
func (g *GRPCConnection) GetRemoteAddress() string {
	return g.address
}
