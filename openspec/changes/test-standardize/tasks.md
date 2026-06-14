# Tasks: test-standardize

## 1. Create testutil Package

> **Post-incident reversal (see design D1)**: the original tasks assumed a shared `docker-compose.pg.yml` Postgres with a `TEST_DB_DSN` env var and a default `localhost:5432/syntopica` DSN. That design connected tests to the developer's production database and, combined with a `DROP TABLE ... CASCADE` step in migrations, wiped business data. Tasks 1.1–1.3 were re-implemented to use testcontainers-go with a fully isolated throwaway container, no default DSN, and no env-var override.

- [x] 1.0 Add testcontainers-go dependency: `go get github.com/testcontainers/testcontainers-go github.com/testcontainers/testcontainers-go/modules/postgres` (added `testcontainers-go v0.42.0` + postgres module)
- [x] 1.1 Create `backend-go/internal/platform/testutil/testutil.go` with `OpenTestDB(t *testing.T) *gorm.DB` — starts an isolated `pgvector/pgvector:pg18-trixie` container via testcontainers-go on first call and caches the `*gorm.DB` in a package-level `sync.Once` (`startOnce`), subsequent calls return the cached instance (D10); **NO default DSN, NO `TEST_DB_DSN` env-var read** (reversed after incident — see D1); fails with a clear message if Docker is unavailable
- [x] 1.2 Add `SetupTestDB(t *testing.T) *gorm.DB` — first line `if testing.Short() { t.Skip("requires Postgres") }` (D11); obtains the cached connection from `OpenTestDB`; runs `AutoMigrate` on ALL domain models exactly once per process via `migrateOnce` `sync.Once` (D10); calls `TruncateAllTables`; sets `database.DB = db`; returns `*gorm.DB`
- [x] 1.3 Add `TruncateAllTables(t *testing.T, db *gorm.DB)` — queries `information_schema.tables` for user tables, executes `TRUNCATE TABLE ... CASCADE` in reverse-dependency order
- [x] 1.4 Verify testutil compiles: `cd backend-go && go build ./internal/platform/testutil/...`

## 2. Split Mixed Test Files (7 files → 7 new unit files + 7 cleaned integration files)

The following files mix unit tests (no DB) and integration tests (need DB). Extract unit tests into `*_unit_test.go` files so `go test -short` only runs unit tests.

- [x] 2.1 Split `backend-go/internal/platform/database/db_test.go` — extract 3 unit tests (`TestPostgresMigrationsDocumentStagedEmbeddingCutover`, `TestTopicTagAnalysisPayloadJSONExplicitlyStaysTextInModel`, `TestSemanticLabelBoardSystemMigrationDocumentsSchemaCutover`) → `db_unit_test.go`
- [x] 2.2 Split `backend-go/internal/reader/service/feed_service_test.go` — extract 1 unit test (`TestBuildArticleFromEntryTracksOnlyRunnableStates`) → `feed_service_unit_test.go`
- [x] 2.3 Split `backend-go/internal/tagmanagement/service/board/semantic_board_matching_test.go` — extract 5 unit tests (`TestEvaluateSemanticBoardMatches_*`, `TestScoreSemanticBoardSimilarity_*`) → `semantic_board_matching_unit_test.go`
- [x] 2.4 Split `backend-go/internal/tagmanagement/service/board/tag_clustering_test.go` — extract 6 unit tests (`TestLoadClusterConfig_Defaults`, `TestFindConnectedComponents_*`) → `tag_clustering_unit_test.go`
- [x] 2.5 Split `backend-go/internal/tagmanagement/service/core/embedding_test.go` — extract 10 unit tests (`TestBuildTagEmbeddingText*`, `TestFloatsToPgVector`, `TestHashTextDeterministic`, `TestContainsAlias`, `TestEmbeddingDimensionMismatch2560`, `TestGenerateEmbeddingBuildsCorrectDimension`, `TestMatchThreshold`, `TestGetEventKeywords`) → `embedding_unit_test.go`
- [x] 2.6 Split `backend-go/internal/tagmanagement/service/core/metadata_test.go` — extract 1 unit test (`TestLimitArticleTagsKeepsTopFiveInOrder`) → `metadata_unit_test.go`
- [x] 2.7 Split `backend-go/internal/tagmanagement/service/core/quality_score_test.go` — extract 1 unit test (`TestPercentileRankStableRange`) → `quality_score_unit_test.go`

