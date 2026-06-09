# Design: test-standardize

## Context

### Current State

1. **23 test files** import `github.com/glebarez/sqlite` to create in-memory SQLite databases. Each test file duplicates its own `setupXxxTestDB()` helper (~15–20 lines each) that opens SQLite, sets `database.DB`, and runs `AutoMigrate` on domain-specific model subsets.

2. **No shared testutil package** exists under `backend-go/internal/platform/`. Every domain test reinvents DB setup independently, leading to:
   - Inconsistent model sets passed to `AutoMigrate` (some tests miss foreign-key-related tables)
   - No standardized test isolation (no `TRUNCATE` between tests)
   - Global `database.DB` mutated directly without cleanup

3. **Tests exercise different code paths than production** because SQLite lacks pgvector. Production uses `pgvector/pgvector:pg18-trixie` with vector columns and cosine distance operators. SQLite tests cannot test:
   - `embedding` (vector) column operations
   - pgvector `<=>` distance queries
   - Migration SQL that creates extensions/indexes

4. **Production code carries SQLite fallback paths**. `auxiliary_label_service.go:167` comment reads "SQL for pgvector, Go fallback for SQLite tests". The `sqlMergeMatcher` function loads embeddings as text and computes cosine similarity in Go — a code path that only exists for SQLite compatibility and is never used in production (production uses pgvector SQL).

5. **One file** (`tag_context_dump_test.go`) already uses `testing.Short()` for real-DB tests, establishing a precedent for tiered testing.

6. **Docker Postgres is already available** via `docker-compose.pg.yml` (`pgvector/pgvector:pg18-trixie`, port 5432, db/user/password all `postgres` → `syntopica`). Developers already run this for local development.

### Problem

Tests pass on SQLite but don't validate production behavior. Embedding similarity, vector queries, and pgvector-specific SQL are untested. The SQLite fallback code in production adds complexity and a maintenance burden solely for test compatibility.

## Goals

1. **Create `testutil` package** with `OpenTestDB`, `SetupTestDB`, `TruncateAllTables` — shared, reusable test DB helpers backed by real Postgres.
2. **Migrate priority test files** from SQLite to Postgres: `semantic_board_matching_test.go`, `embedding_test.go`, `auxiliary_label_service_test.go`, `semantic_board_upgrade_test.go`, `semantic_board_handler_test.go`.
3. **Define test tiers**: `-short` skips DB-dependent tests; default runs integration tests against Postgres.
4. **Remove SQLite dependency** (`github.com/glebarez/sqlite`) from `go.mod`.
5. **Remove SQLite fallback code** from production (Go-side cosine comparison in `sqlMergeMatcher`).

## Non-Goals

- Migrating all 23 SQLite test files in one change. Only the 5 priority tagging/ files are in scope. Remaining files (aiadmin, article, content, feed, narrative, topicgraph) migrate incrementally.
- Introducing testcontainers-go. The existing Docker Postgres is sufficient for now (see Decisions).
- Adding new test cases. The goal is to make existing tests run against Postgres, not to expand coverage.
- Changing the CI pipeline. CI already needs Docker Postgres; no new infrastructure required.

## Decisions

### D1: Pre-existing Docker Postgres over testcontainers

**Decision**: Use the existing `docker-compose.pg.yml` Postgres for tests, not testcontainers-go.

**Rationale**:
- Developers already have `docker compose -f docker-compose.pg.yml up -d` running for local development.
- testcontainers-go adds a dependency, increases test startup time (~5–10s container spin-up per test run), and requires Docker-in-Docker support in CI.
- The testutil `OpenTestDB` connects to `localhost:5432/syntopica` (configurable via env vars `TEST_DB_DSN`), failing fast with a clear error message if Postgres is unavailable.
- If testcontainers is needed later (e.g., parallel test isolation with separate databases), `OpenTestDB` can be extended without changing its callers.

**Connection**: `OpenTestDB` reads `TEST_DB_DSN` env var, defaulting to `host=localhost port=5432 user=postgres password=postgres dbname=syntopica sslmode=disable`.

### D2: Priority test files (5 files in tagging/)

**Decision**: Migrate these 5 files first, ordered by pgvector dependency:

