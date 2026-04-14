package value

import "sort"

// Merge merges multiple value maps. For duplicate keys, the Value with the
// higher Priority wins. It returns the merged map and a MergePlan describing
// the result.
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
	capacity := 0
	for _, m := range maps {
		capacity += len(m)
	}
	result := make(map[string]Value, capacity)
	overridden := make(map[string]bool)
	layerContributions := make(map[string][]string)
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
		Layers:         layerContributions,
	}
	return result, plan
}

// MergeWithLayerNames merges maps and records which layer contributed each key.
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

// MergeWithPriorityPlan merges maps using priority-based resolution.
// For duplicate keys, the Value with the higher Priority wins.
func MergeWithPriorityPlan(maps ...map[string]Value) (map[string]Value, MergePlan) {
	if len(maps) == 0 {
		return make(map[string]Value), MergePlan{Hash: ComputeChecksum(nil)}
	}
	result := make(map[string]Value, len(maps[0]))
	for _, m := range maps {
		for k, v := range m {
			if existing, ok := result[k]; !ok || v.Priority() > existing.Priority() {
				result[k] = v
			}
		}
	}
	plan := MergePlan{
		Order:     SortedKeys(result),
		Hash:      ComputeChecksum(result),
		TotalKeys: len(result),
	}
	return result, plan
}

// MergePlan describes the outcome of a merge operation, including key ordering,
// a checksum, and which keys were overridden.
type MergePlan struct {
	LayerNames     []string            `json:"layer_names,omitempty"`
	Layers         map[string][]string `json:"layers,omitempty"`
	Order          []string            `json:"order"`
	Hash           string              `json:"hash"`
	TotalKeys      int                 `json:"total_keys"`
	OverriddenKeys []string            `json:"overridden_keys,omitempty"`
}

// MutationKind distinguishes between set and delete mutations.
type MutationKind uint8

const (
	// MutationSet indicates a key should be set to a new value.
	MutationSet MutationKind = iota
	// MutationDelete indicates a key should be removed.
	MutationDelete
)

// Mutation represents a single atomic change to a value map.
type Mutation struct {
	Kind  MutationKind
	Key   string
	Value Value
}

// NewSetMutation creates a Mutation that sets key to val.
func NewSetMutation(key string, val Value) Mutation {
	return Mutation{Kind: MutationSet, Key: key, Value: val}
}

// NewDeleteMutation creates a Mutation that deletes key.
func NewDeleteMutation(key string) Mutation {
	return Mutation{Kind: MutationDelete, Key: key}
}

// ApplyDelta applies a sequence of mutations to a base map, returning a new map.
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

// ComputeDeltaEvents applies mutations to base and returns the DiffEvents
// between the original and the result.
func ComputeDeltaEvents(base map[string]Value, mutations []Mutation) []DiffEvent {
	newData := ApplyDelta(base, mutations)
	return ComputeDiff(base, newData)
}
