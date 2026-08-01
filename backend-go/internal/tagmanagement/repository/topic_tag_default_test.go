package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/testutil"
)

// TestTopicTagCreate_StatusDefaultsToActive guards against a regression where
// removing the `default:active` GORM tag caused new tags to be inserted with
// status="" (Go zero value) instead of "active". The getBoardArticles handler
// filters by status='active', so a "" status made tags (and their articles)
// invisible.
//
// Root cause: GORM omits zero-valued columns from INSERT only when the field
// has a `default:` tag. Without it, GORM explicitly inserts "" which overrides
// the DB DEFAULT. The `default:active` tag is therefore functionally required
// (not just a DB constraint), even though the DB-level DEFAULT is also set by
// migration 20260723_0001.
//
// This test runs against a throwaway testcontainer (full migration chain), so
// Docker must be available. Skipped under -short.
func TestTopicTagCreate_StatusDefaultsToActive(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires testcontainer Postgres")
	}
	db := testutil.SetupTestDB(t)

	// Create a tag without explicitly setting Status (simulates LLM tag extraction).
	tag := &models.TopicTag{
		Slug:     "regression-status-default",
		Label:    "regression-status-default",
		Category: "keyword",
	}
	require.NoError(t, db.Create(tag).Error, "create tag without explicit status")

	// Re-read from DB to confirm the persisted value.
	var got models.TopicTag
	require.NoError(t, db.First(&got, tag.ID).Error)
	require.Equal(t, "active", got.Status,
		"new TopicTag without explicit Status must default to 'active' (GORM default tag + DB DEFAULT); "+
			"if this fails, the getBoardArticles handler will hide the tag and its articles")
}
