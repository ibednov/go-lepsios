package shopparser

import "context"

// Adapter discovers catalog URLs and fetches per-variant raw offers for one shop.
type Adapter interface {
	Code() string
	Discover(ctx context.Context) ([]ProductRef, error)
	Fetch(ctx context.Context, ref ProductRef) ([]RawOffer, error)
}
