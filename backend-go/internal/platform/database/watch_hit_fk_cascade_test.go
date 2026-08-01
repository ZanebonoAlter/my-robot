package database_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"syntopica-backend/internal/platform/database"
	"syntopica-backend/internal/platform/testutil"

	// Side-effect: register BoardTopicWatch/TopicWatchHit so RunAutoMigrate
	// creates their tables (the migration target). AutoMigrate runs with
	// DisableForeignKeyConstraintWhenMigrating=true, so it creates NO FK —
	// the versioned migration under test is the source of truth for the FK.
	_ "syntopica-backend/internal/topicgraph/repository"
)

// TestWatchHitFKCascadeMigration exercises migration 20260801_0002 end-to-end
// against a testcontainer PG in the production first-run scenario: AutoMigrate
// creates the watch tables WITHOUT any FK, so an orphan hit (watch_id pointing
// at a non-existent watch) can exist. The migration must clean it BEFORE adding
// the FK (else ADD CONSTRAINT validates existing rows and fails), then the new
// FK must ON DELETE CASCADE. This is the testing.md §146 hard constraint:
// "schema 迁移要在 testcontainer PG + 历史数据下测".
//
// Docker required. Skipped under -short.
func TestWatchHitFKCascadeMigration(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires testcontainer Postgres")
	}
	db := testutil.OpenTestDB(t)
	require.NoError(t, db.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error)
	require.NoError(t, database.RunAutoMigrate(db))

	// Hermetic setup against the shared process-singleton container: drop the FK
	// if a prior run left it, and clear any leftover probe rows.
	require.NoError(t, db.Exec(`ALTER TABLE topic_watch_hits DROP CONSTRAINT IF EXISTS fk_topic_watch_hits_watch`).Error)
	require.NoError(t, db.Exec(`DELETE FROM topic_watch_hits WHERE watch_id IN (9001, 99999)`).Error)
	require.NoError(t, db.Exec(`DELETE FROM board_topic_watches WHERE id = 9001`).Error)

	// Precondition: AutoMigrate must not have created the FK (migrations own FKs).
	var fkExists bool
	require.NoError(t, db.Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name='fk_topic_watch_hits_watch' AND table_name='topic_watch_hits')`).Scan(&fkExists).Error)
	require.False(t, fkExists, "precondition: AutoMigrate must not create the FK (migrations are the source of truth)")

	// Seed pre-migration state: 1 watch (w1=9001) + 2 valid hits + 1 orphan hit
	// (watch_id=99999 references no watch). The orphan insert only succeeds
	// because the FK is absent — precisely the production bug state.
	require.NoError(t, db.Exec(`INSERT INTO board_topic_watches (id, semantic_board_id, label, status, created_at, updated_at) VALUES (9001, 1, 'w1', 'active', now(), now())`).Error)
	require.NoError(t, db.Exec(`INSERT INTO topic_watch_hits (watch_id, section_id, report_id, period_date, reason, created_at) VALUES (9001, 100, 200, CURRENT_DATE, 'valid-1', now())`).Error)
	require.NoError(t, db.Exec(`INSERT INTO topic_watch_hits (watch_id, section_id, report_id, period_date, reason, created_at) VALUES (9001, 101, 201, CURRENT_DATE, 'valid-2', now())`).Error)
	require.NoError(t, db.Exec(`INSERT INTO topic_watch_hits (watch_id, section_id, report_id, period_date, reason, created_at) VALUES (99999, 300, 400, CURRENT_DATE, 'orphan', now())`).Error)

	// Locate migration 20260801_0002's Up closure and run it in-tx (mirrors the
	// production default in-transaction path).
	var up func(*gorm.DB) error
	for _, m := range database.ExportedPostgresMigrations() {
		if m.Version == "20260801_0002" {
			up = m.Up
			break
		}
	}
	require.NotNil(t, up, "migration 20260801_0002 not found in list")
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error { return up(tx) }),
		"migration Up must succeed (clean orphan, then add FK)")

	// ① orphan cleaned before ADD CONSTRAINT; w1's 2 valid hits retained.
	var validCount, orphanCount int
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM topic_watch_hits WHERE watch_id = 9001`).Scan(&validCount).Error)
	require.Equal(t, 2, validCount, "valid hits must survive the orphan cleanup")
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM topic_watch_hits WHERE watch_id = 99999`).Scan(&orphanCount).Error)
	require.Equal(t, 0, orphanCount, "orphan hit must be cleaned so ADD CONSTRAINT FK validation passes")

	// ② FK established.
	require.NoError(t, db.Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name='fk_topic_watch_hits_watch' AND table_name='topic_watch_hits')`).Scan(&fkExists).Error)
	require.True(t, fkExists, "FK fk_topic_watch_hits_watch must exist after migration")

	// ③ ON DELETE CASCADE: deleting w1 removes its hits at the DB level (no app code).
	require.NoError(t, db.Exec(`DELETE FROM board_topic_watches WHERE id = 9001`).Error)
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM topic_watch_hits WHERE watch_id = 9001`).Scan(&validCount).Error)
	require.Equal(t, 0, validCount, "FK ON DELETE CASCADE must remove w1's hits when the watch is deleted")

	// ④ idempotency: re-running Up after the FK exists is a no-op (IF NOT EXISTS guard).
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error { return up(tx) }),
		"migration must be idempotent (re-run after FK exists must not error)")
}
