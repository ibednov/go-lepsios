package recipe

import (
	"encoding/json"
	"strconv"
	"strings"
)

// ExtractAPI parses a JSON API body using cfg API* path lists. id substitutes {id} in paths.
func ExtractAPI(body string, cfg Config, id string) Snapshot {
	cfg = cfg.withDefaults()
	var root any
	if err := json.Unmarshal([]byte(body), &root); err != nil {
		return Snapshot{}
	}
	var out Snapshot

	if req := strings.TrimSpace(cfg.APIRequireTrue); req != "" {
		if !isTruthy(walkJSON(root, substID(req, id))) {
			return Snapshot{}
		}
	}

	for _, path := range cfg.APITitlePaths {
		if s := asString(walkJSON(root, substID(path, id))); s != "" {
			out.Title = s
			break
		}
	}

	if free := strings.TrimSpace(cfg.APIFreePath); free != "" && isTruthy(walkJSON(root, substID(free, id))) {
		out.Price = 0
		out.HasPrice = true
	} else {
		div := cfg.APIPriceDivideBy
		if div <= 0 {
			div = 1
		}
		for _, path := range cfg.APIPricePaths {
			if p, ok := asFloat(walkJSON(root, substID(path, id))); ok {
				p = p / div
				if p > 0 || div != 1 {
					// cents→units may still be 0; only accept >0 unless free handled above
					if p > 0 {
						out.Price = p
						out.HasPrice = true
						break
					}
				}
			}
		}
	}

	for _, path := range cfg.APICurrencyPaths {
		if s := asString(walkJSON(root, substID(path, id))); s != "" {
			out.Currency = s
			break
		}
	}

	for _, path := range cfg.APIImagePaths {
		if s := asString(walkJSON(root, substID(path, id))); s != "" {
			out.ImageURL = s
			break
		}
	}

	if out.Currency == "" {
		out.Currency = cfg.DefaultCurrency
	}
	return out
}

func substID(path, id string) string {
	return strings.ReplaceAll(path, "{id}", id)
}

func asString(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case float64:
		// avoid 1.230000e+02 — only if whole
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case json.Number:
		return x.String()
	case bool:
		if x {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func asFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case string:
		s := strings.ReplaceAll(strings.TrimSpace(x), " ", "")
		s = strings.ReplaceAll(s, ",", ".")
		if s == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(s, 64)
		return f, err == nil
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	default:
		return 0, false
	}
}

func isTruthy(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case float64:
		return x != 0
	case string:
		s := strings.ToLower(strings.TrimSpace(x))
		return s == "true" || s == "1" || s == "yes"
	default:
		return false
	}
}