| Priority | File | Reason |
|----------|------|--------|
| P0 | `semantic_board_matching_test.go` | Tests core board-tag matching with embeddings |
| P0 | `embedding_test.go` | Tests embedding CRUD and similarity queries |
| P1 | `auxiliary_label_service_test.go` | Tests label matching that uses merge embeddings |
| P1 | `semantic_board_upgrade_test.go` | Tests upgrade suggestions using embeddings |
| P2 | `semantic_board_handler_test.go` | Handler tests that depend on matching logic |

**Rationale**: These files exercise the highest-value pgvector-dependent code paths. Once migrated, the `sqlMergeMatcher` fallback can be removed.

### D3: Migration strategy — incremental, file-by-file

**Decision**: Each test file is migrated independently. For each file:
1. Replace `setupXxxTestDB()` with `testutil.SetupTestDB(t)`
2. Replace `database.DB = db` with the returned `*gorm.DB` passed explicitly
3. Add `t.Parallel()` where tests are independent (testutil uses per-test schema or TRUNCATE for isolation)
4. Verify tests pass against Postgres

**Rationale**: File-by-file migration avoids a big-bang rewrite. Each migrated file is independently verifiable. If a migration reveals a test that only passed on SQLite due to missing constraints, it's caught immediately.

### D4: TruncateAllTables for test isolation

**Decision**: `TruncateAllTables` uses `TRUNCATE TABLE ... CASCADE` on all tables in reverse-dependency order. Each test that needs a clean DB calls `SetupTestDB` (which truncates after migrating) or calls `TruncateAllTables` between subtests.

**Rationale**: Parallel tests need separate database connections but can share the same schema. Truncate is fast (~1ms for empty tables) and ensures no cross-test data leakage.

### D5: sqlMergeMatcher simplification

**Decision**: After migrating `auxiliary_label_service_test.go` to Postgres, replace `sqlMergeMatcher`'s Go-side cosine computation with a pgvector SQL query. The function currently loads embeddings as text, parses them, and computes cosine in Go — this is the SQLite fallback. With Postgres, we can use `ORDER BY embedding <=> $1 LIMIT 1`.

**Caveat**: The existing comment notes "pgvector HNSW cannot index vector(2560)" and "halfvec expression indexes are not recognized by the query planner". The Go-side computation may actually be faster than a pgvector full scan for 2560-dim vectors. This decision requires benchmarking before removing the Go-side path. If Go-side is indeed faster, keep it but remove the SQLite-specific comment and acknowledge it as an intentional performance optimization, not a SQLite fallback.

## Risks / Trade-offs

| Risk | Mitigation |
|------|------------|
| Tests require running Docker Postgres | `OpenTestDB` fails with clear message: "Start Postgres: docker compose -f docker-compose.pg.yml up -d". `-short` flag lets unit tests run without Docker. |
| SQLite tests that relied on missing FK constraints may fail on Postgres | This is a feature — catching real constraint violations. Fix test data setup, not the constraint. |
| TruncateAllTables is slow with many tables | Only ~15 domain tables. Truncate on empty tables is sub-millisecond. Acceptable. |
| sqlMergeMatcher may be slower with pgvector SQL for high-dim vectors | Benchmark before changing. Keep Go-side path if faster; just remove SQLite-specific framing. |
| 18 remaining SQLite test files still need migration | Out of scope for this change. They continue working on SQLite until migrated incrementally. The `glebarez/sqlite` dependency stays until all files are migrated. |

## File Impact

| File | Action |
|------|--------|
| `backend-go/internal/platform/testutil/testutil.go` | **Create** — `OpenTestDB`, `SetupTestDB`, `TruncateAllTables` |
| `backend-go/internal/domain/tagging/semantic_board_matching_test.go` | **Modify** — replace SQLite setup with testutil |
| `backend-go/internal/domain/tagging/embedding_test.go` | **Modify** — replace SQLite setup with testutil |
| `backend-go/internal/domain/tagging/auxiliary_label_service_test.go` | **Modify** — replace SQLite setup with testutil |
| `backend-go/internal/domain/tagging/semantic_board_upgrade_test.go` | **Modify** — replace SQLite setup with testutil |
| `backend-go/internal/domain/tagging/semantic_board_handler_test.go` | **Modify** — replace SQLite setup with testutil |
| `backend-go/internal/domain/tagging/auxiliary_label_service.go` | **Modify** — remove/simplify SQLite fallback code |
| `backend-go/go.mod` | **Modify** — remove `github.com/glebarez/sqlite` (only after all 23 files migrated; may remain in this change if only 5 files are migrated) |
