package database_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"syntopica-backend/internal/dataenrichment/repository"
	"syntopica-backend/internal/platform/database"
	"syntopica-backend/internal/platform/testutil"
)

const resultKindMigrationVersion = "20260828_0001"

func resultKindMigration(t *testing.T) database.Migration {
	t.Helper()
	for _, migration := range database.ExportedPostgresMigrations() {
		if migration.Version == resultKindMigrationVersion {
			return migration
		}
	}
	t.Fatalf("migration %s not found", resultKindMigrationVersion)
	return database.Migration{}
}

func prepareResultKindLegacyShape(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error)
	require.NoError(t, database.RunAutoMigrate(db))

	// The testcontainer is shared by the package. Rewind only this migration's
	// constraints so every test can exercise its Up closure independently.
	for _, statement := range []string{
		`DROP TRIGGER IF EXISTS trg_validate_topic_enrichment_result_parent ON topic_enrichment_result`,
		`DROP FUNCTION IF EXISTS validate_topic_enrichment_result_parent()`,
		`ALTER TABLE topic_enrichment_result DROP CONSTRAINT IF EXISTS fk_topic_enrichment_result_parent`,
		`ALTER TABLE topic_enrichment_result DROP CONSTRAINT IF EXISTS fk_topic_enrichment_result_parent_board`,
		`ALTER TABLE topic_enrichment_result DROP CONSTRAINT IF EXISTS uq_topic_enrichment_result_id_board`,
		`ALTER TABLE topic_enrichment_result DROP CONSTRAINT IF EXISTS chk_topic_enrichment_result_kind`,
		`ALTER TABLE topic_enrichment_result DROP CONSTRAINT IF EXISTS chk_topic_enrichment_result_parent_shape`,
		`ALTER TABLE topic_enrichment_result ALTER COLUMN result_kind DROP NOT NULL`,
		`ALTER TABLE topic_enrichment_result ALTER COLUMN result_kind DROP DEFAULT`,
		`DELETE FROM topic_enrichment_result WHERE session_id LIKE 'result-kind-mig-%'`,
	} {
		require.NoError(t, db.Exec(statement).Error, statement)
	}
	t.Cleanup(func() {
		_ = db.Exec(`DELETE FROM topic_enrichment_result WHERE session_id LIKE 'result-kind-mig-%' AND parent_result_id IS NOT NULL`).Error
		_ = db.Exec(`DELETE FROM topic_enrichment_result WHERE session_id LIKE 'result-kind-mig-%'`).Error
	})
}

func runResultKindMigration(t *testing.T, db *gorm.DB) database.Migration {
	t.Helper()
	migration := resultKindMigration(t)
	require.Nil(t, migration.Down, "migration framework has no Down executor; migration must stay forward-only")
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error { return migration.Up(tx) }))
	return migration
}

func TestResultKindMigrationBackfillPreservesSectors(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires testcontainer Postgres")
	}
	db := testutil.OpenTestDB(t)
	prepareResultKindLegacyShape(t, db)

	topicSectors := `{"form":"event_chain","legacy":[1,2]}`
	boardSectors := `{"scope":"board","thesis":"旧论文","argument":{"layers":["原样"]}}`
	require.NoError(t, db.Exec(`INSERT INTO topic_enrichment_result
		(persistent_topic_id, semantic_board_id, analysis_scope, result_kind, sectors, session_id, created_at)
		VALUES
		(98101, NULL, 'topic', NULL, ?::jsonb, 'result-kind-mig-topic', now()),
		(NULL, 98201, 'board', 'topic_analysis', ?::jsonb, 'result-kind-mig-board', now())`, topicSectors, boardSectors).Error)

	migration := runResultKindMigration(t, db)
	// Re-running Up is idempotent and must not rewrite legacy JSON.
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error { return migration.Up(tx) }))

	type row struct {
		SessionID  string
		ResultKind string
		Sectors    string
	}
	var rows []row
	require.NoError(t, db.Raw(`SELECT session_id, result_kind, sectors::text AS sectors
		FROM topic_enrichment_result WHERE session_id LIKE 'result-kind-mig-%' ORDER BY session_id`).Scan(&rows).Error)
	require.Len(t, rows, 2)
	require.Equal(t, "legacy_board_analysis", rows[0].ResultKind)
	require.JSONEq(t, boardSectors, rows[0].Sectors)
	require.Equal(t, "topic_analysis", rows[1].ResultKind)
	require.JSONEq(t, topicSectors, rows[1].Sectors)
}

