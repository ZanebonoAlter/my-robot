# Design: test-standardize

## Context

### Current State

1. **22 test files** import `github.com/glebarez/sqlite`. Each duplicates its own `setupXxxTestDB()` helper (~15–20 lines) that opens SQLite, sets `database.DB`, and runs `AutoMigrate` on domain-specific model subsets.

2. **No shared testutil package** exists. Every domain test reinvents DB setup independently, leading to:
   - Inconsistent model sets passed to `AutoMigrate` (some tests miss FK-related tables)
   - No standardized test isolation (no `TRUNCATE` between tests)
   - Global `database.DB` mutated directly without cleanup

3. **Tests exercise different code paths than production** because SQLite lacks pgvector. Production uses `pgvector/pgvector:pg18-trixie` with vector columns and cosine distance operators. SQLite tests cannot test:
   - `embedding` (vector) column operations
   - pgvector `<=>` distance queries
   - Migration SQL that creates extensions/indexes

4. **Production code carries SQLite fallback paths**. `auxiliary_label_service.go:168` comment reads "SQL for pgvector, Go fallback for SQLite tests". The `sqlMergeMatcher` function loads embeddings as text and computes cosine similarity in Go — a code path that only exists for SQLite compatibility and is never used in production.

5. **Package structure has changed significantly** since the original proposal. The old `internal/domain/tagging/` flat package is now `internal/tagmanagement/` with a handler/repository/service layer split:

   ```
   internal/tagmanagement/
   ├── handler/          ← 3 test files (2 SQLite)
   ├── repository/       ← 2 test files (2 SQLite)
   └── service/
       ├── auxlabel/     ← 1 test file  (1 SQLite)
       ├── board/        ← 4 test files (4 SQLite)
       ├── core/         ← 12 test files (5 SQLite, 7 no-DB)
       └── merge/        ← 1 test file  (pure unit, no SQLite)
   ```

   This means 12 SQLite test files exist in `tagmanagement/` (not the originally estimated 5 in `domain/tagging/`).

6. **Docker Postgres is already available** via `docker-compose.pg.yml` (`pgvector/pgvector:pg18-trixie`, port 5432, db/user/password all `postgres` → `syntopica`). Developers already run this for local development.

### Problem

Tests pass on SQLite but don't validate production behavior. Embedding similarity, vector queries, and pgvector-specific SQL are untested. The SQLite fallback code in production adds complexity solely for test compatibility.

## Goals

1. **Create `testutil` package** with a single `SetupTestDB(t)` function — connect, migrate all models, truncate, set `database.DB`, return `*gorm.DB`.
2. **Migrate ALL 12 tagmanagement SQLite test files** to Postgres, bottom-up: repository → service/core → service/auxlabel+board → handler.
3. **Define test tiers**: `-short` skips DB-dependent tests; default runs integration tests against Postgres.
4. **Evaluate and clean up SQLite fallback** in `auxiliary_label_service.go`.
5. **Leave `glebarez/sqlite` in go.mod** for now — reader/admin/topicgraph still have 10 SQLite files (out of scope).
6. **Establish file naming convention** to separate unit and integration tests within the same package.
7. **Split 7 mixed test files** that currently contain both unit and integration test functions.

## Non-Goals

- Migrating reader/admin/topicgraph SQLite tests. Those are pure CRUD with no pgvector dependency; low ROI.
- Changing how `database.DB` global is used. The testutil will continue to set it for compatibility; refactoring to DI is orthogonal.
- Introducing testcontainers-go. Existing Docker Postgres is sufficient.
- Adding new test cases. Goal is to make existing tests run against Postgres, not expand coverage.

## Decisions

### D1: Pre-existing Docker Postgres over testcontainers

**Decision**: Use the existing `docker-compose.pg.yml` Postgres for tests, not testcontainers-go.

**Rationale**: Developers already have Docker Postgres running for local dev. testcontainers adds a dependency, increases startup time (~5–10s per run), and needs Docker-in-Docker in CI. `OpenTestDB` reads `TEST_DB_DSN` env var, defaulting to `host=localhost port=5432 user=postgres password=postgres dbname=syntopica sslmode=disable`.

### D2: Single SetupTestDB — migrate all models, not per-package subsets

**Decision**: `SetupTestDB(t)` migrates ALL domain models unconditionally. No `SetupTestDB(t, models...)` variant.

**Rationale**: "Migrate all" is only ~0.3s slower than "migrate subset" (AutoMigrate is idempotent and fast on existing tables). Per-package model subsets cause FK issues and cognitive overhead. One function, zero decisions.

### D3: Migrate all 12 tagmanagement files, not just 5

**Decision**: Migrate the full set of 12 SQLite test files in `tagmanagement/`, not the originally scoped 5.

