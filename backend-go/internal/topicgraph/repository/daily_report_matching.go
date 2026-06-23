package repository

import (
	"fmt"
	"math"

	"syntopica-backend/internal/platform/logging"

	"gorm.io/gorm"
)

const (
	MatchPenalty  = 0.28
	SplitGap      = 0.03
	SplitCeiling  = 0.30
	SkipDayThresh = 0.20
	QueryCutoff   = 0.35
	hungarianINF  = 1e6
)

// matchResult represents a matched pair between sections across days.
type matchResult struct {
	FromID   uint
	ToID     uint
	Distance float64
	Type     string // "primary", "split", "merge", "skip_day"
}

// sectionInfo holds section metadata for matching.
type sectionInfo struct {
	ID        uint
	Embedding string
}

// Assignment represents a matched pair in the bipartite assignment.
type Assignment struct {
	Row int
	Col int
}

// hungarianAssignment finds the minimum-cost assignment in a square cost matrix
// using the O(n³) Hungarian (Kuhn-Munkres) algorithm with potentials.
//
// Input must be a square matrix; the caller is responsible for padding
// non-square inputs (e.g., adding rows/cols of large sentinel values).
//
// Returns nil for empty or nil input.
func hungarianAssignment(cost [][]float64) []Assignment {
	n := len(cost)
	if n == 0 {
		return nil
	}
	for _, row := range cost {
		if len(row) != n {
			return nil // non-square — caller should pad
		}
	}

	// 1-indexed arrays (index 0 unused)
	u := make([]float64, n+1) // potential for rows
	v := make([]float64, n+1) // potential for cols
	p := make([]int, n+1)     // p[j] = row assigned to col j
	way := make([]int, n+1)   // way[j] = preceding column in augmenting path

	for i := 1; i <= n; i++ {
		p[0] = i
		j0 := 0
		minv := make([]float64, n+1)
		used := make([]bool, n+1)
		for j := range minv {
			minv[j] = 1e18
		}

		for p[j0] != 0 {
			used[j0] = true
			i0 := p[j0]
			delta := 1e18
			j1 := 0

			for j := 1; j <= n; j++ {
				if used[j] {
					continue
				}
				cur := cost[i0-1][j-1] - u[i0] - v[j]
				if cur < minv[j] {
					minv[j] = cur
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
		}

		// Trace back augmenting path
		for j0 != 0 {
			p[j0] = p[way[j0]]
			j0 = way[j0]
		}
	}

	// Build 0-indexed assignments
	result := make([]Assignment, n)
	for j := 1; j <= n; j++ {
		if p[j] != 0 {
			result[p[j]-1] = Assignment{Row: p[j] - 1, Col: j - 1}
		}
	}
	return result
}

// buildDistMatrix queries all pairwise distances between left and right sections
// using a single SQL cross-join. Returns a map keyed by [2]uint{leftID, rightID}.
func buildDistMatrix(tx *gorm.DB, left, right []sectionInfo) map[[2]uint]float64 {
	dist := make(map[[2]uint]float64)
	if len(left) == 0 || len(right) == 0 {
		return dist
	}

	leftIDs := make([]uint, len(left))
	rightIDs := make([]uint, len(right))
	for i, s := range left {
		leftIDs[i] = s.ID
	}
	for i, s := range right {
		rightIDs[i] = s.ID
	}

	rows, err := tx.Raw(`
		SELECT s1.id AS left_id, s2.id AS right_id, s1.embedding <=> s2.embedding AS dist
		FROM daily_report_sections s1, daily_report_sections s2
		WHERE s1.id IN (?) AND s2.id IN (?)
		  AND s1.embedding IS NOT NULL AND s2.embedding IS NOT NULL
		  AND s1.cluster_label IS NOT NULL AND s1.cluster_label != ''
		  AND s2.cluster_label IS NOT NULL AND s2.cluster_label != ''
		  AND s1.embedding <=> s2.embedding < ?
	`, leftIDs, rightIDs, QueryCutoff).Rows()
	if err != nil {
		return dist
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var leftID, rightID uint
		var d float64
		if err := rows.Scan(&leftID, &rightID, &d); err == nil {
			dist[[2]uint{leftID, rightID}] = d
		}
	}
	return dist
}

// phase1Hungarian runs Phase 1: optimal 1:1 assignment with penalty.
// Returns primary matches and maps of unmatched left/right section IDs.
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

	n := len(leftIDs)
	if len(rightIDs) > n {
		n = len(rightIDs)
	}

	// Build n×n cost matrix
	cost := make([][]float64, n)
	for i := range cost {
		cost[i] = make([]float64, n)
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
			// No distance entry = dist >= QueryCutoff > MatchPenalty → keep default MatchPenalty
		}
	}

	assignments := hungarianAssignment(cost)

	for _, a := range assignments {
		if a.Row < len(leftIDs) && a.Col < len(rightIDs) {
			lid := leftIDs[a.Row]
			rid := rightIDs[a.Col]
			c := cost[a.Row][a.Col]
			if c < MatchPenalty { // strictly less: only truly good matches
				primaries = append(primaries, matchResult{
					FromID: lid, ToID: rid, Distance: c, Type: "primary",
				})
				delete(unmatchedLeft, lid)
				delete(unmatchedRight, rid)
			}
		}
	}

	return primaries, unmatchedLeft, unmatchedRight
}

