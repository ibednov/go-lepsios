package files

const (
	CacheControlPublic  = "public, max-age=86400"
	CacheControlPrivate = "private, no-store"
)

// CacheControlFor returns adapter metadata that any strategy can honor.
// Private objects must never be advertised via PublicURL.
func CacheControlFor(v Visibility) string {
	if v.Normalize() == VisibilityPrivate {
		return CacheControlPrivate
	}
	return CacheControlPublic
}
