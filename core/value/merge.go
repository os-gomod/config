package value

import "sort"

// Merge combines multiple value maps into a single map using priority-based resolution.
// When the same key appears in multiple maps, the value with the highest Priority wins.
// Returns the merged map and a MergePlan describing the merge metadata.
func Merge(maps ...map[string]Value) (map[string]Value, MergePlan) {
	if len(maps) == 0 {
		empty := make(map[string]Value)
		return empty, MergePlan{
			Order:     nil,
			Hash:      ComputeChecksum(nil),
			TotalKeys: 0,
			Layers:    make(map[string][]string),
		}
	}
	result, overridden := mergeByPriority(maps)
	order := SortedKeys(result)
	overriddenKeys := make([]string, 0, len(overridden))
	for k := range overridden {
		overriddenKeys = append(overriddenKeys, k)
	}
	sort.Strings(overriddenKeys)
	plan := MergePlan{
		Order:          order,
		Hash:           ComputeChecksum(result),
		TotalKeys:      len(result),
		OverriddenKeys: overriddenKeys,
		Layers:         make(map[string][]string),
	}
	return result, plan
}

// mergeByPriority is the shared core merge logic used by both Merge and MergeWithPriorityPlan.
// It returns the merged map and a set of keys that were overridden by higher-priority values.
func mergeByPriority(maps []map[string]Value) (merged map[string]Value, overwritten map[string]bool) {
	capacity := 0
	for _, m := range maps {
		capacity += len(m)
	}
	result := make(map[string]Value, capacity)
	overridden := make(map[string]bool)
	for _, m := range maps {
		for k, v := range m {
			existing, exists := result[k]
			if !exists {
				result[k] = v
			} else if v.Priority() > existing.Priority() {
				overridden[k] = true
				result[k] = v
			}
		}
	}
	return result, overridden
}

// MergeWithLayerNames is like Merge but also tracks which layer contributed each key.
// The names slice corresponds 1:1 with the maps slice.
func MergeWithLayerNames(maps []map[string]Value, names []string) (map[string]Value, MergePlan) {
	result, plan := Merge(maps...)
	plan.LayerNames = names
	plan.Layers = make(map[string][]string, len(names))
	for _, k := range plan.Order {
		v := result[k]
		for i := len(maps) - 1; i >= 0; i-- {
			if m := maps[i]; m != nil {
				if mv, ok := m[k]; ok && mv.Equal(v) && mv.Priority() == v.Priority() {
					name := ""
					if i < len(names) {
						name = names[i]
					}
					plan.Layers[name] = append(plan.Layers[name], k)
					break
				}
			}
		}
	}
	for name, keys := range plan.Layers {
		sort.Strings(keys)
		plan.Layers[name] = keys
	}
	return result, plan
}

// MergeWithPriorityPlan combines multiple value maps using priority-based resolution,
// returning only a minimal MergePlan without overridden-key tracking.
// Internally delegates to the shared mergeByPriority core.
func MergeWithPriorityPlan(maps ...map[string]Value) (map[string]Value, MergePlan) {
	if len(maps) == 0 {
		return make(map[string]Value), MergePlan{Hash: ComputeChecksum(nil)}
	}
	result, _ := mergeByPriority(maps)
	plan := MergePlan{
		Order:     SortedKeys(result),
		Hash:      ComputeChecksum(result),
		TotalKeys: len(result),
	}
	return result, plan
}

// MergePlan describes the result of a merge operation, including key ordering,
// checksum, and optional layer attribution.
type MergePlan struct {
	// LayerNames is the ordered list of layer names that contributed to the merge.
	LayerNames []string `json:"layer_names,omitempty"`
	// Layers maps each layer name to the sorted list of keys it contributed.
	Layers map[string][]string `json:"layers,omitempty"`
	// Order is the sorted list of all keys in the merged result.
	Order []string `json:"order"`
	// Hash is the SHA-256 checksum of the merged data.
	Hash string `json:"hash"`
	// TotalKeys is the total number of unique keys in the merged result.
	TotalKeys int `json:"total_keys"`
	// OverriddenKeys is the sorted list of keys whose values were overridden
	// by a higher-priority layer.
	OverriddenKeys []string `json:"overridden_keys,omitempty"`
}

// MutationKind identifies the type of a configuration mutation.
type MutationKind uint8

const (
	// MutationSet represents a key-value set operation.
	MutationSet MutationKind = iota
	// MutationDelete represents a key deletion operation.
	MutationDelete
)

// Mutation describes a single configuration change.
type Mutation struct {
	// Kind is the type of mutation (set or delete).
	Kind MutationKind
	// Key is the configuration key affected by this mutation.
	Key string
	// Value is the new value for set mutations. Unused for delete mutations.
	Value Value
}

// NewSetMutation creates a set mutation for the given key and value.
func NewSetMutation(key string, val Value) Mutation {
	return Mutation{Kind: MutationSet, Key: key, Value: val}
}

// NewDeleteMutation creates a delete mutation for the given key.
func NewDeleteMutation(key string) Mutation {
	return Mutation{Kind: MutationDelete, Key: key}
}

// ApplyDelta applies a list of mutations to a base map, returning a new map.
// The base map is not modified.
func ApplyDelta(base map[string]Value, mutations []Mutation) map[string]Value {
	if base == nil {
		base = make(map[string]Value)
	}
	result := Copy(base)
	for _, m := range mutations {
		switch m.Kind {
		case MutationSet:
			result[m.Key] = m.Value
		case MutationDelete:
			delete(result, m.Key)
		}
	}
	return result
}

// ComputeDeltaEvents computes the diff events that result from applying mutations to base.
func ComputeDeltaEvents(base map[string]Value, mutations []Mutation) []DiffEvent {
	newData := ApplyDelta(base, mutations)
	return ComputeDiff(base, newData)
}
