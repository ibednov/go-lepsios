// Package dates parses user-entered date ranges and computes period boundaries.
package dates

import (
	"fmt"
	"time"
)

const inputLayout = "02.01.2006"

// ParseRange parses "01.09.2026-30.09.2026" (or with spaces around the dash)
// into [from, to] where to is exclusive end-of-day.
func ParseRange(raw string, loc *time.Location) (time.Time, time.Time, error) {
	s := ""
	for _, r := range raw {
		if r != ' ' {
			s += string(r)
		}
	}
	parts := splitRange(s)
	if len(parts) != 2 {
		return time.Time{}, time.Time{}, fmt.Errorf("неверный формат. Используй ДД.ММ.ГГГГ-ДД.ММ.ГГГГ")
	}

	from, err := time.ParseInLocation(inputLayout, parts[0], loc)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("неверный формат. Используй ДД.ММ.ГГГГ-ДД.ММ.ГГГГ")
	}
	to, err := time.ParseInLocation(inputLayout, parts[1], loc)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("неверный формат. Используй ДД.ММ.ГГГГ-ДД.ММ.ГГГГ")
	}
	if to.Before(from) {
		from, to = to, from
	}
	return from, to.AddDate(0, 0, 1), nil
}

// ParseDay parses a single date "ДД.ММ.ГГГГ" in loc (start of that day).
func ParseDay(raw string, loc *time.Location) (time.Time, error) {
	s := ""
	for _, r := range raw {
		if r != ' ' {
			s += string(r)
		}
	}
	t, err := time.ParseInLocation(inputLayout, s, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("неверный формат. Используй ДД.ММ.ГГГГ")
	}
	return t, nil
}

func splitRange(s string) []string {
	// prefer " - " already stripped; support both '-' and '–'
	for _, sep := range []string{"–", "-"} {
		if idx := index(s, sep); idx > 0 {
			return []string{s[:idx], s[idx+len(sep):]}
		}
	}
	return nil
}

func index(s, sep string) int {
	for i := 0; i+len(sep) <= len(s); i++ {
		if s[i:i+len(sep)] == sep {
			return i
		}
	}
	return -1
}

// Period returns [from, to) boundaries for a named preset in loc.
func Period(name string, now time.Time, loc *time.Location) (time.Time, time.Time) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	switch name {
	case "today":
		return today, today.AddDate(0, 0, 1)
	case "week":
		wd := int(today.Weekday())
		if wd == 0 {
			wd = 7
		}
		monday := today.AddDate(0, 0, -(wd - 1))
		return monday, monday.AddDate(0, 0, 7)
	case "month":
		first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
		return first, first.AddDate(0, 1, 0)
	case "halfyear":
		from := today.AddDate(0, 0, -180)
		return from, today.AddDate(0, 0, 1)
	case "year":
		from := today.AddDate(-1, 0, 0)
		return from, today.AddDate(0, 0, 1)
	default:
		return today, today.AddDate(0, 0, 1)
	}
}

// Title renders a human title for a named period in Russian.
func Title(name string, now time.Time, loc *time.Location) string {
	from, to := Period(name, now, loc)
	switch name {
	case "today":
		return "сегодня"
	case "week":
		return fmt.Sprintf("неделя (%s – %s)", from.Format("02.01"), to.AddDate(0, 0, -1).Format("02.01.2006"))
	case "month":
		return RussianMonth(from) + " " + fmt.Sprintf("%d", from.Year())
	case "halfyear":
		return fmt.Sprintf("полгода (%s – %s)", from.Format("02.01.2006"), to.AddDate(0, 0, -1).Format("02.01.2006"))
	case "year":
		return fmt.Sprintf("год (%s – %s)", from.Format("02.01.2006"), to.AddDate(0, 0, -1).Format("02.01.2006"))
	default:
		return fmt.Sprintf("%s – %s", from.Format("02.01.2006"), to.AddDate(0, 0, -1).Format("02.01.2006"))
	}
}

var months = []string{
	"январь", "февраль", "март", "апрель", "май", "июнь",
	"июль", "август", "сентябрь", "октябрь", "ноябрь", "декабрь",
}

// RussianMonth returns "сентябрь" for the month of t.
func RussianMonth(t time.Time) string {
	return months[int(t.Month())-1]
}
