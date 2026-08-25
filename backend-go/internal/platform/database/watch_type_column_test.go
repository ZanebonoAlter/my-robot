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

// findWatchTypeMigration locates migration 20260824_0002's Up closure.
func findWatchTypeMigration() func(*gorm.DB) error {
	for _, m := range database.ExportedPostgresMigrations() {
		if m.Version == "20260824_0002" {
			return m.Up
		}
	}
	return nil
}

// TestWatchTypeColumnMigration exercises migration 20260824_0002 end-to-end
// against a testcontainer PG in the legacy-database scenario: the
// board_topic_watches table exists WITHOUT the type column (pre-change
// production shape), historical rows present. The migration must add the
// column with default 'label', backfill historical rows, enforce NOT NULL +
// the CHECK constraint (label|keyword), and be idempotent on re-run
// (testing.md hard constraint: "schema 迁移要在 testcontainer PG + 历史数据下测").
//
// Docker required. Skipped under -short.
func TestWatchTypeColumnMigration(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires testcontainer Postgres")
	}
	db := testutil.OpenTestDB(t)
	require.NoError(t, db.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error)
	require.NoError(t, database.RunAutoMigrate(db))

	up := findWatchTypeMigration()
	require.NotNil(t, up, "migration 20260824_0002 not found in list")

	// Hermetic setup against the shared process-singleton container: reset to
	// the legacy shape (column absent, constraint absent) and clear leftover
	// probe rows.
	require.NoError(t, db.Exec(`ALTER TABLE board_topic_watches DROP CONSTRAINT IF EXISTS chk_board_topic_watches_type`).Error)
	require.NoError(t, db.Exec(`ALTER TABLE board_topic_watches DROP COLUMN IF EXISTS type`).Error)
	require.NoError(t, db.Exec(`DELETE FROM board_topic_watches WHERE semantic_board_id = 4242`).Error)

	// Seed a historical row on the legacy shape (no type column at all —
	// exactly the pre-migration production state).
	require.NoError(t, db.Exec(`INSERT INTO board_topic_watches (semantic_board_id, label, status, created_at, updated_at) VALUES (4242, 'legacy watch', 'active', now(), now())`).Error)

	// Run the migration in-transaction (mirrors the production path).
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error { return up(tx) }),
		"migration Up must succeed on a legacy (column-less) table")

	// ① historical rows backfilled with the default type='label'.
	var legacyType string
	require.NoError(t, db.Raw(`SELECT type FROM board_topic_watches WHERE semantic_board_id = 4242`).Scan(&legacyType).Error)
	assert.Equal(t, "label", legacyType, "历史 watch 默认类型: legacy rows must read type='label'")

	// ② column constraints: NOT NULL + DEFAULT 'label'.
	var nullable string
	require.NoError(t, db.Raw(`SELECT is_nullable FROM information_schema.columns WHERE table_name='board_topic_watches' AND column_name='type'`).Scan(&nullable).Error)
	assert.Equal(t, "NO", nullable, "type column must be NOT NULL")
	var columnDefault string
	require.NoError(t, db.Raw(`SELECT column_default FROM information_schema.columns WHERE table_name='board_topic_watches' AND column_name='type'`).Scan(&columnDefault).Error)
	assert.Equal(t, "'label'::character varying", columnDefault, "type column must DEFAULT 'label'")

	// ③ CHECK constraint present and enforcing label|keyword.
	var chkExists bool
	require.NoError(t, db.Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name='chk_board_topic_watches_type' AND table_name='board_topic_watches')`).Scan(&chkExists).Error)
	assert.True(t, chkExists, "CHECK chk_board_topic_watches_type must exist after migration")
	err := db.Exec(`INSERT INTO board_topic_watches (semantic_board_id, label, type, status) VALUES (4242, 'bad', 'regex', 'active')`).Error
	require.Error(t, err, "CHECK must reject type='regex'")
	assert.Contains(t, err.Error(), "chk_board_topic_watches_type")

	// ④ idempotency: re-running Up (including after a partial state where the
	// column exists but the CHECK was lost) must succeed and restore the CHECK.
	require.NoError(t, db.Exec(`ALTER TABLE board_topic_watches DROP CONSTRAINT chk_board_topic_watches_type`).Error,
		"simulate a half-applied migration: CHECK missing, column present")
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error { return up(tx) }),
		"migration must be idempotent (re-run after partial state must not error)")
	require.NoError(t, db.Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name='chk_board_topic_watches_type' AND table_name='board_topic_watches')`).Scan(&chkExists).Error)
	assert.True(t, chkExists, "re-run must restore the dropped CHECK")

	// Cleanup probe rows.
	require.NoError(t, db.Exec(`DELETE FROM board_topic_watches WHERE semantic_board_id = 4242`).Error)
}
