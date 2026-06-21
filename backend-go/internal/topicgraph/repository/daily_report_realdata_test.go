package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"syntopica-backend/internal/platform/testutil"
)

// fixtureRow is one section in the production-derived test fixture.
type fixtureRow struct {
	BoardID      uint   `json:"board_id"`
	PeriodDate   string `json:"period_date"`
	ClusterLabel string `json:"cluster_label"`
	Embedding    string `json:"embedding"`
}

// loadPersistentTopicFixture loads the production-derived fixture (108 real
// sections across 3 boards with vector(2560) embeddings) into the test DB.
// Embedding dimension is fixed at 2560; this helper aligns the section/topic
// embedding columns to that dimension before insert, mirroring the startup
// ensurer (which tests bypass).
func loadPersistentTopicFixture(t *testing.T, db *gorm.DB) map[uint]int {
	t.Helper()
	path := filepath.Join("testdata", "persistent_topic_fixture.json")
	raw, err := os.ReadFile(path)
	require.NoError(t, err, "read fixture")
	var rows []fixtureRow
	require.NoError(t, json.Unmarshal(raw, &rows), "parse fixture")
	require.NotEmpty(t, rows)

	// Align embedding columns to the fixture dimension (2560). The startup
	// ensurer normally does this from the embedding_config; in tests we ALTER
	// directly so the columns accept the real vector(2560) rows.
	parsed, err := repoParsePgVector(rows[0].Embedding)
	require.NoError(t, err)
	dim := len(parsed)
	for _, table := range []string{"daily_report_sections", "board_persistent_topics"} {
		require.NoError(t, db.Exec(
			fmt.Sprintf("ALTER TABLE %s ALTER COLUMN embedding TYPE vector(%d)", table, dim)).Error, table)
	}

	// Group by board so we create one semantic_label + N reports per board.
	byBoard := map[uint][]fixtureRow{}
	boardDates := map[uint]map[string]bool{}
	for _, r := range rows {
		byBoard[r.BoardID] = append(byBoard[r.BoardID], r)
		if boardDates[r.BoardID] == nil {
			boardDates[r.BoardID] = map[string]bool{}
		}
		boardDates[r.BoardID][r.PeriodDate] = true
	}

	created := map[uint]int{}
	for boardID, rs := range byBoard {
		// Minimal semantic_label row (board). slug must be unique.
		require.NoError(t, db.Exec(`INSERT INTO semantic_labels (id, label, slug, label_type, source, status, created_at, updated_at)
			VALUES (?, ?, ?, 'board', 'board', 'active', NOW(), NOW())`,
			boardID, fmt.Sprintf("fixture-board-%d", boardID), fmt.Sprintf("fixture-board-%d", boardID)).Error)

		// One report per distinct date.
		reportByDate := map[string]uint{}
		dates := make([]string, 0, len(boardDates[boardID]))
		for d := range boardDates[boardID] {
			dates = append(dates, d)
		}
		sort.Strings(dates)
		for _, d := range dates {
			day, err := time.Parse("2006-01-02", d)
			require.NoError(t, err)
			report := BoardDailyReport{
				SemanticBoardID: boardID, PeriodDate: day,
				Title: d, Status: "completed",
			}
			require.NoError(t, db.Create(&report).Error)
			reportByDate[d] = report.ID
		}

		// Sections with real embeddings.
		for _, r := range rs {
			sec := DailyReportSection{
				ReportID:     reportByDate[r.PeriodDate],
				ClusterLabel: r.ClusterLabel,
				ArticleCount: 1,
				Embedding:    r.Embedding,
			}
			require.NoError(t, db.Create(&sec).Error)
		}
		created[boardID] = len(rs)
	}
	return created
}

