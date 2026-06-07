# Bipartite Relation Matching Implementation Plan

> **REQUIRED SUB-SKILL:** Use the subagent-driven-development skill to implement this plan task-by-task.

**Goal:** Replace incremental greedy section relation matching with Hungarian bipartite algorithm for globally optimal 1:1 assignment, eliminating cumulative error.

**Architecture:** New file `matching.go` houses the Hungarian algorithm and three-phase matching logic. `repository.go` gets a new `RebuildBoardRelations` that clears all board relations and rebuilds via adjacent-day-pair iteration. Old functions (`MatchAndSaveRelations`, `shouldWriteRelation`, `competitiveFilter`, `hasContinuationInIntermediateDays`, `matchCandidate`) are deleted. `BackfillRelations` becomes a thin transaction wrapper.

**Tech Stack:** Go, GORM, pgvector, stretchr/testify

**Test Strategy:** Pure-function unit tests for Hungarian algorithm and three-phase logic (no DB required). Integration validated via backfill on real boards (Task 5).

---

## Task 1: Hungarian Algorithm Core

**Files:**
- Create: `backend-go/internal/domain/daily_report/matching.go`
- Test: `backend-go/internal/domain/daily_report/matching_test.go`

**Step 1: Create matching.go with Assignment struct and hungarianAssignment function**

```go
package daily_report

// Assignment represents one pair in the optimal 1:1 matching.
// Row and Col are indices into the cost matrix.
type Assignment struct {
	Row int
	Col int
}

// hungarianAssignment solves the minimum-cost assignment problem on a square
// cost matrix using the O(n³) Hungarian (Kuhn-Munkres) algorithm.
//
// Input:  n×n cost matrix. Use a large value (1e6) for impossible assignments.
// Output: slice of Assignment, one per row (if assignable), giving the optimal col.
func hungarianAssignment(cost [][]float64) []Assignment {
	n := len(cost)
	if n == 0 {
		return nil
	}

	// u[i] = potential of row i, v[j] = potential of col j
	u := make([]float64, n+1)
	v := make([]float64, n+1)
	// p[j] = row assigned to col j (1-indexed, 0 = unassigned)
	p := make([]int, n+1)
	// way[j] = column preceding j in the augmenting path
	way := make([]int, n+1)

	for i := 1; i <= n; i++ {
		// Start augmenting path from row i
		p[0] = i
		j0 := 0 // virtual column
		minv := make([]float64, n+1)
		used := make([]bool, n+1)
		for j := range minv {
			minv[j] = 1e18
		}

		for {
			used[j0] = true
			j1 := -1
			delta := 1e18
			i0 := p[j0]

			for j := 1; j <= n; j++ {
				if used[j] {
					continue
				}
				val := cost[i0-1][j-1] - u[i0] - v[j]
				if val < minv[j] {
					minv[j] = val
					way[j] = j0
				}
				if minv[j] < delta {
					delta = minv[j]
					j1 = j
				}
			}

			for j := 0; j <= n; j++ {
				if used[j] {
					u[p[j]] += delta
					v[j] -= delta
				} else {
					minv[j] -= delta
				}
			}

			j0 = j1
			if p[j0] == 0 {
				break
			}
		}

		// Update assignments along the augmenting path
		for {
			j1 := way[j0]
			p[j0] = p[j1]
			j0 = j1
			if j0 == 0 {
				break
			}
		}
	}

	// Convert to result
	result := make([]Assignment, 0, n)
	for j := 1; j <= n; j++ {
		if p[j] != 0 {
			result = append(result, Assignment{Row: p[j] - 1, Col: j - 1})
		}
	}
	return result
}
```

**Step 2: Write unit tests for Hungarian algorithm**

Create `matching_test.go`:

```go
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
	totalCost := 0.0
	for _, a := range result {
		totalCost += cost[a.Row][a.Col]
	}
	assert.InDelta(t, 0.03, totalCost, 1e-9, "should find optimal 0.01+0.02")
	assert.Len(t, result, 2)
}

func TestHungarian_3x2_PaddedTo3x3(t *testing.T) {
	// 3 left, 2 right → pad to 3×3
	penalty := 0.28
	inf := 1e6
	cost := [][]float64{
		{0.10, 0.20, penalty},    // left 0 matches right 0 (dist=0.10)
		{0.30, 0.05, penalty},    // left 1 matches right 1 (dist=0.05)
		{inf, inf, penalty},      // left 2 has no valid match → dummy
	}
	result := hungarianAssignment(cost)
	require.Len(t, result, 3)
	// Verify total cost
	totalCost := 0.0
	for _, a := range result {
		totalCost += cost[a.Row][a.Col]
	}
	// Should assign left0→right0 (0.10), left1→right1 (0.05), left2→dummy (0.28)
	assert.InDelta(t, 0.43, totalCost, 1e-9)
}

func TestHungarian_AllINF_NoMatches(t *testing.T) {
	inf := 1e6
	cost := [][]float64{
		{inf, inf},
		{inf, inf},
	}
	result := hungarianAssignment(cost)
	require.Len(t, result, 2)
	// All assigned to whatever column, but cost should be very high
	totalCost := 0.0
	for _, a := range result {
		totalCost += cost[a.Row][a.Col]
	}
	assert.Greater(t, totalCost, 1e5)
}

func TestHungarian_EmptyMatrix(t *testing.T) {
	result := hungarianAssignment([][]float64{})
	assert.Nil(t, result)
}

func TestHungarian_1x1(t *testing.T) {
	cost := [][]float64{{0.15}}
	result := hungarianAssignment(cost)
	require.Len(t, result, 1)
	assert.Equal(t, 0, result[0].Row)
	assert.Equal(t, 0, result[0].Col)
}

func TestHungarian_PrefersLowCostAssignment(t *testing.T) {
	// Row 0 can go to col 0 (0.01) or col 1 (0.02)
	// Row 1 can go to col 0 (0.02) or col 1 (0.20)
	// Optimal: row0→col0 (0.01) + row1→col1 (0.20) = 0.21
	// Suboptimal: row0→col1 (0.02) + row1→col0 (0.02) = 0.04 ← actually this is better!
	// So the algorithm should find total = 0.04
	cost := [][]float64{
		{0.01, 0.02},
		{0.02, 0.20},
	}
	result := hungarianAssignment(cost)
	totalCost := 0.0
	for _, a := range result {
		totalCost += cost[a.Row][a.Col]
	}
	assert.InDelta(t, 0.04, totalCost, 1e-9)
}
```

**Step 3: Run tests**

```bash
cd /mnt/d/project/Syntopica/backend-go && go test ./internal/domain/daily_report -run TestHungarian -v
```

Expected: All PASS.

**Step 4: Commit**

```bash
git add backend-go/internal/domain/daily_report/matching.go backend-go/internal/domain/daily_report/matching_test.go
git commit -m "feat(daily_report): add Hungarian algorithm core for bipartite matching"
```

---

## Task 2: Three-Phase Matching Logic

**Files:**
- Modify: `backend-go/internal/domain/daily_report/matching.go`
- Modify: `backend-go/internal/domain/daily_report/matching_test.go`

This task adds all three phases and their helpers to `matching.go`. These are pure functions (no DB interaction).

**Step 1: Add constants, types, and Phase 1 helper to matching.go**

Append to `matching.go`:

```go
import (
	"database/sql"
	"fmt"
	"math"
	"sort"

	"gorm.io/gorm"
)

// Threshold constants for bipartite matching
const (
	MatchPenalty    = 0.28 // Hungarian unmatch cost ceiling
	SplitGap        = 0.03 // Phase 2 split/merge gap threshold
	SplitCeiling    = 0.30 // Phase 2 candidate max distance
	SkipDayThresh   = 0.20 // Phase 3 skip-day distance threshold
	QueryCutoff     = 0.35 // SQL cross-join distance cutoff
	hungarianINF    = 1e6  // INF value for impossible assignments
)

// matchResult represents one matched pair of sections.
type matchResult struct {
	FromID   uint
	ToID     uint
	Distance float64
	Type     string // "primary", "split", "merge", "skip_day"
}

// sectionInfo holds section metadata for a single day.
type sectionInfo struct {
	ID        uint
	Embedding string
}

// buildDistMatrix queries the database for all pairwise distances between
// left (earlier) and right (later) sections using a single cross-join SQL.
func buildDistMatrix(tx *gorm.DB, left, right []sectionInfo, boardID uint) map[[2]uint]float64 {
	dist := make(map[[2]uint]float64)
	if len(left) == 0 || len(right) == 0 {
		return dist
	}

	rows, err := tx.Raw(`
		SELECT s1.id AS left_id, s2.id AS right_id, s1.embedding <=> s2.embedding AS dist
		FROM daily_report_sections s1, daily_report_sections s2
		WHERE s1.id IN (?) AND s2.id IN (?)
		  AND s1.embedding IS NOT NULL AND s2.embedding IS NOT NULL
		  AND s1.cluster_label IS NOT NULL AND s1.cluster_label != ''
		  AND s2.cluster_label IS NOT NULL AND s2.cluster_label != ''
		  AND s1.embedding <=> s2.embedding < ?
	`, leftIDs(left), rightIDs(right), QueryCutoff).Rows()
	if err != nil {
		return dist
	}
	defer rows.Close()

	for rows.Next() {
		var leftID, rightID uint
		var d float64
		if err := rows.Scan(&leftID, &rightID, &d); err == nil {
			dist[[2]uint{leftID, rightID}] = d
		}
	}
	return dist
}

func leftIDs(ss []sectionInfo) []uint {
	ids := make([]uint, len(ss))
	for i, s := range ss {
		ids[i] = s.ID
	}
	return ids
}

func rightIDs(ss []sectionInfo) []uint { return leftIDs(ss) }

// phase1Hungarian runs Phase 1: 1:1 optimal assignment with penalty.
// Returns primary matches and sets of unmatched left/right section IDs.
func phase1Hungarian(leftIDs, rightIDs []uint, dist map[[2]uint]float64) (primaries []matchResult, unmatchedLeft, unmatchedRight map[uint]bool) {
	unmatchedLeft = make(map[uint]bool, len(leftIDs))
	unmatchedRight = make(map[uint]bool, len(rightIDs))
	for _, id := range leftIDs {
		unmatchedLeft[id] = true
	}
	for _, id := range rightIDs {
		unmatchedRight[id] = true
	}
	if len(leftIDs) == 0 || len(rightIDs) == 0 {
		return nil, unmatchedLeft, unmatchedRight
	}

	// Build cost matrix: size max(nLeft, nRight) × 2 (real + dummy each side)
	// Simpler approach: pad to n×n, add penalty self-loops for "unmatch"
	n := len(leftIDs)
	if len(rightIDs) > n {
		n = len(rightIDs)
	}

	// We use 2n×2n matrix:
	// Rows 0..nLeft-1 = real left, nLeft..2n-1 = dummy left
	// Cols 0..nRight-1 = real right, nRight..2n-1 = dummy right
	// No, simpler: use n×n where extra rows/cols have penalty cost.
	// Actually, let's use a standard approach:
	// n = max(nLeft, nRight), pad with penalty cost.
	// Cost[i][j] = real distance if i < nLeft and j < nRight and dist exists
	//            = penalty if i >= nLeft or j >= nRight (padding)
	//            = INF if real dist > MatchPenalty

	size := n
	cost := make([][]float64, size)
	for i := range cost {
		cost[i] = make([]float64, size)
		for j := range cost {
			cost[i][j] = MatchPenalty // default: unmatch cost
		}
	}

	// Fill real distances
	for i, lid := range leftIDs {
		for j, rid := range rightIDs {
			if d, ok := dist[[2]uint{lid, rid}]; ok {
				if d <= MatchPenalty {
					cost[i][j] = d
				} else {
					cost[i][j] = hungarianINF
				}
			}
			// If no distance entry, it means dist >= QueryCutoff (> MatchPenalty),
			// so keep the default MatchPenalty (prefer unmatch over bad match)
		}
	}

	assignments := hungarianAssignment(cost)

	for _, a := range assignments {
		if a.Row < len(leftIDs) && a.Col < len(rightIDs) {
			lid := leftIDs[a.Row]
			rid := rightIDs[a.Col]
			c := cost[a.Row][a.Col]
			if c <= MatchPenalty && c < hungarianINF/2 {
				primaries = append(primaries, matchResult{
					FromID:   lid,
					ToID:     rid,
					Distance: c,
					Type:     "primary",
				})
				delete(unmatchedLeft, lid)
				delete(unmatchedRight, rid)
			}
		}
		// Row >= nLeft or Col >= nRight → padding row/col, means "unmatched"
		// Cost == MatchPenalty and it's a real-real pair → it's a weak match,
		// we should NOT write it. Only write if cost < MatchPenalty.
	}

	return primaries, unmatchedLeft, unmatchedRight
}

// phase2SplitMerge runs Phase 2: detects split and merge relations
// among sections left unmatched by Phase 1.
func phase2SplitMerge(leftIDs, rightIDs []uint, dist map[[2]uint]float64, primaries []matchResult, unmatchedLeft, unmatchedRight map[uint]bool) []matchResult {
	var results []matchResult

	// Build lookup: which left/right IDs are primary-matched and to whom
	leftPrimaryMatch := make(map[uint]matchResult) // leftID → its primary match
	rightPrimaryMatch := make(map[uint]matchResult) // rightID → its primary match
	for _, p := range primaries {
		leftPrimaryMatch[p.FromID] = p
		rightPrimaryMatch[p.ToID] = p
	}

	// Split detection: unmatched right → check matched left neighbors
	for rightID := range unmatchedRight {
		var bestLeftID uint
		var bestDist float64 = math.MaxFloat64
		for leftID := range leftPrimaryMatch {
			if d, ok := dist[[2]uint{leftID, rightID}]; ok && d <= SplitCeiling && d < bestDist {
				bestDist = d
				bestLeftID = leftID
			}
		}
		if bestLeftID == 0 {
			continue
		}
		primary := leftPrimaryMatch[bestLeftID]
		gap := bestDist - primary.Distance
		if gap < SplitGap {
			results = append(results, matchResult{
				FromID:   bestLeftID,
				ToID:     rightID,
				Distance: bestDist,
				Type:     "split",
			})
			delete(unmatchedRight, rightID)
		}
	}

	// Merge detection: unmatched left → check matched right neighbors
	for leftID := range unmatchedLeft {
		var bestRightID uint
		var bestDist float64 = math.MaxFloat64
		for rightID := range rightPrimaryMatch {
			if d, ok := dist[[2]uint{leftID, rightID}]; ok && d <= SplitCeiling && d < bestDist {
				bestDist = d
				bestRightID = rightID
			}
		}
		if bestRightID == 0 {
			continue
		}
		primary := rightPrimaryMatch[bestRightID]
		gap := bestDist - primary.Distance
		if gap < SplitGap {
			results = append(results, matchResult{
				FromID:   leftID,
				ToID:     bestRightID,
				Distance: bestDist,
				Type:     "merge",
			})
			delete(unmatchedLeft, leftID)
		}
	}

	return results
}

// hasRelationToDay checks if sectionID has any outgoing relation pointing to a section on the given date.
// Uses the already-written relations from the current rebuild (in-memory).
func hasRelationToDay(sectionID uint, targetDate string, writtenRelations []matchResult, sectionDateMap map[uint]string) bool {
	for _, r := range writtenRelations {
		if r.FromID == sectionID {
			if d, ok := sectionDateMap[r.ToID]; ok && d == targetDate {
				return true
			}
		}
	}
	return false
}

// findSectionDayIndex returns the index of the given date in the sorted dates slice, or -1.
func findSectionDayIndex(dates []string, date string) int {
	for i, d := range dates {
		if d == date {
			return i
		}
	}
	return -1
}
```