// phase2SplitMerge detects split/merge relations among Phase 1 unmatched sections.
func phase2SplitMerge(leftIDs, rightIDs []uint, dist map[[2]uint]float64, primaries []matchResult, unmatchedLeft, unmatchedRight map[uint]bool) []matchResult {
	var results []matchResult

	leftPrimaryMatch := make(map[uint]matchResult)
	rightPrimaryMatch := make(map[uint]matchResult)
	for _, p := range primaries {
		leftPrimaryMatch[p.FromID] = p
		rightPrimaryMatch[p.ToID] = p
	}

	// Split: unmatched right → closest matched left
	for rightID := range unmatchedRight {
		var bestLeftID uint
		var bestDist = math.MaxFloat64
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
		if gap := bestDist - primary.Distance; gap < SplitGap {
			results = append(results, matchResult{FromID: bestLeftID, ToID: rightID, Distance: bestDist, Type: "split"})
			delete(unmatchedRight, rightID)
		}
	}

	// Merge: unmatched left → closest matched right
	for leftID := range unmatchedLeft {
		var bestRightID uint
		var bestDist = math.MaxFloat64
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
		if gap := bestDist - primary.Distance; gap < SplitGap {
			results = append(results, matchResult{FromID: leftID, ToID: bestRightID, Distance: bestDist, Type: "merge"})
			delete(unmatchedLeft, leftID)
		}
	}

	return results
}

// hasRelationToDay checks if a section has any outgoing relation to sections on a specific day.
func hasRelationToDay(sectionID uint, targetDay string, written []matchResult, sectionDateMap map[uint]string) bool {
	for _, r := range written {
		if r.FromID == sectionID {
			if d, ok := sectionDateMap[r.ToID]; ok && d == targetDay {
				return true
			}
		}
	}
	return false
}

