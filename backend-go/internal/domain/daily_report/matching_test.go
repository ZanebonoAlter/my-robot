package daily_report

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
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
