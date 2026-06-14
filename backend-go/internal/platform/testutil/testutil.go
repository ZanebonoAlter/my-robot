package testutil

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"syntopica-backend/internal/platform/database"
)

// ── Safety contract ──────────────────────────────────────────────────────────
//
// This package connects ONLY to a throwaway Postgres started in an isolated
// Docker container via testcontainers-go. It has NO default DSN and reads NO
// environment variable that could redirect it at the developer's docker-compose
// Postgres (which is the production database). An earlier revision connected to
// the production database and truncated it — that path no longer exists.

const (
	// pgImage is the throwaway container image. Same image as production
	// (docker-compose.pg.yml) so pgvector behavior matches, but started in its
	// own container that is destroyed when the test process exits.
	pgImage = "pgvector/pgvector:pg18-trixie"

	testDBName = "syntopica"
	testDBUser = "postgres"
	testDBPass = "postgres"
)

var (
	startOnce   sync.Once // starts the container + opens the connection, once per process
	migrateOnce sync.Once // runs migrations, once per process
	cachedDB    *gorm.DB
	startErr    error
	migrateErr  error
)

// OpenTestDB returns a *gorm.DB connected to a throwaway pgvector Postgres
// running in an isolated Docker container (started via testcontainers-go).
//
// The container is started once per test process and reused by every test in
// the process. The Testcontainers Ryuk sidecar terminates it on process exit,
// so no manual cleanup is needed.
//
// The returned connection NEVER points at the production database: there is no
// default DSN and no environment-variable override. This is deliberate.
func OpenTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	startOnce.Do(func() {
		startErr = startContainerAndConnect()
	})

	if startErr != nil {
		t.Fatalf("start test database container (requires Docker running): %v\n"+
			"hint: start Docker Desktop / daemon", startErr)
	}

	return cachedDB
}

// startContainerAndConnect starts the isolated pgvector container and opens a
// cached *gorm.DB against it. Runs exactly once per process via startOnce.
func startContainerAndConnect() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ctr, err := tcpostgres.Run(ctx,
		pgImage,
		tcpostgres.WithDatabase(testDBName),
		tcpostgres.WithUsername(testDBUser),
		tcpostgres.WithPassword(testDBPass),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		return fmt.Errorf("run postgres container: %w", err)
	}
	// Container cleanup is delegated to the Testcontainers Ryuk sidecar, which
	// runs on process exit (including crashes). We intentionally do NOT call
	// TerminateContainer here: the container is process-singleton and must
	// outlive each individual test function.

	connStr, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return fmt.Errorf("get container connection string: %w", err)
	}

	db, err := gorm.Open(postgres.Open(connStr), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("gorm open: %w", err)
	}

	cachedDB = db
	return nil
}

// SetupTestDB is the single entry point for integration tests (D2, D11).
// It:
//  1. Skips when running with -short flag (D11).
//  2. Starts (or reuses) the isolated pgvector container and connection (D10).
//  3. Runs AutoMigrate on ALL domain models exactly once per process (D10).
//  4. Truncates all tables for test isolation (D4).
//  5. Sets database.DB for production code compatibility (D6).
//
// Every integration test should start with: db := testutil.SetupTestDB(t)
func SetupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	if testing.Short() {
		t.Skip("requires Postgres (testcontainers)")
	}

	db := OpenTestDB(t)

	// Migrate all domain models exactly once per process.
	migrateOnce.Do(func() {
		migrateErr = runTestMigrations(db)
	})

	if migrateErr != nil {
		t.Fatalf("test db migration: %v", migrateErr)
	}

	TruncateAllTables(t, db)

	database.DB = db
	return db
}

// TruncateAllTables truncates all user tables using CASCADE (D4).
func TruncateAllTables(t *testing.T, db *gorm.DB) {
	t.Helper()

	var tables []string
	if err := db.Raw(
		"SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE'",
	).Scan(&tables).Error; err != nil {
		t.Fatalf("list tables: %v", err)
	}

	if len(tables) == 0 {
		return
	}

	if err := db.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", strings.Join(tables, ", "))).Error; err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
}

// TestEmbeddingDim is the vector dimension enforced by the production schema
// (semantic_labels.embedding/merge_embedding and topic_tag_embeddings.embedding
// are vector(4096)).
const TestEmbeddingDim = 4096

// PadVector zero-pads vec to dim. Integration tests construct small semantic
// vectors (e.g. []float64{1,0,0}) for deterministic cosine geometry, but the
// Postgres vector(4096) column rejects shorter vectors. PadVector bridges the
// gap without obscuring the test's geometric intent at call sites. The padding
// zeros do not change cosine similarity between two padded vectors.
func PadVector(vec []float64, dim int) []float64 {
	if len(vec) >= dim {
		return vec
	}
	out := make([]float64, dim)
	copy(out, vec)
	return out
}

