package repository

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"syntopica-backend/internal/platform/testutil"
)

// TestListWatchScanArticles exercises the keyword-materialization scan pool
// (watch-materialized-topic §D2) against a testcontainer PG: pub_date window
// boundary, archived exclusion, summary-layer preference, tag-less articles
// included, scan limit.
//
// Docker required. Skipped under -short.
func TestListWatchScanArticles(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires testcontainer Postgres")
	}
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)

	day := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	cleanup := func() {
		db.Exec(`DELETE FROM articles WHERE feed_id = 987654`)
		db.Exec(`DELETE FROM feeds WHERE id = 987654`)
	}
	cleanup()
	t.Cleanup(cleanup)

	require.NoError(t, db.Exec(`INSERT INTO feeds (id, title, url, created_at)
		VALUES (987654, 'watch scan test feed', 'https://example.com', now())`).Error)

	insertArticle := func(id int, title, aiSum, firecrawl, content, desc string, archived bool, pubDate time.Time) {
		require.NoError(t, db.Exec(`INSERT INTO articles (id, feed_id, title, ai_content_summary, firecrawl_content, content, description, archived, pub_date, created_at)
			VALUES (?, 987654, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, ?, now())`,
			id, title, aiSum, firecrawl, content, desc, archived, pubDate).Error)
	}

	// In-window articles covering every summary-preference layer.
	insertArticle(1, "in ai", "AI summary text", "firecrawl text", "raw content", "desc", false, day.Add(10*time.Hour))
	insertArticle(2, "in firecrawl", "", "firecrawl text", "raw content", "desc", false, day.Add(11*time.Hour))
	insertArticle(3, "in content", "", "", "raw content", "desc", false, day.Add(12*time.Hour))
	insertArticle(4, "in desc only", "", "", "", "desc text", false, day.Add(13*time.Hour))
	// Boundary: exactly midnight next day is EXCLUDED ([start, end) window).
	insertArticle(5, "boundary next day", "x", "", "", "", false, day.Add(24*time.Hour))
	// Before window.
	insertArticle(6, "before window", "x", "", "", "", false, day.Add(-time.Second))
	// Archived in-window.
	insertArticle(7, "archived", "x", "", "", "", true, day.Add(1*time.Hour))
	// Tag-less in-window (the 漏网 article the track must catch).
	insertArticle(8, "no tags", "y", "", "", "", false, day.Add(2*time.Hour))

	// One article with tags (distinct ids, join fan-out must dedupe).
	insertArticle(9, "with tags", "z", "", "", "", false, day.Add(3*time.Hour))
	require.NoError(t, db.Exec(`INSERT INTO topic_tags (id, label, slug, category, status, created_at)
		VALUES (6001, 'scan test tag', 'scan-test-tag', 'event', 'active', now()), (6002, 'scan test tag2', 'scan-test-tag2', 'event', 'active', now())`).Error)
	t.Cleanup(func() { db.Exec(`DELETE FROM topic_tags WHERE id IN (6001, 6002)`) })
	for _, tagID := range []int{6001, 6002} {
		require.NoError(t, db.Exec(`INSERT INTO article_topic_tags (article_id, topic_tag_id, created_at)
			VALUES (9, ?, now())`, tagID).Error)
	}
	t.Cleanup(func() { db.Exec(`DELETE FROM article_topic_tags WHERE article_id = 9`) })

	articles, err := repo.ListWatchScanArticles(day, day.Add(24*time.Hour), 0)
	require.NoError(t, err)

	byID := make(map[uint]WatchScanArticle)
	for _, a := range articles {
		byID[a.ID] = a
	}
	// Window + archived filter.
	for _, excluded := range []uint{5, 6, 7} {
		assert.NotContains(t, byID, excluded, "article %d must be excluded (window/archived)", excluded)
	}
	// Summary preference layers.
	require.Contains(t, byID, uint(1))
	assert.Equal(t, "AI summary text", byID[1].Summary, "AI summary wins")
	require.Contains(t, byID, uint(2))
	assert.Equal(t, "firecrawl text", byID[2].Summary, "firecrawl beats raw content")
	require.Contains(t, byID, uint(3))
	assert.Equal(t, "raw content", byID[3].Summary, "raw content beats description")
	require.Contains(t, byID, uint(4))
	assert.Equal(t, "desc text", byID[4].Summary, "description is the last fallback")
	// Tag-less article present with NULL tag ids.
	require.Contains(t, byID, uint(8))
	assert.Nil(t, byID[8].TagIDs, "tag-less article has NULL tag_ids")
	// Tagged article: deduped tag list.
	require.Contains(t, byID, uint(9))
	require.NotNil(t, byID[9].TagIDs)
	assert.Contains(t, *byID[9].TagIDs, "6001")
	assert.Contains(t, *byID[9].TagIDs, "6002")
	assert.Equal(t, 1, countSubstring(*byID[9].TagIDs, "6001"), "join fan-out must dedupe tag ids")

	// Scan limit caps the pool.
	limited, err := repo.ListWatchScanArticles(day, day.Add(24*time.Hour), 2)
	require.NoError(t, err)
	assert.Len(t, limited, 2, "limit caps the scan pool")
}

func countSubstring(s, sub string) int {
	count := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			count++
		}
	}
	return count
}