**Important note on `buildDistMatrix`**: The function uses `IN (?)` with slices. GORM handles this correctly. The `cluster_label` filter is critical to exclude empty-label sections.

**Step 2: Write Phase 1 unit tests**

Append to `matching_test.go`:

```go
func TestPhase1_PerfectOneToOne(t *testing.T) {
	left := []uint{10, 20, 30}
	right := []uint{40, 50, 60}
	dist := map[[2]uint]float64{
		{10, 40}: 0.01,
		{20, 50}: 0.02,
		{30, 60}: 0.03,
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
	dist := map[[2]uint]float64{
		{10, 30}: 0.31,
		{20, 30}: 0.32,
	}
	primaries, unmatchedL, unmatchedR := phase1Hungarian(left, right, dist)
	assert.Empty(t, primaries, "both dists > penalty, should produce no matches")
	assert.Len(t, unmatchedL, 2)
	assert.Len(t, unmatchedR, 1)
}

func TestPhase1_Mixed(t *testing.T) {
	left := []uint{10, 20}
	right := []uint{30, 40}
	dist := map[[2]uint]float64{
		{10, 30}: 0.15,
		{10, 40}: 0.25,
		{20, 30}: 0.27, // > MatchPenalty? No, 0.27 < 0.28
		{20, 40}: 0.10,
	}
	primaries, _, _ := phase1Hungarian(left, right, dist)
	assert.Len(t, primaries, 2)

	totalDist := 0.0
	for _, p := range primaries {
		totalDist += p.Distance
	}
	// Optimal: 10→30(0.15) + 20→40(0.10) = 0.25
	// OR: 10→40(0.25) + 20→30(0.27) = 0.52
	// Algorithm should find 0.25
	assert.InDelta(t, 0.25, totalDist, 1e-9)
}

func TestPhase1_EmptyLeft(t *testing.T) {
	primaries, unmatchedL, unmatchedR := phase1Hungarian(nil, []uint{10}, nil)
	assert.Nil(t, primaries)
	assert.Empty(t, unmatchedL)
	assert.Len(t, unmatchedR, 1)
}
```

