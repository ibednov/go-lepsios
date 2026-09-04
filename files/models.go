package files

import "io"

type CreateInput struct {
	Path   string
	Reader io.Reader
}

type GetInput struct {
	Path string
}

type DeleteInput struct {
	Path string
}
