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
	startOnce sync.Once // starts the container + opens the connection, once per process
	resetMu   sync.Mutex
	cachedDB  *gorm.DB
	startErr  error
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

	// Verbatim copy of production (database.InitDB): AutoMigrate must NOT create
	// FK constraints, because the versioned migrations are the source of truth
	// for which FKs exist. Production drops several FKs on purpose (e.g.
	// fk_topic_tag_relations_*, fk_merge_reembedding_queues_*) so that
	// MergeTags/HardMergeTags can hard-delete a tag still referenced from those
	// tables. With the default gorm.Config{} AutoMigrate would CREATE those FKs
	// and never drop them, leaving the test DB stricter than production and
	// breaking those code paths for no real reason.
	db, err := gorm.Open(postgres.Open(connStr), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return fmt.Errorf("gorm open: %w", err)
	}

	cachedDB = db
	return nil
}

// SetupTestDB is the single entry point for integration tests.
// It:
//  1. Skips when running with -short flag.
//  2. Starts (or reuses) the isolated pgvector container and connection.
//  3. Rebuilds the test schema and imports production migrations and seed data.
//  4. Sets database.DB for production code compatibility.
//
// Every integration test should start with: db := testutil.SetupTestDB(t)
func SetupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	if testing.Short() {
		t.Skip("requires Postgres (testcontainers)")
	}

	db := OpenTestDB(t)
	database.DB = db
	ReimportTestDB(t, db)
	return db
}

// ReimportTestDB rebuilds the isolated test schema and reruns the production
// migration path. Regression tests can call it explicitly to restore the same
// schema and seed data that a fresh production database receives.
func ReimportTestDB(t *testing.T, db *gorm.DB) {
	t.Helper()

	resetMu.Lock()
	defer resetMu.Unlock()

	if err := db.Exec("DROP SCHEMA IF EXISTS public CASCADE").Error; err != nil {
		t.Fatalf("drop test schema: %v", err)
	}
	if err := db.Exec("CREATE SCHEMA public").Error; err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	if err := runTestMigrations(db); err != nil {
		t.Fatalf("import production database state: %v", err)
	}
}

// TruncateAllTables truncates all user tables using CASCADE. This is the reset
// helper for tests that deliberately need an empty database without seed data.
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

// runTestMigrations mirrors the production database initialization path
// (database.InitDB): the same two phases production runs on every startup —
//   - Phase 1: database.RunAutoMigrate (sync all model tables/columns)
//   - Phase 2: database.RunMigrations (versioned migrations — the source of
//     truth for FKs/indexes/triggers and for the ai_settings/embedding_config
//     seed rows tests rely on).
//
// This is intentionally a verbatim copy of production: the testcontainer schema
// and seed data are exactly what production serves. Earlier revisions duplicated
// parts of this by hand (manual embedding column DDL, manual ai_settings /
// embedding_config seeds) — those duplicated production and drifted, so they
// were removed in favor of running the real migration path.
func runTestMigrations(db *gorm.DB) error {
	// Enable pgvector extension (mirrors the first production migration).
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error; err != nil {
		return fmt.Errorf("enable pgvector: %w", err)
	}

	// Phase 1: AutoMigrate — same as database.InitDB phase 1.
	if err := database.RunAutoMigrate(db); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}

	// Phase 2: versioned migrations — same as database.InitDB phase 2. This is
	// the source of truth for FKs/indexes/triggers and the seed data (semantic
	// board / event cluster config) that production and tests both depend on.
	if err := database.RunMigrations(db); err != nil {
		return fmt.Errorf("run versioned migrations: %w", err)
	}

	return nil
}