**Rationale**: After the package split, the original 5 files became scattered across sub-packages and 7 additional files appeared. Leaving 7 files on SQLite means two setup patterns coexist in the same domain, the fallback code can't be fully cleaned, and developers have to remember which tests need Docker. 12 files all use the same template, so migration is mechanical — ~15 lines changed per file.

Migration order (bottom-up):

| Batch | Package | Files | pgvector dependency |
|-------|---------|-------|-------------------|
| 1 | repository | tagger_embedding_test, tag_job_queue_test | 🔴 极高, 🟢 低 |
| 2 | service/core | embedding_test, quality_score_test, metadata_test, hard_merge_test, merge_tags_reembedding_test | 🔴🟢🟢🟡🟡 |
| 3 | service/auxlabel + board | auxiliary_label_service_test, semantic_board_matching_test, semantic_board_upgrade_test, semantic_board_backfill_test, tag_clustering_test | 🔴🔴🟡🟡🟡 |
| 4 | handler | semantic_board_handler_test, merge_reembedding_queue_test | 🟡 🟡 |

### D4: TruncateAllTables for test isolation

**Decision**: `TruncateAllTables` uses `TRUNCATE TABLE ... CASCADE` on all user tables. Called inside `SetupTestDB`.

**Rationale**: Only ~15 domain tables. Truncate on empty tables is sub-millisecond. Ensures no cross-test data leakage. Tests that need a clean slate call `SetupTestDB` (which truncates after migrating).

### D5: sqlMergeMatcher — benchmark before removing Go-side path

**Decision**: After migrating `auxiliary_label_service_test.go` to Postgres, benchmark `sqlMergeMatcher`'s Go-side cosine computation vs pgvector SQL for 2560-dim vectors.

- If pgvector SQL is comparable or faster → replace with `ORDER BY embedding <=> $1`
- If Go-side is faster → keep it, but remove "SQLite fallback" framing and reframe as intentional performance optimization

**Caveat**: The existing comment notes "pgvector HNSW cannot index vector(2560)" and "halfvec expression indexes are not recognized by the query planner". The Go-side computation may genuinely be faster for 2560-dim vectors.

### D7: File naming convention for test tier separation

**Decision**: Use file naming to distinguish unit vs integration tests within the same package:

| Pattern | Meaning | DB required | `testing.Short()` |
|---------|---------|-------------|--------------------|
| `xxx_test.go` | Integration test | Yes (Postgres) | Yes (skip) |
| `xxx_unit_test.go` | Unit test | No | No |
| `tag_context_dump_test.go` | Real LLM verification | Yes (Postgres + LLM) | Yes (skip) |

**Rationale**: Tests stay co-located with source code (Go convention, preserves access to unexported symbols — see D8). File naming gives developers a clear visual distinction. `go test -short` selects unit tests only.

### D8: Same-package co-location over separate test directories

**Decision**: Tests remain in the same directory as source code, not in a separate `tests/` tree.

**Rationale**: Analysis shows **70+ unexported functions** are referenced by test files across the codebase (e.g., `evaluateSemanticBoardMatches`, `buildTagEmbeddingText`, `hungarianAssignment`, `findConnectedComponents`). Moving tests to a separate package would require either:
1. Exporting internal implementation details (breaks encapsulation)
2. Bridge/test_helper files (maintenance burden)
3. `xxx_test` package (loses access to unexported symbols entirely)

None of these are acceptable tradeoffs. The Go community convention is co-location, and it serves this codebase well.

### D9: Split mixed test files

**Decision**: 7 test files currently mix unit and integration test functions. Split each into two files using the naming convention from D7.

Files to split:

| Source file | Unit tests → new file | Integration tests stay |
|-------------|----------------------|----------------------|
| `platform/database/db_test.go` | 3 functions → `db_unit_test.go` | 1 function |
| `reader/service/feed_service_test.go` | 1 function → `feed_service_unit_test.go` | 5 functions |
| `tagmanagement/service/board/semantic_board_matching_test.go` | 5 functions → `semantic_board_matching_unit_test.go` | 6 functions |
| `tagmanagement/service/board/tag_clustering_test.go` | 6 functions → `tag_clustering_unit_test.go` | 3 functions |
| `tagmanagement/service/core/embedding_test.go` | 10 functions → `embedding_unit_test.go` | 3 functions |
| `tagmanagement/service/core/metadata_test.go` | 1 function → `metadata_unit_test.go` | 1 function |
| `tagmanagement/service/core/quality_score_test.go` | 1 function → `quality_score_unit_test.go` | 1 function |

**Rationale**: Splitting ensures `go test -short` runs exactly the unit tests, with no DB dependency. The split is mechanical — move functions, adjust imports, no logic changes.

### D6: Keep database.DB assignment in testutil for compatibility

**Decision**: `SetupTestDB` sets `database.DB = db` before returning. Migrated tests don't need to do this manually.

