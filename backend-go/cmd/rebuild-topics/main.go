// Command rebuild-topics is a one-shot ops tool that repairs the persistent-
// topic graph on the connected database. It exists to close two regressions:
//
//  1. The section_relations unique constraint was widened to (from, to,
//     relation_type), so identity and similarity edges on the same section
//     pair must now coexist. Existing boards still hold the old single-row
//     pairs (identity having overwritten a strong Hungarian match), so every
//     board's relations are rebuilt to materialise both rows.
//  2. Historical reports predating the daily assignment pipeline never got a
//     persistent_topic_id (154/209 sections were NULL). BackfillAllPersistentTopics
//     reconstructs topics for the unassigned boards.
//
// Run from backend-go/:  go run ./cmd/rebuild-topics
package main

import (
	"fmt"
	"os"
	"time"

	"gorm.io/gorm"

	"syntopica-backend/internal/platform/config"
	"syntopica-backend/internal/platform/database"
	"syntopica-backend/internal/platform/logging"
	"syntopica-backend/internal/topicgraph/repository"
)

func main() {
	if err := config.LoadConfig("./configs"); err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}
	if config.AppConfig != nil {
		logging.Init(config.AppConfig.Log.Level, logging.FileConfig{Enabled: false})
	}

	// Phase 1+2: AutoMigrate then versioned migrations — this applies the
	// 20260620_0001 constraint widening on first run.
	if err := database.InitDB(config.AppConfig); err != nil {
		fmt.Fprintf(os.Stderr, "init db (migrations): %v\n", err)
		os.Exit(1)
	}
	db := database.DB
	repository.InitRepository(db)

	// ── Part A: rebuild relations for every board with sections ──
	// Re-materialises identity AND similarity edges as separate rows.
	boardIDs, err := listBoardsWithSections(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "list boards: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\n=== Part A: rebuild relations for %d boards ===\n", len(boardIDs))
	for _, bid := range boardIDs {
		start := time.Now()
		if err := repository.RebuildBoardRelations(db, bid); err != nil {
			fmt.Fprintf(os.Stderr, "  board %d rebuild FAILED: %v\n", bid, err)
			continue
		}
		fmt.Printf("  board %-6d rebuilt (%.1fs)\n", bid, time.Since(start).Seconds())
	}

	// ── Part B: backfill persistent topics for unassigned boards ──
	fmt.Printf("\n=== Part B: backfill persistent topics (unassigned boards) ===\n")
	results, err := repository.Repo.BackfillAllPersistentTopics()
	if err != nil {
		fmt.Fprintf(os.Stderr, "backfill all: %v\n", err)
	} else {
		for bid, cnt := range results {
			fmt.Printf("  board %-6d created %d topics\n", bid, cnt)
		}
		if len(results) == 0 {
			fmt.Println("  (no unassigned boards — nothing to backfill)")
		}
	}

	// ── Verification summary ===
	printVerificationSummary(db)
}

func listBoardsWithSections(db *gorm.DB) ([]uint, error) {
	var ids []uint
	if err := db.Raw(`
		SELECT DISTINCT r.semantic_board_id
		FROM daily_report_sections s
		JOIN board_daily_reports r ON r.id = s.report_id
		ORDER BY r.semantic_board_id
	`).Scan(&ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func printVerificationSummary(db *gorm.DB) {
	type counter struct {
		Type  string
		Count int
	}
	var edgeCounts []counter
	_ = db.Raw(`SELECT relation_type AS type, count(*) AS count
		FROM daily_report_section_relations GROUP BY relation_type ORDER BY type`).Scan(&edgeCounts).Error

	var totalSections, assignedSections, nullSections int64
	_ = db.Raw(`SELECT count(*) FROM daily_report_sections`).Scan(&totalSections).Error
	_ = db.Raw(`SELECT count(*) FROM daily_report_sections WHERE persistent_topic_id IS NOT NULL`).Scan(&assignedSections).Error
	nullSections = totalSections - assignedSections

	// Section pairs carrying BOTH an identity and a similarity edge (the fix target).
	var bothCount int64
	_ = db.Raw(`
		SELECT count(*) FROM (
			SELECT from_section_id, to_section_id
			FROM daily_report_section_relations
			WHERE relation_type = 'identity'
			INTERSECT
			SELECT from_section_id, to_section_id
			FROM daily_report_section_relations
			WHERE relation_type = 'similarity'
		) x`).Scan(&bothCount).Error

	fmt.Println("\n=== Verification summary ===")
	fmt.Printf("sections: %d total, %d assigned (%.1f%%), %d NULL\n",
		totalSections, assignedSections, 100*float64(assignedSections)/float64(max64(totalSections, 1)), nullSections)
	fmt.Println("relation edges by type:")
	for _, c := range edgeCounts {
		fmt.Printf("  %-12s %d\n", c.Type, c.Count)
	}
	fmt.Printf("section pairs carrying BOTH identity + similarity edges: %d (Issue 1 fix target — was 0 before)\n", bothCount)
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
