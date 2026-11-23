package recipe

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Kind is the config-driven HTML/JSON-LD product page parser.
const Kind = "html_jsonld"

// Config describes one shop (or host group) without a Go package.
// Host wires these from DB/CRM/files and RegisterRecipes on the parser Registry.
type Config struct {
	Version         int      `json:"version"`
	Code            string   `json:"code"` // strategy name for gates/logs
	Hosts           []string `json:"hosts"`
	PathRegex       string   `json:"path_regex,omitempty"`
	UserAgent       string   `json:"user_agent,omitempty"`
	DefaultCurrency string   `json:"default_currency,omitempty"`
	// Title/Price/Image/Currency: ordered extractor ids (first hit wins).
	// Known: jsonld_product_name, og_title, jsonld_offer_price, jsonld_aggregate_low_price,
	// jsonld_product_image, og_image, jsonld_price_currency, default_currency
	Title    []string `json:"title,omitempty"`
	Price    []string `json:"price,omitempty"`
	Image    []string `json:"image,omitempty"`
	Currency []string `json:"currency,omitempty"`
	// NoPrice: "error" (default) → SidecarError no_price; "allow" → nil price OK for card
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
		out.UserAgent = "Mozilla/5.0 (compatible; ProductParser/1.0)"
	}
	if out.NoPrice == "" {
		out.NoPrice = "error"
	}
	return out
}
