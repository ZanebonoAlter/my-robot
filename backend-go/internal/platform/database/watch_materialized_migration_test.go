package database_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"syntopica-backend/internal/platform/database"
	"syntopica-backend/internal/platform/testutil"

	// Side-effect: register BoardTopicWatch so RunAutoMigrate creates its
	// table (the migration target).
	_ "syntopica-backend/internal/topicgraph/repository"
)

// findWatchMaterializedMigration locates migration 20260825_0001's Up closure.
func findWatchMaterializedMigration() func(*gorm.DB) error {
	for _, m := range database.ExportedPostgresMigrations() {
		if m.Version == "20260825_0001" {
			return m.Up
		}
	}
	return nil
}

// resetWatchLegacyShape rewinds board_topic_watches to the pre-20260825_0001
// production shape: type VARCHAR(10) with the two-value CHECK, no
// query/embedding_cache/persistent_topic_id columns, no FK.
func resetWatchLegacyShape(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec(`DELETE FROM board_topic_watches WHERE semantic_board_id = 4343`).Error)
	require.NoError(t, db.Exec(`ALTER TABLE board_topic_watches DROP CONSTRAINT IF EXISTS fk_board_topic_watches_topic`).Error)
	require.NoError(t, db.Exec(`ALTER TABLE board_topic_watches DROP COLUMN IF EXISTS persistent_topic_id`).Error)
	require.NoError(t, db.Exec(`ALTER TABLE board_topic_watches DROP COLUMN IF EXISTS embedding_cache`).Error)
	require.NoError(t, db.Exec(`ALTER TABLE board_topic_watches DROP COLUMN IF EXISTS query`).Error)
	require.NoError(t, db.Exec(`ALTER TABLE board_topic_watches DROP CONSTRAINT IF EXISTS chk_board_topic_watches_type`).Error)
	// Legacy shape: VARCHAR(10) + two-value CHECK (exactly 20260824_0002's output).
	require.NoError(t, db.Exec(`ALTER TABLE board_topic_watches ALTER COLUMN type TYPE VARCHAR(10)`).Error)
	require.NoError(t, db.Exec(`ALTER TABLE board_topic_watches
		ADD CONSTRAINT chk_board_topic_watches_type CHECK (type IN ('label', 'keyword'))`).Error)
}

// TestWatchMaterializedMigration exercises migration 20260825_0001 end-to-end
// against a testcontainer PG in the legacy-database scenario: the table has
// the 20260824_0002 shape (VARCHAR(10) + two-value CHECK) and historical rows.
// The migration must widen the column, rebuild the four-value CHECK, add the
// three sentence-track columns, create the FK (ON DELETE SET NULL), and be
// idempotent on re-run (testing.md hard constraint: "schema 迁移要在
// testcontainer PG + 历史数据下测").
//
// Docker required. Skipped under -short.
func TestWatchMaterializedMigration(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires testcontainer Postgres")
	}
	db := testutil.OpenTestDB(t)
	require.NoError(t, db.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error)
	require.NoError(t, database.RunAutoMigrate(db))

	up := findWatchMaterializedMigration()
	require.NotNil(t, up, "migration 20260825_0001 not found in list")

	resetWatchLegacyShape(t, db)
	// Seed a historical keyword-track row on the legacy shape.
	require.NoError(t, db.Exec(`INSERT INTO board_topic_watches (semantic_board_id, label, type, status, created_at, updated_at)
		VALUES (4343, 'legacy keyword watch', 'keyword', 'active', now(), now())`).Error)

	// Run the migration in-transaction (mirrors the production path).
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error { return up(tx) }),
		"migration Up must succeed on the legacy two-value shape")

	// ① column widened: 'keyword_topic' (13 chars) must now be writable.
	require.NoError(t, db.Exec(`INSERT INTO board_topic_watches (semantic_board_id, label, type, status, query, created_at, updated_at)
		VALUES (4343, 'kw topic watch', 'keyword_topic', 'active', NULL, now(), now())`).Error,
		"type VARCHAR must have been widened to fit 'keyword_topic'")

	// ② four-value CHECK enforced: invalid type rejected, sentence_topic accepted.
	err := db.Exec(`INSERT INTO board_topic_watches (semantic_board_id, label, type, status) VALUES (4343, 'bad', 'sentence', 'active')`).Error
	require.Error(t, err, "CHECK must reject type='sentence'")
	assert.Contains(t, err.Error(), "chk_board_topic_watches_type")

	// ③ sentence-track columns: query/embedding_cache/persistent_topic_id all
	// nullable (hint-track rows keep them NULL), embedding_cache takes a vector.
	require.NoError(t, db.Exec(`INSERT INTO board_topic_watches (semantic_board_id, label, type, status, query, embedding_cache, created_at, updated_at)
		VALUES (4343, 'AI 编程工具进展', 'sentence_topic', 'active', 'AI coding assistant 的进展', '[0.1,0.2,0.3]', now(), now())`).Error,
		"sentence_topic row with query + embedding_cache vector must be insertable")
	var q *string
	require.NoError(t, db.Raw(`SELECT query FROM board_topic_watches WHERE semantic_board_id=4343 AND type='sentence_topic'`).Scan(&q).Error)
	require.NotNil(t, q)
	assert.Equal(t, "AI coding assistant 的进展", *q, "query column must round-trip text")

	// ④ FK ON DELETE SET NULL: deleting the referenced topic nulls the watch's
	// persistent_topic_id instead of cascading or failing.
	var topicID int
	require.NoError(t, db.Raw(`INSERT INTO board_persistent_topics
		(semantic_board_id, label, status, source, first_seen_date, last_seen_date, hit_count, consecutive_hits, created_at, updated_at)
		VALUES (4343, 'watch topic probe', 'active', 'manual', '2026-08-25', '2026-08-25', 1, 1, now(), now()) RETURNING id`).Scan(&topicID).Error)
	require.NoError(t, db.Exec(`UPDATE board_topic_watches SET persistent_topic_id = ? WHERE semantic_board_id=4343 AND type='sentence_topic'`, topicID).Error)
	require.NoError(t, db.Exec(`DELETE FROM board_persistent_topics WHERE id = ?`, topicID).Error)
	var pid *int
	require.NoError(t, db.Raw(`SELECT persistent_topic_id FROM board_topic_watches WHERE semantic_board_id=4343 AND type='sentence_topic'`).Scan(&pid).Error)
	assert.Nil(t, pid, "FK must be ON DELETE SET NULL: watch row survives topic deletion with NULL link")
	var fkAction string
	require.NoError(t, db.Raw(`SELECT confdeltype FROM pg_constraint WHERE conname='fk_board_topic_watches_topic'`).Scan(&fkAction).Error)
	assert.Equal(t, "n", fkAction, "confdeltype 'n' = SET NULL (pg_constraint stores lowercase)")

	// ⑤ idempotency: re-running Up on the fully-migrated shape must succeed.
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error { return up(tx) }),
		"migration must be idempotent (re-run must not error)")

	// Cleanup probe rows.
	require.NoError(t, db.Exec(`DELETE FROM board_topic_watches WHERE semantic_board_id = 4343`).Error)
}
