// Package geoip resolves public IP addresses using a locally supplied MaxMind
// GeoLite2 database.
//
// This product includes GeoLite Data created by MaxMind, available from
// https://www.maxmind.com.
package geoip

import (
	"fmt"
	"net/netip"
	"strings"

	"github.com/oschwald/geoip2-golang/v2"
)

var nonPublicNetworks = mustPrefixes(
	"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24",
	"198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4",
	"2001:db8::/32", "2001::/23", "2002::/16", "3fff::/20",
)

func mustPrefixes(values ...string) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		prefixes = append(prefixes, netip.MustParsePrefix(value))
	}
	return prefixes
}

// Country is the country record associated with an IP address.
type Country struct {
	ISOCode string
	Name    string
}

// Resolver reads country data from a MaxMind MMDB database.
type Resolver struct{ db countryDatabase }

type countryDatabase interface {
	Country(netip.Addr) (*geoip2.Country, error)
	Close() error
}

// Open opens a GeoLite2 Country or City database at path.
func Open(path string) (*Resolver, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("geoip database path is empty")
	}
	db, err := geoip2.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open geoip database: %w", err)
	}
	return &Resolver{db: db}, nil
}

// LookupCountry returns the country for a public IP. Private, reserved,
// loopback, and invalid addresses have no result.
func (r *Resolver) LookupCountry(ip netip.Addr) (Country, bool, error) {
	if r == nil || r.db == nil || !ip.IsValid() {
		return Country{}, false, nil
	}
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsInterfaceLocalMulticast() || ip.IsLinkLocalMulticast() {
		return Country{}, false, nil
	}
	for _, prefix := range nonPublicNetworks {
		if prefix.Contains(ip) {
			return Country{}, false, nil
		}
	}
	record, err := r.db.Country(ip)
	if err != nil {
		return Country{}, false, fmt.Errorf("lookup geoip country: %w", err)
	}
	code := strings.ToUpper(strings.TrimSpace(record.Country.ISOCode))
	if code == "" {
		return Country{}, false, nil
	}
	name := record.Country.Names.English
	if name == "" {
		name = record.RegisteredCountry.Names.English
	}
	return Country{ISOCode: code, Name: name}, true, nil
}

// Close releases the database resources.
func (r *Resolver) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}