**Rationale**: Many service constructors and repository helpers read `database.DB` directly. Removing this global would require refactoring DI across the entire codebase — orthogonal to this change. The testutil encapsulates this once, so individual tests stay clean.

## Risks / Trade-offs

| Risk | Mitigation |
|------|------------|
| Tests require running Docker Postgres | `OpenTestDB` fails with clear message: "Start Postgres: docker compose -f docker-compose.pg.yml up -d". `-short` flag lets unit tests run without Docker. |
| SQLite tests that relied on missing FK constraints may fail on Postgres | This is a feature — catching real constraint violations. Fix test data setup. |
| 10 remaining SQLite test files in other packages | Out of scope. They're pure CRUD, no pgvector dependency. `glebarez/sqlite` stays in go.mod. |
| sqlMergeMatcher may be slower with pgvector SQL for high-dim vectors | Benchmark before changing (D5). Keep Go-side path if faster. |
| `database.DB` global means tests can't run in parallel safely | Accepted for now. DI refactoring is orthogonal. |

## File Impact

| File | Action |
|------|--------|
| `backend-go/internal/platform/testutil/testutil.go` | **Create** — `OpenTestDB`, `SetupTestDB`, `TruncateAllTables` |
| `backend-go/internal/tagmanagement/repository/tagger_embedding_test.go` | **Modify** — SQLite → testutil |
| `backend-go/internal/tagmanagement/repository/tag_job_queue_test.go` | **Modify** — SQLite → testutil |
| `backend-go/internal/tagmanagement/service/core/embedding_test.go` | **Modify** — SQLite → testutil |
| `backend-go/internal/tagmanagement/service/core/quality_score_test.go` | **Modify** — SQLite → testutil |
| `backend-go/internal/tagmanagement/service/core/metadata_test.go` | **Modify** — SQLite → testutil |
| `backend-go/internal/tagmanagement/service/core/hard_merge_test.go` | **Modify** — SQLite → testutil |
| `backend-go/internal/tagmanagement/service/core/merge_tags_reembedding_test.go` | **Modify** — SQLite → testutil |
| `backend-go/internal/tagmanagement/service/auxlabel/auxiliary_label_service_test.go` | **Modify** — SQLite → testutil |
| `backend-go/internal/tagmanagement/service/board/semantic_board_matching_test.go` | **Modify** — SQLite → testutil |
| `backend-go/internal/tagmanagement/service/board/semantic_board_upgrade_test.go` | **Modify** — SQLite → testutil |
| `backend-go/internal/tagmanagement/service/board/semantic_board_backfill_test.go` | **Modify** — SQLite → testutil |
| `backend-go/internal/tagmanagement/service/board/tag_clustering_test.go` | **Modify** — SQLite → testutil |
| `backend-go/internal/tagmanagement/handler/semantic_board_handler_test.go` | **Modify** — SQLite → testutil |
| `backend-go/internal/tagmanagement/handler/merge_reembedding_queue_test.go` | **Modify** — SQLite → testutil |
| `backend-go/internal/tagmanagement/service/auxlabel/auxiliary_label_service.go` | **Modify** — evaluate/clean SQLite fallback |
| `backend-go/internal/platform/database/db_unit_test.go` | **Create** — 3 unit tests extracted from db_test.go |
| `backend-go/internal/reader/service/feed_service_unit_test.go` | **Create** — 1 unit test extracted from feed_service_test.go |
| `backend-go/internal/tagmanagement/service/board/semantic_board_matching_unit_test.go` | **Create** — 5 unit tests extracted |
| `backend-go/internal/tagmanagement/service/board/tag_clustering_unit_test.go` | **Create** — 6 unit tests extracted |
| `backend-go/internal/tagmanagement/service/core/embedding_unit_test.go` | **Create** — 10 unit tests extracted |
| `backend-go/internal/tagmanagement/service/core/metadata_unit_test.go` | **Create** — 1 unit test extracted |
| `backend-go/internal/tagmanagement/service/core/quality_score_unit_test.go` | **Create** — 1 unit test extracted |
| `backend-go/internal/platform/database/db_test.go` | **Modify** — remove extracted unit tests |
| `backend-go/internal/reader/service/feed_service_test.go` | **Modify** — remove extracted unit test |
| `backend-go/internal/tagmanagement/service/board/semantic_board_matching_test.go` | **Modify** — remove extracted unit tests |
| `backend-go/internal/tagmanagement/service/board/tag_clustering_test.go` | **Modify** — remove extracted unit tests |
| `backend-go/internal/tagmanagement/service/core/embedding_test.go` | **Modify** — remove extracted unit tests |
| `backend-go/internal/tagmanagement/service/core/metadata_test.go` | **Modify** — remove extracted unit test |
| `backend-go/internal/tagmanagement/service/core/quality_score_test.go` | **Modify** — remove extracted unit test |