**Step 3: Write Phase 2 unit tests**

```go
func TestPhase2_SplitDetection(t *testing.T) {
	left := []uint{10, 20}
	right := []uint{30, 40}
	dist := map[[2]uint]float64{
		{10, 30}: 0.15,
		{10, 40}: 0.17,
		{20, 30}: 0.30,
		{20, 40}: 0.28,
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
	dist := map[[2]uint]float64{
		{10, 30}: 0.10,
		{10, 40}: 0.25, // gap = 0.15 >= 0.03
	}
	primaries := []matchResult{{FromID: 10, ToID: 30, Distance: 0.10, Type: "primary"}}
	unmatchedL := map[uint]bool{}
	unmatchedR := map[uint]bool{40: true}

	results := phase2SplitMerge(left, right, dist, primaries, unmatchedL, unmatchedR)
	assert.Empty(t, results, "gap too large, should not write split")
}

func TestPhase2_MergeDetection(t *testing.T) {
	left := []uint{10, 20}
	right := []uint{30}
	dist := map[[2]uint]float64{
		{10, 30}: 0.14,
		{20, 30}: 0.12,
	}
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
	left := []uint{10}
	right := []uint{30}
	dist := map[[2]uint]float64{} // no distances at all
	primaries := []matchResult{}
	unmatchedL := map[uint]bool{10: true}
	unmatchedR := map[uint]bool{30: true}

	results := phase2SplitMerge(left, right, dist, primaries, unmatchedL, unmatchedR)
	assert.Empty(t, results)
}
```

**Step 4: Write Phase 3 unit tests**

```go
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

func TestFindSectionDayIndex(t *testing.T) {
	dates := []string{"2026-06-01", "2026-06-02", "2026-06-03"}
	assert.Equal(t, 0, findSectionDayIndex(dates, "2026-06-01"))
	assert.Equal(t, 2, findSectionDayIndex(dates, "2026-06-03"))
	assert.Equal(t, -1, findSectionDayIndex(dates, "2026-06-05"))
}
```

**Step 5: Run all new tests**

```bash
cd /mnt/d/project/Syntopica/backend-go && go test ./internal/domain/daily_report -run "TestPhase|TestHas|TestFind" -v
```

Expected: All PASS.

**Step 6: Run existing tests still pass**

```bash
cd /mnt/d/project/Syntopica/backend-go && go test ./internal/domain/daily_report -v
```

Expected: Old tests (`TestShouldWriteRelation*`, `TestCompetitiveFilter*`) still pass (old functions still exist). New tests pass too.

**Step 7: Commit**

