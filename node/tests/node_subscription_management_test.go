package node_test

import (
    "sync"
    "testing"
	"fmt"

    "github.com/lhemerly/Constellation/node"
)

func TestBaseNodeSubscriptionManagement(t *testing.T) {
    const numNodes = 100
    var wg sync.WaitGroup

    nodes := make([]*node.BaseNode, numNodes)
    for i := 0; i < numNodes; i++ {
        node := node.NewBaseNode("node-" + fmt.Sprint(i))
        nodes[i] = node
        if err := node.Create(); err != nil {
            t.Fatalf("Node %d: Create() error = %v", i, err)
        }
    }

    // Test subscription management
    for i := 0; i < numNodes; i++ {
        for j := 0; j < numNodes; j++ {
            if i != j {
                wg.Add(1)
                go func(i, j int) {
                    defer wg.Done()
                    if err := nodes[i].Subscribe(nodes[j]); err != nil {
                        t.Errorf("Node %d: Subscribe() error = %v", i, err)
                    }
                }(i, j)
            }
        }
    }

    wg.Wait()

    // Verify subscriptions
    for i := 0; i < numNodes; i++ {
        for j := 0; j < numNodes; j++ {
            if i != j {
                if nodes[i].GetSubscription(nodes[j].GetID()) == nil {
                    t.Errorf("Node %d: Subscription to node %d not found", i, j)
                }
            }
        }
    }

    // Test unsubscription management
    for i := 0; i < numNodes; i++ {
        for j := 0; j < numNodes; j++ {
            if i != j {
                wg.Add(1)
                go func(i, j int) {
                    defer wg.Done()
                    if err := nodes[i].Unsubscribe(nodes[j]); err != nil {
                        t.Errorf("Node %d: Unsubscribe() error = %v", i, err)
                    }
                }(i, j)
            }
        }
    }

    wg.Wait()

    // Verify unsubscriptions
    for i := 0; i < numNodes; i++ {
        for j := 0; j < numNodes; j++ {
            if i != j {
                if nodes[i].GetSubscription(nodes[j].GetID()) != nil {
                    t.Errorf("Node %d: Subscription to node %d still exists", i, j)
                }
            }
        }
    }

    for i := 0; i < numNodes; i++ {
        if err := nodes[i].Delete(); err != nil {
            t.Fatalf("Node %d: Delete() error = %v", i, err)
        }
    }
}
