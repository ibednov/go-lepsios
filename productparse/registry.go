package productparse

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/ibednov/go-lepsios/log"
)

type Registry struct {
	strategies []Strategy
}

func NewRegistry(strategies ...Strategy) *Registry {
	return &Registry{strategies: strategies}
}

func (r *Registry) StrategyNames() []string {
	if r == nil {
		return nil
	}
	names := make([]string, 0, len(r.strategies))
	seen := make(map[string]struct{}, len(r.strategies))
	for _, s := range r.strategies {
		name := strategyName(s)
		if name == "" || name == "unknown" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return names
}

func (r *Registry) StrategyForHost(host string) string {
	for _, s := range r.strategies {
		if s.CanHandle(host) {
			return strategyName(s)
		}
	}
	return "unknown"
}

func (r *Registry) Parse(ctx context.Context, normalizedURL string) (RawParseResult, error) {
	start := time.Now()
	host := hostFromURL(normalizedURL)
	result, err := r.parse(ctx, normalizedURL)
	logSidecar("parse", normalizedURL, host, r.StrategyForHost(host), time.Since(start), err, result)
	return result, err
}

func (r *Registry) parse(ctx context.Context, normalizedURL string) (RawParseResult, error) {
	u, err := url.Parse(normalizedURL)
	if err != nil {
		return RawParseResult{}, &SidecarError{
			Code:   "invalid_input",
			Source: "parser.registry.parse",
			Err:    err,
		}
	}

	host := strings.ToLower(u.Hostname())
	for _, strategy := range r.strategies {
		if strategy.CanHandle(host) {
			return strategy.Parse(ctx, normalizedURL)
		}
	}

	return RawParseResult{}, &SidecarError{
		Code:   "unsupported_domain",
		Source: "parser.registry.parse",
		Err:    fmt.Errorf("host is not supported: %s", host),
	}
}

func (r *Registry) ParseCard(ctx context.Context, normalizedURL string) (RawParseResult, error) {
	start := time.Now()
	host := hostFromURL(normalizedURL)
	result, err := r.parseCard(ctx, normalizedURL)
	logSidecar("parse_card", normalizedURL, host, r.StrategyForHost(host), time.Since(start), err, result)
	return result, err
}

func (r *Registry) parseCard(ctx context.Context, normalizedURL string) (RawParseResult, error) {
	u, err := url.Parse(normalizedURL)
	if err != nil {
		return RawParseResult{}, &SidecarError{
			Code:   "invalid_input",
			Source: "parser.registry.parse_card",
			Err:    err,
		}
	}

	host := strings.ToLower(u.Hostname())
	for _, strategy := range r.strategies {
		if strategy.CanHandle(host) {
			if cp, ok := strategy.(CardParser); ok {
				return cp.ParseCard(ctx, normalizedURL)
			}
			res, perr := strategy.Parse(ctx, normalizedURL)
			if perr != nil {
				return res, perr
			}
			res.ImageURL = nil
			return res, nil
		}
	}

	return RawParseResult{}, &SidecarError{
		Code:   "unsupported_domain",
		Source: "parser.registry.parse_card",
		Err:    fmt.Errorf("host is not supported: %s", host),
	}
}

func (r *Registry) ParseProductImageURL(ctx context.Context, normalizedURL string) (*string, error) {
	start := time.Now()
	host := hostFromURL(normalizedURL)
	result, err := r.parseProductImageURL(ctx, normalizedURL)
	image := RawParseResult{ImageURL: result}
	logSidecar("parse_product_image", normalizedURL, host, r.StrategyForHost(host), time.Since(start), err, image)
	return result, err
}

func (r *Registry) parseProductImageURL(ctx context.Context, normalizedURL string) (*string, error) {
	u, err := url.Parse(normalizedURL)
	if err != nil {
		return nil, &SidecarError{
			Code:   "invalid_input",
			Source: "parser.registry.parse_product_image",
			Err:    err,
		}
	}

	host := strings.ToLower(u.Hostname())
	for _, strategy := range r.strategies {
		if strategy.CanHandle(host) {
			pip, ok := strategy.(ProductImageParser)
			if !ok {
				return nil, &SidecarError{
					Code:   "unsupported_operation",
					Source: "parser.registry.parse_product_image",
					Err:    fmt.Errorf("product image URL not supported for host: %s", host),
				}
			}
			return pip.ParseProductImageURL(ctx, normalizedURL)
		}
	}

	return nil, &SidecarError{
		Code:   "unsupported_domain",
		Source: "parser.registry.parse_product_image",
		Err:    fmt.Errorf("host is not supported: %s", host),
	}
}

func strategyName(s Strategy) string {
	t := reflect.TypeOf(s)
	if t == nil {
		return "unknown"
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	pkg := t.PkgPath()
	if idx := strings.LastIndex(pkg, "/"); idx >= 0 {
		return pkg[idx+1:]
	}
	return "unknown"
}

func logSidecar(
	operation, normalizedURL, host, strategy string,
	duration time.Duration,
	err error,
	result RawParseResult,
) {
	var sidecarErr *SidecarError
	if errors.As(err, &sidecarErr) {
		if name := sidecarErr.StrategyName(); name != "" {
			strategy = name
		}
	}

	fields := []interface{}{
		"component", "parser",
		"stage", "sidecar",
		"operation", operation,
		"strategy", strategy,
		"host", host,
		"url", normalizedURL,
		"duration_ms", duration.Milliseconds(),
		"success", err == nil,
	}
	if err != nil {
		fields = append(fields,
			"error_code", ParseErrorCode(err),
			"sidecar_source", sidecarErrSource(err),
			"error", err.Error(),
		)
		log.Error("parser.sidecar.finished", fields...)
		return
	}
	fields = append(fields, "parsed", parsedLogFrom(result))
	if len(result.WB) > 0 {
		fields = append(fields, "wb", result.WB)
	}
	log.Info("parser.sidecar.finished", fields...)
}

type sidecarParsedLog struct {
	Title      string   `json:"title,omitempty"`
	Price      *float64 `json:"price"`
	Currency   string   `json:"currency,omitempty"`
	ImageURL   string   `json:"image_url,omitempty"`
	PriceError string   `json:"price_error,omitempty"`
}

func parsedLogFrom(result RawParseResult) sidecarParsedLog {
	out := sidecarParsedLog{
		Title:      result.Title,
		Price:      result.Price,
		PriceError: result.PriceError,
	}
	if result.Currency != "" {
		out.Currency = result.Currency.String()
	}
	if result.ImageURL != nil && *result.ImageURL != "" {
		out.ImageURL = *result.ImageURL
	}
	return out
}

func hostFromURL(normalizedURL string) string {
	u, err := url.Parse(normalizedURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

func sidecarErrSource(err error) string {
	var sidecarErr *SidecarError
	if errors.As(err, &sidecarErr) {
		return sidecarErr.Source
	}
	return ""
}
