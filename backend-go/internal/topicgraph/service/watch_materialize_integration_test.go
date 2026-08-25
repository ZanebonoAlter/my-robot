package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"syntopica-backend/internal/platform/airouter"
	"syntopica-backend/internal/platform/testutil"
	"syntopica-backend/internal/topicgraph/repository"
)

// fakeEmbedder returns an embedFunc answering every request with the given
// vectors (one per input), bypassing the real embedding provider.
func fakeEmbedder(vecs ...[]float64) embedFunc {
	return func(_ context.Context, _ airouter.EmbeddingRequest, _ airouter.Capability) (*airouter.EmbeddingResult, error) {
		return &airouter.EmbeddingResult{Embeddings: vecs}, nil
	}
}

// seedMaterializationWorld provisions the full watch-materialization world:
// board with two aux labels ([1,0,0]-aligned "AI 编程" and orthogonal
// "传统能源"), tags/articles wired per label, plus keyword-track articles
// (one tag-less, both containing "harness"). Returns the seeded board ID.
func seedMaterializationWorld(t *testing.T, db *gorm.DB) (boardID uint, today time.Time) {
	t.Helper()
	boardID = seedBoard(t, db)
	today = repository.NormalizeReportDate(time.Now())

	must := func(q string, args ...interface{}) {
		t.Helper()
		require.NoError(t, db.Exec(q, args...).Error)
	}
	day := today.Add(-12 * time.Hour) // inside today's window

	// Aux labels + board composition.
	must(`INSERT INTO semantic_labels (id, label, slug, label_type, embedding, status, created_at, updated_at)
		VALUES (660001, 'AI 编程', 'watch-int-ai', 'auxiliary', '[1,0,0]', 'active', now(), now()),
		       (660002, '传统能源', 'watch-int-energy', 'auxiliary', '[0,1,0]', 'active', now(), now())`)
	must(`INSERT INTO board_composition (board_id, auxiliary_label_id) VALUES (?, 660001), (?, 660002)`, boardID, boardID)

	// Tags: one linked to the AI aux label.
	must(`INSERT INTO topic_tags (id, label, slug, category, status, created_at)
		VALUES (660101, 'AI 编程工具', 'watch-int-ai-tag', 'event', 'active', now())`)
	must(`INSERT INTO topic_tag_semantic_labels (topic_tag_id, semantic_label_id) VALUES (660101, 660001)`)

	// Articles: feed + three articles — one tagged AI, one tagged but off-topic,
	// one UNTAGGED containing "harness" (the 漏网 article).
	must(`INSERT INTO feeds (id, title, url, created_at) VALUES (660190, 'watch int feed', 'https://example.com', now())`)
	must(`INSERT INTO articles (id, feed_id, title, ai_content_summary, pub_date, created_at)
		VALUES (660001, 660190, 'Copilot 发布新版本', 'AI 编程工具更新', ?, now()),
		       (660002, 660190, 'harness 工具链发布', NULL, ?, now()),
		       (660003, 660190, 'Harness v3 发布公告', '开源 harness 更新', ?, now())`, day, day, day)
	must(`INSERT INTO article_topic_tags (article_id, topic_tag_id, created_at)
		VALUES (660001, 660101, now()), (660002, 660101, now())`)

	t.Cleanup(func() {
		db.Exec(`DELETE FROM board_topic_watches WHERE semantic_board_id = ?`, boardID)
		db.Exec(`DELETE FROM board_persistent_topics WHERE semantic_board_id = ?`, boardID)
		db.Exec(`DELETE FROM board_daily_reports WHERE semantic_board_id = ?`, boardID)
		db.Exec(`DELETE FROM article_topic_tags WHERE article_id IN (660001, 660002, 660003)`)
		db.Exec(`DELETE FROM articles WHERE feed_id = 660190`)
		db.Exec(`DELETE FROM feeds WHERE id = 660190`)
		db.Exec(`DELETE FROM topic_tag_semantic_labels WHERE semantic_label_id IN (660001, 660002)`)
		db.Exec(`DELETE FROM topic_tags WHERE id = 660101`)
		db.Exec(`DELETE FROM board_composition WHERE board_id = ?`, boardID)
		db.Exec(`DELETE FROM semantic_labels WHERE id IN (660001, 660002) OR slug = 'test-board-watch'`)
	})
	return boardID, today
}

