// Package woocommerce provides one configured strategy for WooCommerce Store API shops.
package woocommerce

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	entity "github.com/ibednov/go-lepsios/shopparser"
	crawlerr "github.com/ibednov/go-lepsios/shopparser/errors"
	"github.com/ibednov/go-lepsios/shopparser/httpclient"
	"github.com/ibednov/go-lepsios/shopparser/woocommerce/storeapi"
)

const Kind = "woocommerce_store_api"

type Config struct {
	Version                 int      `json:"version"`
	IncludeCategoryKeywords []string `json:"include_category_keywords"`
	ExcludeCategoryKeywords []string `json:"exclude_category_keywords"`
	IncludeCategoryIDs      []int    `json:"include_category_ids"`
	ExcludePaths            []string `json:"exclude_paths"`
}

type Adapter struct {
	code          string
	inner         *storeapi.Adapter
	excludedPaths map[string]struct{}
}

func New(cli *httpclient.Client, code, baseURL string, raw json.RawMessage) (*Adapter, error) {
	cfg, err := decodeConfig(baseURL, raw)
	if err != nil {
		return nil, err
	}
	inner := storeapi.NewWithBaseURL(cli, strings.TrimRight(baseURL, "/")).WithFilter(storeapi.CategoryFilter{
		NameContains: cfg.IncludeCategoryKeywords, ExcludeContains: cfg.ExcludeCategoryKeywords, IDs: cfg.IncludeCategoryIDs,
	})
	excludedPaths := make(map[string]struct{}, len(cfg.ExcludePaths))
	for _, path := range cfg.ExcludePaths {
		excludedPaths[canonicalPath(path)] = struct{}{}
	}
	return &Adapter{code: code, inner: inner, excludedPaths: excludedPaths}, nil
}

func ValidateConfig(baseURL string, raw json.RawMessage) error {
	_, err := decodeConfig(baseURL, raw)
	return err
}

func decodeConfig(baseURL string, raw json.RawMessage) (Config, error) {
	base, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || base.Hostname() == "" || (base.Scheme != "https" && base.Scheme != "http") || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return Config{}, fmt.Errorf("%s requires an http(s) base_url without credentials", Kind)
	}
	var cfg Config
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return Config{}, fmt.Errorf("decode %s config: %w", Kind, err)
		}
	}
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if cfg.Version != 1 {
		return Config{}, fmt.Errorf("unsupported %s config version %d", Kind, cfg.Version)
	}
	if len(cfg.IncludeCategoryKeywords)+len(cfg.ExcludeCategoryKeywords) > 50 || len(cfg.IncludeCategoryIDs) > 100 || len(cfg.ExcludePaths) > 100 {
		return Config{}, fmt.Errorf("%s filters exceed supported limits", Kind)
	}
	seenPaths := make(map[string]struct{}, len(cfg.ExcludePaths))
	for _, rawPath := range cfg.ExcludePaths {
		parsed, parseErr := url.Parse(strings.TrimSpace(rawPath))
		if parseErr != nil || parsed.IsAbs() || parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
			return Config{}, fmt.Errorf("%s exclude_paths must be absolute URL paths without query or fragment", Kind)
		}
		path := canonicalPath(parsed.Path)
		if _, exists := seenPaths[path]; exists {
			return Config{}, fmt.Errorf("%s exclude_paths contains duplicate path %q", Kind, rawPath)
		}
		seenPaths[path] = struct{}{}
	}
	for _, word := range append(append([]string{}, cfg.IncludeCategoryKeywords...), cfg.ExcludeCategoryKeywords...) {
		if strings.TrimSpace(word) == "" || len(word) > 100 {
			return Config{}, fmt.Errorf("%s category keywords must contain 1–100 characters", Kind)
		}
	}
	for _, id := range cfg.IncludeCategoryIDs {
		if id <= 0 {
			return Config{}, fmt.Errorf("%s category IDs must be positive", Kind)
		}
	}
	return cfg, nil
}

func (a *Adapter) Code() string { return a.code }

func (a *Adapter) Discover(ctx context.Context) ([]entity.ProductRef, error) {
	refs, err := a.inner.Discover(ctx)
	if err != nil {
		return nil, a.attributeError(err)
	}
	filtered := refs[:0]
	for i := range refs {
		if len(a.excludedPaths) > 0 {
			parsed, parseErr := url.Parse(refs[i].URL)
			if parseErr == nil {
				if _, excluded := a.excludedPaths[canonicalPath(parsed.Path)]; excluded {
					continue
				}
			}
		}
		refs[i].SourceCode = a.code
		filtered = append(filtered, refs[i])
	}
	return filtered, nil
}

func canonicalPath(path string) string {
	if path == "/" {
		return path
	}
	return strings.TrimRight(path, "/")
}

func (a *Adapter) Fetch(ctx context.Context, ref entity.ProductRef) ([]entity.RawOffer, error) {
	offers, err := a.inner.Fetch(ctx, ref)
	if err != nil {
		return nil, a.attributeError(err)
	}
	for i := range offers {
		offers[i].Source = a.code
		offers[i].Attrs["adapter"] = Kind
	}
	return offers, nil
}

func (a *Adapter) attributeError(err error) error {
	var crawlError *crawlerr.CrawlError
	if !errors.As(err, &crawlError) {
		return err
	}
	copy := *crawlError
	copy.Source = a.code
	return &copy
}

func (a *Adapter) LastDiagnosticResponse() (httpclient.DiagnosticResponse, bool) {
	return a.inner.LastDiagnosticResponse()
}
