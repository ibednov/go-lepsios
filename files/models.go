package files

import "io"

type CreateInput struct {
	Path   string
	Reader io.Reader

	// ContentType and CacheControl set optional metadata on object storage.
	// Empty values preserve each adapter's existing defaults.
	ContentType  string
	CacheControl string
}

type GetInput struct {
	Path string
}

type DeleteInput struct {
	Path string
}