func TestBoardInvestigationParentConstraint(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires testcontainer Postgres")
	}
	db := testutil.OpenTestDB(t)
	prepareResultKindLegacyShape(t, db)
	runResultKindMigration(t, db)

	var briefID uint
	require.NoError(t, db.Raw(`INSERT INTO topic_enrichment_result
		(semantic_board_id, analysis_scope, result_kind, session_id, created_at)
		VALUES (98211, 'board', 'board_brief', 'result-kind-mig-brief', now()) RETURNING id`).Scan(&briefID).Error)
	var otherBoardBriefID uint
	require.NoError(t, db.Raw(`INSERT INTO topic_enrichment_result
		(semantic_board_id, analysis_scope, result_kind, session_id, created_at)
		VALUES (98212, 'board', 'board_brief', 'result-kind-mig-other-brief', now()) RETURNING id`).Scan(&otherBoardBriefID).Error)
	var legacyID uint
	require.NoError(t, db.Raw(`INSERT INTO topic_enrichment_result
		(semantic_board_id, analysis_scope, result_kind, session_id, created_at)
		VALUES (98211, 'board', 'legacy_board_analysis', 'result-kind-mig-legacy', now()) RETURNING id`).Scan(&legacyID).Error)

	validKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	require.NoError(t, db.Exec(`INSERT INTO topic_enrichment_result
		(semantic_board_id, analysis_scope, result_kind, parent_result_id, question_key, session_id, created_at)
		VALUES (98211, 'board', 'board_investigation', ?, ?, 'result-kind-mig-valid-child', now())`, briefID, validKey).Error)

	err := db.Exec(`INSERT INTO topic_enrichment_result
		(semantic_board_id, analysis_scope, result_kind, parent_result_id, question_key, session_id, created_at)
		VALUES (98211, 'board', 'board_investigation', ?, ?, 'result-kind-mig-invalid-cross-board', now())`, otherBoardBriefID, validKey).Error
	require.Error(t, err, "composite FK must reject a parent from another board")

	// Parent kind is a cross-row semantic check (not expressible by PostgreSQL
	// CHECK); repository validation rejects a same-board non-brief parent.
	keyCopy := validKey
	invalidParent := &repository.TopicEnrichmentResult{
		SemanticBoardID: repository.BoardIDPtr(98211),
		AnalysisScope:   "board",
		ParentResultID:  repository.TopicIDPtr(legacyID),
		QuestionKey:     &keyCopy,
		SessionID:       "result-kind-mig-invalid-non-brief",
	}
	require.Error(t, repository.NewRepository(db).CreateBoardInvestigationResult(context.Background(), invalidParent))

	err = db.Exec(`INSERT INTO topic_enrichment_result
		(semantic_board_id, analysis_scope, result_kind, parent_result_id, question_key, session_id, created_at)
		VALUES (98211, 'board', 'board_brief', ?, NULL, 'result-kind-mig-illegal-parent', now())`, briefID).Error
	require.Error(t, err, "non-investigation kinds must not carry a parent")
}

func TestQuestionKeyConstraint(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires testcontainer Postgres")
	}
	db := testutil.OpenTestDB(t)
	prepareResultKindLegacyShape(t, db)
	runResultKindMigration(t, db)

	var briefID uint
	require.NoError(t, db.Raw(`INSERT INTO topic_enrichment_result
		(semantic_board_id, analysis_scope, result_kind, session_id, created_at)
		VALUES (98221, 'board', 'board_brief', 'result-kind-mig-question-brief', now()) RETURNING id`).Scan(&briefID).Error)

	for name, key := range map[string]any{
		"null":       nil,
		"empty":      "",
		"not-hex":    "z123",
		"wrong-size": "abcdef",
	} {
		t.Run(name, func(t *testing.T) {
			err := db.Exec(`INSERT INTO topic_enrichment_result
				(semantic_board_id, analysis_scope, result_kind, parent_result_id, question_key, session_id, created_at)
				VALUES (98221, 'board', 'board_investigation', ?, ?, ?, now())`, briefID, key, "result-kind-mig-key-"+name).Error
			require.Error(t, err)
		})
	}
}
