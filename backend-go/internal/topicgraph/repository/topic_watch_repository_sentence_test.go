package repository

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"syntopica-backend/internal/platform/testutil"
)

// setupSentenceMaterializeDB provisions a testcontainer PG and a seeded
// sentence-track world:
//   - board 5511 with two aux labels (one aligned to query vec [1,0,0], one orthogonal)
//   - label L1 → tag T1 → two articles today; label L2 → tag T2 → one article today
//   - an inactive tag T3 linked to L1 (must be excluded)
func setupSentenceMaterializeDB(t *testing.T) *TopicGraphRepository {
	t.Helper()
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)

	cleanup := func() {
		db.Exec(`DELETE FROM board_topic_watches WHERE semantic_board_id = 5511`)
		db.Exec(`DELETE FROM board_persistent_topics WHERE semantic_board_id = 5511`)
		db.Exec(`DELETE FROM article_topic_tags WHERE article_id IN (SELECT id FROM articles WHERE feed_id = 551190)`)
		db.Exec(`DELETE FROM articles WHERE feed_id = 551190`)
		db.Exec(`DELETE FROM feeds WHERE id = 551190`)
		db.Exec(`DELETE FROM topic_tag_semantic_labels WHERE semantic_label_id IN (551101, 551102)`)
		db.Exec(`DELETE FROM topic_tags WHERE id IN (551201, 551202, 551203)`)
		db.Exec(`DELETE FROM board_composition WHERE board_id = 5511`)
		db.Exec(`DELETE FROM semantic_labels WHERE id IN (551100, 551101, 551102)`)
	}
	cleanup()
	t.Cleanup(cleanup)

	// Board label + aux labels. 3-dim vectors: L1 aligned with [1,0,0], L2 orthogonal.
	mustExec := func(q string, args ...interface{}) {
		t.Helper()
		require.NoError(t, db.Exec(q, args...).Error)
	}
	mustExec(`INSERT INTO semantic_labels (id, label, slug, label_type, embedding, status, created_at, updated_at)
		VALUES (551100, '测试板块', 'test-board-5511', 'board', NULL, 'active', now(), now())`)
	mustExec(`INSERT INTO semantic_labels (id, label, slug, label_type, embedding, status, created_at, updated_at)
		VALUES (551101, 'AI 编程', 'test-aux-ai', 'auxiliary', '[1,0,0]', 'active', now(), now())`)
	mustExec(`INSERT INTO semantic_labels (id, label, slug, label_type, embedding, status, created_at, updated_at)
		VALUES (551102, '传统能源', 'test-aux-energy', 'auxiliary', '[0,1,0]', 'active', now(), now())`)
	mustExec(`INSERT INTO board_composition (board_id, auxiliary_label_id) VALUES (5511, 551101), (5511, 551102)`)

	// Tags: T1/T2 active, T3 inactive (linked to L1 — must be filtered by status).
	mustExec(`INSERT INTO topic_tags (id, label, slug, category, status, created_at)
		VALUES (551201, 'AI tag', 'test-ai-tag', 'event', 'active', now()),
		       (551202, 'energy tag', 'test-energy-tag', 'event', 'active', now()),
		       (551203, 'inactive tag', 'test-inactive-tag', 'event', 'inactive', now())`)
	mustExec(`INSERT INTO topic_tag_semantic_labels (topic_tag_id, semantic_label_id)
		VALUES (551201, 551101), (551202, 551102), (551203, 551101)`)

	mustExec(`INSERT INTO feeds (id, title, url, created_at) VALUES (551190, 'sentence test feed', 'https://example.com', now())`)
	day := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	mustExec(`INSERT INTO articles (id, feed_id, title, ai_content_summary, pub_date, created_at)
		VALUES (551001, 551190, 'AI article 1', 'ai summary', ?, now()),
		       (551002, 551190, 'AI article 2', NULL, ?, now()),
		       (551003, 551190, 'energy article', 'energy summary', ?, now())`, day, day, day)
	mustExec(`INSERT INTO article_topic_tags (article_id, topic_tag_id, created_at)
		VALUES (551001, 551201, now()), (551002, 551201, now()), (551003, 551202, now())`)

	return repo
}

