package recipe

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ParseConfigs unmarshals a JSON array of recipe configs.
func ParseConfigs(raw []byte) ([]Config, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil, nil
	}
	var list []Config
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("recipe configs: %w", err)
	}
	for i := range list {
		if err := list[i].Validate(); err != nil {
			return nil, err
		}
	}
	return list, nil
}

// LoadConfigsFile reads a JSON array of configs from path (empty path → nil).
func LoadConfigsFile(path string) ([]Config, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("recipe file %s: %w", path, err)
	}
	return ParseConfigs(b)
}

// Merge overlays extra on base by Code (extra wins). Order: base then new codes from extra.
func Merge(base []Config, extra ...Config) []Config {
	idx := map[string]int{}
	out := make([]Config, 0, len(base)+len(extra))
	for _, c := range base {
		code := strings.TrimSpace(c.Code)
		idx[code] = len(out)
		out = append(out, c)
	}
	for _, c := range extra {
		code := strings.TrimSpace(c.Code)
		if i, ok := idx[code]; ok {
			out[i] = c
			continue
		}
		idx[code] = len(out)
		out = append(out, c)
	}
	return out
}
