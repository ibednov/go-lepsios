package files

import (
	"context"
	"io"
)

type Adapter interface {
	Create(ctx context.Context, in CreateInput) error
	Get(ctx context.Context, in GetInput) (io.ReadCloser, error)
	Delete(ctx context.Context, in DeleteInput) error
	PublicURL(path string) string
}
