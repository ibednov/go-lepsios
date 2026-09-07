package featureflags

// Normalize trims empties and deduplicates flag keys (order preserved).
func Normalize(keys []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}

// Contains reports whether key is in flags.
func Contains(flags []string, key string) bool {
	for _, f := range flags {
		if f == key {
			return true
		}
	}
	return false
}

// Merge adds keys from extra that are not already in base.
func Merge(base, extra []string) []string {
	return Normalize(append(append([]string{}, base...), extra...))
}

// Without returns flags with remove keys filtered out.
func Without(flags, remove []string) []string {
	if len(remove) == 0 {
		return Normalize(flags)
	}
	drop := map[string]struct{}{}
	for _, k := range remove {
		if k != "" {
			drop[k] = struct{}{}
		}
	}
	out := make([]string, 0, len(flags))
	seen := map[string]struct{}{}
	for _, f := range flags {
		if f == "" {
			continue
		}
		if _, bad := drop[f]; bad {
			continue
		}
		if _, ok := seen[f]; ok {
			continue
		}
		seen[f] = struct{}{}
		out = append(out, f)
	}
	return out
}
