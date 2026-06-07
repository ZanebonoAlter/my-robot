package daily_report

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHungarian_Simple2x2(t *testing.T) {
	cost := [][]float64{
		{0.01, 0.30},
		{0.25, 0.02},
	}
	result := hungarianAssignment(cost)
	assert.Len(t, result, 2)

	total := assignmentCost(cost, result)
	assert.InDelta(t, 0.03, total, 1e-9)
}

func TestHungarian_3x2_PaddedTo3x3(t *testing.T) {
	// 3 sections on day A, 2 on day B — caller pads to 3x3
	cost := [][]float64{
		{0.10, 0.50, 1e6},
		{0.40, 0.20, 1e6},
		{0.80, 0.70, 1e6},
	}
	result := hungarianAssignment(cost)
	assert.Len(t, result, 3)

	// Optimal: row0→col0(0.10), row1→col1(0.20), row2→col2(1e6)
	total := 0.0
	for _, a := range result {
		total += cost[a.Row][a.Col]
	}
	assert.InDelta(t, 1e6+0.30, total, 1e-9)
}

func TestHungarian_AllINF_NoMatches(t *testing.T) {
	inf := 1e6
	cost := [][]float64{
		{inf, inf},
		{inf, inf},
	}
	result := hungarianAssignment(cost)
	assert.Len(t, result, 2)

	total := assignmentCost(cost, result)
	assert.InDelta(t, 2e6, total, 1e-9)
}

func TestHungarian_EmptyMatrix(t *testing.T) {
	assert.Nil(t, hungarianAssignment(nil))
	assert.Nil(t, hungarianAssignment([][]float64{}))
}

func TestHungarian_1x1(t *testing.T) {
	cost := [][]float64{{4.2}}
	result := hungarianAssignment(cost)
	assert.Len(t, result, 1)
	assert.Equal(t, Assignment{Row: 0, Col: 0}, result[0])
}

func TestHungarian_PrefersLowCostAssignment(t *testing.T) {
	// Greedy row-by-row would pick row0→col0(1), row1→col1(5) total=6.
	// Optimal is row0→col1(2), row1→col0(3) total=5.
	cost := [][]float64{
		{1, 2},
		{3, 5},
	}
	result := hungarianAssignment(cost)
	total := assignmentCost(cost, result)
	assert.InDelta(t, 5.0, total, 1e-9)
}

// assignmentCost sums the cost matrix values at each assignment position.
func assignmentCost(cost [][]float64, assignments []Assignment) float64 {
	total := 0.0
	for _, a := range assignments {
		total += cost[a.Row][a.Col]
	}
	return total
}

// TestHungarian_LargerMatrix verifies correctness on a 4x4 matrix
// with a known optimal assignment.
func TestHungarian_LargerMatrix(t *testing.T) {
	// Classic example: optimal = row0→col3(0) + row1→col2(3) + row2→col1(5) + row3→col0(7) = 15
	cost := [][]float64{
		{90, 75, 75, 0},
		{35, 85, 3, 55},
		{45, 5, 65, 80},
		{50, 74, 77, 7},
	}
	// But minimum cost is: check all permutations —
	// row0→col3=0, row1→col2=3, row2→col1=5, row3→col0=50 => 58
	// row0→col3=0, row1→col2=3, row2→col0=45, row3→col1=74 => 122
	// Best known: row0→col3=0, row1→col2=3, row2→col1=5, row3→col0=50 => 58
	result := hungarianAssignment(cost)
	total := assignmentCost(cost, result)

	// Verify against brute-force minimum
	minCost := bruteForceMinCost(cost)
	assert.InDelta(t, minCost, total, 1e-9)
}

// bruteForceMinCost computes minimum assignment cost by checking all permutations.
func bruteForceMinCost(cost [][]float64) float64 {
	n := len(cost)
	perm := make([]int, n)
	for i := range perm {
		perm[i] = i
	}
	minCost := math.Inf(1)
	for {
		total := 0.0
		for i, j := range perm {
			total += cost[i][j]
		}
		if total < minCost {
			minCost = total
		}
		if !nextPermutation(perm) {
			break
		}
	}
	return minCost
}

func nextPermutation(a []int) bool {
	n := len(a)
	i := n - 2
	for i >= 0 && a[i] >= a[i+1] {
		i--
	}
	if i < 0 {
		return false
	}
	j := n - 1
	for a[j] <= a[i] {
		j--
	}
	a[i], a[j] = a[j], a[i]
	for l, r := i+1, n-1; l < r; l, r = l+1, r-1 {
		a[l], a[r] = a[r], a[l]
	}
	return true
}

// === Phase 1 tests ===

