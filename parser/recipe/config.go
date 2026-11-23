package recipe

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Kind is the config-driven HTML product page parser (jsonld / og / microdata / embedded).
const Kind = "html_jsonld"

// Config describes one shop (or host group) without a Go package.
// Host wires these via RecipeStrategies (Builtin or CRM JSON later).
type Config struct {
	Version         int      `json:"version"`
	Code            string   `json:"code"` // strategy name for gates/logs
	Hosts           []string `json:"hosts"`
	PathRegex       string   `json:"path_regex,omitempty"`
	// PathIDGroup — 1-based capture group used by embedded_*_by_id extractors (0 = off).
	PathIDGroup int `json:"path_id_group,omitempty"`
	// RejectPathGroup1 — if PathRegex group 1 is in this list → invalid_input.
	RejectPathGroup1 []string `json:"reject_path_group1,omitempty"`
	// TitleStripSuffixes — strip first matching suffix from og_title (e.g. " - на OZ.by").
	TitleStripSuffixes []string `json:"title_strip_suffixes,omitempty"`
	UserAgent          string   `json:"user_agent,omitempty"`
	DefaultCurrency    string   `json:"default_currency,omitempty"`
	// Title/Price/Image/Currency: ordered extractor ids (first hit wins).
	// jsonld_*: Product ld+json; og_*; microdata_*; embedded_*_by_id (needs PathIDGroup);
	// ad_product_*; default_currency
	Title    []string `json:"title,omitempty"`
	Price    []string `json:"price,omitempty"`
	Image    []string `json:"image,omitempty"`
	Currency []string `json:"currency,omitempty"`
	// NoPrice: "error" (default) → no_price; "allow" → nil price OK
	NoPrice string `json:"no_price,omitempty"`
}

func ParseConfig(raw json.RawMessage) (Config, error) {
	var c Config
	if len(raw) == 0 {
		return c, fmt.Errorf("recipe: empty config")
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return c, fmt.Errorf("recipe: %w", err)
	}
	return c, c.Validate()
}

func (c Config) Validate() error {
	if c.Version != 0 && c.Version != 1 {
		return fmt.Errorf("recipe: unsupported version %d", c.Version)
	}
	if strings.TrimSpace(c.Code) == "" {
		return fmt.Errorf("recipe: code required")
	}
	if len(c.Hosts) == 0 {
		return fmt.Errorf("recipe %q: hosts required", c.Code)
	}
	for _, h := range c.Hosts {
		if strings.TrimSpace(h) == "" {
			return fmt.Errorf("recipe %q: empty host", c.Code)
		}
	}
	if r := strings.TrimSpace(c.PathRegex); r != "" {
		if _, err := regexp.Compile(r); err != nil {
			return fmt.Errorf("recipe %q path_regex: %w", c.Code, err)
		}
	}
	return nil
}

func (c Config) withDefaults() Config {
	out := c
	if out.Version == 0 {
		out.Version = 1
	}
	if len(out.Title) == 0 {
		out.Title = []string{"jsonld_product_name", "og_title"}
	}
	if len(out.Price) == 0 {
		out.Price = []string{"jsonld_offer_price", "jsonld_aggregate_low_price"}
	}
	if len(out.Image) == 0 {
		out.Image = []string{"jsonld_product_image", "og_image"}
	}
	if len(out.Currency) == 0 {
		out.Currency = []string{"jsonld_price_currency", "default_currency"}
	}
	if out.DefaultCurrency == "" {
		out.DefaultCurrency = "BYN"
	}
	if out.UserAgent == "" {
		out.UserAgent = "Mozilla/5.0 (compatible; Parser/1.0)"
	}
	if out.NoPrice == "" {
		out.NoPrice = "error"
	}
	return out
}
