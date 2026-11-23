package crawler

import (
	"context"
	"errors"
	"fmt"
	"net"

	crawlerr "github.com/ibednov/go-lepsios/crawler/errors"
	"github.com/ibednov/go-lepsios/crawler/httpclient"
)

// Outcome describes what a universal parser established about a catalog or product.
type Outcome string

const (
	OutcomeOK             Outcome = "ok"
	OutcomeConfirmedEmpty Outcome = "confirmed_empty"
	OutcomeFiltered Outcome = "filtered"
	OutcomeDiscontinued   Outcome = "discontinued"
	OutcomeSkipped        Outcome = "skipped"
	OutcomeParseFailed    Outcome = "parse_failed"
	OutcomeBlocked        Outcome = "blocked"
	OutcomeTransient      Outcome = "transient"
	OutcomeTruncated      Outcome = "truncated"
)

type DiscoveryResult struct {
	Refs     []ProductRef
	Outcome  Outcome
	Evidence string
}

type FetchResult struct {
	Offers  []RawOffer
	Outcome Outcome
	Reason  string
}

// OutcomeAdapter is the opt-in extended contract. Evidence is required before
// an implementation may return OutcomeConfirmedEmpty.
type OutcomeAdapter interface {
	DiscoverResult(context.Context) (DiscoveryResult, error)
	FetchResult(context.Context, ProductRef) (FetchResult, error)
}

// DiscoverResult calls the extended contract when available and safely adapts
// adapters that only implement the base contract. An empty discovery is never
// evidence of an empty shop without explicit confirmation.
func DiscoverResultFor(ctx context.Context, adapter Adapter) (DiscoveryResult, error) {
	if extended, ok := adapter.(OutcomeAdapter); ok {
		result, err := extended.DiscoverResult(ctx)
		if err != nil {
			return result, err
		}
		if err := validateDiscoveryResult(result); err != nil {
			return DiscoveryResult{Outcome: OutcomeParseFailed}, crawlerr.ParseFailed(adapter.Code(), err)
		}
		return result, nil
	}
	refs, err := adapter.Discover(ctx)
	if err != nil {
		return DiscoveryResult{Outcome: OutcomeForError(err)}, err
	}
	if len(refs) == 0 {
		err := crawlerr.ParseFailed(adapter.Code(), errors.New("adapter returned no product references without empty-catalog evidence"))
		return DiscoveryResult{Outcome: OutcomeParseFailed}, err
	}
	return DiscoveryResult{Refs: refs, Outcome: OutcomeOK}, nil
}

// FetchResultFor calls the extended contract when available. An empty legacy
// Fetch result is treated as an intentional skip, not a successful page.
func FetchResultFor(ctx context.Context, adapter Adapter, ref ProductRef) (FetchResult, error) {
	if extended, ok := adapter.(OutcomeAdapter); ok {
		result, err := extended.FetchResult(ctx, ref)
		if err != nil {
			return result, err
		}
		if err := validateFetchResult(result); err != nil {
			return FetchResult{Outcome: OutcomeParseFailed}, crawlerr.ParseFailed(adapter.Code(), err)
		}
		if result.Outcome == OutcomeFiltered || result.Outcome == OutcomeDiscontinued {
			return result, crawlerr.SkippedProduct(adapter.Code(), errors.New(result.Reason))
		}
		if result.Outcome == OutcomeSkipped {
			return result, crawlerr.SkippedProduct(adapter.Code(), errors.New(result.Reason))
		}
		return result, nil
	}
	offers, err := adapter.Fetch(ctx, ref)
	if err != nil {
		return FetchResult{Outcome: OutcomeForError(err)}, err
	}
	if len(offers) == 0 {
		err := crawlerr.SkippedProduct(adapter.Code(), errors.New("adapter returned no priced offers"))
		return FetchResult{Outcome: OutcomeSkipped, Reason: "no priced offers"}, err
	}
	return FetchResult{Offers: offers, Outcome: OutcomeOK}, nil
}