// TestListBoardAuxLabelEmbeddings verifies the retrieval pool: only the
// board's aux labels with embeddings return, vectors parsed.
func TestListBoardAuxLabelEmbeddings(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires testcontainer Postgres")
	}
	repo := setupSentenceMaterializeDB(t)

	pool, err := repo.ListBoardAuxLabelEmbeddings(5511)
	require.NoError(t, err)
	require.Len(t, pool, 2, "board label itself (no embedding) and foreign boards excluded")
	byLabel := map[string]WatchSentenceLabel{}
	for _, l := range pool {
		byLabel[l.Label] = l
	}
	require.Contains(t, byLabel, "AI 编程")
	require.Equal(t, []float64{1, 0, 0}, byLabel["AI 编程"].Embedding)
	require.Contains(t, byLabel, "传统能源")
}

// TestListActiveEventTagsBySemanticLabels verifies label→tag resolution with
// the day window and active-status filters.
func TestListActiveEventTagsBySemanticLabels(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires testcontainer Postgres")
	}
	repo := setupSentenceMaterializeDB(t)

	day := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	end := day.Add(24 * time.Hour)

	// L1 (AI) → only T1 (T3 inactive filtered).
	tags, err := repo.ListActiveEventTagsBySemanticLabels([]uint{551101}, day, end)
	require.NoError(t, err)
	assert.Equal(t, []uint{551201}, tags)

	// Both labels → T1 + T2 (T3 still filtered).
	tags, err = repo.ListActiveEventTagsBySemanticLabels([]uint{551101, 551102}, day, end)
	require.NoError(t, err)
	assert.Equal(t, []uint{551201, 551202}, tags)

	// Wrong day window → nothing.
	tags, err = repo.ListActiveEventTagsBySemanticLabels([]uint{551101, 551102}, day.AddDate(0, 0, -1), day)
	require.NoError(t, err)
	assert.Empty(t, tags)
}

// TestListArticlesByTagsForDay verifies the article union: dedupe across
// tags, summary preference, id order.
func TestListArticlesByTagsForDay(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires testcontainer Postgres")
	}
	repo := setupSentenceMaterializeDB(t)

	day := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	end := day.Add(24 * time.Hour)

	articles, err := repo.ListArticlesByTagsForDay([]uint{551201, 551202}, day, end)
	require.NoError(t, err)
	require.Len(t, articles, 3, "union across tags, deduped")
	assert.Equal(t, uint(551001), articles[0].ID)
	assert.Equal(t, uint(551002), articles[1].ID)
	assert.Equal(t, uint(551003), articles[2].ID)
	assert.Equal(t, "ai summary", articles[0].Summary)
}

// TestCreateWatchTopic verifies the dedicated-topic creation contract: active
// + manual source, Embedding AND Centroid both seeded (Centroid is the lane
// anchor), dates seeded to the first materialization day.
func TestCreateWatchTopic(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires testcontainer Postgres")
	}
	repo := setupSentenceMaterializeDB(t)

	day := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	topicID, err := repo.CreateWatchTopic(5511, "AI 编程工具进展", "[1,0,0]", day)
	require.NoError(t, err)
	require.NotZero(t, topicID)

	var topic BoardPersistentTopic
	require.NoError(t, repo.db.First(&topic, topicID).Error)
	assert.Equal(t, TopicStatusActive, topic.Status)
	assert.Equal(t, TopicSourceManual, topic.Source)
	assert.Equal(t, "[1,0,0]", topic.Embedding)
	assert.Equal(t, "[1,0,0]", topic.Centroid, "Centroid must be seeded alongside Embedding (lane anchor)")
	assert.Equal(t, "2026-08-25", topic.FirstSeenDate.UTC().Format("2006-01-02"), "first_seen_date is the materialization day (noon-normalized)")
}
