package storeapi

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	entity "github.com/ibednov/go-lepsios/shopparser"
	crawlerr "github.com/ibednov/go-lepsios/shopparser/errors"
	"github.com/ibednov/go-lepsios/shopparser/httpclient"
	"github.com/ibednov/go-lepsios/shopparser/pricing"
	"github.com/shopspring/decimal"
)

const Code = "woocommerce_store_api"

// Shared WooCommerce Store API and HTML variation extraction.
type Adapter struct {
	cli          *httpclient.Client
	base         string
	filter       CategoryFilter
	diagnosticMu sync.Mutex
	diagnostic   *httpclient.DiagnosticResponse
}

// CategoryFilter controls Discover include/exclude selection.
type CategoryFilter struct {
	NameContains    []string // include if category name contains any (case-fold)
	IDs             []int    // include if category id listed
	ExcludeContains []string // exclude if category name contains any
}

func defaultFilter() CategoryFilter {
	// empty = accept all products; product-specific keywords come from adapter config
	return CategoryFilter{}
}

// NewWithBaseURL is for tests (httptest) and mirrors.
func NewWithBaseURL(cli *httpclient.Client, baseURL string) *Adapter {
	return &Adapter{
		cli:    cli,
		base:   stringsTrimRightSlash(baseURL),
		filter: defaultFilter(),
	}
}

// WithFilter overrides category include/exclude rules.
func (a *Adapter) WithFilter(f CategoryFilter) *Adapter {
	if len(f.NameContains) == 0 && len(f.IDs) == 0 && len(f.ExcludeContains) == 0 {
		return a
	}
	a.filter = f
	return a
}

func (a *Adapter) Code() string { return Code }

func (a *Adapter) Discover(ctx context.Context) ([]entity.ProductRef, error) {
	a.resetDiagnostic()
	var out []entity.ProductRef
	page := 1
	total := -1
	for {
		body, resp, err := a.fetchProductsPage(ctx, page)
		if err != nil {
			return nil, classifyHTTPError(err)
		}
		if resp.StatusCode != 200 {
			return nil, classifyHTTPError(httpclient.StatusError(resp.StatusCode, fmt.Sprintf("%s/wp-json/wc/store/v1/products?page=%d", a.base, page)))
		}
		if total < 0 {
			if v := resp.Header.Get("X-WP-Total"); v != "" {
				if n, err := strconv.Atoi(v); err == nil {
					total = n
				}
			}
		}
		products, err := parseStoreProducts(body)
		if err != nil {
			return nil, crawlerr.ParseFailed(Code, err)
		}
		if len(products) == 0 {
			break
		}
		for _, p := range products {
			if !productAllowed(p, a.filter) {
				continue
			}
			out = append(out, entity.ProductRef{
				SourceCode: Code,
				URL:        p.Permalink,
				SourceID:   strconv.Itoa(p.ID),
			})
		}
		if total >= 0 && page*100 >= total {
			break
		}
		if len(products) < 100 {
			break
		}
		page++
	}
	if len(out) == 0 {
		return nil, crawlerr.ParseFailed(Code, fmt.Errorf("empty product discovery result"))
	}
	return out, nil
}

func (a *Adapter) Fetch(ctx context.Context, ref entity.ProductRef) ([]entity.RawOffer, error) {
	a.resetDiagnostic()
	if ref.SourceID == "" && ref.URL == "" {
		return nil, crawlerr.ParseFailed(Code, fmt.Errorf("empty product ref"))
	}
	if ref.URL != "" && !sameOrigin(a.base, ref.URL) {
		return nil, crawlerr.ParseFailed(Code, fmt.Errorf("product URL is outside configured source origin"))
	}
	var (
		p   storeProduct
		err error
	)
	if ref.SourceID != "" {
		p, err = a.fetchProductByID(ctx, ref.SourceID)
	} else {
		p, err = a.fetchProductByPermalink(ctx, ref.URL)
	}
	if err != nil {
		var crawlError *crawlerr.CrawlError
		if errors.As(err, &crawlError) {
			return nil, err
		}
		return nil, classifyHTTPError(err)
	}
	if p.Permalink != "" && !sameOrigin(a.base, p.Permalink) {
		return nil, crawlerr.ParseFailed(Code, fmt.Errorf("product permalink is outside configured source origin"))
	}
	observed := time.Now().UTC()
	if isVariable(p) {
		permalink := p.Permalink
		if permalink == "" {
			permalink = ref.URL
		}
		htmlBody, err := a.fetchHTML(ctx, permalink)
		if err != nil {
			return nil, classifyHTTPError(err)
		}
		vars, err := parseProductVariationsHTML(htmlBody)
		if err != nil {
			return nil, crawlerr.ParseFailed(Code, err)
		}
		return variationsToOffers(p, vars, observed)
	}
	return simpleToOffers(p, observed)
}

