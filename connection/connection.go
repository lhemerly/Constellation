// Package connection provides an abstraction layer for network connections.
//
// This package defines a common interface for various types of network connections,
// allowing for a unified approach to connecting, disconnecting, sending, and receiving data.
// It currently supports gRPC connections and can be extended to support other protocols.
//
// The main components of this package are:
//
//  - Connection: An interface that defines the common methods for all connection types.
//  - ConnectionFactory: A factory for creating different types of connections.
//  - GRPCConnection: An implementation of the Connection interface for gRPC connections.
//
// Usage:
//
//  factory := connection.NewConnectionFactory()
//  conn, err := factory.NewConnection(ctx, "grpc", "localhost:50051")
//  if err != nil {
//      log.Fatalf("Failed to create connection: %v", err)
//  }
//
//  err = conn.Connect(ctx)
//  if err != nil {
//      log.Fatalf("Failed to connect: %v", err)
//  }
//
//  defer conn.Disconnect()
//
//  err = conn.Send(ctx, []byte("Hello, server!"))
//  if err != nil {
//      log.Fatalf("Failed to send data: %v", err)
//  }
//
//  data, err := conn.Receive(ctx)
//  if err != nil {
//      log.Fatalf("Failed to receive data: %v", err)
//  }
//
//  fmt.Printf("Received: %s\n", string(data))
//
// This package is designed to be extensible. To add support for a new connection type,
// implement the Connection interface and add a new case to the ConnectionFactory's
// NewConnection method.
package connection

import (
	"context"
	"fmt"
)

// Connection defines the interface for different types of connections
type Connection interface {
	Connect(ctx context.Context) error
	Disconnect() error
	IsConnected() bool
	Send(ctx context.Context, data []byte) error
	Receive(ctx context.Context) ([]byte, error)
	GetRemoteAddress() string
}

// ConnectionFactory is responsible for creating new connections
type ConnectionFactory struct{}

// NewConnectionFactory creates a new ConnectionFactory
func NewConnectionFactory() *ConnectionFactory {
	return &ConnectionFactory{}
}

// NewConnection creates a new connection based on the given type and address
func (f *ConnectionFactory) NewConnection(ctx context.Context, connectionType, address string, opts ...interface{}) (*Connection, error) {
	switch connectionType {
	case "grpc":
		return NewGRPCConnection(ctx, address, opts...)
	default:
		return nil, fmt.Errorf("unsupported connection type: %s", connectionType)
	}
}