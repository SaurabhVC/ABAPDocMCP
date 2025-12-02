package cache

import (
	"context"
	"sync"
	"time"
)

// MemoryCache is an in-memory implementation of Cache
type MemoryCache struct {
	mu     sync.RWMutex
	config Config

	nodes map[string]*Node                     // nodeID -> Node
	edges map[string][]*Edge                   // fromID -> []*Edge
	apis  map[string]*API                      // apiKey(name,type) -> API

	// Indexes for faster lookups
	nodesByPackage map[string][]*Node           // package -> []*Node
	edgesTo        map[string][]*Edge           // toID -> []*Edge
	edgesIndex     map[edgeKey]struct{}         // for deduplication
}

type edgeKey struct {
	fromID   string
	toID     string
	edgeType string
}

// NewMemoryCache creates a new in-memory cache
func NewMemoryCache(config Config) *MemoryCache {
	return &MemoryCache{
		config:         config,
		nodes:          make(map[string]*Node),
		edges:          make(map[string][]*Edge),
		apis:           make(map[string]*API),
		nodesByPackage: make(map[string][]*Node),
		edgesTo:        make(map[string][]*Edge),
		edgesIndex:     make(map[edgeKey]struct{}),
	}
}

// PutNode stores a node in the cache
func (m *MemoryCache) PutNode(ctx context.Context, node *Node) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Set cache timestamp
	if node.CachedAt.IsZero() {
		node.CachedAt = time.Now()
	}

	// Store node
	m.nodes[node.ID] = node

	// Update package index
	if node.Package != "" {
		m.nodesByPackage[node.Package] = append(m.nodesByPackage[node.Package], node)
	}

	// Check memory limits
	if m.config.MaxNodes > 0 && len(m.nodes) > m.config.MaxNodes {
		m.evictOldestNode()
	}

	return nil
}

// GetNode retrieves a node from the cache
func (m *MemoryCache) GetNode(ctx context.Context, id string) (*Node, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	node, exists := m.nodes[id]
	if !exists {
		return nil, ErrNotFound
	}

	// Check validity
	if !node.Valid {
		return nil, ErrInvalidated
	}

	// Check TTL
	if m.config.InvalidationPolicy.UseTTL {
		if time.Since(node.CachedAt) > m.config.InvalidationPolicy.TTL {
			return nil, ErrExpired
		}
	}

	return node, nil
}

// DeleteNode removes a node from the cache
func (m *MemoryCache) DeleteNode(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, exists := m.nodes[id]
	if !exists {
		return ErrNotFound
	}

	// Remove from main map
	delete(m.nodes, id)

	// Remove from package index
	if node.Package != "" {
		nodes := m.nodesByPackage[node.Package]
		for i, n := range nodes {
			if n.ID == id {
				m.nodesByPackage[node.Package] = append(nodes[:i], nodes[i+1:]...)
				break
			}
		}
	}

	return nil
}

// InvalidateNode marks a node as invalid
func (m *MemoryCache) InvalidateNode(ctx context.Context, id string, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, exists := m.nodes[id]
	if !exists {
		return ErrNotFound
	}

	now := time.Now()
	node.Valid = false
	node.InvalidatedAt = &now
	node.InvalidationReason = reason

	// Invalidate related edges if policy says so
	if m.config.InvalidationPolicy.InvalidateEdges {
		m.invalidateEdgesForNode(id)
	}

	return nil
}

// GetNodesByPackage returns all nodes in a package
func (m *MemoryCache) GetNodesByPackage(ctx context.Context, pkg string) ([]*Node, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes, exists := m.nodesByPackage[pkg]
	if !exists {
		return []*Node{}, nil
	}

	// Filter out invalid nodes
	validNodes := make([]*Node, 0, len(nodes))
	for _, node := range nodes {
		if node.Valid {
			validNodes = append(validNodes, node)
		}
	}

	return validNodes, nil
}

