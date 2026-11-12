# geoip

Country lookup for public IP addresses using a local MaxMind GeoLite2 Country
or City database. The package never downloads data and does not resolve private,
reserved, documentation, loopback, or link-local addresses. Reserved ranges are
based on the IANA special-purpose address registries, which can change over time.

```go
resolver, err := geoip.Open("/var/lib/GeoIP/GeoLite2-Country.mmdb")
if err != nil { /* handle unavailable database */ }
defer resolver.Close()

country, found, err := resolver.LookupCountry(netip.MustParseAddr("8.8.8.8"))
```

Obtain the database directly from MaxMind after registering for GeoLite access.
Respect its license and keep the database and account credentials out of source
control. Refresh the database according to your deployment policy.

**Attribution:** This product includes GeoLite Data created by MaxMind,
available from https://www.maxmind.com.

The GeoIP reader is provided by [`github.com/oschwald/geoip2-golang/v2`](https://github.com/oschwald/geoip2-golang).
