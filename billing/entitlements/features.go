package entitlements

// IntFromFeatures reads a numeric feature/limit from a plan features map.
// Missing or non-numeric values return 0.
func IntFromFeatures(features map[string]any, key string) int {
	if features == nil {
		return 0
	}
	v, ok := features[key]
	if !ok || v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case float32:
		return int(n)
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	default:
		return 0
	}
}

// GetLimit returns (limit, unlimited). Convention: stored value < 0 ⇒ unlimited.
func GetLimit(features map[string]any, key string) (limit int, unlimited bool) {
	if features == nil {
		return 0, false
	}
	v, ok := features[key]
	if !ok || v == nil {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		if int(n) < 0 {
			return 0, true
		}
		return int(n), false
	case float32:
		if int(n) < 0 {
			return 0, true
		}
		return int(n), false
	case int:
		if n < 0 {
			return 0, true
		}
		return n, false
	case int32:
		if n < 0 {
			return 0, true
		}
		return int(n), false
	case int64:
		if n < 0 {
			return 0, true
		}
		return int(n), false
	default:
		return 0, false
	}
}

// EnableIfLimitNonZero appends featureName when features[limitKey] != 0
// (including unlimited / negative). Used to derive boolean capabilities from caps.
func EnableIfLimitNonZero(features map[string]any, limitKey, featureName string) []string {
	if featureName == "" {
		return nil
	}
	if IntFromFeatures(features, limitKey) != 0 {
		return []string{featureName}
	}
	return nil
}

// MergeUnique concatenates feature name slices without duplicates (first wins).
func MergeUnique(parts ...[]string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, part := range parts {
		for _, f := range part {
			if f == "" {
				continue
			}
			if _, ok := seen[f]; ok {
				continue
			}
			seen[f] = struct{}{}
			out = append(out, f)
		}
	}
	return out
}

// Has reports whether name is present in features.
func Has(features []string, name string) bool {
	for _, f := range features {
		if f == name {
			return true
		}
	}
	return false
}
