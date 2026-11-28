package sitemapjsonld

import (
	"bytes"
	"strings"

	htmlnode "golang.org/x/net/html"
)

// SpecAttrPrefix prefixes specification attributes extracted from HTML rows.
const SpecAttrPrefix = "spec:"

const maxSpecRows = 300

// pageSpecRows extracts name/value pairs from rows matching selector. Each row
// contributes its first and last non-empty element children (td/th/dt/dd/span...).
func pageSpecRows(body []byte, selector string) map[string]string {
	if strings.TrimSpace(selector) == "" {
		return nil
	}
	doc, err := htmlnode.Parse(bytes.NewReader(body))
	if err != nil {
		return nil
	}
	specs := map[string]string{}
	var walk func(*htmlnode.Node)
	walk = func(n *htmlnode.Node) {
		if len(specs) >= maxSpecRows {
			return
		}
		if n.Type == htmlnode.ElementNode && matchesSelector(n, selector) {
			var cells []string
			for child := n.FirstChild; child != nil && len(cells) < 8; child = child.NextSibling {
				if child.Type == htmlnode.ElementNode {
					if text := strings.Join(strings.Fields(textContent(child)), " "); text != "" {
						cells = append(cells, text)
					}
				}
			}
			// Name is the first non-empty cell and value the last one, which
			// also fits "label / dotted filler / value" list layouts.
			if len(cells) >= 2 && len(cells[0]) <= 200 && len(cells[len(cells)-1]) <= 500 {
				cells = []string{cells[0], cells[len(cells)-1]}
				name := strings.TrimSpace(strings.TrimSuffix(cells[0], ":"))
				if _, exists := specs[SpecAttrPrefix+name]; !exists {
					specs[SpecAttrPrefix+name] = cells[1]
				}
			}
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return specs
}
