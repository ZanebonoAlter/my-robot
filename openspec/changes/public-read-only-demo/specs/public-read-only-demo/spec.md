# Spec — public-read-only-demo

## ADDED Requirements

### Requirement: One-command read-only demo launch

The system SHALL provide a self-contained Docker Compose configuration that launches a fully browsable, read-only demo instance with a single command, requiring no external AI credentials, no RSS sources, and no pre-existing data.

#### Scenario: First-time visitor launches the demo

- **WHEN** a user runs `docker compose -f demo/docker-compose.demo.yml up -d --build` on a machine with only Docker installed
- **THEN** the PostgreSQL (pgvector) and syntopica-demo containers start, the schema is auto-created via AutoMigrate + migrations, the sanitized seed data is imported, and `http://localhost:5000` serves the full application within the startup window
- **AND** all four primary pages (home `/`, topic graph `/topics`, tag management `/tags` with detective wall, settings `/settings`) render real-looking content without errors

#### Scenario: Read-only enforcement blocks all mutations

- **WHEN** the demo is launched with `DEMO_READ_ONLY=1`
- **THEN** all non-GET requests to `/api/*` return HTTP 405 with `{"error":"read-only demo"}`
- **AND** the two side-effecting GET SSE endpoints (`/api/topic-tags/merge-preview/scan/stream`, `/api/topic-tags/merge-preview/evaluate/stream`) return 405
- **AND** no background schedulers run (no RSS refresh, no LLM daily-report generation, no firecrawl crawling)
- **AND** the `/ws` WebSocket endpoint is not registered (frontend degrades silently)

### Requirement: Sanitized seed data export tool

The system SHALL provide a Go CLI tool (`backend-go/cmd/dump-sanitizer`) that connects to a real running database, exports recent business data (configurable window, default 30 days), applies conservative sanitization, and emits a portable `seed.sql` reproducible on a fresh empty database.

#### Scenario: Export produces a sanitized, import-safe seed file

- **WHEN** the operator runs `cd backend-go && go run ./cmd/dump-sanitizer` against a populated database
- **THEN** `demo/seed/seed.sql` is generated containing `INSERT` statements in topological order (respecting the `topic_tags_merged_into_id_fkey` self-reference by splitting `topic_tags` into two batches)
- **AND** the file contains no `api_providers.api_key` values, no `ai_call_logs` rows, no `schema_migrations` rows, and all `reading_behaviors.session_id` values are SHA-256 hashed
- **AND** all pgvector embedding columns (`topic_tag_embeddings.embedding`, `semantic_labels.embedding`, `semantic_labels.merge_embedding`, `daily_report_sections.embedding`) are emitted as NULL
- **AND** each exported table is followed by a `SELECT setval(...)` statement resetting its `id` sequence to `MAX(id)+1`
- **AND** the file is idempotent when re-imported into a fresh database (re-running `docker compose down && up` yields consistent data)

### Requirement: Self-contained demo image with same-origin frontend

The system SHALL build a demo Docker image that compiles both frontend and backend from source (no pre-built local binary), serving the frontend static files from the backend on a single port with same-origin API access.

#### Scenario: Image builds without local prerequisites

- **WHEN** `docker compose -f demo/docker-compose.demo.yml build` runs on a clean checkout
- **THEN** the frontend is built with `NUXT_PUBLIC_API_BASE=/api` injected at generate time (same-origin relative path, no absolute localhost URL baked in)
- **AND** the Go backend is compiled inside the image (multi-stage, no host `go build` required)
- **AND** the runtime stage includes `curl` and `postgresql-client` for the entrypoint health-check and seed import

### Requirement: Zero impact on production paths when demo mode is inactive

The read-only middleware and runtime-skip logic MUST be no-ops when `DEMO_READ_ONLY` is unset or not `"1"`, ensuring production deployments are unaffected.

#### Scenario: Production deployment behaves identically without the flag

- **WHEN** the server starts without `DEMO_READ_ONLY` set
- **THEN** the read-only middleware passes all requests through unconditionally
- **AND** `StartRuntime()` and graceful-shutdown wiring execute exactly as before
- **AND** all write endpoints and schedulers function normally
