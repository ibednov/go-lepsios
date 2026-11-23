package productparse

import "context"

type Strategy interface {
	CanHandle(host string) bool
	Parse(ctx context.Context, normalizedURL string) (RawParseResult, error)
}

// CardParser — быстрый парсинг карточки без поиска изображения.
type CardParser interface {
	ParseCard(ctx context.Context, normalizedURL string) (RawParseResult, error)
}

// ProductImageParser — внешний URL превью товара (тяжёлый I/O).
type ProductImageParser interface {
	ParseProductImageURL(ctx context.Context, normalizedURL string) (*string, error)
}