func OutcomeForError(err error) Outcome {
	if errors.Is(err, crawlerr.ErrSkippedProduct) {
		return OutcomeSkipped
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return OutcomeTransient
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return OutcomeTransient
	}
	var challengeErr *httpclient.ChallengeError
	if errors.As(err, &challengeErr) {
		return OutcomeBlocked
	}
	var bodyErr *httpclient.BodyTooLargeError
	if errors.As(err, &bodyErr) {
		return OutcomeTruncated
	}
	var readErr *httpclient.BodyReadError
	if errors.As(err, &readErr) {
		return OutcomeTruncated
	}
	var statusErr *httpclient.HTTPStatusError
	if errors.As(err, &statusErr) {
		if statusErr.Status == 401 || statusErr.Status == 403 {
			return OutcomeBlocked
		}
		if statusErr.Status == 429 || statusErr.Status >= 500 {
			return OutcomeTransient
		}
	}
	var crawlError *crawlerr.CrawlError
	if errors.As(err, &crawlError) {
		switch crawlError.Code {
		case "blocked":
			return OutcomeBlocked
		case "transient":
			return OutcomeTransient
		case "parse", "currency":
			return OutcomeParseFailed
		}
	}
	return OutcomeParseFailed
}

// ReasonForError maps parser failures to bounded diagnostic codes suitable
// for persistence and metrics. It never includes URL, SKU, or raw error text.
func ReasonForError(err error) string {
	if err == nil {
		return "none"
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}
	var challengeErr *httpclient.ChallengeError
	if errors.As(err, &challengeErr) {
		return "challenge"
	}
	var bodyErr *httpclient.BodyTooLargeError
	var bodyReadErr *httpclient.BodyReadError
	if errors.As(err, &bodyErr) || errors.As(err, &bodyReadErr) {
		return "truncated"
	}
	var statusErr *httpclient.HTTPStatusError
	if errors.As(err, &statusErr) {
		switch {
		case statusErr.Status == 401 || statusErr.Status == 403:
			return "blocked"
		case statusErr.Status == 429:
			return "rate_limited"
		case statusErr.Status >= 500 && statusErr.Status <= 599:
			return "server_error"
		case statusErr.Status == 404:
			return "not_found"
		case statusErr.Status >= 300 && statusErr.Status <= 399:
			return "redirect"
		case statusErr.Status >= 400 && statusErr.Status <= 499:
			return "client_error"
		}
	}
	var crawlError *crawlerr.CrawlError
	if errors.As(err, &crawlError) {
		switch crawlError.Code {
		case "blocked":
			return "blocked"
		case "transient":
			return "transient"
		case "parse", "currency":
			return "parse"
		}
	}
	return "transport"
}

func validateDiscoveryResult(result DiscoveryResult) error {
	switch result.Outcome {
	case OutcomeOK:
		if len(result.Refs) == 0 {
			return errors.New("ok discovery must contain product references")
		}
	case OutcomeConfirmedEmpty:
		if len(result.Refs) != 0 || result.Evidence == "" {
			return errors.New("confirmed_empty discovery requires no references and explicit evidence")
		}
	default:
		return fmt.Errorf("unsupported discovery outcome %q", result.Outcome)
	}
	return nil
}

func validateFetchResult(result FetchResult) error {
	switch result.Outcome {
	case OutcomeOK:
		if len(result.Offers) == 0 {
			return errors.New("ok fetch must contain offers")
		}
	case OutcomeFiltered, OutcomeDiscontinued, OutcomeSkipped:
		if len(result.Offers) != 0 || result.Reason == "" {
			return fmt.Errorf("%s fetch requires no offers and an explicit reason", result.Outcome)
		}
	default:
		return fmt.Errorf("unsupported fetch outcome %q", result.Outcome)
	}
	return nil
}
