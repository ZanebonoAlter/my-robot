package service

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Period helpers for topic lifeline context archival storage.
// See design.md §2.1: period format — week=2026-W27, month=2026-06, year=2026, all=all.

// FormatWeek returns the ISO week string for t, e.g. "2026-W27".
func FormatWeek(t time.Time) string {
	y, w := t.ISOWeek()
	return fmt.Sprintf("%d-W%02d", y, w)
}

// FormatMonth returns the month string for t, e.g. "2026-06".
func FormatMonth(t time.Time) string {
	return t.Format("2006-01")
}

// FormatYear returns the year string for t, e.g. "2026".
func FormatYear(t time.Time) string {
	return strconv.Itoa(t.Year())
}

// PeriodForGranularity computes the period string for the given granularity + time.
func PeriodForGranularity(t time.Time, granularity string) string {
	switch granularity {
	case "week":
		return FormatWeek(t)
	case "month":
		return FormatMonth(t)
	case "year":
		return FormatYear(t)
	case "all":
		return "all"
	default:
		return FormatWeek(t)
	}
}

// weekRegex matches ISO week strings like "2026-W07" or "2026-W7".
var weekRegex = regexp.MustCompile(`^(\d{4})-W(\d{1,2})$`)

// ParsePeriodRange converts a period string + granularity into the corresponding
// date range [from, to). Returns error if the period format is invalid.
func ParsePeriodRange(period string, granularity string) (from, to time.Time, err error) {
	switch granularity {
	case "week":
		matches := weekRegex.FindStringSubmatch(period)
		if matches == nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid week period format: %s (expected YYYY-WNN)", period)
		}
		year, _ := strconv.Atoi(matches[1])
		week, _ := strconv.Atoi(matches[2])
		// Find the Monday of the given ISO week.
		// Jan 4 is always in ISO week 1.
		jan4 := time.Date(year, 1, 4, 0, 0, 0, 0, time.UTC)
		// Adjust to the Monday of week 1.
		weekday := jan4.Weekday()
		if weekday == 0 {
			weekday = 7
		}
		mondayWeek1 := jan4.AddDate(0, 0, -int(weekday)+1)
		from = mondayWeek1.AddDate(0, 0, (week-1)*7)
		to = from.AddDate(0, 0, 7)
		return from, to, nil
	case "month":
		t, parseErr := time.Parse("2006-01", period)
		if parseErr != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid month period format: %s (expected YYYY-MM)", period)
		}
		from = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
		to = from.AddDate(0, 1, 0)
		return from, to, nil
	case "year":
		y, parseErr := strconv.Atoi(period)
		if parseErr != nil || y < 2000 || y > 2100 {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid year period format: %s (expected YYYY)", period)
		}
		from = time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC)
		to = from.AddDate(1, 0, 0)
		return from, to, nil
	case "all":
		return time.Time{}, time.Now().UTC(), nil
	default:
		return time.Time{}, time.Time{}, fmt.Errorf("unknown granularity: %s", granularity)
	}
}

// ComparePeriods returns a negative number if a < b, 0 if equal, positive if a > b.
// Periods are compared by granularity first, then by natural ordering within each
// granularity. "all" is always the largest.
func ComparePeriods(a, b string) int {
	// If same string, equal.
	if a == b {
		return 0
	}
	return strings.Compare(a, b)
}

// MaxPeriod returns the larger of two period strings.
func MaxPeriod(a, b string) string {
	if ComparePeriods(a, b) >= 0 {
		return a
	}
	return b
}
