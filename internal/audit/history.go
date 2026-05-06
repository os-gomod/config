// Package audit provides an implementation of an audit recorder that maintains a history of configuration changes and secret redactions.
package audit

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/os-gomod/config/v2/internal/domain/event"
	"github.com/os-gomod/config/v2/internal/observability"
)

// HistoricalRecorder extends the observability.Recorder interface with history capabilities.
type HistoricalRecorder struct {
	recorder observability.Recorder
	store    HistoryStore
	mu       sync.RWMutex
	maxSize  int
	enabled  bool
}

// HistoryStore defines the interface for persisting audit history.
type HistoryStore interface {
	Append(entry HistoryEntry) error
	Query(filter HistoryFilter) ([]HistoryEntry, error)
	Close() error
}

// HistoryEntry represents a historical audit record with full context.
type HistoryEntry struct {
	ID            string            `json:"id"`
	Timestamp     time.Time         `json:"timestamp"`
	Operation     string            `json:"operation"`
	Action        event.AuditAction `json:"action"`
	Key           string            `json:"key"`
	OldValue      string            `json:"old_value,omitempty"`
	NewValue      string            `json:"new_value,omitempty"`
	Source        string            `json:"source"`
	Actor         string            `json:"actor"`
	TraceID       string            `json:"trace_id"`
	CorrelationID string            `json:"correlation_id"`
	Redacted      bool              `json:"redacted"`
	Metadata      map[string]any    `json:"metadata,omitempty"`
}

// HistoryFilter allows querying the audit history.
type HistoryFilter struct {
	Since     time.Time
	Until     time.Time
	Action    event.AuditAction
	KeyPrefix string
	Source    string
	Actor     string
	TraceID   string
	Limit     int
	Offset    int
}

// InMemoryHistoryStore is an in-memory implementation for testing.
type InMemoryHistoryStore struct {
	mu      sync.RWMutex
	entries []HistoryEntry
	maxSize int
}

func NewInMemoryHistoryStore(maxSize int) *InMemoryHistoryStore {
	return &InMemoryHistoryStore{
		entries: make([]HistoryEntry, 0, maxSize),
		maxSize: maxSize,
	}
}

func (s *InMemoryHistoryStore) Append(entry HistoryEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, entry)
	if len(s.entries) > s.maxSize {
		s.entries = s.entries[len(s.entries)-s.maxSize:]
	}
	return nil
}

func (s *InMemoryHistoryStore) Query(filter HistoryFilter) ([]HistoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]HistoryEntry, 0)
	for i := len(s.entries) - 1; i >= 0; i-- {
		entry := s.entries[i]
		if !s.matchesFilter(entry, filter) {
			continue
		}
		result = append(result, entry)
		if filter.Limit > 0 && len(result) >= filter.Limit {
			break
		}
	}
	return result, nil
}

func (s *InMemoryHistoryStore) matchesFilter(entry HistoryEntry, filter HistoryFilter) bool {
	if !filter.Since.IsZero() && entry.Timestamp.Before(filter.Since) {
		return false
	}
	if !filter.Until.IsZero() && entry.Timestamp.After(filter.Until) {
		return false
	}
	if filter.Action != "" && entry.Action != filter.Action {
		return false
	}
	if filter.KeyPrefix != "" && len(entry.Key) >= len(filter.KeyPrefix) && entry.Key[:len(filter.KeyPrefix)] != filter.KeyPrefix {
		return false
	}
	if filter.Source != "" && entry.Source != filter.Source {
		return false
	}
	if filter.Actor != "" && entry.Actor != filter.Actor {
		return false
	}
	if filter.TraceID != "" && entry.TraceID != filter.TraceID {
		return false
	}
	return true
}

func (s *InMemoryHistoryStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = nil
	return nil
}

// NewHistoricalRecorder wraps an existing Recorder with history capabilities.
func NewHistoricalRecorder(recorder observability.Recorder, store HistoryStore, maxSize int) *HistoricalRecorder {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &HistoricalRecorder{
		recorder: recorder,
		store:    store,
		maxSize:  maxSize,
		enabled:  true,
	}
}

// RecordConfigChangeEvent implements the config change recording.
// FIXED: Changed to match observability.Recorder signature
func (hr *HistoricalRecorder) RecordConfigChangeEvent(ctx context.Context, operation, actor string) {
	// Forward to inner recorder with correct signature
	if hr.recorder != nil {
		hr.recorder.RecordConfigChangeEvent(ctx, operation, actor)
	}

	if !hr.enabled || hr.store == nil {
		return
	}

	// Create history entry from the parameters
	historyEntry := HistoryEntry{
		Timestamp: time.Now().UTC(),
		Action:    event.AuditActionConfigChange,
		Operation: operation,
		Actor:     actor,
		Source:    "config",
	}
	_ = hr.store.Append(historyEntry)
}

// RecordSecretRedacted implements the secret redaction recording.
// FIXED: Changed to match observability.Recorder signature
func (hr *HistoricalRecorder) RecordSecretRedacted(ctx context.Context, operation string) {
	// Forward to inner recorder with correct signature
	if hr.recorder != nil {
		hr.recorder.RecordSecretRedacted(ctx, operation)
	}

	if !hr.enabled || hr.store == nil {
		return
	}

	historyEntry := HistoryEntry{
		Timestamp: time.Now().UTC(),
		Action:    event.AuditActionSecretRedacted,
		Operation: operation,
		Source:    "config",
	}
	_ = hr.store.Append(historyEntry)
}

// RecordConfigChangeEventWithEntry is a convenience method that accepts an AuditEntry
func (hr *HistoricalRecorder) RecordConfigChangeEventWithEntry(ctx context.Context, entry event.AuditEntry) {
	hr.RecordConfigChangeEvent(ctx, string(entry.Action), entry.Actor)
}

// QueryHistory retrieves historical audit records.
func (hr *HistoricalRecorder) QueryHistory(filter HistoryFilter) ([]HistoryEntry, error) {
	if hr.store == nil {
		return nil, nil
	}
	return hr.store.Query(filter)
}

// Enable enables history recording.
func (hr *HistoricalRecorder) Enable() {
	hr.mu.Lock()
	defer hr.mu.Unlock()
	hr.enabled = true
}

// Disable disables history recording.
func (hr *HistoricalRecorder) Disable() {
	hr.mu.Lock()
	defer hr.mu.Unlock()
	hr.enabled = false
}

// Close closes the underlying store.
func (hr *HistoricalRecorder) Close() error {
	if hr.store != nil {
		return hr.store.Close()
	}
	return nil
}

// ExportJSON exports history entries as JSON.
func (hr *HistoricalRecorder) ExportJSON(filter HistoryFilter) ([]byte, error) {
	entries, err := hr.QueryHistory(filter)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(entries, "", "  ")
}