func simpleToOffers(p storeProduct, observed time.Time) ([]entity.RawOffer, error) {
	cur, err := pricing.NormalizeCurrency(p.Prices.CurrencyCode)
	if err != nil {
		return nil, crawlerr.ParseFailed(Code, err)
	}
	price, err := pricing.PriceFromMinorUnits(p.Prices.Price, p.Prices.CurrencyMinorUnit)
	if err != nil {
		return nil, crawlerr.ParseFailed(Code, err)
	}
	if !price.IsPositive() {
		return nil, crawlerr.SkippedProduct(Code, fmt.Errorf("product price must be positive"))
	}
	img := ""
	if len(p.Images) > 0 {
		img = p.Images[0].Src
	}
	w := weightFromStoreProduct(p)
	offer := entity.RawOffer{
		Source:     Code,
		URL:        p.Permalink,
		SourceSKU:  p.SKU,
		Title:      p.Name,
		Price:      price,
		Currency:   cur,
		WeightG:    w,
		UnitHint:   unitHint(w),
		InStock:    p.IsInStock,
		ImageURL:   img,
		ObservedAt: observed,
		Attrs:      map[string]string{"product_type": p.Type},
	}
	return []entity.RawOffer{offer}, nil
}

func variationsToOffers(p storeProduct, vars []wooVariation, observed time.Time) ([]entity.RawOffer, error) {
	out := make([]entity.RawOffer, 0, len(vars))
	invalidPrice := false
	for _, v := range vars {
		offer, ok := variationToOffer(p, v, observed)
		if ok {
			out = append(out, offer)
		} else if !decimalFromFloat(v.DisplayPrice).IsPositive() && v.IsInStock {
			invalidPrice = true
		}
	}
	if len(out) == 0 {
		if invalidPrice {
			return nil, crawlerr.SkippedProduct(Code, fmt.Errorf("in-stock variation has nonpositive price"))
		}
		return nil, crawlerr.SkippedProduct(Code, crawlerr.ErrEmptyOffers)
	}
	return out, nil
}

func variationToOffer(p storeProduct, v wooVariation, observed time.Time) (entity.RawOffer, bool) {
	// Ignore «Цена за кг» — never use it as price; display_price is source of truth.
	price := decimalFromFloat(v.DisplayPrice)
	if !price.IsPositive() && v.IsInStock {
		return entity.RawOffer{}, false
	}
	w, absent := weightFromVariation(v)
	inStock := v.IsInStock && !absent
	if absent {
		w = nil
		inStock = false
	}
	img := ""
	if v.Image != nil && v.Image.URL != "" {
		img = v.Image.URL
	} else if len(p.Images) > 0 {
		img = p.Images[0].Src
	}
	sku := v.SKU
	if sku == "" {
		sku = p.SKU
	}
	var grams *decimal.Decimal
	if w != nil {
		value := decimal.NewFromInt(int64(*w))
		grams = &value
	}
	url := p.Permalink
	return entity.RawOffer{
		Source:      Code,
		URL:         url,
		SourceSKU:   sku,
		Title:       p.Name,
		Price:       price,
		Currency:    "BYN",
		WeightG:     w,
		WeightGrams: grams,
		UnitHint:    unitHint(w),
		InStock:     inStock,
		ImageURL:    img,
		ObservedAt:  observed,
		Attrs:       variationAttrs(v),
	}, true
}

func unitHint(w *int) entity.UnitHint {
	if w == nil || *w <= 0 {
		return entity.UnitUnknown
	}
	return entity.UnitG
}

func stringsTrimRightSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
