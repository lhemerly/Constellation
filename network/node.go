// Package network provides abstractions for creating and managing nodes
// within a networked system. It includes core functionality for node lifecycle
// management, event handling, and inter-node communication, optimized for
// asynchronous operations and concurrency.
//
// The network package is designed to offer a seamless developer experience
// with a focus on modularity and ease of use. It leverages the latest
// technologies and best practices to ensure high performance, scalability,
// and ease of use.
//
// Main components:
// 
// - Node Interface: Defines the basic operations and properties that all
//   node types must implement.
// - BaseNode Struct: Provides common functionality that can be extended by
//   specific node types.
// - Lifecycle Management: Methods to create and delete nodes.
// - Data Processing: Mechanism to process input data and return output.
// - Subscription Management: Manage subscriptions to other nodes for event
//   notifications.
// - Event Notification: Notify all subscribed nodes asynchronously.
//
// Example usage:
// 
//  package main
// 
//  import (
//      "fmt"
//      "github.com/lhemerly/Constellation/network"
//  )
// 
//  func main() {
//      // Create and initialize nodes
//      node1 := network.NewBaseNode("node-1")
//      node2 := network.NewBaseNode("node-2")
// 
//      if err := node1.Create(); err != nil {
//          fmt.Printf("Error creating node1: %v\n", err)
//          return
//      }
//      if err := node2.Create(); err != nil {
//          fmt.Printf("Error creating node2: %v\n", err)
//          return
//      }
// 
//      // Set a custom process function for node2
//      node2.SetProcessFunc(func(input []byte) ([]byte, error) {
//          return []byte(fmt.Sprintf("Processed by node2: %s", input)), nil
//      })
// 
//      // Subscribe node2 to node1
//      if err := node1.Subscribe(node2); err != nil {
//          fmt.Printf("Error subscribing node2 to node1: %v\n", err)
//          return
//      }
// 
//      // Notify event from node1 to its subscribers
//      event := []byte("Hello from node1")
//      if err := node1.Notify(event); err != nil {
//          fmt.Printf("Error notifying event: %v\n", err)
//      }
// 
//      // Process input directly on node1
//      input := []byte("Direct input to node1")
//      output, err := node1.Process(input)
//      if err != nil {
//          fmt.Printf("Error processing input on node1: %v\n", err)
//      } else {
//          fmt.Printf("Output from node1: %s\n", output)
//      }
// 
//      // Ensure node2 received the event
//      output, err = node2.Process(event)
//      if err != nil {
//          fmt.Printf("Error processing event on node2: %v\n", err)
//      } else {
//          fmt.Printf("Output from node2: %s\n", output)
//      }
// 
//      // Clean up nodes
//      if err := node1.Delete(); err != nil {
//          fmt.Printf("Error deleting node1: %v\n", err)
//      }
//      if err := node2.Delete(); err != nil {
//          fmt.Printf("Error deleting node2: %v\n", err)
//      }
//  }
//
// The network package is suitable for building scalable and high-performance
// networked applications, simplifying node lifecycle management, inter-node
// communication, and event handling.
package network

// Node defines the basic operations of any node in the network.
type Node interface {
	// Create initializes the node, setting up any necessary resources.
	Create() error

	// Delete removes the node, releasing any resources.
	Delete() error

	// Process takes an input, processes it, and returns the output.
	Process(input []byte) ([]byte, error)

	// GetID returns the node's unique identifier.
	GetID() string

	// Subscribe adds a node to the subscription list for event notifications.
	Subscribe(node Node) error

	// Unsubscribe removes a node from the subscription list.
	Unsubscribe(node Node) error

	// Notify sends an event to all subscribed nodes asynchronously.
	Notify(event []byte) error
}
