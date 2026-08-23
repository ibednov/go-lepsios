package exchange

import (
	"strings"
)

type Registry struct {
	providers []Provider
}

func NewRegistry(providers ...Provider) *Registry {
	return &Registry{providers: providers}
}

func (r *Registry) Get(country string) (Provider, error) {
	code := strings.ToUpper(strings.TrimSpace(country))
	for _, p := range r.providers {
		for _, c := range p.Countries() {
			if strings.ToUpper(strings.TrimSpace(c)) == code {
				return p, nil
			}
		}
	}
	return nil, ErrProviderNotFound
}
