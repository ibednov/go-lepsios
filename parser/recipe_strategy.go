package parser

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/ibednov/go-lepsios/currency"
	"github.com/ibednov/go-lepsios/parser/fetchretry"
	"github.com/ibednov/go-lepsios/parser/recipe"
)

// RecipeStrategy is a config-driven Strategy (html_jsonld kind).
type RecipeStrategy struct {
	cfg    recipe.Config
	path   *regexp.Regexp
	client *http.Client
}

func NewRecipeStrategy(cfg recipe.Config) (*RecipeStrategy, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	// apply defaults via extract path
	var path *regexp.Regexp
	if r := strings.TrimSpace(cfg.PathRegex); r != "" {
		re, err := regexp.Compile(r)
		if err != nil {
			return nil, fmt.Errorf("recipe %q path_regex: %w", cfg.Code, err)
		}
		path = re
	}
	return &RecipeStrategy{
		cfg:    cfg,
		path:   path,
		client: &http.Client{Timeout: 12 * time.Second},
	}, nil
}

func RecipeStrategies(cfgs ...recipe.Config) ([]Strategy, error) {
	out := make([]Strategy, 0, len(cfgs))
	for _, c := range cfgs {
		s, err := NewRecipeStrategy(c)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func (s *RecipeStrategy) Name() string { return s.cfg.Code }

func (s *RecipeStrategy) CanHandle(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	h = strings.TrimPrefix(h, "www.")
	for _, allowed := range s.cfg.Hosts {
		a := strings.ToLower(strings.TrimSpace(allowed))
		a = strings.TrimPrefix(a, "www.")
		if h == a {
			return true
		}
	}
	return false
}

func (s *RecipeStrategy) Parse(ctx context.Context, normalizedURL string) (RawParseResult, error) {
	card, err := s.ParseCard(ctx, normalizedURL)
	if err != nil {
		return RawParseResult{}, err
	}
	img, err := s.ParseProductImageURL(ctx, normalizedURL)
	if err != nil {
		img = nil
	}
	card.ImageURL = img
	return card, nil
}

func (s *RecipeStrategy) ParseCard(ctx context.Context, normalizedURL string) (RawParseResult, error) {
	if err := s.validateURL(normalizedURL); err != nil {
		return RawParseResult{}, &SidecarError{Code: "invalid_input", Source: "parser." + s.cfg.Code + ".parse_card", Err: err}
	}
	body, err := s.fetch(ctx, normalizedURL)
	if err != nil {
		return RawParseResult{}, &SidecarError{Code: "parse_failed", Source: "parser." + s.cfg.Code + ".parse_card", Err: err}
	}
	snap := recipe.ExtractPage(body, s.cfg, s.pathProductID(normalizedURL))
	if strings.TrimSpace(snap.Title) == "" {
		return RawParseResult{}, &SidecarError{Code: "parse_failed", Source: "parser." + s.cfg.Code + ".parse_card", Err: fmt.Errorf("title not found")}
	}
	def := s.cfg.DefaultCurrency
	if def == "" {
		def = "BYN"
	}
	out := RawParseResult{
		Title:    snap.Title,
		Currency: currency.Normalize(snap.Currency, currency.Code(def)),
	}
	if snap.HasPrice && snap.Price > 0 {
		p := snap.Price
		out.Price = &p
	} else if s.cfg.NoPrice != "allow" {
		return RawParseResult{}, &SidecarError{
			Code: "no_price", Source: "parser." + s.cfg.Code + ".parse_card",
			Err: fmt.Errorf("no priced offers for %s", snap.Title),
		}
	}
	return out, nil
}

func (s *RecipeStrategy) ParseProductImageURL(ctx context.Context, normalizedURL string) (*string, error) {
	if err := s.validateURL(normalizedURL); err != nil {
		return nil, &SidecarError{Code: "invalid_input", Source: "parser." + s.cfg.Code + ".parse_product_image", Err: err}
	}
	body, err := s.fetch(ctx, normalizedURL)
	if err != nil {
		return nil, &SidecarError{Code: "parse_failed", Source: "parser." + s.cfg.Code + ".parse_product_image", Err: err}
	}
	snap := recipe.ExtractPage(body, s.cfg, s.pathProductID(normalizedURL))
	if snap.ImageURL == "" {
		return nil, nil
	}
	u := snap.ImageURL
	return &u, nil
}

func (s *RecipeStrategy) validateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if !s.CanHandle(u.Hostname()) {
		return fmt.Errorf("host not in recipe %s", s.cfg.Code)
	}
	if s.path != nil {
		m := s.path.FindStringSubmatch(u.Path)
		if m == nil {
			return fmt.Errorf("path %q does not match path_regex", u.Path)
		}
		if len(s.cfg.RejectPathGroup1) > 0 && len(m) > 1 {
			g1 := strings.ToLower(m[1])
			for _, bad := range s.cfg.RejectPathGroup1 {
				if g1 == strings.ToLower(strings.TrimSpace(bad)) {
					return fmt.Errorf("path group1 %q rejected", m[1])
				}
			}
		}
	}
	return nil
}

func (s *RecipeStrategy) pathProductID(raw string) string {
	if s.path == nil || s.cfg.PathIDGroup <= 0 {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	m := s.path.FindStringSubmatch(u.Path)
	if m == nil || s.cfg.PathIDGroup >= len(m) {
		return ""
	}
	return m[s.cfg.PathIDGroup]
}

func (s *RecipeStrategy) fetch(ctx context.Context, pageURL string) (string, error) {
	ua := s.cfg.UserAgent
	if ua == "" {
		ua = "Mozilla/5.0 (compatible; Parser/1.0)"
	}
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", ua)
		req.Header.Set("Accept", "text/html,application/xhtml+xml")
		resp, err := s.client.Do(req)
		if err != nil {
			last = err
			if fetchretry.Retryable(err) && attempt < 2 {
				_ = fetchretry.Wait(ctx, attempt+1, 0)
				continue
			}
			return "", err
		}
		b, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		resp.Body.Close()
		if readErr != nil {
			last = readErr
			continue
		}
		if resp.StatusCode >= 500 || resp.StatusCode == 429 {
			last = &fetchretry.HTTPStatusError{StatusCode: resp.StatusCode}
			if attempt < 2 {
				_ = fetchretry.Wait(ctx, attempt+1, 0)
				continue
			}
			return "", last
		}
		if resp.StatusCode >= 400 {
			return "", &fetchretry.HTTPStatusError{StatusCode: resp.StatusCode}
		}
		return string(b), nil
	}
	if last != nil {
		return "", last
	}
	return "", fmt.Errorf("fetch failed")
}
