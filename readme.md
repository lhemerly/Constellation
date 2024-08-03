# Constellation Package

[![codecov](https://codecov.io/gh/lhemerly/Constellation/branch/main/graph/badge.svg)](https://codecov.io/gh/lhemerly/Constellation)

The `Constellation` package provides abstractions for creating and managing nodes within a networked system. It includes core functionality for node lifecycle management, event handling, and inter-node communication, optimized for asynchronous operations and concurrency.

## Overview

The `Constellation` package is designed to offer a seamless developer experience with a focus on modularity and ease of use. It leverages the latest technologies and best practices to ensure high performance, scalability, and ease of use.

## Components

### Node Interface

The `Node` interface defines the basic operations and properties that all node types must implement:

- **Create() error**: Initializes the node, setting up any necessary resources.
- **Delete() error**: Cleans up the node, releasing any resources.
- **Process(input []byte) ([]byte, error)**: Processes input data and returns output.
- **GetID() string**: Returns the node's unique identifier.
- **Subscribe(node Node) error**: Adds a node to the subscription list for event notifications.
- **Unsubscribe(node Node) error**: Removes a node from the subscription list.
- **Notify(event []byte) error**: Notifies all subscribed nodes with an event.

### BaseNode Struct

The `BaseNode` struct provides common functionality that can be extended by specific node types:

- Implements the `Node` interface.
- Manages lifecycle events (create, delete), subscriptions, and notifications.
- Ensures thread-safe operations using synchronization primitives.
- Allows setting custom processing functions for data processing.

## Features

### Node Lifecycle Management

- **Create**: Initializes the node, setting up necessary resources.
- **Delete**: Cleans up the node, releasing resources.

### Data Processing

- **Process**: Handles input data and returns output. By default, it performs a simple echo of the input data.

### Subscription Management

- **Subscribe**: Adds a node to the subscription list for event notifications.
- **Unsubscribe**: Removes a node from the subscription list.
- Ensures thread-safe operations for managing subscriptions.

### Event Notification

- **Notify**: Sends an event to all subscribed nodes asynchronously.
- Handles high concurrency efficiently using goroutines.

## Usage

### Example

```go
package main

import (
    "fmt"
    "github.com/lhemerly/Constellation"
)

func main() {
    // Create a new BaseNode
    node := network.NewBaseNode("node-1")
    
    // Initialize the node
    if err := node.Create(); err != nil {
        fmt.Printf("Error creating node: %v\n", err)
        return
    }

    // Set a custom process function
    node.SetProcessFunc(func(input []byte) ([]byte, error) {
        // Custom processing logic
        return []byte(fmt.Sprintf("Processed: %s", input)), nil
    })

    // Process data
    input := []byte("Hello, Node!")
    output, err := node.Process(input)
    if err != nil {
        fmt.Printf("Error processing data: %v\n", err)
    } else {
        fmt.Printf("Output: %s\n", output)
    }

    // Subscribe to another node
    otherNode := network.NewBaseNode("node-2")
    otherNode.Create()
    node.Subscribe(otherNode)

    // Notify subscribed nodes
    event := []byte("Event data")
    node.Notify(event)

    // Unsubscribe from the other node
    node.Unsubscribe(otherNode)

    // Clean up the node
    if err := node.Delete(); err != nil {
        fmt.Printf("Error deleting node: %v\n", err)
    }
}
```

## Testing

The package includes comprehensive tests to ensure correct functionality. The tests cover:

1. **Node Initialization and Cleanup**
   - Verify that nodes can be created and deleted asynchronously without errors.

2. **Data Processing**
   - Ensure nodes can process input data asynchronously and return appropriate output.

3. **Unique Identification**
   - Check that each node returns its unique identifier correctly.

4. **Subscription Management**
   - Verify that nodes can manage subscriptions asynchronously and handle concurrency.

5. **Event Notification**
   - Ensure nodes can notify all subscribed nodes asynchronously and handle high concurrency.

Run the tests using the `go test` command:

```sh
go test ./network/tests
```
