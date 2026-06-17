# demo/seed/seed.sql

Sanitized snapshot of real Syntopica business data, used to bootstrap the
public read-only demo instance. Imported by `demo/entrypoint.sh` after the
backend has created the schema via AutoMigrate + versioned migrations.

## What's in it

- Recent business data only (last 30 days by default; configurable via
  `EXPORT_DAYS`). This sidesteps the backend's `CURRENT_DATE - N days` query
  filters, so the demo is never blank.
- `INSERT` statements + `setval(...)` sequence resets, in topological order
  (the only physical FK — `topic_tags_merged_into_id_fkey` — is satisfied by
  splitting `topic_tags` into an unmerged batch then a merged batch).
- **No schema**: tables/indexes/triggers/pgvector columns are built by the app
  at startup. Do not hand-edit the DDL out of this file — there isn't any.

## What's sanitized (conservative policy)

| Field | Treatment |
| --- | --- |
| `ai_providers.api_key`, `base_url`, `metadata` | cleared / `{}` |
| `articles.link`, `image_url` | URL query string stripped |
| `articles.content`, `ai_content_summary` | capped at 2000 chars |
| `articles.firecrawl_content`, `firecrawl_error`, `completion_error` | cleared |
| `feeds.url` | URL query string stripped |
| `feeds.refresh_error` | cleared |
| `reading_behaviors.session_id` | SHA-256 hashed (joins still work) |
| All pgvector `embedding` columns | `NULL` |
| `scheduler_tasks.last_error`, `last_execution_result` | cleared |

## What's excluded entirely

`schema_migrations` (would suppress migrations on the fresh DB), `ai_call_logs`
(request/response snippets), `otel_spans`, `topic_tag_analyses`,
`topic_analysis_cursors`, `tag_merge_suggestions`, queue tables
(`embedding_queues`, `tag_jobs`, `firecrawl_jobs`).

## Regenerating

From a machine with access to a real running database:

```bash
cd backend-go
go run ./cmd/dump-sanitizer
# → writes ../demo/seed/seed.sql
```

Flags / env:

| Var / flag | Default | Purpose |
| --- | --- | --- |
| `DATABASE_DSN` | `configs/config.yaml` | source DB DSN |
| `EXPORT_DAYS` / `--days` | `30` | recent-data window |
| `SEED_OUT` / `--out` | `../demo/seed/seed.sql` | output path |

> **Security review**: after regenerating, confirm the file contains no
> `INSERT INTO ai_call_logs`, no `INSERT INTO schema_migrations`, and that every
> `ai_providers` row has empty `api_key`. See the verification steps in
> `openspec/changes/public-read-only-demo/tasks.md` §9.
