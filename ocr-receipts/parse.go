package ocrreceipts

import (
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Parse reads merchant, payable total and date from OCR text using a locale strategy.
func Parse(text string, s Strategy, loc *time.Location) Result {
	if s == nil {
		return Result{Title: "Чек"}
	}
	if loc == nil {
		loc = time.UTC
	}
	lines := splitLines(text)
	var out Result
	bestScore := 0
	for i, line := range lines {
		if out.Day.IsZero() {
			if day, ok := findDate(line, loc); ok {
				out.Day = day
			}
		}
		if out.Title == "" && i < 12 && looksLikeTitle(line, s) {
			out.Title = s.CanonicalTitle(cleanTitle(line))
		}
		score := s.TotalScore(line)
		if score == 0 || score < bestScore {
			continue
		}
		cents, ok := amountNear(lines, i, s)
		if !ok {
			continue
		}
		bestScore = score
		out.AmountCents = cents
		out.Kind = kindFromScore(score, line)
	}
	if out.AmountCents == 0 {
		out.AmountCents = fallbackAmount(lines, s)
	}
	if out.Title == "" {
		// Second pass: ООО / ИП lines anywhere in the header half.
		for i, line := range lines {
			if i > len(lines)/2 {
				break
			}
			if merchant, ok := companyName(line); ok && !s.IsBankNoise(line) && !s.IsPromo(line) {
				out.Title = s.CanonicalTitle(merchant)
				break
			}
		}
	}
	if out.Title == "" {
		out.Title = "Чек"
	}
	if out.Kind == KindUnknown && out.AmountCents > 0 {
		out.Kind = KindCash
	}
	return out
}

func kindFromScore(score int, line string) Kind {
	l := normalizeKey(line)
	if strings.Contains(l, "сумма") && (strings.Contains(l, "byn") || strings.HasPrefix(l, "сумма")) {
		return KindCard
	}
	if score >= 2 {
		return KindCash
	}
	return KindUnknown
}

func splitLines(text string) []string {
	raw := strings.Split(text, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func fallbackAmount(lines []string, s Strategy) int64 {
	itogAt := -1
	for i, line := range lines {
		if s.TotalScore(line) >= 2 {
			itogAt = i
		}
	}
	if itogAt >= 0 {
		var best int64
		for i := itogAt; i < len(lines); i++ {
			if s.IsPromo(lines[i]) {
				continue
			}
			if cents, ok := lastAmount(lines[i]); ok && cents > best {
				best = cents
			}
		}
		if best > 0 {
			return best
		}
	}
	start := len(lines) * 2 / 3
	if start < 0 {
		start = 0
	}
	freq := map[int64]int{}
	var best int64
	var bestN int
	for _, line := range lines[start:] {
		if s.IsPromo(line) {
			continue
		}
		cents, ok := lastAmount(line)
		if !ok || cents < 100 {
			continue
		}
		freq[cents]++
		if freq[cents] > bestN || (freq[cents] == bestN && cents > best) {
			bestN = freq[cents]
			best = cents
		}
	}
	if bestN >= 2 {
		return best
	}
	return best
}

func amountNear(lines []string, i int, s Strategy) (int64, bool) {
	// First payable amount after the anchor. Stop before cash-tendered / change.
	for j := i; j < len(lines) && j <= i+4; j++ {
		low := normalizeKey(lines[j])
		if strings.Contains(low, "налич") || strings.Contains(low, "сдач") ||
			strings.Contains(low, "внесен") || strings.Contains(low, "оплачено") {
			break
		}
		if s.IsPromo(lines[j]) {
			continue
		}
		if cents, ok := lastAmount(lines[j]); ok {
			return cents, true
		}
	}
	return 0, false
}

func cleanTitle(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `"'«»„“`)
	return s
}

func looksLikeTitle(line string, s Strategy) bool {
	if s.IsPromo(line) || s.IsBankNoise(line) || s.TotalScore(line) > 0 || s.IsTitleReject(line) {
		return false
	}
	if merchant, ok := companyName(line); ok {
		_ = merchant
		return true
	}
	if _, ok := lastAmount(line); ok && letterCount(line) < 6 {
		return false
	}
	return letterCount(line) >= 3
}

func companyName(line string) (string, bool) {
	l := normalizeKey(line)
	if strings.Contains(l, "ооо") || strings.Contains(l, "зао") || strings.Contains(l, "ип ") || strings.HasPrefix(l, "ип") {
		return cleanTitle(line), true
	}
	if strings.Contains(l, "магазин") {
		return cleanTitle(line), true
	}
	return "", false
}

func letterCount(s string) int {
	n := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			n++
		}
	}
	return n
}

func findDate(line string, loc *time.Location) (time.Time, bool) {
	m := dateRE.FindStringSubmatch(line)
	if m == nil {
		return time.Time{}, false
	}
	day, _ := strconv.Atoi(m[1])
	month, _ := strconv.Atoi(m[2])
	year, _ := strconv.Atoi(m[3])
	if len(m[3]) == 2 {
		year += 2000
	}
	if month < 1 || month > 12 || day < 1 || day > 31 || year < 2000 {
		return time.Time{}, false
	}
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, loc), true
}

// TotalScoreHint is the best TotalScore across lines (for OCR pass selection).
func TotalScoreHint(text string, s Strategy) int {
	if s == nil {
		return 0
	}
	best := 0
	for _, line := range splitLines(text) {
		if sc := s.TotalScore(line); sc > best {
			best = sc
		}
	}
	return best
}