Per-file split checklist:
1. Create new `xxx_unit_test.go` with same `package` declaration
2. Move unit test functions to the new file
3. Move only the imports needed by those functions
4. Move helper functions only used by extracted tests (if any)
5. Remove moved functions from original file, clean up unused imports
6. Verify both files compile: `cd backend-go && go build ./...`

## 3. Migrate tagmanagement SQLite Test Files → Postgres (14 files, bottom-up)

### Batch 1: repository layer

- [x] 3.1 Migrate `backend-go/internal/tagmanagement/repository/tagger_embedding_test.go` — delete `setupTaggerEmbeddingTestDB`, replace with `testutil.SetupTestDB(t)`, remove SQLite import, add `testing.Short()` skip
- [x] 3.2 Migrate `backend-go/internal/tagmanagement/repository/tag_job_queue_test.go` — delete `setupTagJobQueueTestDB`, replace with `testutil.SetupTestDB(t)`, remove SQLite import, add `testing.Short()` skip

### Batch 2: service/core layer (integration part only — unit tests already split)

- [x] 3.3 Migrate `backend-go/internal/tagmanagement/service/core/embedding_test.go` — delete `setupEmbeddingTestDB`, replace with `testutil.SetupTestDB(t)`, remove SQLite import, add `testing.Short()` skip (3 integration tests remain after split)
- [x] 3.4 Migrate `backend-go/internal/tagmanagement/service/core/quality_score_test.go` — delete `setupQualityScoreTestDB`, replace with `testutil.SetupTestDB(t)`, remove SQLite import, add `testing.Short()` skip (1 integration test remains)
- [x] 3.5 Migrate `backend-go/internal/tagmanagement/service/core/metadata_test.go` — delete `setupTopicExtractionTestDB`, replace with `testutil.SetupTestDB(t)`, remove SQLite import, add `testing.Short()` skip (1 integration test remains)
- [x] 3.6 Migrate `backend-go/internal/tagmanagement/service/core/hard_merge_test.go` — delete `setupHardMergeTestDB`, replace with `testutil.SetupTestDB(t)`, remove SQLite import, add `testing.Short()` skip
- [x] 3.7 Migrate `backend-go/internal/tagmanagement/service/core/merge_tags_reembedding_test.go` — delete `setupMergeReembeddingTestDB`, replace with `testutil.SetupTestDB(t)`, remove SQLite import, add `testing.Short()` skip

### Batch 3: service/auxlabel + service/board layer

- [x] 3.8 Migrate `backend-go/internal/tagmanagement/service/auxlabel/auxiliary_label_service_test.go` — delete `setupAuxiliaryLabelTestDB`, replace with `testutil.SetupTestDB(t)`, remove SQLite import, add `testing.Short()` skip
- [x] 3.9 Migrate `backend-go/internal/tagmanagement/service/board/semantic_board_matching_test.go` — delete `setupSemanticBoardMatchingTestDB`, replace with `testutil.SetupTestDB(t)`, remove SQLite import, add `testing.Short()` skip (6 integration tests remain after split)
- [x] 3.10 Migrate `backend-go/internal/tagmanagement/service/board/semantic_board_upgrade_test.go` — delete setup function, replace with `testutil.SetupTestDB(t)`, remove SQLite import, add `testing.Short()` skip
- [x] 3.11 Migrate `backend-go/internal/tagmanagement/service/board/semantic_board_backfill_test.go` — this file does NOT directly import `glebarez/sqlite` but reuses `setupSemanticBoardMatchingTestDB` (deleted in 3.9); update call sites to `testutil.SetupTestDB(t)` (skip is built-in per D11)
- [x] 3.12 Migrate `backend-go/internal/tagmanagement/service/board/tag_clustering_test.go` — delete `setupClusteringTestDB`, replace with `testutil.SetupTestDB(t)`, remove SQLite import, add `testing.Short()` skip (3 integration tests remain after split)

