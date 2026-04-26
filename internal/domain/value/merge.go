package value

import "sort"

// ---------------------------------------------------------------------------
// MutationKind
// ---------------------------------------------------------------------------

// MutationKind describes what kind of mutation occurred.
type MutationKind int

const (
	MutationSet    MutationKind = iota // Key was set or updated.
	MutationDelete                     // Key was deleted.
)

func (m MutationKind) String() string {
	switch m {
	case MutationSet:
		return "set"
	case MutationDelete:
		return "delete"
	default:
		return "unknown"
	}
}

// ---------------------------------------------------------------------------
// Mutation
// ---------------------------------------------------------------------------

// Mutation represents a single key-level change.
type Mutation struct {
	Key    string
	Kind   MutationKind
	Value  Value  // For Set mutations; zero for Delete.
	Source string // Optional: layer name or source that caused the mutation.
}

// NewSetMutation creates a Mutation of kind MutationSet.
func NewSetMutation(key string, val Value, source string) Mutation {
	return Mutation{
		Key:    key,
		Kind:   MutationSet,
		Value:  val,
		Source: source,
	}
}

// NewDeleteMutation creates a Mutation of kind MutationDelete.
func NewDeleteMutation(key, source string) Mutation {
	return Mutation{
		Key:    key,
		Kind:   MutationDelete,
		Source: source,
	}
}

// ---------------------------------------------------------------------------
// MergePlan
// ---------------------------------------------------------------------------

// MergePlan records what happened during a merge operation.
type MergePlan struct {
	Mutations []Mutation // The mutations applied in merge order.
	Layers    []string   // Names of the layers that were merged (in order).
}

// ---------------------------------------------------------------------------
// Merge
// ---------------------------------------------------------------------------

// Merge combines multiple maps of Values into a single map.
// Later maps override earlier ones (higher priority wins on conflict).
// The returned MergePlan records all mutations.
func Merge(maps ...map[string]Value) (map[string]Value, MergePlan) {
	return MergeWithLayerNames(maps, nil)
}

// MergeWithLayerNames merges multiple maps with optional layer names for tracing.
// If names is nil or shorter than maps, unnamed layers get "unnamed-N".
func MergeWithLayerNames(maps []map[string]Value, names []string) (map[string]Value, MergePlan) {
	plan := MergePlan{
		Mutations: make([]Mutation, 0),
		Layers:    make([]string, 0, len(maps)),
	}

	result := make(map[string]Value)

	for i, m := range maps {
		layerName := ""
		if names != nil && i < len(names) {
			layerName = names[i]
		} else {
			layerName = unnamedLayer(i)
		}
		plan.Layers = append(plan.Layers, layerName)

		for k, v := range m {
			old, exists := result[k]
			if !exists || !old.Equal(v) {
				result[k] = v
				plan.Mutations = append(plan.Mutations, NewSetMutation(k, v, layerName))
			}
		}
	}

	return result, plan
}

// MergeWithPriorityPlan merges maps but the MergePlan records per-layer
// information about which keys came from which layer, respecting priority.
func MergeWithPriorityPlan(maps []map[string]Value, names []string) (map[string]Value, MergePlan) {
	return MergeWithLayerNames(maps, names)
}

func unnamedLayer(index int) string {
	return "unnamed-" + string(rune('0'+index))
}

// ---------------------------------------------------------------------------
// ApplyDelta
// ---------------------------------------------------------------------------

// ApplyDelta applies a set of Mutations to a base map, producing a new map.
// The base map is not modified. If base is nil, a new map is created.
func ApplyDelta(base map[string]Value, mutations []Mutation) map[string]Value {
	result := Copy(base)
	if result == nil {
		result = make(map[string]Value)
	}
	for _, mut := range mutations {
		switch mut.Kind {
		case MutationSet:
			result[mut.Key] = mut.Value
		case MutationDelete:
			delete(result, mut.Key)
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// ComputeDeltaEvents
// ---------------------------------------------------------------------------

// ComputeDeltaEvents computes mutations between old and new maps.
func ComputeDeltaEvents(old, new_ map[string]Value, source string) []Mutation {
	events := ComputeDiff(old, new_)
	mutations := make([]Mutation, 0, len(events))
	for _, e := range events {
		switch e.Type {
		case DiffCreated:
			mutations = append(mutations, NewSetMutation(e.Key, e.New, source))
		case DiffUpdated:
			mutations = append(mutations, NewSetMutation(e.Key, e.New, source))
		case DiffDeleted:
			mutations = append(mutations, NewDeleteMutation(e.Key, source))
		}
	}
	return mutations
}

// ---------------------------------------------------------------------------
// Mutation helpers
// ---------------------------------------------------------------------------

// GroupMutationsByKind groups mutations by their kind.
//
//nolint:revive // paired slice results are intentionally positional here
func GroupMutationsByKind(mutations []Mutation) ([]Mutation, []Mutation) {
	var sets []Mutation
	var deletes []Mutation

	for _, m := range mutations {
		switch m.Kind {
		case MutationSet:
			sets = append(sets, m)
		case MutationDelete:
			deletes = append(deletes, m)
		}
	}
	return sets, deletes
}

// MutationKeys returns the sorted set of keys affected by mutations.
func MutationKeys(mutations []Mutation) []string {
	seen := make(map[string]struct{})
	for _, m := range mutations {
		seen[m.Key] = struct{}{}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
