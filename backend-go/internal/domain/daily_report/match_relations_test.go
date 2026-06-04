package daily_report

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func parseDate(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

func TestShouldWriteRelation_AdjacentDay(t *testing.T) {
	dateSet := map[string]bool{"2026-06-01": true, "2026-06-02": true}
	result := shouldWriteRelation(
		100, parseDate("2026-06-01"),
		200, parseDate("2026-06-02"),
		0.30,
		map[uint][]uint{},
		map[uint]time.Time{100: parseDate("2026-06-01")},
		dateSet,
	)
	require.True(t, result, "adjacent day match < 0.35 should be written")
}

func TestShouldWriteRelation_AdjacentDay_ExceedsThreshold(t *testing.T) {
	dateSet := map[string]bool{"2026-06-01": true, "2026-06-02": true}
	result := shouldWriteRelation(
		100, parseDate("2026-06-01"),
		200, parseDate("2026-06-02"),
		0.36,
		map[uint][]uint{},
		map[uint]time.Time{100: parseDate("2026-06-01")},
		dateSet,
	)
	require.False(t, result, "adjacent day match >= 0.35 should not be written")
}

func TestShouldWriteRelation_SkipDay_NoIntermediateContinuation(t *testing.T) {
	dateSet := map[string]bool{"2026-06-01": true, "2026-06-02": true, "2026-06-03": true}
	result := shouldWriteRelation(
		100, parseDate("2026-06-01"),
		300, parseDate("2026-06-03"),
		0.094,
		map[uint][]uint{}, // section 100 has no outgoing relations
		map[uint]time.Time{100: parseDate("2026-06-01")},
		dateSet,
	)
	require.True(t, result, "skip-day with no continuation and dist < 0.25 should be written")
}

func TestShouldWriteRelation_SkipDay_HasIntermediateContinuation(t *testing.T) {
	dateSet := map[string]bool{"2026-06-01": true, "2026-06-02": true, "2026-06-03": true}
	adjacency := map[uint][]uint{100: {200}}
	sectionDateMap := map[uint]time.Time{
		100: parseDate("2026-06-01"),
		200: parseDate("2026-06-02"),
	}
	result := shouldWriteRelation(
		100, parseDate("2026-06-01"),
		300, parseDate("2026-06-03"),
		0.213,
		adjacency,
		sectionDateMap,
		dateSet,
	)
	require.False(t, result, "skip-day with intermediate continuation should be filtered")
}

func TestShouldWriteRelation_SkipDay_DistanceTooHigh(t *testing.T) {
	dateSet := map[string]bool{"2026-06-01": true, "2026-06-02": true, "2026-06-03": true}
	result := shouldWriteRelation(
		100, parseDate("2026-06-01"),
		300, parseDate("2026-06-03"),
		0.27,
		map[uint][]uint{},
		map[uint]time.Time{100: parseDate("2026-06-01")},
		dateSet,
	)
	require.False(t, result, "skip-day with dist >= 0.25 should be filtered even without continuation")
}

func TestShouldWriteRelation_DiscontinuousDates_TreatedAsAdjacent(t *testing.T) {
	// Board only has 6/1 and 6/3 reports, no 6/2 report
	dateSet := map[string]bool{"2026-06-01": true, "2026-06-03": true}
	result := shouldWriteRelation(
		100, parseDate("2026-06-01"),
		300, parseDate("2026-06-03"),
		0.30,
		map[uint][]uint{},
		map[uint]time.Time{100: parseDate("2026-06-01")},
		dateSet,
	)
	require.True(t, result, "discontinuous dates (no 6/2 report) should be treated as adjacent day")
}

func TestShouldWriteRelation_MultipleAdjacentMatches_Split(t *testing.T) {
	dateSet := map[string]bool{"2026-06-01": true, "2026-06-02": true}
	result1 := shouldWriteRelation(
		80, parseDate("2026-06-01"),
		200, parseDate("2026-06-02"),
		0.20,
		map[uint][]uint{},
		map[uint]time.Time{80: parseDate("2026-06-01")},
		dateSet,
	)
	result2 := shouldWriteRelation(
		85, parseDate("2026-06-01"),
		200, parseDate("2026-06-02"),
		0.30,
		map[uint][]uint{},
		map[uint]time.Time{85: parseDate("2026-06-01")},
		dateSet,
	)
	require.True(t, result1, "first adjacent match should be written")
	require.True(t, result2, "second adjacent match should be written (split)")
}
