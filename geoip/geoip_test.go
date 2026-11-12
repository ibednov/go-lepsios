package geoip

import (
	"net/netip"
	"testing"

	"github.com/oschwald/geoip2-golang/v2"
)

type fakeDatabase struct{ calls int }

func (f *fakeDatabase) Country(ip netip.Addr) (*geoip2.Country, error) {
	f.calls++
	return &geoip2.Country{Country: geoip2.CountryRecord{ISOCode: "us", Names: geoip2.Names{English: "United States"}}}, nil
}
func (*fakeDatabase) Close() error { return nil }

func TestLookupCountryNormalizesPublicAddress(t *testing.T) {
	db := &fakeDatabase{}
	country, ok, err := (&Resolver{db: db}).LookupCountry(netip.MustParseAddr("::ffff:8.8.8.8"))
	if err != nil || !ok || country.ISOCode != "US" || country.Name != "United States" {
		t.Fatalf("LookupCountry() = %#v, %v, %v", country, ok, err)
	}
	if db.calls != 1 {
		t.Fatalf("database calls = %d, want 1", db.calls)
	}
}

func TestLookupCountrySkipsNonPublicAddresses(t *testing.T) {
	for _, input := range []string{"::1", "127.0.0.1", "10.0.0.1", "169.254.1.1", "192.0.2.1", ""} {
		t.Run(input, func(t *testing.T) {
			db := &fakeDatabase{}
			ip, _ := netip.ParseAddr(input)
			country, ok, err := (&Resolver{db: db}).LookupCountry(ip)
			if err != nil || ok || country != (Country{}) || db.calls != 0 {
				t.Fatalf("LookupCountry() = %#v, %v, %v; calls=%d", country, ok, err, db.calls)
			}
		})
	}
}

func TestOpenRejectsEmptyPath(t *testing.T) {
	if _, err := Open(" "); err == nil {
		t.Fatal("Open() accepted an empty path")
	}
}