// TestWatchMaterializationIntegration_KeywordAndSentence asserts the full
// materialization → SaveReport flow against a testcontainer PG:
//
//  1. keyword_topic: tag-less "harness" articles aggregate into an ephemeral
//     watch_keyword section with one thread each;
//  2. sentence_topic: aux-label retrieval materializes a watch_sentence
//     section owned by the watch's dedicated topic, whose lifecycle advances
//     (consecutive_hits=1 after the first day);
//  3. the keyword section never gains a persistent topic (no candidate
//     adoption), no relations touch watch sections;
//  4. hint tracks never produce hits for materialized watches.
func TestWatchMaterializationIntegration_KeywordAndSentence(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires testcontainer Postgres")
	}
	db := testutil.SetupTestDB(t)
	repository.Repo = repository.NewTopicGraphRepository(db)
	ctx := context.Background()

	boardID, today := seedMaterializationWorld(t, db)

	// One keyword_topic watch and one sentence_topic watch (cached vector
	// aligned with the AI aux label).
	kwWatch, err := repository.Repo.CreateWatch(repository.CreateWatchInput{
		SemanticBoardID: boardID, Label: "harness", Type: repository.WatchTypeKeywordTopic,
	})
	require.NoError(t, err)
	stWatch, err := repository.Repo.CreateWatch(repository.CreateWatchInput{
		SemanticBoardID: boardID,
		Label:           "AI 编程工具进展",
		Type:            repository.WatchTypeSentenceTopic,
		Query:           "AI coding assistant 的进展",
		EmbeddingCache:  strPtrHelper(repository.FloatsToPgVector([]float64{1, 0, 0})),
	})
	require.NoError(t, err)

	// ── Materialize both tracks (the orchestrator's Step 7.5 internals) ──
	cfg := DefaultWatchSentenceConfig()
	stSec, stThreads, err := MaterializeSentenceWatch(ctx, *stWatch, today, cfg, fakeEmbedder([]float64{1, 0, 0}))
	require.NoError(t, err)
	require.NotNil(t, stSec, "aligned aux label + day articles ⇒ sentence section")
	require.Len(t, stThreads, 2, "tagged articles of the AI tag")
	require.NotNil(t, stSec.PersistentTopicID, "section owns the dedicated topic")

	kwSecs, kwBatches, err := MaterializeKeywordWatches(ctx, boardID, []repository.BoardTopicWatch{*kwWatch}, 1)
	require.NoError(t, err)
	require.Len(t, kwSecs, 1)
	require.Len(t, kwBatches[0], 2, "harness articles: one tagged + one tag-less (漏网捞回)")
	assert.Nil(t, kwSecs[0].PersistentTopicID)

	// The dedicated topic was created and linked.
	linked, err := repository.Repo.GetWatchByID(stWatch.ID)
	require.NoError(t, err)
	require.NotNil(t, linked.PersistentTopicID)
	assert.Equal(t, *stSec.PersistentTopicID, *linked.PersistentTopicID)

	// ── SaveReport with regular + materialized sections ──
	report := &repository.BoardDailyReport{
		SemanticBoardID: boardID,
		PeriodDate:      today,
		Title:           "materialization int test",
		Status:          "completed",
	}
	regular := repository.DailyReportSection{
		ClusterIndex: 0, ClusterLabel: "常规聚类节", ClusterTagIDs: repository.JSON("[]"),
		Embedding: repository.FloatsToPgVector([]float64{0, 1, 0}), LaneTier: "l3_new",
	}
	stSec.ClusterIndex = 1
	kwSecs[0].ClusterIndex = 2
	sections := []repository.DailyReportSection{regular, *stSec, kwSecs[0]}
	threadsBy := [][]repository.DailyReportThread{nil, stThreads, kwBatches[0]}
	require.NoError(t, repository.Repo.SaveReport(report, sections, threadsBy))

	// 1. keyword section stays topic-less and lane-preserved.
	var kwReload repository.DailyReportSection
	require.NoError(t, db.Where("report_id = ? AND lane_tier = ?", report.ID, LaneTierWatchKeyword).First(&kwReload).Error)
	assert.Nil(t, kwReload.PersistentTopicID, "keyword section must never gain a topic")
	assert.Equal(t, 2, kwReload.ArticleCount)
	assert.Equal(t, "关键字『harness』相关话题", kwReload.ClusterLabel)

	// No extra candidate topic was adopted for the KEYWORD section: the
	// board's topics are exactly the watch topic + the regular section's own
	// l3 candidate (normal lane behavior — not keyword adoption).
	var topicCount int64
	db.Model(&repository.BoardPersistentTopic{}).Where("semantic_board_id = ?", boardID).Count(&topicCount)
	assert.Equal(t, int64(2), topicCount, "watch topic + regular l3 candidate — nothing for the keyword section")

	// 2. sentence topic lifecycle advanced by its section: day one ⇒ 1.
	var watchTopic repository.BoardPersistentTopic
	require.NoError(t, db.First(&watchTopic, *stSec.PersistentTopicID).Error)
	assert.Equal(t, repository.TopicStatusActive, watchTopic.Status)
	assert.Equal(t, 1, watchTopic.HitCount, "day-1 hit counted once (no double count)")
	assert.Equal(t, 1, watchTopic.ConsecutiveHits)

	// 3. no relations involve watch sections.
	var relCount int64
	db.Raw(`SELECT COUNT(*) FROM daily_report_section_relations rel
		JOIN daily_report_sections s ON s.id = rel.from_section_id
		WHERE s.report_id = ? AND s.lane_tier LIKE 'watch_%'`, report.ID).Scan(&relCount)
	assert.Zero(t, relCount, "watch sections must have no relations")

	// 4. hint tracks: materialized watches produce zero hits (both tracks off).
	hits, err := repository.Repo.GetWatchHitsByReport(report.ID)
	require.NoError(t, err)
	assert.Empty(t, hits)

	// ── Day 2: sentence topic continues (consecutive_hits → 2) ──
	day2 := today.AddDate(0, 0, 1)
	// Move the articles into day 2 first (same feed rows, bump pub_date),
	// then materialize — day-2 retrieval must hit the moved articles.
	require.NoError(t, db.Exec(`UPDATE articles SET pub_date = ? WHERE id IN (660001, 660002)`, day2.Add(-12*time.Hour)).Error)
	stSec2, stThreads2, err := MaterializeSentenceWatch(ctx, *linked, day2, cfg, fakeEmbedder([]float64{1, 0, 0}))
	require.NoError(t, err)
	require.NotNil(t, stSec2, "day-2 retrieval hits again")

	report2 := &repository.BoardDailyReport{
		SemanticBoardID: boardID, PeriodDate: day2, Title: "day 2", Status: "completed",
	}
	stSec2.ClusterIndex = 0
	require.NoError(t, repository.Repo.SaveReport(report2, []repository.DailyReportSection{*stSec2}, [][]repository.DailyReportThread{stThreads2}))
	require.NoError(t, db.First(&watchTopic, *stSec.PersistentTopicID).Error)
	assert.Equal(t, 2, watchTopic.ConsecutiveHits, "second materialized day advances the streak")
	assert.Equal(t, 2, watchTopic.HitCount)
}