// TestRealData_BackfillTopicConvergence runs the full backfill pipeline against
// 108 REAL production sections (3 boards, vector(2560) embeddings, 6 days each)
// and asserts the algorithm produces a sensible topic distribution — the core
// "does the threshold work on real data" check. It also prints the per-board
// distribution so the verification-report can be back-filled.
func TestRealData_BackfillTopicConvergence(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)
	Repo = repo

	boardCounts := loadPersistentTopicFixture(t, db)

	cfg := DefaultPersistentTopicConfig()
	t.Logf("=== 真实数据回刷指标 (cluster_threshold=%.2f) ===", cfg.ClusterThreshold)

	totalTopics, totalSections := 0, 0
	for boardID := range boardCounts {
		created, err := repo.BackfillPersistentTopics(boardID)
		require.NoError(t, err)

		topics, err := repo.ListAllTopicsByBoard(boardID)
		require.NoError(t, err)
		// A healthy board collapses its sections into a handful of durable
		// narratives; if backfill produced ~1 topic per section the threshold
		// is far too tight, and a single mega-topic means it's too loose.
		assert.Greater(t, len(topics), 0, "board %d must produce topics", boardID)
		assert.Less(t, len(topics), boardCounts[boardID],
			"board %d: topics should be fewer than sections (clustering happened)", boardID)

		// No section left unassigned.
		var orphan int64
		db.Raw(`SELECT count(*) FROM daily_report_sections s
			JOIN board_daily_reports r ON r.id=s.report_id
			WHERE r.semantic_board_id=? AND s.persistent_topic_id IS NULL`, boardID).Scan(&orphan)
		assert.Equal(t, int64(0), orphan, "board %d: all sections must be assigned", boardID)

		// Topic size distribution (members per topic).
		sizes := map[uint]int{}
		var rows []struct {
			TopicID uint
		}
		db.Raw(`SELECT s.persistent_topic_id AS topic_id FROM daily_report_sections s
			JOIN board_daily_reports r ON r.id=s.report_id
			WHERE r.semantic_board_id=?`, boardID).Scan(&rows)
		for _, rw := range rows {
			sizes[rw.TopicID]++
		}
		sizeList := make([]int, 0, len(sizes))
		maxTopic, maxSize := uint(0), 0
		for tid, c := range sizes {
			sizeList = append(sizeList, c)
			if c > maxSize {
				maxSize, maxTopic = c, tid
			}
		}
		sort.Sort(sort.Reverse(sort.IntSlice(sizeList)))

		t.Logf("board %d: %d sections → %d topics (sizes desc: %v), largest topic #%d=%d",
			boardID, boardCounts[boardID], created, sizeList, maxTopic, maxSize)
		totalTopics += created
		totalSections += boardCounts[boardID]
	}
	t.Logf("=== 合计: %d sections → %d topics ===", totalSections, totalTopics)
}

// TestRealData_IdentityEdgesAfterBackfill verifies identity edges are written
// for same-topic adjacent-day sections across the real dataset, and that the
// similarity + identity edge split looks sane (similarity edges should be a
// minority compared to pre-backfill, since identity edges now carry the chain).
func TestRealData_IdentityEdgesAfterBackfill(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)
	Repo = repo

	loadPersistentTopicFixture(t, db)
	for _, boardID := range []uint{1980, 2197, 1974} {
		_, err := repo.BackfillPersistentTopics(boardID)
		require.NoError(t, err)
	}

	var counts []struct {
		Type  string
		Count int
	}
	db.Raw(`SELECT relation_type AS type, count(*) AS count
		FROM daily_report_section_relations GROUP BY relation_type ORDER BY type`).Scan(&counts)
	for _, c := range counts {
		t.Logf("relations: type=%s count=%d", c.Type, c.Count)
	}

	// At least some identity edges must exist after backfill (proves same-topic
	// chaining worked on real data, not just the synthetic fixtures).
	var identity int64
	db.Raw(`SELECT count(*) FROM daily_report_section_relations WHERE relation_type='identity'`).Scan(&identity)
	assert.Greater(t, identity, int64(0), "identity edges must exist after backfill on real data")
}