### Batch 4: handler layer

- [x] 3.13 Migrate `backend-go/internal/tagmanagement/handler/semantic_board_handler_test.go` — delete setup function, replace with `testutil.SetupTestDB(t)`, remove SQLite import, add `testing.Short()` skip
- [x] 3.14 Migrate `backend-go/internal/tagmanagement/handler/merge_reembedding_queue_test.go` — delete `setupMergeReembeddingTestDB`, replace with `testutil.SetupTestDB(t)`, remove SQLite import, add `testing.Short()` skip

### Per-file migration checklist (apply to each file above)

For each file:
1. Delete the `setupXxxTestDB()` function (typically 15–20 lines)
2. Replace call sites with `db := testutil.SetupTestDB(t)` — this also handles the `testing.Short()` skip (D11), so no per-function guard is needed
3. Remove `database.DB = db` assignment (testutil handles this)
4. Remove `"github.com/glebarez/sqlite"` from imports
5. Remove `"syntopica-backend/internal/platform/database"` import if no longer used directly

## 4. Evaluate and Clean SQLite Fallback in Production Code

- [x] 4.1 Benchmark `sqlMergeMatcher` Go-side cosine computation vs pgvector `ORDER BY embedding <=> $1` for 2560-dim vectors (per design decision D5)
- [x] 4.2 If pgvector SQL is comparable or faster → replace Go-side cosine with pgvector query
- [x] 4.3 If Go-side is faster → remove "SQLite tests" comments, reframe as intentional performance optimization
- [x] 4.4 Verify no SQLite-specific code paths remain: `grep -ri "sqlite" backend-go/internal/tagmanagement/` (excluding `_test.go` files not yet migrated in other packages)

## 5. Verify

- [x] 5.1 Ensure Docker Postgres is running: `docker compose -f docker-compose.pg.yml up -d`
- [x] 5.2 Run migrated tagmanagement tests: `cd backend-go && go test ./internal/tagmanagement/... -v` — all pass
- [x] 5.3 Run with `-short` flag: `cd backend-go && go test ./internal/tagmanagement/... -short` — migrated tests are skipped, unit tests still run
- [x] 5.4 Run full backend build: `cd backend-go && go build ./...` — no compilation errors
- [x] 5.5 Run linter on changed packages: `cd backend-go && golangci-lint run ./internal/platform/testutil/... ./internal/tagmanagement/...`
- [x] 5.6 Confirm no SQLite imports remain in tagmanagement: `grep -r "glebarez/sqlite" backend-go/internal/tagmanagement/ --include="*_test.go"` → empty

## 6. Documentation

- [x] 6.1 Update `backend-go/AGENTS.md` or `docs/reference/testing.md` with:
  - Test tier convention: `-short` = unit only (no Docker); default = integration (requires Docker Postgres via `TEST_DB_DSN`)
  - File naming convention: `xxx_test.go` = integration, `xxx_unit_test.go` = unit
  - `testutil.SetupTestDB` usage pattern

## 7. CI Configuration (D12)

- [x] 7.1 Update the CI workflow (`.github/workflows/test-ci.yml`) with two jobs:
  - **Unit job**: `go test -short ./...` — no Docker, runs on every PR
  - **Integration job**: `go test ./...` — testcontainers-go starts an isolated `pgvector/pgvector:pg18-trixie` container per test process on the runner (ubuntu-latest ships Docker). No `services.postgres` block, no `TEST_DB_DSN`, no healthcheck step — testcontainers owns the lifecycle (reversed from the original service-container design, see D12)
- [x] 7.2 testcontainers manages readiness internally (`BasicWaitStrategies` waits for `pg_isready` before returning); no separate CI gate needed
- [x] 7.3 Confirm the unit job passes with no Docker available (pure `-short` run)
