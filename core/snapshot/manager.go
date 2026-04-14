package snapshot

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/os-gomod/config/core/value"
)

// VersionNode represents a node in the snapshot version graph.
type VersionNode struct {
	ID        string    `json:"id"`
	ParentIDs []string  `json:"parents,omitempty"`
	Checksum  string    `json:"checksum"`
	Timestamp time.Time `json:"timestamp"`
	MergeHash string    `json:"merge_hash"`
}

// VersionGraph stores snapshot version nodes indexed by ID.
type VersionGraph struct {
	nodes map[string]*VersionNode
	mu    sync.RWMutex // protects nodes
}

// NewVersionGraph creates an empty VersionGraph.
func NewVersionGraph() *VersionGraph {
	return &VersionGraph{nodes: make(map[string]*VersionNode)}
}

// Add inserts a VersionNode into the graph.
func (g *VersionGraph) Add(node VersionNode) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.nodes[node.ID] = &node
}

// Manager manages a collection of snapshots with optional branching and limits.
type Manager struct {
	mu       sync.RWMutex // protects snaps and branches
	snaps    []*Snapshot
	branches map[string]*Branch
	nextID   atomic.Uint64
	maxSnaps int
	graph    *VersionGraph
}

// NewManager creates a Manager that retains up to maxSnapshots snapshots.
// A non-positive value defaults to 100.
func NewManager(maxSnapshots int) *Manager {
	if maxSnapshots <= 0 {
		maxSnapshots = 100
	}
	return &Manager{
		snaps:    make([]*Snapshot, 0, maxSnapshots),
		branches: make(map[string]*Branch),
		maxSnaps: maxSnapshots,
		graph:    NewVersionGraph(),
	}
}

// Take creates a snapshot from the current config state and registers it.
func (m *Manager) Take(version uint64, data map[string]value.Value, opts ...Option) *Snapshot {
	id := m.nextID.Add(1)
	s := New(id, version, data, opts...)

	m.mu.Lock()
	m.snaps = append(m.snaps, s)
	if len(m.snaps) > m.maxSnaps {
		m.snaps = m.snaps[len(m.snaps)-m.maxSnaps:]
	}
	m.mu.Unlock()

	m.graph.Add(VersionNode{
		ID:        fmt.Sprintf("%d", id),
		Checksum:  s.Checksum(),
		Timestamp: s.Timestamp(),
	})
	return s
}

// Latest returns the most recent snapshot, or nil if none exist.
func (m *Manager) Latest() *Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.snaps) == 0 {
		return nil
	}
	return m.snaps[len(m.snaps)-1]
}

// Get returns a snapshot by ID.
func (m *Manager) Get(id uint64) *Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, s := range m.snaps {
		if s.ID() == id {
			return s
		}
	}
	return nil
}

// List returns all snapshots in creation order.
func (m *Manager) List() []*Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Snapshot, len(m.snaps))
	copy(out, m.snaps)
	return out
}

// CreateBranch creates a new branch rooted at the given base snapshot.
func (m *Manager) CreateBranch(name string, baseID uint64) *Branch {
	m.mu.Lock()
	defer m.mu.Unlock()
	b := &Branch{
		name:      name,
		baseID:    baseID,
		snapshots: make([]*Snapshot, 0),
		created:   time.Now(),
	}
	m.branches[name] = b
	return b
}

// Branch retrieves a branch by name.
func (m *Manager) Branch(name string) (*Branch, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.branches[name]
	return b, ok
}
