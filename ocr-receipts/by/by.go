// Package by is the Belarus receipt strategy: cash slips, card slips, merchant aliases.
package by

import (
	"strings"

	ocrreceipts "github.com/ibednov/go-lepsios/ocr-receipts"
)

// Strategy implements ocrreceipts.Strategy for BY thermal and bank slips.
type Strategy struct{}

// New returns the Belarus strategy.
func New() ocrreceipts.Strategy { return Strategy{} }

func (Strategy) Code() string { return "by" }

func (Strategy) TotalScore(line string) int {
	if (Strategy{}).IsPromo(line) {
		return 0
	}
	l := norm(line)
	switch {
	case strings.Contains(l, "итого к оплате") || strings.Contains(l, "к оплате") ||
		strings.Contains(l, "к уплате") || strings.Contains(l, "к оплат"):
		return 3
	case strings.HasPrefix(l, "сумма") || strings.Contains(l, "сумма byn") ||
		(strings.Contains(l, "сумма") && strings.Contains(l, "byn")):
		return 3
	case strings.Contains(l, "итого") || strings.Contains(l, "итог") || strings.Contains(l, "total"):
		return 2
	case strings.Contains(l, "сумма по чеку"):
		return 2
	case strings.Contains(l, "сумма") && !strings.Contains(l, "скид") && !strings.Contains(l, "ндс"):
		return 1
	default:
		return 0
	}
}

func (Strategy) IsPromo(line string) bool {
	l := norm(line)
	for _, bad := range []string{"подароч", "акци", "купон", "скидк", "бонус", "на акционный"} {
		if strings.Contains(l, bad) {
			return true
		}
	}
	return false
}

func (Strategy) IsBankNoise(line string) bool {
	l := norm(line)
	for _, bad := range []string{
		"альфа банк", "альца", "беларусбанк", "беларус банк",
		"карт чек", "карт-чек", "терминал", "mastercard", "visa",
		"одобрено", "rrn", "aid", "бесконтакт", "для банка", "для клиента",
		"код авториз", "завершено успешно",
	} {
		if strings.Contains(l, bad) {
			return true
		}
	}
	return false
}

func (Strategy) IsTitleReject(line string) bool {
	l := norm(line)
	for _, bad := range []string{
		"кассир", "фн", "фд", "унп", "безнал", "чек", "карт", "рубл", "ндс",
		"платеж", "документ", "наличн", "итог", "сумма", "сдача", "оплата",
		"кассир", "тел", "уид", "скко", "благодар", "спасибо",
	} {
		if strings.Contains(l, bad) {
			return true
		}
	}
	return false
}

func (Strategy) CanonicalTitle(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.Trim(raw, `"'«»„“`)
	n := norm(raw)
	return applyAlias(n, raw)
}

func norm(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		switch r {
		case '.', ',', ':', ';', '·', '•', '"', '\'', '«', '»', '*', '#', '_':
			return ' '
		default:
			return r
		}
	}, s)
	return strings.Join(strings.Fields(s), " ")
}
