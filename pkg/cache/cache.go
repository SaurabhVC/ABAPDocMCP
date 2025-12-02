// Package cache provides in-memory and persistent caching for graph nodes and API surface data.
package cache

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound    = errors.New("cache: entry not found")
	ErrInvalidated = errors.New("cache: entry invalidated")
	ErrExpired     = errors.New("cache: entry expired")
)

// Cache is the main interface for all cache backends
type Cache interface {
	// Node operations
	PutNode(ctx context.Context, node *Node) error
	GetNode(ctx context.Context, id string) (*Node, error)
	DeleteNode(ctx context.Context, id string) error
	InvalidateNode(ctx context.Context, id string, reason string) error
	GetNodesByPackage(ctx context.Context, pkg string) ([]*Node, error)

	// Edge operations
	PutEdge(ctx context.Context, edge *Edge) error
	GetEdgesFrom(ctx context.Context, fromID string) ([]*Edge, error)
	GetEdgesTo(ctx context.Context, toID string) ([]*Edge, error)
	DeleteEdge(ctx context.Context, fromID, toID, edgeType string) error

	// API operations
	PutAPI(ctx context.Context, api *API) error
	GetAPI(ctx context.Context, name, typ string) (*API, error)
	GetTopAPIs(ctx context.Context, limit int) ([]*API, error)

	// Batch operations
	PutNodes(ctx context.Context, nodes []*Node) error
	PutEdges(ctx context.Context, edges []*Edge) error
	PutAPIs(ctx context.Context, apis []*API) error

	// Cache management
	Clear(ctx context.Context) error
	Stats(ctx context.Context) (*Stats, error)

	// Cleanup
	Close() error
}

// Node represents a cached graph node (ABAP object)
type Node struct {
	ID          string                 // Unique identifier (e.g., "ME.ZCL_CLASS\ME:METHOD")
	ObjectType  string                 // CLAS, PROG, FUNC, FUGR, TABL, TRAN, etc.
	ObjectName  string                 // Object name
	Package     string                 // DEVCLASS
	EnclosingType string               // Parent type (for methods, functions)
	EnclosingName string               // Parent name

	// Change detection
	SourceHash      string    // SHA256 of source code
	LastModifiedADT time.Time // ADT timestamp
	CachedAt        time.Time // When cached

	// Validity
	Valid              bool       // false = invalidated
	InvalidatedAt      *time.Time // When invalidated
	InvalidationReason string     // Why invalidated

	// Flexible metadata
	Metadata map[string]interface{}
}

// Edge represents a cached graph edge (relationship between objects)
type Edge struct {
	FromID       string    // Source node ID
	ToID         string    // Target node ID
	EdgeType     string    // CALLS, USES, IMPLEMENTS, INCLUDES, etc.
	Source       string    // CROSS, WBCROSSGT
	DiscoveredAt time.Time // When discovered
	Valid        bool      // false = invalidated
}

// API represents a cached API surface entry (SAP standard API)
type API struct {
	Name        string    // API name (e.g., BAPI_SALESORDER_CREATEFROMDAT2)
	Type        string    // F, ME, TY, etc.
	Source      string    // CROSS, WBCROSSGT
	UsageCount  int       // Total usage count
	UsedByCount int       // Number of Z* objects using it
	UsedByList  []string  // List of Z* includes using it

	// Enrichment data
	Package      string    // TADIR devclass
	Module       string    // SD, MM, FI, etc.
	Component    string    // SAP component
	Description  string    // Short text
	IsDeprecated bool      // Marked as deprecated
	Replacement  string    // Suggested replacement

	// Cache metadata
	CachedAt time.Time
	Valid    bool
}

// Stats provides cache statistics
type Stats struct {
	NodeCount       int
	ValidNodeCount  int
	EdgeCount       int
	ValidEdgeCount  int
	APICount        int
	ValidAPICount   int
	OldestEntry     time.Time
	NewestEntry     time.Time
	TotalSize       int64 // Approximate size in bytes
}

// Config holds cache configuration
type Config struct {
	// Type of cache: "memory" (default) or "sqlite"
	Type string

	// For SQLite backend
	Path string // Path to SQLite file (e.g., ".cache/graph.db")

	// TTL settings
	DefaultTTL time.Duration // Default time-to-live (0 = no expiration)
	MaxAge     time.Duration // Maximum age before forced refresh

	// Invalidation policy
	InvalidationPolicy InvalidationPolicy

	// Memory limits (for in-memory cache)
	MaxNodes int // Maximum nodes in memory (0 = unlimited)
	MaxEdges int // Maximum edges in memory (0 = unlimited)
	MaxAPIs  int // Maximum APIs in memory (0 = unlimited)
}

// InvalidationPolicy defines when and how to invalidate cached entries
type InvalidationPolicy struct {
	// When to check
	CheckOnRead  bool          // Check validity on every cache read
	CheckPeriod  time.Duration // Background check interval (0 = no background checks)

	// What to check
	UseHashCheck      bool          // Compare source code hash
	UseTimestampCheck bool          // Compare ADT timestamp
	UseTTL            bool          // Use time-to-live expiration
	TTL               time.Duration // Time-to-live duration

	// How to handle
	InvalidateEdges bool // Also invalidate related edges
	Cascade         bool // Invalidate dependent nodes
}

// Predefined invalidation policies
var (
	// NoInvalidation - cache entries never expire (useful for testing)
	NoInvalidation = InvalidationPolicy{
		CheckOnRead: false,
	}

	// LazyInvalidation - TTL only, no active checks
	LazyInvalidation = InvalidationPolicy{
		CheckOnRead: false,
		CheckPeriod: 24 * time.Hour,
		UseTTL:      true,
		TTL:         7 * 24 * time.Hour, // 7 days
	}

	// BalancedInvalidation - TTL + periodic hash checks (RECOMMENDED)
	BalancedInvalidation = InvalidationPolicy{
		CheckOnRead:       false,
		CheckPeriod:       1 * time.Hour,
		UseHashCheck:      true,
		UseTTL:            true,
		TTL:               24 * time.Hour, // 1 day
		InvalidateEdges:   true,
	}

	// AggressiveInvalidation - always check on read
	AggressiveInvalidation = InvalidationPolicy{
		CheckOnRead:       true,
		UseHashCheck:      true,
		UseTimestampCheck: true,
		UseTTL:            true,
		TTL:               1 * time.Hour,
		InvalidateEdges:   true,
		Cascade:           true,
	}
)

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		Type:               "memory",
		DefaultTTL:         24 * time.Hour,
		InvalidationPolicy: BalancedInvalidation,
	}
}

// NewCache creates a new cache with the given configuration
func NewCache(config Config) (Cache, error) {
	switch config.Type {
	case "memory", "":
		return NewMemoryCache(config), nil
	case "sqlite":
		return NewSQLiteCache(config)
	default:
		return nil, errors.New("unsupported cache type: " + config.Type)
	}
}
