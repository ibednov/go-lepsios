package crawler

import (
	"context"
	"errors"
	"testing"

	crawlerr "github.com/ibednov/go-lepsios/crawler/errors"
	"github.com/ibednov/go-lepsios/crawler/httpclient"
	"github.com/shopspring/decimal"
)

type outcomeTestAdapter struct {
	refs   []ProductRef
	offers []RawOffer
}

func (a outcomeTestAdapter) Code() string { return "test" }
func (a outcomeTestAdapter) Discover(context.Context) ([]ProductRef, error) {
	return a.refs, nil
}
func (a outcomeTestAdapter) Fetch(context.Context, ProductRef) ([]RawOffer, error) {
	return a.offers, nil
}

func TestLegacyEmptyResultsAreNotConfirmedEmpty(t *testing.T) {
	adapter := outcomeTestAdapter{}
	if result, err := DiscoverResultFor(context.Background(), adapter); !errors.As(err, new(*crawlerr.CrawlError)) || result.Outcome != OutcomeParseFailed {
		t.Fatalf("empty discovery result=%+v err=%v, want parse failure", result, err)
	}
	if result, err := FetchResultFor(context.Background(), adapter, ProductRef{URL: "https://shop/product"}); !errors.Is(err, crawlerr.ErrSkippedProduct) || result.Outcome != OutcomeSkipped {
		t.Fatalf("empty fetch result=%+v err=%v, want skip", result, err)
	}
}

func TestOutcomeContractRequiresEvidenceForEmptyAndOffersForOK(t *testing.T) {
	if err := validateDiscoveryResult(DiscoveryResult{Outcome: OutcomeConfirmedEmpty}); err == nil {
		t.Fatal("confirmed_empty without evidence was accepted")
	}
	if err := validateDiscoveryResult(DiscoveryResult{Outcome: OutcomeConfirmedEmpty, Evidence: "valid explicit empty marker"}); err != nil {
		t.Fatalf("confirmed empty with evidence: %v", err)
	}
	if err := validateFetchResult(FetchResult{Outcome: OutcomeOK}); err == nil {
		t.Fatal("empty successful fetch was accepted")
	}
	if err := validateFetchResult(FetchResult{Outcome: OutcomeFiltered, Reason: "category: teaware"}); err != nil {
		t.Fatalf("explicit filtered result: %v", err)
	}
}

func TestLegacyOffersKeepOKOutcome(t *testing.T) {
	adapter := outcomeTestAdapter{refs: []ProductRef{{URL: "https://shop/product"}}, offers: []RawOffer{{Price: decimal.NewFromInt(12)}}}
	result, err := FetchResultFor(context.Background(), adapter, adapter.refs[0])
	if err != nil || result.Outcome != OutcomeOK || len(result.Offers) != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestOutcomeForHTTPFailures(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want Outcome
	}{
		{err: httpclient.StatusError(403, "https://shop.test/p?token=secret"), want: OutcomeBlocked},
		{err: httpclient.StatusError(429, "https://shop.test/p"), want: OutcomeTransient},
		{err: httpclient.StatusError(503, "https://shop.test/p"), want: OutcomeTransient},
		{err: &httpclient.BodyTooLargeError{Limit: 8, URL: "https://shop.test/p"}, want: OutcomeTruncated},
		{err: &httpclient.BodyReadError{URL: "https://shop.test/p", BytesRead: 12, Err: errors.New("unexpected EOF")}, want: OutcomeTruncated},
	} {
		if got := OutcomeForError(tc.err); got != tc.want {
			t.Errorf("OutcomeForError(%v)=%q, want %q", tc.err, got, tc.want)
		}
	}
}

func TestReasonForErrorUsesStableLowCardinalityCodes(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{err: httpclient.StatusError(403, "https://shop.test/p?token=secret"), want: "blocked"},
		{err: httpclient.StatusError(429, "https://shop.test/p"), want: "rate_limited"},
		{err: httpclient.StatusError(503, "https://shop.test/p"), want: "server_error"},
		{err: httpclient.StatusError(302, "https://shop.test/p"), want: "redirect"},
		{err: &httpclient.ChallengeError{Status: 200, URL: "https://shop.test/p"}, want: "challenge"},
		{err: context.DeadlineExceeded, want: "timeout"},
		{err: &httpclient.BodyTooLargeError{Limit: 8, URL: "https://shop.test/p"}, want: "truncated"},
		{err: &httpclient.BodyReadError{URL: "https://shop.test/p", BytesRead: 12, Err: errors.New("unexpected EOF")}, want: "truncated"},
	} {
		if got := ReasonForError(tc.err); got != tc.want {
			t.Errorf("ReasonForError(%T)=%q, want %q", tc.err, got, tc.want)
		}
	}
}
