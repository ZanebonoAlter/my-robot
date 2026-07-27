package repository

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"syntopica-backend/internal/platform/testutil"
)

// TestTopicLineageSurvivesClusterDrift is the cross-feature regression guard
// for the coupling documented in architecture/coupling-map.md. Under the
// lane-driven model (daily-report-lane-driven-clustering) the attribution is
// decided upstream by bucketing: a section flagged lane_tier=l2_llm anchors to
// its MatchedTopicID (the LLM keep/switch target), NOT to the embedding-nearest
// topic. This integration test proves the full DB write path
// (assignAndUpdateTopics) persists that anchor onto the LLM-chosen topic and
// opens NO spurious candidate.
//
// This is an INTEGRATION test (testcontainer pgvector) because the unit test
// TestPlanTopicAssignments_LaneDriven_AnchorHit covers only the pure planner.
func TestTopicLineageSurvivesClusterDrift(t *testing.T) {
	db := testutil.SetupTestDB(t)
	Repo = NewTopicGraphRepository(db)

	// Align embedding columns to the 3-dim test vectors below (mirrors the
	// realdata fixture's ALTER, since tests bypass the startup dimension ensurer).
	for _, table := range []string{"daily_report_sections", "board_persistent_topics"} {
		require.NoError(t, db.Exec(
			fmt.Sprintf("ALTER TABLE %s ALTER COLUMN embedding TYPE vector(3)", table)).Error, table)
	}

	boardID := seedTestBoard(t, db)
	now := NormalizeReportDate(time.Now())
	reportID := seedTestReport(t, db, boardID, now)

	// Two pre-existing ACTIVE topics. T1 is the embedding nearest to the new
	// section; T2 is 2nd-nearest but still well inside MatchThreshold (0.30).
	t1 := BoardPersistentTopic{
		SemanticBoardID: boardID, Label: "T1-nearest", Status: TopicStatusActive,
		Embedding: vecStr(1, 0, 0), FirstSeenDate: now, LastSeenDate: now, HitCount: 5,
	}
	t2 := BoardPersistentTopic{
		SemanticBoardID: boardID, Label: "T2-within-threshold", Status: TopicStatusActive,
		Embedding: vecStr(0.9, 0.43, 0), FirstSeenDate: now, LastSeenDate: now, HitCount: 3,
	}
	require.NoError(t, db.Create(&t1).Error)
	require.NoError(t, db.Create(&t2).Error)
	// section→T1 ≈ 0.00005 (nearest), section→T2 ≈ 0.093 (2nd-nearest, < 0.30).

	// New section whose embedding is near both topics. Its MatchedTopicID points
	// at T2 — simulating LLM clustering drift (e.g. triggered by a change to the
	// quality-truncation input that altered which tags entered clustering).
	sec := DailyReportSection{
		ReportID:     reportID,
		ClusterLabel: "new section",
		ArticleCount: 1,
		Embedding:    vecStr(1, 0.01, 0),
	}
	require.NoError(t, db.Create(&sec).Error) // gives it an ID
	mit := t2.ID
	sec.MatchedTopicID = &mit
	// Lane-driven attribution: an L2 section anchors to its MatchedTopicID
	// (the LLM keep/switch target) regardless of which topic is embedding-nearest.
	sec.LaneTier = "l2_llm"

	// Run the link under test against the real DB.
	_, err := assignAndUpdateTopics(db, boardID, now, []DailyReportSection{sec})
	require.NoError(t, err)

	// Lineage must survive: the section anchors to the LLM-chosen topic T2 (NOT
	// the nearest T1, and NOT a freshly-created candidate).
	var got DailyReportSection
	require.NoError(t, db.First(&got, sec.ID).Error)
	require.NotNil(t, got.PersistentTopicID)
	assert.Equal(t, t2.ID, *got.PersistentTopicID,
		"L2 section must anchor to its MatchedTopicID, not the embedding-nearest topic")
	assert.Equal(t, TopicConfAnchorHit, got.TopicMatchConfidence)

	// No spurious candidate must be created — that was the regression symptom
	// (frontend "all emerging topics").
	var newCandidates int64
	db.Model(&BoardPersistentTopic{}).
		Where("status = ? AND first_seen_date = ?", TopicStatusCandidate, now.Format("2006-01-02")).
		Count(&newCandidates)
	assert.Equal(t, int64(0), newCandidates, "no spurious candidate must be created on drift")
}