// runTestMigrations applies all migrations needed for tests on the freshly
// started container: pgvector extension, AutoMigrate (all domain models),
// vector column type, unique indexes, and seed data tests depend on.
//
// The container is always a brand-new empty database, so there is no DROP step
// (an earlier revision dropped tables here — that was both unnecessary and
// dangerous; it is intentionally absent).
func runTestMigrations(db *gorm.DB) error {
	// Enable pgvector extension.
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error; err != nil {
		return fmt.Errorf("enable pgvector: %w", err)
	}

	// AutoMigrate all domain models (reuses the same list as production).
	if err := database.RunAutoMigrate(db); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}

	// Ensure the embedding column is vector(4096) — GORM AutoMigrate cannot
	// change column types, so we do it explicitly (same as production migration).
	if err := db.Exec("ALTER TABLE topic_tag_embeddings ADD COLUMN IF NOT EXISTS embedding vector(4096)").Error; err != nil {
		return fmt.Errorf("add embedding column: %w", err)
	}
	if err := db.Exec("ALTER TABLE topic_tag_embeddings ALTER COLUMN embedding TYPE vector(4096)").Error; err != nil {
		return fmt.Errorf("set embedding type: %w", err)
	}

	// Unique index on (topic_tag_id, embedding_type, text_hash) — required for
	// SaveEmbedding upsert logic.
	if err := db.Exec(
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_topic_tag_embeddings_tag_type_hash ON topic_tag_embeddings(topic_tag_id, embedding_type, text_hash)",
	).Error; err != nil {
		return fmt.Errorf("create embedding unique index: %w", err)
	}

	// Seed semantic board settings that tests depend on.
	if err := seedSemanticBoardSettings(db); err != nil {
		return fmt.Errorf("seed semantic board settings: %w", err)
	}

	// Seed event clustering config keys.
	if err := seedEventClusterConfig(db); err != nil {
		return fmt.Errorf("seed event cluster config: %w", err)
	}

	return nil
}

// seedSemanticBoardSettings inserts the AISettings defaults required by
// semantic board matching/upgrade tests. Uses ON CONFLICT to be idempotent.
func seedSemanticBoardSettings(db *gorm.DB) error {
	settings := []struct{ Key, Value, Description string }{
		{"semantic_board_match_sim_threshold", "0.6", "Minimum auxiliary label similarity counted as a SemanticBoard match"},
		{"semantic_board_match_direct_hit_rate", "0.5", "Minimum direct auxiliary label hit rate for a SemanticBoard match"},
		{"semantic_board_match_direct_max_sim", "0.8", "Maximum similarity threshold for direct SemanticBoard matching"},
		{"semantic_board_match_direct_max_sim_min_hits", "2", "Minimum number of auxiliary label hits required for max_sim matching rule"},
		{"semantic_board_match_direct_max_sim_min_hit_rate", "0.3", "Minimum auxiliary label hit rate required for max_sim matching rule"},
		{"semantic_board_match_min_effective_sample", "3", "Minimum denominator for hit rate calculation"},
		{"semantic_board_match_hit_rate_sim_blend", "0.7", "Weight of maxSimilarity in hit_rate rule score"},
		{"semantic_board_match_weight_sim", "0.6", "Similarity weight used in weighted SemanticBoard matching"},
		{"semantic_board_match_weight_density", "0.4", "Density weight used in weighted SemanticBoard matching"},
		{"semantic_board_match_weighted_threshold", "0.6", "Minimum weighted score for assigning a topic tag to a SemanticBoard"},
		{"semantic_board_match_direct_hit_min_overlap", "2", "Minimum auxiliary label overlap count for direct_hit matching rule"},
		{"semantic_board_match_max_boards", "3", "Maximum SemanticBoard matches retained for each topic tag"},
		{"semantic_board_upgrade_ref_count_threshold", "5", "Minimum reference count before suggesting a new SemanticBoard"},
		{"semantic_board_upgrade_cluster_distance_threshold", "0.35", "Cluster distance threshold for SemanticBoard upgrade suggestions"},
		{"semantic_board_upgrade_cotag_window_days", "30", "Co-tag analysis window in days"},
		{"semantic_board_upgrade_cotag_top_n", "20", "Maximum co-tag candidates considered"},
		{"semantic_board_upgrade_cotag_dedupe_sim_threshold", "0.85", "Similarity threshold for deduplicating co-tag upgrade candidates"},
		{"semantic_board_upgrade_cotag_hard_limit", "15", "Hard limit for co-tag upgrade candidates"},
	}
	for _, s := range settings {
		if err := db.Exec(
			"INSERT INTO ai_settings (key, value, description, created_at, updated_at) VALUES (?, ?, ?, NOW(), NOW()) ON CONFLICT (key) DO NOTHING",
			s.Key, s.Value, s.Description,
		).Error; err != nil {
			return err
		}
	}
	return nil
}

// seedEventClusterConfig inserts the embedding_config defaults for event clustering.
func seedEventClusterConfig(db *gorm.DB) error {
	defaults := []struct{ Key, Value, Description string }{
		{"event_cluster_kw_min_overlap", "2", "Minimum shared keyword count for Stage 1 event tag keyword-overlap clustering"},
		{"event_cluster_sem_threshold", "0.80", "Minimum semantic cosine similarity for Stage 2 event tag clustering filter"},
	}
	for _, d := range defaults {
		if err := db.Exec(
			"INSERT INTO embedding_config (key, value, description) VALUES (?, ?, ?) ON CONFLICT (key) DO NOTHING",
			d.Key, d.Value, d.Description,
		).Error; err != nil {
			return err
		}
	}
	return nil
}
