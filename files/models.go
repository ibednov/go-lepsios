package files

import "io"

type CreateInput struct {
	Path   string
	Reader io.Reader

	// ContentType and CacheControl set optional metadata on object storage.
	// Empty values preserve each adapter's existing defaults.
	ContentType  string
	CacheControl string

	// Visibility is storage-agnostic. Adapters keep using CacheControl;
	// callers should set CacheControl via CacheControlFor(Visibility).
	Visibility Visibility
}

type GetInput struct {
	Path string
}

type DeleteInput struct {
	Path string
}