```bash
git add backend-go/internal/domain/daily_report/matching.go backend-go/internal/domain/daily_report/matching_test.go
git commit -m "feat(daily_report): add three-phase bipartite matching logic"
```

---

## Task 3: RebuildBoardRelations Integration

**Files:**
- Modify: `backend-go/internal/domain/daily_report/matching.go` (add RebuildBoardRelations)
- Modify: `backend-go/internal/domain/daily_report/repository.go` (SaveReport, BackfillRelations, delete old code)
- Modify: `backend-go/internal/domain/daily_report/match_relations_test.go` (delete old tests, replace with new)

**Step 1: Add RebuildBoardRelations to matching.go**

```go
// RebuildBoardRelations clears all relations for a board and rebuilds them
// using three-phase bipartite matching on adjacent day pairs.
// It receives a *gorm.DB (which may be a transaction) and does NOT manage
// its own transaction lifecycle.
func RebuildBoardRelations(tx *gorm.DB, boardID uint) error {
	// 1. Delete all existing relations for this board
	if err := tx.Exec(`
		DELETE FROM daily_report_section_relations
		WHERE from_section_id IN (
			SELECT s.id FROM daily_report_sections s
			JOIN board_daily_reports r ON r.id = s.report_id
			WHERE r.semantic_board_id = ?
		) OR to_section_id IN (
			SELECT s.id FROM daily_report_sections s
			JOIN board_daily_reports r ON r.id = s.report_id
			WHERE r.semantic_board_id = ?
		)
	`, boardID, boardID).Error; err != nil {
		return fmt.Errorf("delete old relations: %w", err)
	}

	// 2. Load all sections with embeddings, grouped by date
	type dateSection struct {
		ID        uint
		Embedding string
		Day       string // formatted date
	}
	var allSections []dateSection
	if err := tx.Raw(`
		SELECT s.id, s.embedding, r.period_date::date AS day
		FROM daily_report_sections s
		JOIN board_daily_reports r ON r.id = s.report_id
		WHERE r.semantic_board_id = ?
		  AND s.embedding IS NOT NULL
		  AND s.cluster_label IS NOT NULL AND s.cluster_label != ''
		ORDER BY r.period_date ASC, s.id ASC
	`, boardID).Scan(&allSections).Error; err != nil {
		return fmt.Errorf("query sections: %w", err)
	}

	if len(allSections) == 0 {
		return nil
	}

	// 3. Group sections by date
	dateSections := make(map[string][]sectionInfo)
	var sortedDates []string
	for _, sec := range allSections {
		if _, exists := dateSections[sec.Day]; !exists {
			sortedDates = append(sortedDates, sec.Day)
		}
		dateSections[sec.Day] = append(dateSections[sec.Day], sectionInfo{
			ID:        sec.ID,
			Embedding: sec.Embedding,
		})
	}

	if len(sortedDates) < 2 {
		return nil // first report, no matching needed
	}

	// 4. Build sectionDateMap for Phase 3 skip-day checks
	sectionDateMap := make(map[uint]string, len(allSections))
	for _, sec := range allSections {
		sectionDateMap[sec.ID] = sec.Day
	}

	// 5. Process each adjacent day pair
	var allWritten []matchResult
	for i := 0; i < len(sortedDates)-1; i++ {
		leftDay := sortedDates[i]
		rightDay := sortedDates[i+1]
		left := dateSections[leftDay]
		right := dateSections[rightDay]

		// Phase 1: build dist matrix + Hungarian
		dist := buildDistMatrix(tx, left, right, boardID)
		primaries, unmatchedLeft, unmatchedRight := phase1Hungarian(
			leftIDs(left), rightIDs(right), dist,
		)

		// Phase 2: split/merge
		splitMerges := phase2SplitMerge(
			leftIDs(left), rightIDs(right), dist, primaries, unmatchedLeft, unmatchedRight,
		)

		// Collect written relations for this pair
		var pairWritten []matchResult
		pairWritten = append(pairWritten, primaries...)
		pairWritten = append(pairWritten, splitMerges...)
		allWritten = append(allWritten, pairWritten...)

		// Write to DB
		for _, r := range pairWritten {
			if err := tx.Exec(`
				INSERT INTO daily_report_section_relations (from_section_id, to_section_id, distance)
				VALUES (?, ?, ?)
				ON CONFLICT (from_section_id, to_section_id) DO UPDATE SET distance = EXCLUDED.distance
			`, r.FromID, r.ToID, r.Distance).Error; err != nil {
				return fmt.Errorf("write relation %d→%d: %w", r.FromID, r.ToID, err)
			}
		}

		// Phase 3: skip-day reconnect (only for Day_i → Day_{i+2})
		if i+2 < len(sortedDates) {
			skipDay := sortedDates[i+2]
			skipDist := buildDistMatrix(tx, left, dateSections[skipDay], boardID)

			// Sections from leftDay that were completely unmatched
			for _, lSec := range left {
				if _, stillUnmatched := unmatchedLeft[lSec.ID]; !stillUnmatched {
					continue
				}
				// Check: does this section have continuation on rightDay (Day_{i+1})?
				if hasRelationToDay(lSec.ID, rightDay, allWritten, sectionDateMap) {
					continue
				}
				// Check skip-day candidates
				for _, rSec := range dateSections[skipDay] {
					if d, ok := skipDist[[2]uint{lSec.ID, rSec.ID}]; ok && d < SkipDayThresh {
						if err := tx.Exec(`
							INSERT INTO daily_report_section_relations (from_section_id, to_section_id, distance)
							VALUES (?, ?, ?)
							ON CONFLICT (from_section_id, to_section_id) DO UPDATE SET distance = EXCLUDED.distance
						`, lSec.ID, rSec.ID, d).Error; err != nil {
							return fmt.Errorf("write skip-day relation %d→%d: %w", lSec.ID, rSec.ID, err)
						}
						allWritten = append(allWritten, matchResult{
							FromID: lSec.ID, ToID: rSec.ID, Distance: d, Type: "skip_day",
						})
					}
				}
			}
		}
	}

	logging.Infof("RebuildBoardRelations: board %d rebuilt %d relations across %d days",
		boardID, len(allWritten), len(sortedDates))
	return nil
}
```