// RebuildBoardRelations clears all relations for a board and rebuilds them
// using three-phase bipartite matching on adjacent day pairs.
// Receives *gorm.DB (may be a transaction); does NOT manage its own transaction.
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
		Day       string
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

	// 4. Build section→date map for Phase 3 skip-day checks
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

		leftIDs := make([]uint, len(left))
		rightIDs := make([]uint, len(right))
		for k, s := range left {
			leftIDs[k] = s.ID
		}
		for k, s := range right {
			rightIDs[k] = s.ID
		}

		// Phase 1
		dist := buildDistMatrix(tx, left, right)
		primaries, unmatchedLeft, unmatchedRight := phase1Hungarian(leftIDs, rightIDs, dist)

		// Phase 2
		splitMerges := phase2SplitMerge(leftIDs, rightIDs, dist, primaries, unmatchedLeft, unmatchedRight)

		// Collect and write
		var pairWritten []matchResult
		pairWritten = append(pairWritten, primaries...)
		pairWritten = append(pairWritten, splitMerges...)
		allWritten = append(allWritten, pairWritten...)

		for _, r := range pairWritten {
			if err := tx.Exec(`
				INSERT INTO daily_report_section_relations (from_section_id, to_section_id, distance, relation_type)
				VALUES (?, ?, ?, 'similarity')
				ON CONFLICT (from_section_id, to_section_id, relation_type) DO UPDATE SET distance = EXCLUDED.distance
			`, r.FromID, r.ToID, r.Distance).Error; err != nil {
				return fmt.Errorf("write relation %d→%d: %w", r.FromID, r.ToID, err)
			}
		}

		// Phase 3: skip-day reconnect (Day_i → Day_{i+2} only)
		if i+2 < len(sortedDates) {
			skipDay := sortedDates[i+2]
			skipRight := dateSections[skipDay]
			skipDist := buildDistMatrix(tx, left, skipRight)

			for _, lSec := range left {
				if _, stillUnmatched := unmatchedLeft[lSec.ID]; !stillUnmatched {
					continue
				}
				if hasRelationToDay(lSec.ID, rightDay, allWritten, sectionDateMap) {
					continue
				}
				for _, rSec := range skipRight {
					if d, ok := skipDist[[2]uint{lSec.ID, rSec.ID}]; ok && d < SkipDayThresh {
						if err := tx.Exec(`
							INSERT INTO daily_report_section_relations (from_section_id, to_section_id, distance, relation_type)
							VALUES (?, ?, ?, 'similarity')
							ON CONFLICT (from_section_id, to_section_id, relation_type) DO UPDATE SET distance = EXCLUDED.distance
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

	// Identity-edge overlay: for each persistent topic, connect adjacent-day
	// sections that share it. These edges bypass the 0.28 match penalty so a
	// narrative chain survives cluster-label drift across days (root cause B).
	// The unique constraint is (from, to, relation_type), so identity and
	// similarity edges on the same section pair coexist as two rows — neither
	// overwrites the other. The UI renders identity as a solid line (same
	// topic) and similarity as a dashed line (Hungarian match).
	if err := writeIdentityEdges(tx, boardID); err != nil {
		// Non-fatal: identity edges are an enhancement, not a correctness gate.
		logging.Warnf("RebuildBoardRelations: identity-edge overlay failed for board %d: %v", boardID, err)
	}

	logging.Infof("RebuildBoardRelations: board %d rebuilt %d relations across %d days",
		boardID, len(allWritten), len(sortedDates))
	return nil
}

// writeIdentityEdges connects adjacent-day sections that share a persistent
// topic. Unlike the Hungarian similarity edges, identity edges are written
// regardless of embedding distance, so a chain persists through cluster-label
// drift. The distance is the true cosine distance (which may exceed
// MatchPenalty); relation_type marks the edge so the UI can render it as a
// solid line.
//
// "Adjacent day" means the next report date for that board (calendar-adjacent
// days may be missing reports, so we use the ordered distinct period_date set).
// Identity edges run strictly forward in time (earlier → later).
//
// Identity and similarity edges coexist: the ON CONFLICT target is the
// (from, to, relation_type) triple, so an identity INSERT only clashes with
// a prior identity row, never with a similarity row on the same pair. This
// fixes the regression where a strong Hungarian match (distance << 0.28) on
// an adjacent-day same-topic pair was silently replaced by an identity row,
// making the timeline view (similarity-only) drop the edge entirely.
func writeIdentityEdges(tx *gorm.DB, boardID uint) error {
	// For each topic, pair each section with the earliest later section in the
	// same topic. Using DISTINCT ON per (topic, from_section) keeps the chain
	// to the immediate successor, avoiding a fully-connected topic subgraph.
	if err := tx.Exec(`
		WITH ordered AS (
			SELECT s.id AS section_id,
			       s.persistent_topic_id AS topic_id,
			       r.period_date::date AS day,
			       s.embedding
			FROM daily_report_sections s
			JOIN board_daily_reports r ON r.id = s.report_id
			WHERE r.semantic_board_id = ?
			  AND s.persistent_topic_id IS NOT NULL
			  AND s.embedding IS NOT NULL
		),
		pairs AS (
			SELECT DISTINCT ON (o1.section_id)
			       o1.section_id AS from_id,
			       o2.section_id AS to_id,
			       o1.embedding <=> o2.embedding AS dist
			FROM ordered o1
			JOIN ordered o2
			  ON o2.topic_id = o1.topic_id
			 AND o2.day > o1.day
			ORDER BY o1.section_id, o2.day ASC, o2.section_id ASC
		)
		INSERT INTO daily_report_section_relations (from_section_id, to_section_id, distance, relation_type)
		SELECT from_id, to_id, dist, 'identity'
		FROM pairs
		WHERE to_id IS NOT NULL
		ON CONFLICT (from_section_id, to_section_id, relation_type) DO UPDATE
			SET distance = EXCLUDED.distance
	`, boardID).Error; err != nil {
		return fmt.Errorf("write identity edges: %w", err)
	}
	return nil
}
