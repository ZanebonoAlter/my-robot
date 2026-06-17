# database-docs

## Purpose

TBD: This spec governs the accuracy and maintenance of database documentation files (`docs/reference/database/*.md`) relative to the actual codebase.

## Requirements

### Requirement: DATABASE_FIELDS.md SHALL reference existing Go models

The `database/DATABASE_FIELDS.md` document SHALL reference Go model paths and table-model mappings that exist in the codebase.

#### Scenario: topic_analysis_jobs mapping is corrected
- **WHEN** a developer reads the main table mapping for `topic_analysis_jobs`
- **THEN** the document SHALL NOT claim the table maps to `topicanalysis.topicAnalysisJobRecord` (the `topicanalysis` package and `TopicAnalysisJob` model do not exist; the table is not registered in `migrator.go`)
- **AND** SHALL either remove the row or relocate it to the "no Go code reference / deprecated" section

#### Scenario: model path uses current location
- **WHEN** a developer reads the `ai_summaries` status note
- **THEN** the document SHALL reference `internal/models/` (the current shared model location)
- **AND** SHALL NOT reference `internal/domain/models/` (deleted)

### Requirement: DATA_LIFECYCLE.md SHALL NOT reference removed schedulers

The `database/DATA_LIFECYCLE.md` document SHALL describe data flows using schedulers that are actually registered in `runtime.go`.

#### Scenario: No AutoTagMerge scheduler references
- **WHEN** a developer reads the tag merge data flow diagram
- **THEN** the document SHALL NOT reference an `AutoTagMerge` scheduler running at 3600s (not registered in `runtime.go`)
- **AND** SHALL align the merge flow with the actual mechanisms (manual `HardMergeTags` + `tag_quality_score` recomputation)

#### Scenario: No narrative_summary / NarrativeSummaryScheduler references
- **WHEN** a developer reads the narrative / ai_call_logs data flow
- **THEN** the document SHALL NOT reference `narrative_summary` as a capability/scheduler or `NarrativeSummaryScheduler` (not registered in `runtime.go`)
- **AND** SHALL align the narrative flow with the actual current schedulers registered in `runtime.go` (e.g. `daily_report`)
