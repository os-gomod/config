package version

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/os-gomod/config/v2/internal/domain/value"
)

// VersionInfo stores version metadata about the configuration.
type VersionInfo struct {
	CurrentVersion string `json:"current_version"`
	SchemaVersion  string `json:"schema_version"`
	LastMigrated   string `json:"last_migrated"`
}

// Migration defines a version transition.
type Migration struct {
	FromVersion string
	ToVersion   string
	Upgrade     func(ctx context.Context, data map[string]value.Value) (map[string]value.Value, error)
	Downgrade   func(ctx context.Context, data map[string]value.Value) (map[string]value.Value, error)
}

// Manager handles configuration schema migrations.
// This integrates with the existing State system rather than duplicating it.
type Manager struct {
	mu         sync.RWMutex
	migrations map[string]Migration // keyed by "from->to"
	current    VersionInfo
	store      VersionStore
}

// VersionStore persists version information.
type VersionStore interface {
	LoadVersion() (VersionInfo, error)
	SaveVersion(info VersionInfo) error
}

// InMemoryVersionStore is an in-memory implementation.
type InMemoryVersionStore struct {
	mu  sync.RWMutex
	ver VersionInfo
}

func NewInMemoryVersionStore() *InMemoryVersionStore {
	return &InMemoryVersionStore{}
}

func (s *InMemoryVersionStore) LoadVersion() (VersionInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ver, nil
}

func (s *InMemoryVersionStore) SaveVersion(info VersionInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ver = info
	return nil
}

// NewMigrationManager creates a new migration manager.
func NewMigrationManager(store VersionStore, currentVersion string) *Manager {
	return &Manager{
		migrations: make(map[string]Migration),
		current: VersionInfo{
			CurrentVersion: currentVersion,
		},
		store: store,
	}
}

// Register adds a migration step.
func (m *Manager) Register(migration Migration) error {
	key := m.key(migration.FromVersion, migration.ToVersion)
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.migrations[key]; exists {
		return fmt.Errorf("migration from %s to %s already registered", migration.FromVersion, migration.ToVersion)
	}
	m.migrations[key] = migration
	return nil
}

// MigrateTo upgrades the configuration to the target version.
func (m *Manager) MigrateTo(ctx context.Context, data map[string]value.Value, targetVersion string) (map[string]value.Value, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	current := m.current.CurrentVersion
	result := data

	for current != targetVersion {
		nextVersion, migration, ok := m.findNextMigration(current)
		if !ok {
			return nil, fmt.Errorf("no migration path from %s to %s", current, targetVersion)
		}

		var err error
		result, err = migration.Upgrade(ctx, result)
		if err != nil {
			return nil, fmt.Errorf("migration from %s to %s failed: %w", current, nextVersion, err)
		}
		current = nextVersion
	}

	// Update stored version
	m.current.CurrentVersion = current
	if m.store != nil {
		_ = m.store.SaveVersion(m.current)
	}

	return result, nil
}

// CanMigrateTo checks if a migration path exists.
func (m *Manager) CanMigrateTo(targetVersion string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	current := m.current.CurrentVersion
	visited := make(map[string]bool)

	for current != targetVersion {
		if visited[current] {
			return false // cycle detected
		}
		visited[current] = true

		next, _, ok := m.findNextMigration(current)
		if !ok {
			return false
		}
		current = next
	}
	return true
}

// Rollback reverts to the previous version.
func (m *Manager) Rollback(ctx context.Context, data map[string]value.Value) (map[string]value.Value, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	current := m.current.CurrentVersion
	// Find migration that leads to current version
	for _, migration := range m.migrations {
		if migration.ToVersion == current {
			result, err := migration.Downgrade(ctx, data)
			if err != nil {
				return nil, fmt.Errorf("rollback from %s to %s failed: %w", current, migration.FromVersion, err)
			}
			m.current.CurrentVersion = migration.FromVersion
			if m.store != nil {
				_ = m.store.SaveVersion(m.current)
			}
			return result, nil
		}
	}
	return nil, fmt.Errorf("no rollback path from %s", current)
}

// CurrentVersion returns the current version.
func (m *Manager) CurrentVersion() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current.CurrentVersion
}

func (m *Manager) key(from, to string) string {
	return from + "->" + to
}

func (m *Manager) findNextMigration(current string) (string, Migration, bool) {
	for _, migration := range m.migrations {
		if migration.FromVersion == current {
			return migration.ToVersion, migration, true
		}
	}
	return "", Migration{}, false
}

// VersionedConfig extends the config with version awareness.
// This can be embedded or composed with existing Config.
type VersionedConfig struct {
	Manager *Manager
	Data    map[string]value.Value
}

// LoadWithVersion loads configuration with version checking.
func (vc *VersionedConfig) LoadWithVersion(ctx context.Context, data map[string]value.Value, expectedVersion string) (map[string]value.Value, error) {
	if vc.Manager.CurrentVersion() != expectedVersion {
		return vc.Manager.MigrateTo(ctx, data, expectedVersion)
	}
	return data, nil
}

// ExportVersion exports version info as JSON.
func (vc *VersionedConfig) ExportVersion() ([]byte, error) {
	return json.MarshalIndent(vc.Manager.current, "", "  ")
}
