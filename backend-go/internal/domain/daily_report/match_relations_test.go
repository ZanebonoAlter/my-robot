package daily_report

import (
	"sort"
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

func TestCompetitiveFilter_Empty(t *testing.T) {
	result := competitiveFilter(nil)
	require.Empty(t, result)
}

func TestCompetitiveFilter_SingleCandidate(t *testing.T) {
	c := matchCandidate{FromID: 1, FromDate: parseDate("2026-06-01"), Distance: 0.15}
	result := competitiveFilter([]matchCandidate{c})
	require.Len(t, result, 1)
	require.Equal(t, uint(1), result[0].FromID)
}

func TestCompetitiveFilter_GapAboveThreshold_KeepBest(t *testing.T) {
	candidates := []matchCandidate{
		{FromID: 1, FromDate: parseDate("2026-06-01"), Distance: 0.15},
		{FromID: 2, FromDate: parseDate("2026-06-01"), Distance: 0.22},
		{FromID: 3, FromDate: parseDate("2026-06-01"), Distance: 0.30},
	}
	result := competitiveFilter(candidates)
	require.Len(t, result, 1)
	require.Equal(t, uint(1), result[0].FromID)
	require.Equal(t, 0.15, result[0].Distance)
}

func TestCompetitiveFilter_GapBelowThreshold_KeepCluster(t *testing.T) {
	candidates := []matchCandidate{
		{FromID: 1, FromDate: parseDate("2026-06-01"), Distance: 0.20},
		{FromID: 2, FromDate: parseDate("2026-06-01"), Distance: 0.22},
		{FromID: 3, FromDate: parseDate("2026-06-01"), Distance: 0.24},
		{FromID: 4, FromDate: parseDate("2026-06-01"), Distance: 0.28},
	}
	result := competitiveFilter(candidates)
	ids := make([]uint, len(result))
	for i, c := range result {
		ids[i] = c.FromID
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	require.Equal(t, []uint{1, 2}, ids)
}

func TestCompetitiveFilter_ExactGapThreshold_KeepBest(t *testing.T) {
	candidates := []matchCandidate{
		{FromID: 1, FromDate: parseDate("2026-06-01"), Distance: 0.10},
		{FromID: 2, FromDate: parseDate("2026-06-01"), Distance: 0.13},
	}
	result := competitiveFilter(candidates)
	require.Len(t, result, 1)
	require.Equal(t, uint(1), result[0].FromID)
}

func TestCompetitiveFilter_AllSimilarDistances_KeepAll(t *testing.T) {
	candidates := []matchCandidate{
		{FromID: 1, FromDate: parseDate("2026-06-01"), Distance: 0.298},
		{FromID: 2, FromDate: parseDate("2026-06-01"), Distance: 0.300},
		{FromID: 3, FromDate: parseDate("2026-06-01"), Distance: 0.300},
		{FromID: 4, FromDate: parseDate("2026-06-01"), Distance: 0.303},
	}
	result := competitiveFilter(candidates)
	require.Len(t, result, 4)
}
