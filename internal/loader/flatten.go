package loader

import "fmt"

// flattenConfigMap converts nested decoded file content into dot-notation keys
// so it merges consistently with memory and env layers.
func flattenConfigMap(src map[string]any) map[string]any {
	if src == nil {
		return make(map[string]any)
	}

	dst := make(map[string]any, len(src))
	flattenConfigInto(dst, src, "")

	return dst
}

func flattenConfigInto(dst, src map[string]any, prefix string) {
	for k, v := range src {
		key := k
		if prefix != "" {
			key = prefix + "." + key
		}

		switch val := v.(type) {
		case map[string]any:
			flattenConfigInto(dst, val, key)
		case map[any]any:
			normalized := make(map[string]any, len(val))
			for nestedKey, nestedVal := range val {
				normalized[fmt.Sprint(nestedKey)] = nestedVal
			}
			flattenConfigInto(dst, normalized, key)
		default:
			dst[key] = v
		}
	}
}
