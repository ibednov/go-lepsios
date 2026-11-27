package sitemapjsonld

// repairUnescapedQuotes escapes double quotes that appear inside a JSON string
// but are not followed by a structural token. It targets the common template
// bug where product names contain an unescaped inch mark (`13.6" M5`). Valid
// JSON is returned unchanged.
func repairUnescapedQuotes(raw []byte) []byte {
	out := make([]byte, 0, len(raw)+16)
	inString, escaped := false, false
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if !inString {
			if c == '"' {
				inString = true
			}
			out = append(out, c)
			continue
		}
		if escaped {
			escaped = false
			out = append(out, c)
			continue
		}
		switch c {
		case '\\':
			escaped = true
			out = append(out, c)
		case '\n', '\r', '\t':
			// Raw control characters are invalid inside JSON strings.
			out = append(out, ' ')
		case '"':
			if closesString(raw[i+1:]) {
				inString = false
				out = append(out, c)
			} else {
				out = append(out, '\\', '"')
			}
		default:
			out = append(out, c)
		}
	}
	return out
}

func closesString(rest []byte) bool {
	for _, c := range rest {
		switch c {
		case ' ', '\t', '\n', '\r':
			continue
		case ',', '}', ']', ':':
			return true
		default:
			return false
		}
	}
	return true
}
