# Constellation Project

[![codecov](https://codecov.io/gh/lhemerly/Constellation/branch/main/graph/badge.svg)](https://codecov.io/gh/lhemerly/Constellation)

The Constellation project provides a robust framework for creating and managing networked systems. It consists of three main packages: `node` for node management, `connection` for handling network connections, and `event` for handling event dispatching and streaming.

## Overview

This project is designed to offer a seamless developer experience with a focus on modularity, ease of use, and high performance. It leverages the latest technologies and best practices to ensure scalability and efficiency in networked systems.

## Packages

### 1. Node Package

The `node` package provides abstractions for creating and managing nodes within a networked system. It includes core functionality for node lifecycle management, event handling, and inter-node communication, optimized for asynchronous operations and concurrency.

#### Key Components

- **Node Interface**: Defines basic operations and properties for all node types.
- **BaseNode Struct**: Provides common functionality that can be extended by specific node types.

#### Features

- Node Lifecycle Management
- Data Processing
- Subscription Management
- Event Notification
- **Advanced Node Types**: Includes `FailoverNode` (primary/secondary logic), `LoadBalancerNode` (round-robin distribution), `PipelineNode` (sequential stage processing), `RouterNode` (conditional message routing), and `MapReduceNode` (parallel mappers and a single reducer).
- **Middlewares**: Supports composing node processing chains with middlewares such as `LoggingMiddleware`, `RetryMiddleware`, `RecoveryMiddleware`, `CacheMiddleware`, `TimeoutMiddleware`, and `CircuitBreakerMiddleware`.

### 2. Connection Package

The `connection` package provides an abstraction layer for network connections, allowing for a unified approach to connecting, disconnecting, sending, and receiving data across different protocols.

#### Key Components

- **Connection Interface**: Defines common methods for all connection types.
- **ConnectionFactory**: Factory for creating different types of connections.
- **GRPCConnection**: Implementation of the Connection interface for gRPC connections.
- **ChannelConnection**: Implementation of the Connection interface using Go channels for local, in-memory communication.

#### Features

- Protocol-agnostic connection management
- Support for gRPC and local channel-based connections (extensible to other protocols)
- Unified interface for sending and receiving data

### 3. Event Package

The `event` package provides interfaces and implementations for creating, dispatching, and handling events and streaming events within the Constellation system. It is designed for high concurrency and efficient streaming between nodes.

#### Key Components

- **Event Interface**: Defines the basic structure of an event.
- **StreamEvent Interface**: Extends Event for streaming data between nodes.
- **EventDispatcher**: Manages event listeners and dispatches events to them in a thread-safe, asynchronous manner.
- **StreamEventDispatcher**: Extends EventDispatcher to handle streaming events.

#### Features

- High concurrency event dispatching
- Support for regular events and streaming events
- Asynchronous and thread-safe listener management

## Usage Examples

### Node Package

```go
package main

import (
    "fmt"
    "github.com/lhemerly/Constellation/node"
)

func main() {
    // Create and initialize a new BaseNode
    n := node.NewBaseNode("node-1")
    if err := n.Create(); err != nil {
        fmt.Printf("Error creating node: %v\n", err)
        return
    }

    // Set a custom process function
    n.SetProcessFunc(func(input []byte) ([]byte, error) {
        return []byte(fmt.Sprintf("Processed: %s", input)), nil
    })

    // Process data
    output, err := n.Process([]byte("Hello, Node!"))
    if err != nil {
        fmt.Printf("Error processing data: %v\n", err)
    } else {
        fmt.Printf("Output: %s\n", output)
    }

    // Clean up the node
    if err := n.Delete(); err != nil {
        fmt.Printf("Error deleting node: %v\n", err)
    }
}
```

### Event Package

```go
package main

import (
    "fmt"
    "time"
    "github.com/lhemerly/Constellation/event"
)

func main() {
    dispatcher := event.NewEventDispatcher()
    dispatcher.RegisterListener("greet", func(e event.Event) {
        fmt.Printf("Received event type=%s data=%s\n", e.GetType(), e.GetData())
    })

    evt := event.NewBaseEvent("greet", []byte("hello"))
    dispatcher.Dispatch(evt)

    // Streaming example
    streamDisp := event.NewStreamEventDispatcher()
    streamDisp.RegisterStreamListener("sensor", func(e event.StreamEvent) {
        fmt.Printf("Stream chunk seq=%d data=%s\n", e.GetSequence(), e.GetData())
    })

    chunk := event.NewBaseStreamEvent("sensor", []byte("data-chunk"), 1, true)
    streamDisp.DispatchStream(chunk)

    // Give async dispatchers time to print before main exits
    time.Sleep(100 * time.Millisecond)
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
