package repository

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"syntopica-backend/internal/platform/testutil"
)

func TestDeriveSectionStatuses_IgnoresPersistentTopicIdentityEdges(t *testing.T) {
	day1 := time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC)
	day2 := day1.AddDate(0, 0, 1)
	dates := map[uint]time.Time{1: day1, 2: day2}
	relations := []SectionRelationResult{{
		FromID: 1, ToID: 2, Distance: 0.1, RelationType: "identity",
	}}

	statuses := DeriveSectionStatuses([]uint{1, 2}, relations, dates, day2)

	assert.Equal(t, "emerging", statuses[2])
}

func TestDeriveSectionStatuses_UsesHungarianSimilarityEdges(t *testing.T) {
	day1 := time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC)
	day2 := day1.AddDate(0, 0, 1)
	dates := map[uint]time.Time{1: day1, 2: day2}
	relations := []SectionRelationResult{{
		FromID: 1, ToID: 2, Distance: 0.1, RelationType: "similarity",
	}}

	statuses := DeriveSectionStatuses([]uint{1, 2}, relations, dates, day2)

	assert.Equal(t, "continuing", statuses[2])
}

func TestGetBoardSectionTimeline_DaysReturnsExactLatestWindow(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)
	boardID := seedTestBoard(t, db)
	latest := NormalizeReportDate(time.Now())
	for offset := 0; offset < 8; offset++ {
		reportID := seedTestReport(t, db, boardID, latest.AddDate(0, 0, -offset))
		seedTopicSection(t, db, reportID, "topic", vecStr(1, 0, 0))
	}

	result, err := repo.GetBoardSectionTimeline(boardID, 7)
	require.NoError(t, err)
	assert.Len(t, result.Sections, 7)
}
