package snapshot

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/os-gomod/config/core/value"
)

// VersionNode represents a single node in the version history graph.
// Each node corresponds to a snapshot with its checksum and timestamp.
type VersionNode struct {
	ID        string    `json:"id"`
	ParentIDs []string  `json:"parents,omitempty"`
	Checksum  string    `json:"checksum"`
	Timestamp time.Time `json:"timestamp"`
	MergeHash string    `json:"merge_hash"`
}

// VersionGraph maintains a directed graph of version history nodes.
// It is safe for concurrent use.
type VersionGraph struct {
	nodes map[string]*VersionNode
	mu    sync.RWMutex
}

// NewVersionGraph creates an empty version graph.
func NewVersionGraph() *VersionGraph {
	return &VersionGraph{nodes: make(map[string]*VersionNode)}
}

// Add inserts a version node into the graph.
func (g *VersionGraph) Add(node *VersionNode) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.nodes[node.ID] = node
}

// Manager manages configuration snapshots, their history, and branches.
// It maintains a bounded list of snapshots (oldest are evicted when the
// limit is reached) and supports creating named branches.
// The manager is safe for concurrent use.
type Manager struct {
	mu       sync.RWMutex
	snaps    []*Snapshot
	branches map[string]*Branch
	nextID   atomic.Uint64
	maxSnaps int
	graph    *VersionGraph
}

// NewManager creates a new snapshot Manager with the given maximum number of
// snapshots to retain. If maxSnapshots <= 0, a default of 100 is used.
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

// Take captures a new snapshot with the given version and data, and adds it
// to the history. If the number of snapshots exceeds the maximum, the oldest
// snapshots are evicted. Returns the new snapshot.
func (m *Manager) Take(version uint64, data map[string]value.Value, opts ...Option) *Snapshot {
	id := m.nextID.Add(1)
	s := New(id, version, data, opts...)
	m.mu.Lock()
	m.snaps = append(m.snaps, s)
	if len(m.snaps) > m.maxSnaps {
		m.snaps = m.snaps[len(m.snaps)-m.maxSnaps:]
	}
	m.mu.Unlock()
	m.graph.Add(&VersionNode{
		ID:        fmt.Sprintf("%d", id),
		Checksum:  s.Checksum(),
		Timestamp: s.Timestamp(),
	})
	return s
}

// Latest returns the most recent snapshot, or nil if no snapshots exist.
func (m *Manager) Latest() *Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.snaps) == 0 {
		return nil
	}
	return m.snaps[len(m.snaps)-1]
}

// Get returns the snapshot with the given ID, or nil if not found.
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

// List returns all snapshots in chronological order.
func (m *Manager) List() []*Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Snapshot, len(m.snaps))
	copy(out, m.snaps)
	return out
}

// CreateBranch creates a new named branch based on the snapshot with baseID.
// The branch starts empty and snapshots can be appended to it.
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

// Branch returns the named branch and whether it exists.
func (m *Manager) Branch(name string) (*Branch, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.branches[name]
	return b, ok
}
