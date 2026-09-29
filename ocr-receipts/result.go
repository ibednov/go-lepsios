package ocrreceipts

import "time"

// Result is a confirmed-looking merchant, payable total and date from a slip.
type Result struct {
	Title       string
	AmountCents int64
	Day         time.Time
	Kind        Kind // cash, card, or unknown
}

// Kind is the slip family detected under a locale strategy.
type Kind string

const (
	KindUnknown Kind = ""
	KindCash    Kind = "cash" // ИТОГ / Итого к оплате
	KindCard    Kind = "card" // СУММА: on a bank terminal slip
)
