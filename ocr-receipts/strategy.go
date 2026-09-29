package ocrreceipts

// Strategy is a locale-specific receipt reader (keywords, promos, merchant aliases).
type Strategy interface {
	// Code is a short locale id, e.g. "by".
	Code() string
	// TotalScore ranks a line as a payable-total anchor. Higher wins.
	// Typical: 3 = к оплате / СУММА:, 2 = ИТОГ, 1 = сумма (weak).
	TotalScore(line string) int
	// IsPromo is true for gift-card / discount noise that must not become title or total.
	IsPromo(line string) bool
	// IsBankNoise is true for bank/terminal chrome that is not the merchant name.
	IsBankNoise(line string) bool
	// IsTitleReject is true for lines that must never be the merchant title.
	IsTitleReject(line string) bool
	// CanonicalTitle maps OCR merchant text to a stable display name (aliases).
	CanonicalTitle(raw string) string
}