// PutEdge stores an edge in the cache
func (m *MemoryCache) PutEdge(ctx context.Context, edge *Edge) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Set timestamp
	if edge.DiscoveredAt.IsZero() {
		edge.DiscoveredAt = time.Now()
	}

	// Check for duplicates
	key := edgeKey{fromID: edge.FromID, toID: edge.ToID, edgeType: edge.EdgeType}
	if _, exists := m.edgesIndex[key]; exists {
		return nil // Already exists
	}

	// Store edge
	m.edges[edge.FromID] = append(m.edges[edge.FromID], edge)
	m.edgesTo[edge.ToID] = append(m.edgesTo[edge.ToID], edge)
	m.edgesIndex[key] = struct{}{}

	// Check memory limits
	if m.config.MaxEdges > 0 && len(m.edgesIndex) > m.config.MaxEdges {
		m.evictOldestEdge()
	}

	return nil
}

// GetEdgesFrom returns all edges originating from a node
func (m *MemoryCache) GetEdgesFrom(ctx context.Context, fromID string) ([]*Edge, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	edges, exists := m.edges[fromID]
	if !exists {
		return []*Edge{}, nil
	}

	// Filter valid edges
	validEdges := make([]*Edge, 0, len(edges))
	for _, edge := range edges {
		if edge.Valid {
			validEdges = append(validEdges, edge)
		}
	}

	return validEdges, nil
}

// GetEdgesTo returns all edges pointing to a node
func (m *MemoryCache) GetEdgesTo(ctx context.Context, toID string) ([]*Edge, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	edges, exists := m.edgesTo[toID]
	if !exists {
		return []*Edge{}, nil
	}

	// Filter valid edges
	validEdges := make([]*Edge, 0, len(edges))
	for _, edge := range edges {
		if edge.Valid {
			validEdges = append(validEdges, edge)
		}
	}

	return validEdges, nil
}

// DeleteEdge removes an edge from the cache
func (m *MemoryCache) DeleteEdge(ctx context.Context, fromID, toID, edgeType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := edgeKey{fromID: fromID, toID: toID, edgeType: edgeType}
	delete(m.edgesIndex, key)

	// Remove from fromID index
	if edges, exists := m.edges[fromID]; exists {
		for i, edge := range edges {
			if edge.ToID == toID && edge.EdgeType == edgeType {
				m.edges[fromID] = append(edges[:i], edges[i+1:]...)
				break
			}
		}
	}

	// Remove from toID index
	if edges, exists := m.edgesTo[toID]; exists {
		for i, edge := range edges {
			if edge.FromID == fromID && edge.EdgeType == edgeType {
				m.edgesTo[toID] = append(edges[:i], edges[i+1:]...)
				break
			}
		}
	}

	return nil
}

// PutAPI stores an API in the cache
func (m *MemoryCache) PutAPI(ctx context.Context, api *API) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if api.CachedAt.IsZero() {
		api.CachedAt = time.Now()
	}

	key := apiKey(api.Name, api.Type)
	m.apis[key] = api

	// Check memory limits
	if m.config.MaxAPIs > 0 && len(m.apis) > m.config.MaxAPIs {
		m.evictOldestAPI()
	}

	return nil
}

// GetAPI retrieves an API from the cache
func (m *MemoryCache) GetAPI(ctx context.Context, name, typ string) (*API, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := apiKey(name, typ)
	api, exists := m.apis[key]
	if !exists {
		return nil, ErrNotFound
	}

	if !api.Valid {
		return nil, ErrInvalidated
	}

	return api, nil
}

// GetTopAPIs returns the most-used APIs
func (m *MemoryCache) GetTopAPIs(ctx context.Context, limit int) ([]*API, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Collect all valid APIs
	apis := make([]*API, 0, len(m.apis))
	for _, api := range m.apis {
		if api.Valid {
			apis = append(apis, api)
		}
	}

	// Sort by usage count (simple bubble sort for now)
	for i := 0; i < len(apis)-1; i++ {
		for j := i + 1; j < len(apis); j++ {
			if apis[i].UsageCount < apis[j].UsageCount {
				apis[i], apis[j] = apis[j], apis[i]
			}
		}
	}

	// Return top N
	if limit > 0 && limit < len(apis) {
		apis = apis[:limit]
	}

	return apis, nil
}

