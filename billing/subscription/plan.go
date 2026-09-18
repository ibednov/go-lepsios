package subscription

import (
	"time"

	"github.com/ibednov/go-lepsios/currency"
)

// Visibility controls plan exposure (landing vs admin-only).
type Visibility string

const (
	VisibilityPublic   Visibility = "public"
	VisibilityPrivate  Visibility = "private"
	VisibilityDisabled Visibility = "disabled"
	VisibilityArchived Visibility = "archived"
)

func (v Visibility) IsValid() bool {
	switch v {
	case VisibilityPublic, VisibilityPrivate, VisibilityDisabled, VisibilityArchived:
		return true
	default:
		return false
	}
}

func (v Visibility) String() string { return string(v) }

// Plan is a reusable subscription plan definition (no ORM tags — product maps storage).
type Plan struct {
	ID            string
	Slug          string
	Name          string
	Visibility    Visibility
	PriceAmount   float64
	PriceCurrency currency.Code
	SortOrder     int
	Features      map[string]any
	CreatedAt     time.Time
}

// CreateInput is used when creating a plan.
type CreateInput struct {
	Slug          string
	Name          string
	Visibility    Visibility
	PriceAmount   float64
	PriceCurrency currency.Code
	SortOrder     int
	Features      map[string]any
}

// Patch is a partial plan update.
type Patch struct {
	Name          *string
	Visibility    *Visibility
	PriceAmount   *float64
	PriceCurrency *currency.Code
	SortOrder     *int
	Features      map[string]any
}
