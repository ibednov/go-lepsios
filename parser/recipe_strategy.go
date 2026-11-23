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

// RecipeStrategy is a config-driven Strategy (html_jsonld and/or api_json).
type RecipeStrategy struct {
	cfg    recipe.Config
	path   *regexp.Regexp
	client *http.Client
}

func NewRecipeStrategy(cfg recipe.Config) (*RecipeStrategy, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
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
	snap, err := s.extract(ctx, normalizedURL)
	if err != nil {
		return RawParseResult{}, &SidecarError{Code: "parse_failed", Source: "parser." + s.cfg.Code + ".parse_card", Err: err}
	}
	if strings.TrimSpace(snap.Title) == "" {
		return RawParseResult{}, &SidecarError{Code: "parse_failed", Source: "parser." + s.cfg.Code + ".parse_card", Err: fmt.Errorf("title not found")}
	}
	def := s.currencyForURL(normalizedURL, snap.Currency)
	out := RawParseResult{
		Title:    snap.Title,
		Currency: currency.Normalize(def, currency.Code(s.cfg.DefaultCurrency)),
	}
	if snap.HasPrice && snap.Price > 0 {
		p := snap.Price
		out.Price = &p
	} else if snap.HasPrice && snap.Price == 0 && s.cfg.NoPrice == "allow" {
		p := snap.Price
		out.Price = &p
	} else if s.cfg.NoPrice == "allow" {
		// title-only OK
	} else if snap.HasPrice && snap.Price == 0 {
		// free product with price 0 (steam) — still set pointer
		p := snap.Price
		out.Price = &p
	} else {
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
	snap, err := s.extract(ctx, normalizedURL)
	if err != nil {
		return nil, &SidecarError{Code: "parse_failed", Source: "parser." + s.cfg.Code + ".parse_product_image", Err: err}
	}
	if snap.ImageURL == "" {
		return nil, nil
	}
	u := snap.ImageURL
	return &u, nil
}

func (s *RecipeStrategy) extract(ctx context.Context, pageURL string) (recipe.Snapshot, error) {
	id := s.pathProductID(pageURL)
	if tpl := strings.TrimSpace(s.cfg.APIURLTemplate); tpl != "" {
		apiURL, err := s.buildAPIURL(pageURL, id)
		if err != nil {
			return recipe.Snapshot{}, err
		}
		body, err := s.fetchURL(ctx, apiURL, "application/json")
		if err == nil {
			snap := recipe.ExtractAPI(body, s.cfg, id)
			if strings.TrimSpace(snap.Title) != "" {
				return snap, nil
			}
			err = fmt.Errorf("api extract empty")
		}
		if !s.cfg.APIHTMLFallback {
			if err != nil {
				return recipe.Snapshot{}, err
			}
			return recipe.Snapshot{}, fmt.Errorf("api extract empty")
		}
		// fall through to HTML
	}
	body, err := s.fetchURL(ctx, pageURL, "text/html,application/xhtml+xml")
	if err != nil {
		return recipe.Snapshot{}, err
	}
	return recipe.ExtractPage(body, s.cfg, id), nil
}

func (s *RecipeStrategy) buildAPIURL(pageURL, id string) (string, error) {
	if id == "" && strings.Contains(s.cfg.APIURLTemplate, "{id}") {
		return "", fmt.Errorf("missing path id for api url")
	}
	u, err := url.Parse(pageURL)
	if err != nil {
		return "", err
	}
	host := strings.ToLower(u.Hostname())
	host = strings.TrimPrefix(host, "www.")
	out := s.cfg.APIURLTemplate
	out = strings.ReplaceAll(out, "{id}", url.PathEscape(id))
	out = strings.ReplaceAll(out, "{page_host}", host)
	// query escape for id in query string templates that used PathEscape wrongly —
	// steam uses appids= in query; PathEscape is fine for digits
	return out, nil
}

func (s *RecipeStrategy) validateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if !s.CanHandle(u.Hostname()) {
		return fmt.Errorf("host not in recipe %s", s.cfg.Code)
	}
	pathLower := strings.ToLower(u.Path)
	for _, bad := range s.cfg.RejectPathContains {
		if bad != "" && strings.Contains(pathLower, strings.ToLower(bad)) {
			return fmt.Errorf("path contains rejected %q", bad)
		}
	}
	mode := strings.ToLower(strings.TrimSpace(s.cfg.PathKeyMode))
	if mode == "last_segment" {
		if s.pathProductID(raw) == "" {
			return fmt.Errorf("path %q has no product key", u.Path)
		}
		return nil
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

func (s *RecipeStrategy) currencyForURL(pageURL, extracted string) string {
	if c := strings.TrimSpace(extracted); c != "" {
		if _, err := currency.Parse(c); err == nil {
			return c
		}
	}
	u, err := url.Parse(pageURL)
	if err == nil {
		host := strings.ToLower(u.Hostname())
		host = strings.TrimPrefix(host, "www.")
		if s.cfg.CurrencyByPageHost != nil {
			if c, ok := s.cfg.CurrencyByPageHost[host]; ok && c != "" {
				return c
			}
		}
		path := strings.ToLower(u.Path)
		type kv struct{ k, v string }
		var pairs []kv
		for k, v := range s.cfg.CurrencyPathPrefix {
			pairs = append(pairs, kv{k, v})
		}
		for i := 0; i < len(pairs); i++ {
			for j := i + 1; j < len(pairs); j++ {
				if len(pairs[j].k) > len(pairs[i].k) {
					pairs[i], pairs[j] = pairs[j], pairs[i]
				}
			}
		}
		for _, p := range pairs {
			if p.k != "" && strings.HasPrefix(path, strings.ToLower(p.k)) {
				return p.v
			}
		}
		if strings.HasSuffix(host, ".ru") {
			return "RUB"
		}
		if strings.HasSuffix(host, ".kz") {
			return "KZT"
		}
	}
	if s.cfg.DefaultCurrency != "" {
		return s.cfg.DefaultCurrency
	}
	return "BYN"
}

func (s *RecipeStrategy) pathProductID(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	mode := strings.ToLower(strings.TrimSpace(s.cfg.PathKeyMode))
	if mode == "last_segment" {
		return lastSegmentKey(u.Path, s.cfg.RejectPathRoots, s.cfg.StripPathSuffixes)
	}
	if s.path == nil || s.cfg.PathIDGroup <= 0 {
		// still try group 1 if path matches and PathIDGroup unset but {id} needed
		if s.path != nil {
			m := s.path.FindStringSubmatch(u.Path)
			if len(m) >= 2 && s.cfg.PathIDGroup == 0 && strings.Contains(s.cfg.APIURLTemplate, "{id}") {
				return m[1]
			}
			if m != nil && s.cfg.PathIDGroup > 0 && s.cfg.PathIDGroup < len(m) {
				return m[s.cfg.PathIDGroup]
			}
		}
		return ""
	}
	m := s.path.FindStringSubmatch(u.Path)
	if m == nil || s.cfg.PathIDGroup >= len(m) {
		return ""
	}
	return m[s.cfg.PathIDGroup]
}

func lastSegmentKey(path string, rejectRoots, stripSuffixes []string) string {
	path = strings.Trim(path, "/")
	if path == "" {
		return ""
	}
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return ""
	}
	root := strings.ToLower(parts[0])
	for _, bad := range rejectRoots {
		if root == strings.ToLower(strings.TrimSpace(bad)) {
			return ""
		}
	}
	last := parts[len(parts)-1]
	for _, suf := range stripSuffixes {
		if strings.EqualFold(last, strings.TrimSpace(suf)) {
			if len(parts) < 3 {
				return ""
			}
			last = parts[len(parts)-2]
			break
		}
	}
	return last
}

func (s *RecipeStrategy) fetchURL(ctx context.Context, targetURL, accept string) (string, error) {
	ua := s.cfg.UserAgent
	if ua == "" {
		ua = "Mozilla/5.0 (compatible; Parser/1.0)"
	}
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", ua)
		req.Header.Set("Accept", accept)
		if al := strings.TrimSpace(s.cfg.AcceptLanguage); al != "" {
			req.Header.Set("Accept-Language", al)
		}
		resp, err := s.client.Do(req)
		if err != nil {
			last = err
			if fetchretry.Retryable(err) && attempt < 2 {
				_ = fetchretry.Wait(ctx, attempt+1, 0)
				continue
			}
			return "", err
		}
		b, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
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