**Step 2: Modify repository.go — SaveReport**

In `SaveReport`, replace the old relation deletion block and the `MatchAndSaveRelations` call:

Find (around line 60-70):
```go
if findErr == nil {
    // Delete old relations involving old section IDs
    var oldSectionIDs []uint
    tx.Model(&DailyReportSection{}).Where("report_id = ?", existing.ID).Pluck("id", &oldSectionIDs)
    if len(oldSectionIDs) > 0 {
        tx.Where("from_section_id IN ? OR to_section_id IN ?", oldSectionIDs, oldSectionIDs).Delete(&SectionRelation{})
    }
```

Replace the `if len(oldSectionIDs) > 0 { ... Delete }` block with:
```go
// (relation deletion removed — RebuildBoardRelations handles full board cleanup)
```

Find (around line 86-89):
```go
if err := MatchAndSaveRelations(tx, report.SemanticBoardID, report.PeriodDate, sections); err != nil {
    logging.Warnf("SaveReport: relation matching failed: %v", err)
}
```

Replace with:
```go
if err := RebuildBoardRelations(tx, report.SemanticBoardID); err != nil {
    logging.Warnf("SaveReport: relation rebuild failed: %v", err)
}
```

**Step 3: Delete old functions from repository.go**

Delete these functions entirely (lines ~390-590):
- `MatchAndSaveRelations` (line 390-497)
- `shouldWriteRelation` (line 498-532)
- `hasContinuationInIntermediateDays` (line 532-558)
- `matchCandidate` struct (line 554-558)
- `competitiveFilter` (line 560-590)

Also remove `"strings"` from imports if it becomes unused.

**Step 4: Rewrite BackfillRelations**

Replace `BackfillRelations` (line ~797) with:

```go
// BackfillRelations deletes all relations for a board and rebuilds them
// using bipartite matching. Thin transaction wrapper around RebuildBoardRelations.
func BackfillRelations(boardID uint) (rebuilt int, err error) {
	tx := database.DB.Begin()
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	if err = RebuildBoardRelations(tx, boardID); err != nil {
		return 0, fmt.Errorf("rebuild board relations: %w", err)
	}

	// Count rebuilt relations
	if err = tx.Raw(`
		SELECT COUNT(*) FROM daily_report_section_relations
		WHERE from_section_id IN (
			SELECT s.id FROM daily_report_sections s
			JOIN board_daily_reports r ON r.id = s.report_id
			WHERE r.semantic_board_id = ?
		)
	`, boardID).Scan(&rebuilt).Error; err != nil {
		return 0, fmt.Errorf("count relations: %w", err)
	}

	if err = tx.Commit().Error; err != nil {
		return 0, fmt.Errorf("commit backfill: %w", err)
	}
	return rebuilt, nil
}
```

**Step 5: Delete old test file and write new**

Delete `match_relations_test.go` entirely — all its tests reference deleted functions.

Create new tests in `matching_test.go` that test the DB-less pure functions (already written in Task 2).

**Step 6: Verify compilation**

```bash
cd /mnt/d/project/Syntopica/backend-go && go build ./...
```

Expected: No errors.

**Step 7: Run all remaining tests**

```bash
cd /mnt/d/project/Syntopica/backend-go && go test ./internal/domain/daily_report -v
```

Expected: Old `TestShouldWriteRelation*` and `TestCompetitiveFilter*` are gone (file deleted). New `TestHungarian*`, `TestPhase*` tests pass. `TestNormalizeReportDateKeepsRequestedDate` still passes.

**Step 8: Run lint**

```bash
cd /mnt/d/project/Syntopica/backend-go && golangci-lint run ./internal/domain/daily_report/...
```

Expected: Clean.

**Step 9: Commit**

```bash
git add -A backend-go/internal/domain/daily_report/
git commit -m "feat(daily_report): replace greedy matching with bipartite Hungarian algorithm

- Add matching.go: Hungarian algorithm + three-phase matching
- Add RebuildBoardRelations for board-level full rebuild
- Delete MatchAndSaveRelations, shouldWriteRelation, competitiveFilter, hasContinuationInIntermediateDays
- Rewrite BackfillRelations as thin transaction wrapper
- Remove SaveReport old relation deletion logic"
```

---

## Task 4: Data Verification (Backfill Real Boards)

This task validates the implementation on real data. No new code, manual verification.

**Step 1: Ensure backend compiles and starts**

```bash
cd /mnt/d/project/Syntopica/backend-go && go build ./...
```

**Step 2: Start backend**

```bash
cd /mnt/d/project/Syntopica/backend-go && go run cmd/server/main.go
```

**Step 3: Backfill Board 2853 (Iran situation, ~37 sections)**

```bash
curl -X POST http://localhost:5000/api/daily-reports/backfill-relations -H "Content-Type: application/json" -d '{"board_id": 2853}'
```

Verify:
- Response shows rebuilt relation count ≈ 20 (down from 36)
- No errors in backend logs

**Step 4: Backfill Board 3639 (AI tech, ~71 sections)**

```bash
curl -X POST http://localhost:5000/api/daily-reports/backfill-relations -H "Content-Type: application/json" -d '{"board_id": 3639}'
```

Verify:
- Response shows rebuilt relation count ≈ 46 (down from 118)
- No errors in backend logs

**Step 5: Check section statuses**

Use the timeline API to verify status distribution is reasonable:
```bash
curl http://localhost:5000/api/semantic-boards/3639/section-timeline?days=30
```

Verify: continuing count much higher than before, merge count much lower (was 31, should be ~4).

**Step 6: Final verification**

```bash
cd /mnt/d/project/Syntopica/backend-go && golangci-lint run ./... && go vet ./... && go test ./internal/domain/daily_report && go build ./...
```

Expected: All pass.

---

## Task 5: Update Documentation

**Files:**
- Modify: `docs/reference/` relevant docs if they mention the old algorithm

**Step 1: Check if docs mention old functions**

```bash
grep -rn "MatchAndSaveRelations\|shouldWriteRelation\|competitiveFilter\|incremental greedy" docs/
```

If any matches found, update to reference `RebuildBoardRelations` and bipartite matching.

**Step 2: Commit**

```bash
git add docs/
git commit -m "docs: update relation matching algorithm references"
```