func TestPhase1_PerfectOneToOne(t *testing.T) {
	left := []uint{10, 20, 30}
	right := []uint{40, 50, 60}
	dist := map[[2]uint]float64{
		{10, 40}: 0.01, {20, 50}: 0.02, {30, 60}: 0.03,
	}
	primaries, unmatchedL, unmatchedR := phase1Hungarian(left, right, dist)
	assert.Len(t, primaries, 3)
	assert.Empty(t, unmatchedL)
	assert.Empty(t, unmatchedR)
	totalDist := 0.0
	for _, p := range primaries {
		totalDist += p.Distance
		assert.Equal(t, "primary", p.Type)
	}
	assert.InDelta(t, 0.06, totalDist, 1e-9)
}

func TestPhase1_PenaltyBlocksWeakMatch(t *testing.T) {
	left := []uint{10, 20}
	right := []uint{30}
	dist := map[[2]uint]float64{{10, 30}: 0.31, {20, 30}: 0.32}
	primaries, unmatchedL, unmatchedR := phase1Hungarian(left, right, dist)
	assert.Empty(t, primaries)
	assert.Len(t, unmatchedL, 2)
	assert.Len(t, unmatchedR, 1)
}

func TestPhase1_Mixed(t *testing.T) {
	left := []uint{10, 20}
	right := []uint{30, 40}
	dist := map[[2]uint]float64{
		{10, 30}: 0.15, {10, 40}: 0.25,
		{20, 30}: 0.27, {20, 40}: 0.10,
	}
	primaries, _, _ := phase1Hungarian(left, right, dist)
	assert.Len(t, primaries, 2)
	totalDist := 0.0
	for _, p := range primaries {
		totalDist += p.Distance
	}
	// Optimal: 10→30(0.15) + 20→40(0.10) = 0.25
	assert.InDelta(t, 0.25, totalDist, 1e-9)
}

func TestPhase1_EmptySides(t *testing.T) {
	primaries, unmatchedL, unmatchedR := phase1Hungarian(nil, []uint{10}, nil)
	assert.Nil(t, primaries)
	assert.Empty(t, unmatchedL)
	assert.Len(t, unmatchedR, 1)
}

// === Phase 2 tests ===

func TestPhase2_SplitDetection(t *testing.T) {
	left := []uint{10, 20}
	right := []uint{30, 40}
	dist := map[[2]uint]float64{
		{10, 30}: 0.15, {10, 40}: 0.17, {20, 30}: 0.30, {20, 40}: 0.28,
	}
	primaries := []matchResult{{FromID: 10, ToID: 30, Distance: 0.15, Type: "primary"}}
	unmatchedL := map[uint]bool{20: true}
	unmatchedR := map[uint]bool{40: true}
	results := phase2SplitMerge(left, right, dist, primaries, unmatchedL, unmatchedR)
	require.Len(t, results, 1)
	assert.Equal(t, "split", results[0].Type)
	assert.Equal(t, uint(10), results[0].FromID)
	assert.Equal(t, uint(40), results[0].ToID)
	assert.InDelta(t, 0.17, results[0].Distance, 1e-9)
}

func TestPhase2_SplitGapTooLarge(t *testing.T) {
	left := []uint{10}
	right := []uint{30, 40}
	dist := map[[2]uint]float64{{10, 30}: 0.10, {10, 40}: 0.25}
	primaries := []matchResult{{FromID: 10, ToID: 30, Distance: 0.10, Type: "primary"}}
	unmatchedL := map[uint]bool{}
	unmatchedR := map[uint]bool{40: true}
	results := phase2SplitMerge(left, right, dist, primaries, unmatchedL, unmatchedR)
	assert.Empty(t, results)
}

func TestPhase2_MergeDetection(t *testing.T) {
	left := []uint{10, 20}
	right := []uint{30}
	dist := map[[2]uint]float64{{10, 30}: 0.14, {20, 30}: 0.12}
	primaries := []matchResult{{FromID: 20, ToID: 30, Distance: 0.12, Type: "primary"}}
	unmatchedL := map[uint]bool{10: true}
	unmatchedR := map[uint]bool{}
	results := phase2SplitMerge(left, right, dist, primaries, unmatchedL, unmatchedR)
	require.Len(t, results, 1)
	assert.Equal(t, "merge", results[0].Type)
	assert.Equal(t, uint(10), results[0].FromID)
	assert.Equal(t, uint(30), results[0].ToID)
	assert.InDelta(t, 0.14, results[0].Distance, 1e-9)
}

func TestPhase2_NoCandidates(t *testing.T) {
	dist := map[[2]uint]float64{}
	results := phase2SplitMerge([]uint{10}, []uint{30}, dist, nil, map[uint]bool{10: true}, map[uint]bool{30: true})
	assert.Empty(t, results)
}

// === Phase 3 helper tests ===

func TestHasRelationToDay(t *testing.T) {
	written := []matchResult{
		{FromID: 10, ToID: 20},
		{FromID: 20, ToID: 30},
	}
	dateMap := map[uint]string{20: "2026-06-02", 30: "2026-06-03"}
	assert.True(t, hasRelationToDay(10, "2026-06-02", written, dateMap))
	assert.False(t, hasRelationToDay(10, "2026-06-03", written, dateMap))
	assert.False(t, hasRelationToDay(20, "2026-06-01", written, dateMap))
}
