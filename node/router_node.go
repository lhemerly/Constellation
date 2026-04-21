package node

import (
	"sync"
)

// RoutePredicate determines if an event should be routed to a specific node.
type RoutePredicate func(event []byte) bool

// route defines a routing rule.
type route struct {
	target    Node
	predicate RoutePredicate
}

// RouterNode extends BaseNode to provide advanced message routing capabilities.
// It allows sending events conditionally to specific nodes rather than broadcasting
// to all subscribers.
type RouterNode struct {
	*BaseNode
	routesMutex sync.RWMutex
	routes      []route
}

// NewRouterNode creates a new RouterNode with a given ID.
func NewRouterNode(id string) *RouterNode {
	return &RouterNode{
		BaseNode: NewBaseNode(id),
		routes:   make([]route, 0),
	}
}

// AddRoute adds a new routing rule. The event will be forwarded to the target
// node if the predicate evaluates to true.
func (r *RouterNode) AddRoute(predicate RoutePredicate, target Node) {
	r.routesMutex.Lock()
	defer r.routesMutex.Unlock()

	r.routes = append(r.routes, route{
		target:    target,
		predicate: predicate,
	})

	// Also subscribe it so we track it as a child in BaseNode
	r.BaseNode.Subscribe(target)
}

// ClearRoutes removes all existing routing rules.
func (r *RouterNode) ClearRoutes() {
	r.routesMutex.Lock()
	defer r.routesMutex.Unlock()

	for _, route := range r.routes {
		r.BaseNode.Unsubscribe(route.target)
	}
	r.routes = make([]route, 0)
}

// Notify overrides the BaseNode Notify to only send events to nodes
// where the corresponding routing predicate evaluates to true, OR if they are
// normal subscribers (no route applied).
func (r *RouterNode) Notify(event []byte) error {
	var wg sync.WaitGroup

	r.routesMutex.RLock()
	// Collect all targets that are part of routes, regardless of whether the predicate matches.
	// This helps us avoid broadcasting to them later.
	allRouteTargets := make(map[string]struct{})

	for _, route := range r.routes {
		allRouteTargets[route.target.GetID()] = struct{}{}
		if route.predicate(event) {
			wg.Add(1)
			go func(n Node) {
				defer wg.Done()
				n.Process(event)
			}(route.target)
		}
	}
	r.routesMutex.RUnlock()

	// Handle standard subscriptions that are NOT in routes
	// They receive everything (broadcast style)
	r.BaseNode.mutex.RLock()
	for _, sub := range r.BaseNode.subscriptions {
		// If the node is part of the routes, it only gets things it's routed to.
		if _, inRoute := allRouteTargets[sub.GetID()]; !inRoute {
			wg.Add(1)
			go func(n Node) {
				defer wg.Done()
				n.Process(event)
			}(sub)
		}
	}
	r.BaseNode.mutex.RUnlock()

	wg.Wait()
	return nil
}