// Batch operations
func (m *MemoryCache) PutNodes(ctx context.Context, nodes []*Node) error {
	for _, node := range nodes {
		if err := m.PutNode(ctx, node); err != nil {
			return err
		}
	}
	return nil
}

func (m *MemoryCache) PutEdges(ctx context.Context, edges []*Edge) error {
	for _, edge := range edges {
		if err := m.PutEdge(ctx, edge); err != nil {
			return err
		}
	}
	return nil
}

func (m *MemoryCache) PutAPIs(ctx context.Context, apis []*API) error {
	for _, api := range apis {
		if err := m.PutAPI(ctx, api); err != nil {
			return err
		}
	}
	return nil
}

// Clear removes all entries from the cache
func (m *MemoryCache) Clear(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.nodes = make(map[string]*Node)
	m.edges = make(map[string][]*Edge)
	m.apis = make(map[string]*API)
	m.nodesByPackage = make(map[string][]*Node)
	m.edgesTo = make(map[string][]*Edge)
	m.edgesIndex = make(map[edgeKey]struct{})

	return nil
}

// Stats returns cache statistics
func (m *MemoryCache) Stats(ctx context.Context) (*Stats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &Stats{
		NodeCount: len(m.nodes),
		EdgeCount: len(m.edgesIndex),
		APICount:  len(m.apis),
	}

	// Count valid entries
	for _, node := range m.nodes {
		if node.Valid {
			stats.ValidNodeCount++
		}
		// Track oldest/newest
		if stats.OldestEntry.IsZero() || node.CachedAt.Before(stats.OldestEntry) {
			stats.OldestEntry = node.CachedAt
		}
		if node.CachedAt.After(stats.NewestEntry) {
			stats.NewestEntry = node.CachedAt
		}
	}

	for _, edges := range m.edges {
		for _, edge := range edges {
			if edge.Valid {
				stats.ValidEdgeCount++
			}
		}
	}

	for _, api := range m.apis {
		if api.Valid {
			stats.ValidAPICount++
		}
	}

	// Approximate size (very rough estimate)
	stats.TotalSize = int64(len(m.nodes)*256 + len(m.edgesIndex)*128 + len(m.apis)*512)

	return stats, nil
}

// Close does nothing for in-memory cache
func (m *MemoryCache) Close() error {
	return nil
}

// Helper methods

func (m *MemoryCache) invalidateEdgesForNode(nodeID string) {
	// Invalidate edges from this node
	if edges, exists := m.edges[nodeID]; exists {
		for _, edge := range edges {
			edge.Valid = false
		}
	}

	// Invalidate edges to this node
	if edges, exists := m.edgesTo[nodeID]; exists {
		for _, edge := range edges {
			edge.Valid = false
		}
	}
}

func (m *MemoryCache) evictOldestNode() {
	var oldestID string
	var oldestTime time.Time

	for id, node := range m.nodes {
		if oldestTime.IsZero() || node.CachedAt.Before(oldestTime) {
			oldestID = id
			oldestTime = node.CachedAt
		}
	}

	if oldestID != "" {
		delete(m.nodes, oldestID)
	}
}

func (m *MemoryCache) evictOldestEdge() {
	var oldestKey edgeKey
	var oldestTime time.Time

	for _, edges := range m.edges {
		for _, edge := range edges {
			if oldestTime.IsZero() || edge.DiscoveredAt.Before(oldestTime) {
				oldestKey = edgeKey{
					fromID:   edge.FromID,
					toID:     edge.ToID,
					edgeType: edge.EdgeType,
				}
				oldestTime = edge.DiscoveredAt
			}
		}
	}

	delete(m.edgesIndex, oldestKey)
}

func (m *MemoryCache) evictOldestAPI() {
	var oldestKey string
	var oldestTime time.Time

	for key, api := range m.apis {
		if oldestTime.IsZero() || api.CachedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = api.CachedAt
		}
	}

	if oldestKey != "" {
		delete(m.apis, oldestKey)
	}
}

func apiKey(name, typ string) string {
	return typ + ":" + name
}
