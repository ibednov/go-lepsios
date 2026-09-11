// Package money parses and formats money amounts (stored as integer kopecks/cents).
package money

import (
	"fmt"
	"strings"
)

// Parse parses a user-entered ruble amount into cents.
// Supported: "450", "450.50", "450,50", "1 230", "1 230.55".
func Parse(raw string) (int64, error) {
	s := strings.TrimSpace(raw)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "\u00a0", "")
	if s == "" {
		return 0, fmt.Errorf("пустая сумма")
	}

	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = strings.TrimPrefix(s, "-")
	}

	s = strings.Replace(s, ",", ".", 1)
	if strings.Count(s, ".") > 1 {
		return 0, fmt.Errorf("неверный формат суммы")
	}

	rubles, kops := s, ""
	if idx := strings.Index(s, "."); idx >= 0 {
		rubles, kops = s[:idx], s[idx+1:]
	}
	if rubles == "" {
		rubles = "0"
	}
	for _, r := range rubles {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("неверный формат суммы")
		}
	}
	switch len(kops) {
	case 0:
	case 1:
		kops += "0"
	case 2:
	default:
		return 0, fmt.Errorf("не более двух знаков после запятой")
	}
	for _, r := range kops {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("неверный формат суммы")
		}
	}

	var total int64
	_, _ = fmt.Sscanf(rubles, "%d", &total)
	cents := total * 100
	if kops != "" {
		var k int64
		_, _ = fmt.Sscanf(kops, "%d", &k)
		cents += k
	}
	if cents <= 0 {
		return 0, fmt.Errorf("сумма должна быть больше нуля")
	}
	if neg {
		cents = -cents
	}
	return cents, nil
}

// Format renders cents as "12 400р" or "450,50р" (BYN shorthand).
func Format(cents int64) string {
	return FormatCur(cents, "BYN")
}

// FormatCur renders cents with a currency suffix (BYN → "р", else ISO code).
func FormatCur(cents int64, code string) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	rubles := cents / 100
	kops := cents % 100

	digits := fmt.Sprintf("%d", rubles)
	var b strings.Builder
	for i, d := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(d)
	}

	suffix := "р"
	if code != "" && code != "BYN" {
		suffix = " " + code
	}
	if kops == 0 {
		return fmt.Sprintf("%s%s%s", sign, b.String(), suffix)
	}
	return fmt.Sprintf("%s%s,%02d%s", sign, b.String(), kops, suffix)
}
