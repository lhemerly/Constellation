# Constellation Project

[![codecov](https://codecov.io/gh/lhemerly/Constellation/branch/main/graph/badge.svg)](https://codecov.io/gh/lhemerly/Constellation)

The Constellation project provides a robust framework for creating and managing networked systems. It consists of two main packages: `Constellation` for node management and `connection` for handling network connections.

## Overview

This project is designed to offer a seamless developer experience with a focus on modularity, ease of use, and high performance. It leverages the latest technologies and best practices to ensure scalability and efficiency in networked systems.

## Packages

### 1. Constellation Package

The `Constellation` package provides abstractions for creating and managing nodes within a networked system. It includes core functionality for node lifecycle management, event handling, and inter-node communication, optimized for asynchronous operations and concurrency.

#### Key Components

- **Node Interface**: Defines basic operations and properties for all node types.
- **BaseNode Struct**: Provides common functionality that can be extended by specific node types.

#### Features

- Node Lifecycle Management
- Data Processing
- Subscription Management
- Event Notification

### 2. Connection Package

The `connection` package provides an abstraction layer for network connections, allowing for a unified approach to connecting, disconnecting, sending, and receiving data across different protocols.

#### Key Components

- **Connection Interface**: Defines common methods for all connection types.
- **ConnectionFactory**: Factory for creating different types of connections.
- **GRPCConnection**: Implementation of the Connection interface for gRPC connections.

#### Features

- Protocol-agnostic connection management
- Support for gRPC connections (extensible to other protocols)
- Unified interface for sending and receiving data

## Usage Examples

### Constellation Package

```go
package main

import (
    "fmt"
    "github.com/lhemerly/Constellation/network"
)

func main() {
    // Create and initialize a new BaseNode
    node := network.NewBaseNode("node-1")
    if err := node.Create(); err != nil {
        fmt.Printf("Error creating node: %v\n", err)
        return
    }

    // Set a custom process function
    node.SetProcessFunc(func(input []byte) ([]byte, error) {
        return []byte(fmt.Sprintf("Processed: %s", input)), nil
    })

    // Process data
    output, err := node.Process([]byte("Hello, Node!"))
    if err != nil {
        fmt.Printf("Error processing data: %v\n", err)
    } else {
        fmt.Printf("Output: %s\n", output)
    }

    // Clean up the node
    if err := node.Delete(); err != nil {
        fmt.Printf("Error deleting node: %v\n", err)
    }
}
```

### Connection Package

```go
package main

import (
    "context"
    "fmt"
    "log"
    "github.com/lhemerly/Constellation/connection"
)

func main() {
    factory := connection.NewConnectionFactory()
    ctx := context.Background()

    // Create a new gRPC connection
    conn, err := factory.NewConnection(ctx, "grpc", "localhost:50051")
    if err != nil {
        log.Fatalf("Failed to create connection: %v", err)
    }

    // Connect
    if err := (*conn).Connect(ctx); err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }
    defer (*conn).Disconnect()

    // Send data
    if err := (*conn).Send(ctx, []byte("Hello, server!")); err != nil {
        log.Fatalf("Failed to send data: %v", err)
    }

    // Receive data
    data, err := (*conn).Receive(ctx)
    if err != nil {
        log.Fatalf("Failed to receive data: %v", err)
    }

    fmt.Printf("Received: %s\n", string(data))
}
```

## Testing

Both packages include comprehensive tests to ensure correct functionality. Run the tests using the `go test` command:

```sh
go test ./...
```

## Extensibility

The project is designed to be easily extensible:

- New node types can be created by implementing the `Node` interface or extending the `BaseNode` struct.
- Additional connection types can be added by implementing the `Connection` interface and updating the `ConnectionFactory`.

## Contributing

Contributions to the Constellation project are welcome! Please refer to the `CONTRIBUTING.md` file for guidelines on how to contribute.

## License

This project is licensed under the MIT License. See the `LICENSE` file for details.
