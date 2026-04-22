package node

import "sync"

// Route defines a condition for routing to a specific destination node.
type Route struct {
	Condition   func([]byte) bool
	Destination Node
}

// RouterNode extends BaseNode to provide conditional message routing capabilities.
type RouterNode struct {
	*BaseNode
	routesMutex sync.RWMutex
	routes      []Route
}

// NewRouterNode creates a new RouterNode with the given ID.
func NewRouterNode(id string) *RouterNode {
	r := &RouterNode{
		BaseNode: NewBaseNode(id),
		routes:   make([]Route, 0),
	}

	// The default behavior for a RouterNode is to route incoming data.
	r.SetProcessFunc(r.routeData)
	return r
}

// AddRoute adds a new conditional route.
func (r *RouterNode) AddRoute(condition func([]byte) bool, destination Node) {
	r.routesMutex.Lock()
	defer r.routesMutex.Unlock()
	r.routes = append(r.routes, Route{Condition: condition, Destination: destination})
}

// routeData evaluates the input against registered routes and processes it
// on the first matching destination node. If no routes match, it acts as a passthrough.
func (r *RouterNode) routeData(input []byte) ([]byte, error) {
	// Acquire read lock just long enough to copy the matching destination,
	// avoiding holding the lock during the actual Process call.
	r.routesMutex.RLock()
	var dest Node
	for _, route := range r.routes {
		if route.Condition(input) {
			dest = route.Destination
			break
		}
	}
	r.routesMutex.RUnlock()

	if dest != nil {
		return dest.Process(input)
	}

	// Fallback to passthrough if no routes matched.
	return input, nil
}
