package files

import "testing"

func TestCacheControlFor(t *testing.T) {
	if got := CacheControlFor(VisibilityPrivate); got != CacheControlPrivate {
		t.Fatalf("private cache = %q", got)
	}
	if got := CacheControlFor(VisibilityPublic); got != CacheControlPublic {
		t.Fatalf("public cache = %q", got)
	}
	if got := CacheControlFor(""); got != CacheControlPublic {
		t.Fatalf("empty cache = %q", got)
	}
}

func TestVisibilityNormalize(t *testing.T) {
	if Visibility("").Normalize() != VisibilityPublic {
		t.Fatal("empty visibility should be public")
	}
	if !VisibilityPrivate.IsValid() || Visibility("secret").IsValid() {
		t.Fatal("visibility validity")
	}
	if Purpose("").Normalize() != PurposeImage {
		t.Fatal("empty purpose should be image")
	}
}
