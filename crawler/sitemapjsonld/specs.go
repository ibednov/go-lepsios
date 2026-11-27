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
// contributes its first two element children (td/th/dt/dd/div/span...).
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
			for child := n.FirstChild; child != nil && len(cells) < 2; child = child.NextSibling {
				if child.Type == htmlnode.ElementNode {
					cells = append(cells, strings.Join(strings.Fields(textContent(child)), " "))
				}
			}
			if len(cells) == 2 && cells[0] != "" && cells[1] != "" && len(cells[0]) <= 200 && len(cells[1]) <= 500 {
				name := strings.TrimSuffix(cells[0], ":")
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
