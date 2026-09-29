package ocrreceipts

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	dateRE = regexp.MustCompile(`\b(\d{2})[./](\d{2})[./](\d{2}|\d{4})\b`)
	// Full dates / date+time only (not money like 12.50).
	dateStampRE = regexp.MustCompile(`\b\d{2}\.\d{2}\.\d{2,4}(?:\s+\d{2}:\d{2}(?::\d{2})?)?\b|\b\d{2}\.\d{2}\s+\d{2}:\d{2}(?::\d{2})?\b`)
	amountRE    = regexp.MustCompile(`\d{1,3}(?:[ \x{00A0}]\d{3})*[.,]\d{2}|\d+[.,]\d{2}`)
)

func lastAmount(line string) (int64, bool) {
	line = confuseDigits(line)
	line = dateStampRE.ReplaceAllString(line, " ")
	// Tendered cash / change must not win over payable total.
	low := strings.ToLower(normalizeKey(line))
	if strings.Contains(low, "сдач") || strings.Contains(low, "внесен") {
		return 0, false
	}
	all := standaloneAmounts(line)
	if len(all) == 0 {
		return 0, false
	}
	raw := all[len(all)-1]
	cents, ok := parseAmount(raw)
	if !ok {
		return 0, false
	}
	if strings.Contains(raw, ".") && !strings.Contains(raw, ",") && isDottedDateFragment(raw) {
		return 0, false
	}
	return cents, true
}

func standaloneAmounts(line string) []string {
	idxs := amountRE.FindAllStringIndex(line, -1)
	if len(idxs) == 0 {
		return nil
	}
	out := make([]string, 0, len(idxs))
	for _, m := range idxs {
		start, end := m[0], m[1]
		if start > 0 {
			prev := line[start-1]
			if prev >= '0' && prev <= '9' {
				continue
			}
		}
		if end < len(line) {
			next := line[end]
			if next >= '0' && next <= '9' {
				continue
			}
		}
		out = append(out, line[start:end])
	}
	return out
}

func isDottedDateFragment(raw string) bool {
	parts := strings.Split(raw, ".")
	if len(parts) != 2 {
		return false
	}
	day, err1 := strconv.Atoi(parts[0])
	month, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return false
	}
	return day >= 1 && day <= 31 && month >= 1 && month <= 12
}

func confuseDigits(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case 'О', 'о', 'O', 'o', 'Ө', 'ө':
			b.WriteByte('0')
		case 'З', 'з':
			b.WriteByte('3')
		case 'б':
			b.WriteByte('6')
		case 'І', 'і', '|':
			b.WriteByte('1')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func parseAmount(raw string) (int64, bool) {
	raw = strings.ReplaceAll(raw, "\u00a0", "")
	raw = strings.ReplaceAll(raw, " ", "")
	raw = strings.ReplaceAll(raw, ",", ".")
	parts := strings.Split(raw, ".")
	if len(parts) != 2 || len(parts[1]) != 2 {
		return 0, false
	}
	rubles, err1 := strconv.ParseInt(parts[0], 10, 64)
	kops, err2 := strconv.ParseInt(parts[1], 10, 64)
	if err1 != nil || err2 != nil {
		return 0, false
	}
	return rubles*100 + kops, true
}

func normalizeKey(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		switch r {
		case '.', ',', ':', ';', '·', '•', '"', '\'', '«', '»':
			return ' '
		default:
			return r
		}
	}, s)
	return strings.Join(strings.Fields(s), " ")
}
