package database

import (
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/logging"
	"syntopica-backend/internal/platform/textutil"
)

// PruneRelationsRebuild is an optional hook called after PruneUnderqualifiedCandidates
// deletes topics. When set, it is invoked once per affected semantic board to
// rebuild daily_report_section_relations (drop stale identity/similarity edges).
// Set from a package that imports both database and the relation-rebuild logic.
var PruneRelationsRebuild func(tx *gorm.DB, boardID uint) error

// tableExists reports whether a table exists in the public schema. Migrations
// that target optional/domain-registered tables (e.g. the daily_report_* tables
// registered by internal/topicgraph) use it to no-op when the table is absent,
// rather than failing on CREATE INDEX / ALTER TABLE. This keeps migrations
// safe on deployments that don't register those domain models.
func tableExists(db *gorm.DB, table string) bool {
	var exists bool
	if err := db.Raw(`SELECT to_regclass(?) IS NOT NULL`, "public."+table).Row().Scan(&exists); err != nil {
		return false
	}
	return exists
}

// columnIsNullable reports whether a column is nullable (is_nullable = 'YES').
// Returns true (treat as nullable/safe-to-constrain) on any lookup error so
// callers proceed to apply the constraint.
func columnIsNullable(db *gorm.DB, table, column string) bool {
	var nullable string
	if err := db.Raw(
		`SELECT is_nullable FROM information_schema.columns WHERE table_schema='public' AND table_name=? AND column_name=?`,
		table, column,
	).Row().Scan(&nullable); err != nil {
		return true
	}
	return nullable == "YES"
}

// ensureNotNullDefault backfills NULL rows with a literal default value, then
// sets the column NOT NULL — but only if it is currently nullable, so the
// migration is idempotent (re-running on an already-NOT-NULL column no-ops).
// defaultLit is a raw SQL literal (e.g. ”, 0, false, 'active') used for the
// one-shot backfill UPDATE. SET DEFAULT (if needed) is the caller's separate
// responsibility and is itself idempotent in PostgreSQL.
func ensureNotNullDefault(db *gorm.DB, table, column, defaultLit string) error {
	if !columnIsNullable(db, table, column) {
		return nil // already NOT NULL — idempotent no-op
	}
	if err := db.Exec(fmt.Sprintf(`UPDATE %s SET %s = %s WHERE %s IS NULL`, table, column, defaultLit, column)).Error; err != nil {
		return fmt.Errorf("backfill %s.%s: %w", table, column, err)
	}
	if err := db.Exec(fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN %s SET NOT NULL`, table, column)).Error; err != nil {
		return fmt.Errorf("set %s.%s NOT NULL: %w", table, column, err)
	}
	return nil
}

// postgresMigrations returns versioned migrations for operations that GORM AutoMigrate
// cannot handle: extensions, custom indexes, triggers, data migrations, column/table drops.
//
// Pure ADD COLUMN and CREATE TABLE migrations have been removed — AutoMigrate handles those
// automatically on every startup via RunAutoMigrate(). Only operations requiring explicit SQL
// are kept here.
func postgresMigrations() []Migration {
	return []Migration{
		// ── Extensions ──────────────────────────────────────────────
		{
			Version:     "20260403_0001",
			Description: "Enable pgvector support.",
			Up: func(db *gorm.DB) error {
				return db.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error
			},
		},

		// ── Column type adjustments ─────────────────────────────────
		{
			Version:     "20260403_0003",
			Description: "Staged rollout: set topic_tag_embeddings.embedding column type to vector(4096); runtime dimension configured via embedding_config JSON (embedding_dimension).",
			Up: func(db *gorm.DB) error {
				if err := db.Exec("ALTER TABLE topic_tag_embeddings ADD COLUMN IF NOT EXISTS embedding vector(4096)").Error; err != nil {
					return fmt.Errorf("add topic_tag_embeddings.embedding column: %w", err)
				}
				if err := db.Exec("ALTER TABLE topic_tag_embeddings ALTER COLUMN embedding TYPE vector(4096)").Error; err != nil {
					return fmt.Errorf("set topic_tag_embeddings.embedding dimensions: %w", err)
				}
				return nil
			},
		},

		// ── Seed data ───────────────────────────────────────────────
		{
			Version:     "20260413_0002",
			Description: "Seed embedding_config default values.",
			Up: func(db *gorm.DB) error {
				defaults := []models.EmbeddingConfig{
					{Key: "high_similarity_threshold", Value: "0.97", Description: "Auto-reuse existing tag if similarity >= this value"},
					{Key: "low_similarity_threshold", Value: "0.78", Description: "Auto-create new tag if similarity < this value"},
					{Key: "embedding_model", Value: "", Description: "Override embedding model name (empty = read from provider)"},
					{Key: "embedding_dimension", Value: "1024", Description: "Embedding vector dimension"},
				}
				for _, d := range defaults {
					var existing models.EmbeddingConfig
					if err := db.Where("key = ?", d.Key).First(&existing).Error; err != nil {
						if err := db.Create(&d).Error; err != nil {
							logging.Warnf("Warning: failed to seed embedding_config key %s: %v", d.Key, err)
						}
					}
				}
				return nil
			},
		},
		{
			Version:     "20260514_0002",
			Description: "Seed event clustering config keys into embedding_config.",
			Up: func(db *gorm.DB) error {
				defaults := []models.EmbeddingConfig{
					{Key: "event_cluster_kw_min_overlap", Value: "2", Description: "Minimum shared keyword count for Stage 1 event tag keyword-overlap clustering"},
					{Key: "event_cluster_sem_threshold", Value: "0.80", Description: "Minimum semantic cosine similarity for Stage 2 event tag clustering filter"},
				}
				for _, d := range defaults {
					var existing models.EmbeddingConfig
					if err := db.Where("key = ?", d.Key).First(&existing).Error; err != nil {
						if err := db.Create(&d).Error; err != nil {
							logging.Warnf("Warning: failed to seed embedding_config key %s: %v", d.Key, err)
						}
					}
				}
				return nil
			},
		},

		// ── Indexes ─────────────────────────────────────────────────
		{
			Version:     "20260417_0001",
			Description: "Add missing indexes for CRUD performance optimization.",
			Up: func(db *gorm.DB) error {
				indexes := []string{
					"CREATE INDEX IF NOT EXISTS idx_articles_read ON articles(read)",
					"CREATE INDEX IF NOT EXISTS idx_articles_favorite ON articles(favorite)",
					"CREATE INDEX IF NOT EXISTS idx_articles_feed_pub_date ON articles(feed_id, pub_date DESC)",
					"CREATE INDEX IF NOT EXISTS idx_article_topic_tags_article_id ON article_topic_tags(article_id)",
					"CREATE INDEX IF NOT EXISTS idx_feeds_category_id ON feeds(category_id)",
					"CREATE INDEX IF NOT EXISTS idx_articles_feed_id_title ON articles(feed_id, title)",
				}
				for _, s := range indexes {
					if err := db.Exec(s).Error; err != nil {
						return fmt.Errorf("create index: %w", err)
					}
				}
				return nil
			},
		},
		{
			Version:     "20260418_0001",
			Description: "Add embedding_type to topic_tag_embeddings and set unique index (topic_tag_id, embedding_type).",
			Up: func(db *gorm.DB) error {
				if err := db.Exec(`UPDATE topic_tag_embeddings SET embedding_type = 'identity' WHERE embedding_type IS NULL OR embedding_type = ''`).Error; err != nil {
					return fmt.Errorf("backfill embedding_type: %w", err)
				}
				if err := db.Exec(`DROP INDEX IF EXISTS idx_topic_tag_embeddings_topic_tag_id`).Error; err != nil {
					return fmt.Errorf("drop old unique index: %w", err)
				}
				if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_topic_tag_embeddings_tag_type ON topic_tag_embeddings(topic_tag_id, embedding_type)`).Error; err != nil {
					return fmt.Errorf("create topic_tag_embeddings(tag_id, type) unique index: %w", err)
				}
				return nil
			},
		},
		{
			Version:     "20260420_0001",
			Description: "Add indexes for narrative_summaries scope columns.",
			Up: func(db *gorm.DB) error {
				indexes := []string{
					"CREATE INDEX IF NOT EXISTS idx_narrative_scope ON narrative_summaries(scope_category_id)",
					"CREATE INDEX IF NOT EXISTS idx_narrative_scope_period ON narrative_summaries(scope_type, scope_category_id, period_date)",
				}
				for _, s := range indexes {
					if err := db.Exec(s).Error; err != nil {
						return fmt.Errorf("narrative scope index: %w", err)
					}
				}
				return nil
			},
		},
		{
			Version:     "20260430_0001",
			Description: "Add indexes for narrative_boards and narrative_summaries.board_id.",
			Up: func(db *gorm.DB) error {
				indexes := []string{
					"CREATE INDEX IF NOT EXISTS idx_narrative_boards_period ON narrative_boards(period_date)",
					"CREATE INDEX IF NOT EXISTS idx_narrative_boards_scope ON narrative_boards(scope_category_id)",
					"CREATE INDEX IF NOT EXISTS idx_narrative_summaries_board_id ON narrative_summaries(board_id)",
				}
				for _, s := range indexes {
					if err := db.Exec(s).Error; err != nil {
						return fmt.Errorf("narrative_boards index: %w", err)
					}
				}
				return nil
			},
		},

		// ── Full-text search ────────────────────────────────────────
		{
			Version:     "20260417_0002",
			Description: "Add GIN index for article full-text search using tsvector.",
			Up: func(db *gorm.DB) error {
				stmts := []string{
					`ALTER TABLE articles ADD COLUMN IF NOT EXISTS search_vector tsvector`,
					`CREATE INDEX IF NOT EXISTS idx_articles_search_vector ON articles USING GIN (search_vector)`,
					`CREATE OR REPLACE FUNCTION articles_search_vector_update() RETURNS trigger AS $$
					BEGIN
						NEW.search_vector :=
							setweight(to_tsvector('simple', COALESCE(NEW.title, '')), 'A') ||
							setweight(to_tsvector('simple', COALESCE(NEW.description, '')), 'B');
						RETURN NEW;
					END;
					$$ LANGUAGE plpgsql`,
					`DROP TRIGGER IF EXISTS articles_search_vector_trigger ON articles`,
					`CREATE TRIGGER articles_search_vector_trigger
						BEFORE INSERT OR UPDATE OF title, description ON articles
						FOR EACH ROW EXECUTE FUNCTION articles_search_vector_update()`,
					`UPDATE articles SET search_vector =
						setweight(to_tsvector('simple', COALESCE(title, '')), 'A') ||
						setweight(to_tsvector('simple', COALESCE(description, '')), 'B')`,
				}
				for _, s := range stmts {
					if err := db.Exec(s).Error; err != nil {
						return fmt.Errorf("full-text search migration: %w", err)
					}
				}
				return nil
			},
		},

		// ── Embedding index changes ─────────────────────────────────
		{
			Version:     "20260514_0001",
			Description: "Change topic_tag_embeddings unique index to (topic_tag_id, embedding_type, text_hash).",
			Up: func(db *gorm.DB) error {
				if err := db.Exec("DROP INDEX IF EXISTS idx_topic_tag_embeddings_tag_type").Error; err != nil {
					return fmt.Errorf("drop old idx_topic_tag_embeddings_tag_type: %w", err)
				}
				if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_topic_tag_embeddings_tag_type_hash ON topic_tag_embeddings(topic_tag_id, embedding_type, text_hash)").Error; err != nil {
					return fmt.Errorf("create idx_topic_tag_embeddings_tag_type_hash: %w", err)
				}
				return nil
			},
		},

		// ── Semantic label board indexes + seed ─────────────────────
		{
			Version:     "20260521_0001",
			Description: "Add indexes and seed settings for semantic label board system.",
			Up: func(db *gorm.DB) error {
				indexes := []string{
					"CREATE UNIQUE INDEX IF NOT EXISTS idx_semantic_labels_slug ON semantic_labels(slug)",
					"CREATE INDEX IF NOT EXISTS idx_semantic_labels_label_type ON semantic_labels(label_type)",
					"CREATE INDEX IF NOT EXISTS idx_semantic_labels_status ON semantic_labels(status)",
					"CREATE INDEX IF NOT EXISTS idx_topic_tag_semantic_labels_topic_tag_id ON topic_tag_semantic_labels(topic_tag_id)",
					"CREATE INDEX IF NOT EXISTS idx_topic_tag_semantic_labels_semantic_label_id ON topic_tag_semantic_labels(semantic_label_id)",
					"CREATE INDEX IF NOT EXISTS idx_topic_tag_board_labels_topic_tag_id ON topic_tag_board_labels(topic_tag_id)",
					"CREATE INDEX IF NOT EXISTS idx_topic_tag_board_labels_semantic_board_id ON topic_tag_board_labels(semantic_board_id)",
					"CREATE INDEX IF NOT EXISTS idx_board_composition_board_id ON board_composition(board_id)",
					"CREATE INDEX IF NOT EXISTS idx_board_composition_auxiliary_label_id ON board_composition(auxiliary_label_id)",
					"CREATE INDEX IF NOT EXISTS idx_narrative_boards_semantic_board_id ON narrative_boards(semantic_board_id)",
				}
				for _, s := range indexes {
					if err := db.Exec(s).Error; err != nil {
						return fmt.Errorf("semantic label board index: %w", err)
					}
				}

				settingsDefaults := []struct {
					Key, Value, Description string
				}{
					{"semantic_board_match_sim_threshold", "0.6", "Minimum auxiliary label similarity counted as a SemanticBoard match"},
					{"semantic_board_match_direct_hit_rate", "0.5", "Minimum direct auxiliary label hit rate for a SemanticBoard match"},
					{"semantic_board_match_direct_max_sim", "0.8", "Maximum similarity threshold for direct SemanticBoard matching"},
					{"semantic_board_match_direct_max_sim_min_hits", "2", "Minimum number of auxiliary label hits required for max_sim matching rule"},
					{"semantic_board_match_direct_max_sim_min_hit_rate", "0.3", "Minimum auxiliary label hit rate required for max_sim matching rule"},
					{"semantic_board_match_min_effective_sample", "3", "Minimum denominator for hit rate calculation to prevent inflated scores from low auxiliary label counts"},
					{"semantic_board_match_hit_rate_sim_blend", "0.7", "Weight of maxSimilarity in hit_rate rule score (score = α×maxSim + (1-α)×hitRate)"},
					{"semantic_board_match_weight_sim", "0.6", "Similarity weight used in weighted SemanticBoard matching"},
					{"semantic_board_match_weight_density", "0.4", "Density weight used in weighted SemanticBoard matching"},
					{"semantic_board_match_weighted_threshold", "0.6", "Minimum weighted score for assigning a topic tag to a SemanticBoard"},
					{"semantic_board_match_direct_hit_min_overlap", "2", "Minimum auxiliary label overlap count for direct_hit matching rule"},
					{"semantic_board_match_max_boards", "3", "Maximum SemanticBoard matches retained for each topic tag"},
					{"semantic_board_upgrade_ref_count_threshold", "5", "Minimum reference count before suggesting a new SemanticBoard"},
					{"semantic_board_upgrade_cluster_distance_threshold", "0.35", "Cluster distance threshold for SemanticBoard upgrade suggestions (cosine distance; lower = stricter clustering, prevents unrelated candidates from being absorbed into existing boards)"},
					{"semantic_board_upgrade_cotag_window_days", "30", "Co-tag analysis window in days for SemanticBoard upgrade suggestions"},
					{"semantic_board_upgrade_cotag_top_n", "20", "Maximum co-tag candidates considered for SemanticBoard upgrade suggestions"},
					{"semantic_board_upgrade_cotag_dedupe_sim_threshold", "0.85", "Similarity threshold for deduplicating co-tag upgrade candidates"},
					{"semantic_board_upgrade_cotag_hard_limit", "15", "Hard limit for co-tag upgrade candidates"},
				}
				for _, d := range settingsDefaults {
					var existing models.AISettings
					if err := db.Where("key = ?", d.Key).First(&existing).Error; err != nil {
						if err := db.Create(&models.AISettings{
							Key:         d.Key,
							Value:       d.Value,
							Description: d.Description,
						}).Error; err != nil {
							logging.Warnf("Warning: failed to seed ai_settings key %s: %v", d.Key, err)
						}
					}
				}
				return nil
			},
		},

		// ── Drops (legacy cleanup) ─────────────────────────────────
		{
			Version:     "20260522_0001",
			Description: "Drop legacy board_concepts and hierarchy system tables/columns.",
			Up: func(db *gorm.DB) error {
				columnDrops := []string{
					"ALTER TABLE topic_tags DROP COLUMN IF EXISTS concept_id CASCADE",
					"ALTER TABLE narrative_boards DROP COLUMN IF EXISTS abstract_tag_id CASCADE",
					"ALTER TABLE narrative_boards DROP COLUMN IF EXISTS board_concept_id CASCADE",
					"ALTER TABLE narrative_boards DROP COLUMN IF EXISTS is_system",
					"ALTER TABLE narrative_boards DROP COLUMN IF EXISTS abstract_tag_ids",
				}
				for _, s := range columnDrops {
					if err := db.Exec(s).Error; err != nil {
						return fmt.Errorf("drop column: %w", err)
					}
				}
				tableDrops := []string{
					"DROP TABLE IF EXISTS board_concepts CASCADE",
					"DROP TABLE IF EXISTS hierarchy_configs CASCADE",
					"DROP TABLE IF EXISTS hierarchy_config_versions CASCADE",
					"DROP TABLE IF EXISTS hierarchy_pending_changes CASCADE",
					"DROP TABLE IF EXISTS hierarchy_anchor_signals CASCADE",
					"DROP TABLE IF EXISTS rebuild_jobs CASCADE",
					"DROP TABLE IF EXISTS abstract_tag_update_queues CASCADE",
					"DROP TABLE IF EXISTS adopt_narrower_queues CASCADE",
				}
				for _, s := range tableDrops {
					if err := db.Exec(s).Error; err != nil {
						return fmt.Errorf("drop table: %w", err)
					}
				}
				deprecatedSettings := []string{
					"narrative_board_embedding_threshold",
					"narrative_board_hotspot_threshold",
				}
				for _, key := range deprecatedSettings {
					if err := db.Exec("DELETE FROM ai_settings WHERE key = ?", key).Error; err != nil {
						logging.Warnf("Warning: failed to delete deprecated ai_settings key %s: %v", key, err)
					}
				}
				return nil
			},
		},
		{
			Version:     "20260523_0001",
			Description: "Drop topic_tags sub_type column.",
			Up: func(db *gorm.DB) error {
				stmts := []string{
					"DROP INDEX IF EXISTS idx_topic_tags_sub_type",
					"ALTER TABLE topic_tags DROP COLUMN IF EXISTS sub_type",
				}
				for _, s := range stmts {
					if err := db.Exec(s).Error; err != nil {
						return fmt.Errorf("drop topic_tags sub_type: %w", err)
					}
				}
				return nil
			},
		},

		// ── Daily report indexes ────────────────────────────────────
		{
			Version:     "20260526_0001",
			Description: "Add indexes for board_daily_reports.",
			Up: func(db *gorm.DB) error {
				// The daily_report_* tables are registered by internal/topicgraph;
				// deployments that don't import it (incl. the test harness) won't
				// have them. Skip rather than fail on CREATE INDEX over a missing
				// table.
				if !tableExists(db, "board_daily_reports") || !tableExists(db, "daily_report_sections") {
					return nil
				}
				indexes := []string{
					"CREATE INDEX IF NOT EXISTS idx_board_daily_reports_semantic_board_id ON board_daily_reports(semantic_board_id)",
					"CREATE INDEX IF NOT EXISTS idx_daily_report_sections_report_id ON daily_report_sections(report_id)",
				}
				for _, s := range indexes {
					if err := db.Exec(s).Error; err != nil {
						return fmt.Errorf("daily report index: %w", err)
					}
				}
				return nil
			},
		},

		// ── Daily report thread indexes + data migration ────────────
		{
			Version:     "20260529_0001",
			Description: "Add indexes for daily_report_threads.",
			Up: func(db *gorm.DB) error {
				if !tableExists(db, "daily_report_threads") {
					return nil
				}
				indexes := []string{
					"CREATE INDEX IF NOT EXISTS idx_daily_report_threads_report_id ON daily_report_threads(report_id)",
					"CREATE INDEX IF NOT EXISTS idx_daily_report_threads_section_id ON daily_report_threads(section_id)",
				}
				for _, s := range indexes {
					if err := db.Exec(s).Error; err != nil {
						return fmt.Errorf("daily_report_threads index: %w", err)
					}
				}
				// prev_thread_id index — only if column exists (dropped in later migration)
				var colExists bool
				if err := db.Raw(`SELECT EXISTS (
					SELECT 1 FROM information_schema.columns
					WHERE table_name = 'daily_report_threads' AND column_name = 'prev_thread_id'
				)`).Scan(&colExists).Error; err == nil && colExists {
					if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_daily_report_threads_prev_thread_id ON daily_report_threads(prev_thread_id) WHERE prev_thread_id IS NOT NULL").Error; err != nil {
						return fmt.Errorf("daily_report_threads prev_thread_id index: %w", err)
					}
				}
				return nil
			},
		},
		{
			Version:     "20260529_0002",
			Description: "Migrate existing thread data from daily_report_sections.threads JSONB to daily_report_threads rows.",
			Up: func(db *gorm.DB) error {
				var colExists bool
				if err := db.Raw(`SELECT EXISTS (
					SELECT 1 FROM information_schema.columns
					WHERE table_name = 'daily_report_sections' AND column_name = 'threads'
				)`).Scan(&colExists).Error; err != nil {
					return fmt.Errorf("check threads column: %w", err)
				}
				if !colExists {
					return nil
				}

				err := db.Exec(`
					INSERT INTO daily_report_threads (report_id, section_id, title, summary, status, tag_ids, confidence, prev_thread_id, created_at)
					SELECT
						s.report_id,
						s.id,
						COALESCE(t->>'title', ''),
						t->>'summary',
						COALESCE(t->>'status', 'emerging'),
						COALESCE(t->'related_tag_ids', '[]'::jsonb),
						COALESCE((t->>'confidence')::double precision, 0),
						NULL,
						s.created_at
					FROM daily_report_sections s
					CROSS JOIN jsonb_array_elements(s.threads) AS t
					WHERE s.threads IS NOT NULL
					  AND jsonb_array_length(s.threads) > 0
				`).Error
				if err != nil {
					return fmt.Errorf("migrate threads JSONB to rows: %w", err)
				}
				return nil
			},
		},
		{
			Version:     "20260529_0003",
			Description: "Drop threads JSONB column from daily_report_sections after migration.",
			Up: func(db *gorm.DB) error {
				var colExists bool
				if err := db.Raw(`SELECT EXISTS (
					SELECT 1 FROM information_schema.columns
					WHERE table_name = 'daily_report_sections' AND column_name = 'threads'
				)`).Scan(&colExists).Error; err != nil {
					return fmt.Errorf("check threads column: %w", err)
				}
				if !colExists {
					return nil
				}
				if err := db.Exec(`ALTER TABLE daily_report_sections DROP COLUMN threads`).Error; err != nil {
					return fmt.Errorf("drop threads column: %w", err)
				}
				return nil
			},
		},

		// ── FK cascade: remove duplicate NO ACTION constraints on topic_tags children ──
		{
			Version:     "20260601_0001",
			Description: "Remove duplicate NO ACTION foreign keys on topic_tags child tables, ensure all are ON DELETE CASCADE.",
			Up: func(db *gorm.DB) error {
				stmts := []string{
					// embedding_queues
					`DO $$ BEGIN
						IF EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name = 'fk_embedding_queues_tag' AND table_name = 'embedding_queues') THEN
							ALTER TABLE embedding_queues DROP CONSTRAINT fk_embedding_queues_tag;
						END IF;
					END $$`,

					// merge_reembedding_queues
					`DO $$ BEGIN
						IF EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name = 'fk_merge_reembedding_queues_target_tag' AND table_name = 'merge_reembedding_queues') THEN
							ALTER TABLE merge_reembedding_queues DROP CONSTRAINT fk_merge_reembedding_queues_target_tag;
						END IF;
					END $$`,
					`DO $$ BEGIN
						IF EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name = 'fk_merge_reembedding_queues_source_tag' AND table_name = 'merge_reembedding_queues') THEN
							ALTER TABLE merge_reembedding_queues DROP CONSTRAINT fk_merge_reembedding_queues_source_tag;
						END IF;
					END $$`,

					// topic_tag_embeddings
					`DO $$ BEGIN
						IF EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name = 'fk_topic_tags_embedding' AND table_name = 'topic_tag_embeddings') THEN
							ALTER TABLE topic_tag_embeddings DROP CONSTRAINT fk_topic_tags_embedding;
						END IF;
					END $$`,
					`DO $$ BEGIN
						IF EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name = 'fk_topic_tags_embeddings' AND table_name = 'topic_tag_embeddings') THEN
							ALTER TABLE topic_tag_embeddings DROP CONSTRAINT fk_topic_tags_embeddings;
						END IF;
					END $$`,

					// topic_tag_relations
					`DO $$ BEGIN
						IF EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name = 'fk_topic_tag_relations_child' AND table_name = 'topic_tag_relations') THEN
							ALTER TABLE topic_tag_relations DROP CONSTRAINT fk_topic_tag_relations_child;
						END IF;
					END $$`,
					`DO $$ BEGIN
						IF EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name = 'fk_topic_tag_relations_parent' AND table_name = 'topic_tag_relations') THEN
							ALTER TABLE topic_tag_relations DROP CONSTRAINT fk_topic_tag_relations_parent;
						END IF;
					END $$`,

					// topic_tags self-ref (merged_into): drop old NO ACTION, add CASCADE
					`DO $$ BEGIN
						IF EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name = 'fk_topic_tags_merged_into' AND table_name = 'topic_tags') THEN
							ALTER TABLE topic_tags DROP CONSTRAINT fk_topic_tags_merged_into;
						END IF;
					END $$`,
					`DO $$ BEGIN
						IF EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name = 'topic_tags_merged_into_id_fkey' AND table_name = 'topic_tags') THEN
							ALTER TABLE topic_tags DROP CONSTRAINT topic_tags_merged_into_id_fkey;
						END IF;
					END $$`,
					`ALTER TABLE topic_tags ADD CONSTRAINT topic_tags_merged_into_id_fkey
						FOREIGN KEY (merged_into_id) REFERENCES topic_tags(id) ON DELETE CASCADE`,
				}
				for _, s := range stmts {
					if err := db.Exec(s).Error; err != nil {
						return fmt.Errorf("topic_tags FK cascade migration: %w", err)
					}
				}
				return nil
			},
		},

		// ── Drop deprecated vector text column ────────────────────
		{
			Version:     "20260601_0001b",
			Description: "Drop deprecated vector text column from topic_tag_embeddings.",
			Up: func(db *gorm.DB) error {
				return db.Exec("ALTER TABLE topic_tag_embeddings DROP COLUMN IF EXISTS vector").Error
			},
		},

		// ── Section relations (many-to-many) + drop legacy columns ────────
		{
			Version:     "20260603_0001",
			Description: "Add unique constraint to section_relations, migrate prev_section_id, drop status/prev_thread_id columns.",
			Up: func(db *gorm.DB) error {
				// daily_report_* tables are optional (registered by internal/topicgraph);
				// skip the whole block if the core table is absent.
				if !tableExists(db, "daily_report_section_relations") {
					return nil
				}

				// 1. Add unique constraint (table created by AutoMigrate)
				if err := db.Exec(`
					ALTER TABLE daily_report_section_relations
					DROP CONSTRAINT IF EXISTS uq_section_relations_pair
				`).Error; err != nil {
					return fmt.Errorf("drop old constraint: %w", err)
				}
				if err := db.Exec(`
					ALTER TABLE daily_report_section_relations
					ADD CONSTRAINT uq_section_relations_pair UNIQUE (from_section_id, to_section_id)
				`).Error; err != nil {
					return fmt.Errorf("add unique constraint: %w", err)
				}

				// 2. Migrate existing prev_section_id data into relations (if column exists)
				var prevColExists bool
				if err := db.Raw(`SELECT EXISTS (
					SELECT 1 FROM information_schema.columns
					WHERE table_name = 'daily_report_sections' AND column_name = 'prev_section_id'
				)`).Scan(&prevColExists).Error; err == nil && prevColExists {
					if err := db.Exec(`
						INSERT INTO daily_report_section_relations (from_section_id, to_section_id, distance, created_at)
						SELECT ps.id, s.id, 0.0, NOW()
						FROM daily_report_sections s
						JOIN daily_report_sections ps ON ps.id = s.prev_section_id
						WHERE s.prev_section_id IS NOT NULL
						ON CONFLICT (from_section_id, to_section_id) DO NOTHING
					`).Error; err != nil {
						logging.Warnf("migration: migrate prev_section_id data: %v", err)
					}
				}

				// 3. Drop prev_section_id column
				if err := db.Exec(`ALTER TABLE daily_report_sections DROP COLUMN IF EXISTS prev_section_id CASCADE`).Error; err != nil {
					logging.Warnf("migration: drop prev_section_id: %v", err)
				}

				// 4. Drop status column from sections
				if err := db.Exec(`ALTER TABLE daily_report_sections DROP COLUMN IF EXISTS status CASCADE`).Error; err != nil {
					logging.Warnf("migration: drop sections.status: %v", err)
				}

				// 5. Drop status column from threads
				if err := db.Exec(`ALTER TABLE daily_report_threads DROP COLUMN IF EXISTS status CASCADE`).Error; err != nil {
					logging.Warnf("migration: drop threads.status: %v", err)
				}

				// 6. Drop prev_thread_id column from threads
				if err := db.Exec(`DROP INDEX IF EXISTS idx_daily_report_threads_prev_thread_id`).Error; err != nil {
					logging.Warnf("migration: drop prev_thread_id index: %v", err)
				}
				if err := db.Exec(`ALTER TABLE daily_report_threads DROP COLUMN IF EXISTS prev_thread_id CASCADE`).Error; err != nil {
					logging.Warnf("migration: drop prev_thread_id: %v", err)
				}

				return nil
			},
		},

		// ── Clean up dead embedding config keys ──────────────────────────
		{
			Version:     "20260614_0001",
			Description: "Remove dead embedding_config and ai_settings rows that are no longer consumed by any runtime code.",
			Up: func(db *gorm.DB) error {
				deadEmbeddingKeys := []string{
					"high_similarity_threshold",
					"low_similarity_threshold",
					"embedding_dimension",
					"embedding_model",
				}
				if err := db.Exec("DELETE FROM embedding_config WHERE key IN (?, ?, ?, ?)",
					deadEmbeddingKeys[0], deadEmbeddingKeys[1], deadEmbeddingKeys[2], deadEmbeddingKeys[3]).Error; err != nil {
					return fmt.Errorf("delete dead embedding_config keys: %w", err)
				}
				if err := db.Exec("DELETE FROM ai_settings WHERE key = ?", "narrative_board_embedding_threshold").Error; err != nil {
					return fmt.Errorf("delete dead ai_settings key: %w", err)
				}
				return nil
			},
		},

		// ── Section embedding column ────────────────────────────────────
		{
			Version:     "20260601_0002",
			Description: "Add embedding vector column to daily_report_sections (dimension set at runtime).",
			Up: func(db *gorm.DB) error {
				if !tableExists(db, "daily_report_sections") {
					return nil
				}
				if err := db.Exec(`ALTER TABLE daily_report_sections ADD COLUMN IF NOT EXISTS embedding vector`).Error; err != nil {
					return fmt.Errorf("add daily_report_sections.embedding column: %w", err)
				}
				return nil
			},
		},

		// ── Seed daily_report_time default ───────────────────────────────
		{
			Version:     "20260614_0002",
			Description: "Seed daily_report_time default value into ai_settings.",
			Up: func(db *gorm.DB) error {
				var existing models.AISettings
				if err := db.Where("key = ?", "daily_report_time").First(&existing).Error; err == nil {
					return nil // already exists
				}
				return db.Create(&models.AISettings{
					Key:         "daily_report_time",
					Value:       "21:00",
					Description: "日报生成时刻（HH:MM）",
				}).Error
			},
		},

		// ── Feed icon_source backfill ───────────────────────────────────
		// AutoMigrate adds feeds.icon_source with default 'fallback', but
		// existing rows carry legacy icon values with mixed semantics. This
		// one-shot data migration classifies them by their icon value: URLs
		// -> auto, placeholder strings -> fallback, other iconify ids ->
		// custom (conservatively preserved as user-owned).
		//
		// Idempotency: versioned migrations run exactly once (tracked in
		// schema_migrations), so unconditional classification is safe here.
		// Rows created after this migration set icon_source correctly at write
		// time and are never touched by this block again.
		{
			Version:     "20260617_0001",
			Description: "Backfill feeds.icon_source from legacy icon values and normalize placeholders.",
			Up: func(db *gorm.DB) error {
				if !tableExists(db, "feeds") {
					return nil
				}
				// Classify every legacy row by its current icon value.
				if err := db.Exec(`
					UPDATE feeds SET icon_source = CASE
						WHEN icon IS NULL OR icon = '' OR icon = 'rss' OR icon = 'mdi:rss' THEN 'fallback'
						WHEN icon LIKE 'http://%' OR icon LIKE 'https://%' THEN 'auto'
						ELSE 'custom'
					END
					WHERE icon_source IS DISTINCT FROM (
						CASE
							WHEN icon IS NULL OR icon = '' OR icon = 'rss' OR icon = 'mdi:rss' THEN 'fallback'
							WHEN icon LIKE 'http://%' OR icon LIKE 'https://%' THEN 'auto'
							ELSE 'custom'
						END
					)
				`).Error; err != nil {
					return fmt.Errorf("backfill feeds.icon_source: %w", err)
				}
				// Normalize placeholder icon values for fallback rows (rss/"" -> mdi:rss)
				if err := db.Exec(`
				UPDATE feeds SET icon = 'mdi:rss'
				WHERE icon_source = 'fallback' AND icon IN ('', 'rss')
			`).Error; err != nil {
					return fmt.Errorf("normalize feeds.icon placeholders: %w", err)
				}
				return nil
			},
		},

		// ── Persistent topic layer ───────────────────────────────────────
		// board_persistent_topics is created by AutoMigrate (registered by
		// internal/topicgraph). This migration adds the domain-specific constraints
		// AutoMigrate cannot express: the status CHECK, the (board, status) lookup
		// index, and the relation_type column on section relations with its default
		// backfill. The embedding HNSW index is created at startup by
		// ensurePersistentTopicEmbeddingDimension (dimension-dependent, ≤2000 only),
		// so it is NOT created here.
		{
			Version:     "20260619_0001",
			Description: "Add persistent topic constraints and section relation_type column.",
			Up: func(db *gorm.DB) error {
				if tableExists(db, "board_persistent_topics") {
					if err := db.Exec(`
					DO $$ BEGIN
						IF NOT EXISTS (
							SELECT 1 FROM information_schema.table_constraints
							WHERE constraint_name = 'chk_board_persistent_topics_status'
							  AND table_name = 'board_persistent_topics'
						) THEN
							ALTER TABLE board_persistent_topics
								ADD CONSTRAINT chk_board_persistent_topics_status
								CHECK (status IN ('candidate', 'active', 'archived'));
						END IF;
					END $$
				`).Error; err != nil {
						return fmt.Errorf("add board_persistent_topics status CHECK: %w", err)
					}
					if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_persistent_topics_board_status
					ON board_persistent_topics(semantic_board_id, status)`).Error; err != nil {
						return fmt.Errorf("add board_persistent_topics (board, status) index: %w", err)
					}
				}

				if tableExists(db, "daily_report_section_relations") {
					// Add relation_type column (AutoMigrate adds it on fresh DBs; this
					// ALTER covers DBs that had the table before the model gained the
					// column, and backfills legacy rows to 'similarity').
					var typeColExists bool
					if err := db.Raw(`SELECT EXISTS (
					SELECT 1 FROM information_schema.columns
					WHERE table_name = 'daily_report_section_relations' AND column_name = 'relation_type'
				)`).Scan(&typeColExists).Error; err == nil && !typeColExists {
						if err := db.Exec(`ALTER TABLE daily_report_section_relations
						ADD COLUMN relation_type VARCHAR(20) NOT NULL DEFAULT 'similarity'`).Error; err != nil {
							return fmt.Errorf("add daily_report_section_relations.relation_type: %w", err)
						}
					}
					if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_section_relations_type
					ON daily_report_section_relations(relation_type)`).Error; err != nil {
						return fmt.Errorf("add section_relations relation_type index: %w", err)
					}
				}
				return nil
			},
		},

		// ── Seed persistent topic config keys ───────────────────────────
		{
			Version:     "20260619_0002",
			Description: "Seed persistent topic lifecycle config into ai_settings.",
			Up: func(db *gorm.DB) error {
				defaults := []struct {
					Key, Value, Description string
				}{
					{"persistent_topic_match_threshold", "0.30", "Section-to-topic assignment embedding distance ceiling (dual confirmation)"},
					{"persistent_topic_upgrade_threshold", "3", "Consecutive hit days for a candidate topic to auto-promote to active"},
					{"persistent_topic_decay_window", "30", "Days an active topic can go without a hit before auto-archiving"},
					{"persistent_topic_cluster_threshold", "0.28", "Complete-link clustering distance for backfilling topics from historical sections (0.28 calibrated on real data: 0.30 chained to 1 topic, 0.25 fragmented)"},
				}
				for _, d := range defaults {
					var existing models.AISettings
					if err := db.Where("key = ?", d.Key).First(&existing).Error; err != nil {
						if err := db.Create(&models.AISettings{
							Key:         d.Key,
							Value:       d.Value,
							Description: d.Description,
						}).Error; err != nil {
							logging.Warnf("Warning: failed to seed ai_settings key %s: %v", d.Key, err)
						}
					}
				}
				return nil
			},
		},

		// ── Quality scoring observability ──────────────────────────────
		{
			Version:     "20260625_0001",
			Description: "Add quality_breakdown JSONB column to daily_report_sections.",
			Up: func(db *gorm.DB) error {
				if !tableExists(db, "daily_report_sections") {
					return nil
				}
				return db.Exec("ALTER TABLE daily_report_sections ADD COLUMN IF NOT EXISTS quality_breakdown JSONB NULL").Error
			},
		},

		// ── Widen section_relations unique constraint to (from, to, type) ───
		// Lets an identity edge (same persistent topic) and a similarity edge
		// (Hungarian match) on the same section pair coexist as two rows.
		// Before this, identity silently overwrote a strong Hungarian match,
		// dropping the edge from the similarity-only timeline view. No data
		// migration is needed: the old (from, to) constraint already forbade
		// duplicates per pair, so widening to (from, to, relation_type) cannot
		// introduce a violation — every existing (from, to, type) is already
		// unique. Relation rows are rebuilt by RebuildBoardRelations.
		{
			Version:     "20260620_0001",
			Description: "Widen section_relations unique constraint to (from_section_id, to_section_id, relation_type) so identity and similarity edges coexist.",
			Up: func(db *gorm.DB) error {
				if !tableExists(db, "daily_report_section_relations") {
					return nil
				}
				if err := db.Exec(`
					ALTER TABLE daily_report_section_relations
					DROP CONSTRAINT IF EXISTS uq_section_relations_pair
				`).Error; err != nil {
					return fmt.Errorf("drop old section_relations pair constraint: %w", err)
				}
				if err := db.Exec(`
					ALTER TABLE daily_report_section_relations
					ADD CONSTRAINT uq_section_relations_pair UNIQUE (from_section_id, to_section_id, relation_type)
				`).Error; err != nil {
					return fmt.Errorf("add widened section_relations pair constraint: %w", err)
				}
				return nil
			},
		},

		// ── Data migration: enable_thinking semantics flip ──────────
		// AIProvider.EnableThinking's meaning changed from "strip <think> tags after-the-fact"
		// to "enable model reasoning (propagate chat_template_kwargs.enable_thinking)". Stale
		// true values (set under the old meaning) would accidentally enable thinking and slow
		// down topic tagging, so reset all to false on deploy. Idempotent.
		{
			Version:     "20260626_0001",
			Description: "Reset ai_providers.enable_thinking to false (semantics flipped from strip-think-tags to enable-thinking).",
			Up: func(db *gorm.DB) error {
				return db.Exec("UPDATE ai_providers SET enable_thinking = false").Error
			},
		},

		// ── Persistent topic attention separation ─────────────────────
		{
			Version:     "20260627_0001",
			Description: "Snapshot report-time topic status and seed candidate retention limits.",
			Up: func(db *gorm.DB) error {
				if tableExists(db, "daily_report_sections") {
					if err := db.Exec(`ALTER TABLE daily_report_sections
						ADD COLUMN IF NOT EXISTS topic_status_at_report VARCHAR(20) NULL`).Error; err != nil {
						return fmt.Errorf("add daily_report_sections.topic_status_at_report: %w", err)
					}
				}

				defaults := []struct {
					Key, Value, Description string
				}{
					{"persistent_topic_candidate_decay_window", "7", "Days a candidate topic remains anchorable without another hit"},
					{"persistent_topic_candidate_prompt_limit", "20", "Maximum candidate topics injected into clustering and assignment per board"},
				}
				for _, d := range defaults {
					var existing models.AISettings
					if err := db.Where("key = ?", d.Key).First(&existing).Error; err == nil {
						continue
					}
					if err := db.Create(&models.AISettings{
						Key: d.Key, Value: d.Value, Description: d.Description,
					}).Error; err != nil {
						return fmt.Errorf("seed ai_settings key %s: %w", d.Key, err)
					}
				}
				return nil
			},
		},

		// ── Prune underqualified observing candidates ───────────────
		{
			Version:     "20260628_0001",
			Description: "Hard-delete candidate topics with hit_count < upgrade_threshold (one-shot cleanup of historical observing candidates).",
			Up: func(db *gorm.DB) error {
				if !tableExists(db, "board_persistent_topics") || !tableExists(db, "daily_report_sections") {
					return nil
				}
				threshold := 3 // default
				var key models.AISettings
				if err := db.Where("key = ?", "persistent_topic_upgrade_threshold").First(&key).Error; err == nil {
					if v, err2 := strconv.Atoi(key.Value); err2 == nil && v > 0 {
						threshold = v
					}
				}
				deleted, err := PruneUnderqualifiedCandidates(db, threshold)
				if err != nil {
					return fmt.Errorf("prune underqualified candidates: %w", err)
				}
				logging.Infof("Migration 20260628_0001: pruned %d underqualified candidate topics (hit_count < %d)", deleted, threshold)
				return nil
			},
		},

		// ── Topic watch CHECK constraint ───────────────────────────────
		// board_topic_watches and topic_watch_hits tables are created by
		// AutoMigrate (registered by internal/topicgraph). This migration
		// adds the domain-specific CHECK constraint AutoMigrate cannot
		// express: board_topic_watches.status IN ('active','paused').
		// Idempotent — second run is a no-op.
		{
			Version:     "20260630_0001",
			Description: "Add CHECK constraint on board_topic_watches.status (active|paused).",
			Up: func(db *gorm.DB) error {
				if !tableExists(db, "board_topic_watches") {
					return nil
				}
				if err := db.Exec(`
					DO $$ BEGIN
						IF NOT EXISTS (
							SELECT 1 FROM information_schema.table_constraints
							WHERE constraint_name = 'chk_board_topic_watches_status'
							  AND table_name = 'board_topic_watches'
						) THEN
							ALTER TABLE board_topic_watches
								ADD CONSTRAINT chk_board_topic_watches_status
								CHECK (status IN ('active', 'paused'));
						END IF;
					END $$
				`).Error; err != nil {
					return fmt.Errorf("add board_topic_watches status CHECK: %w", err)
				}
				return nil
			},
		},
		{
			Version:     "20260630_0002",
			Description: "Add composite unique index on topic_watch_hits(watch_id, section_id, report_id).",
			Up: func(db *gorm.DB) error {
				if !tableExists(db, "topic_watch_hits") {
					return nil
				}
				// Rollback: DROP INDEX IF EXISTS idx_watch_section_report;
				if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_watch_section_report ON topic_watch_hits(watch_id, section_id, report_id)`).Error; err != nil {
					return fmt.Errorf("add topic_watch_hits unique index: %w", err)
				}
				return nil
			},
		},

		// ── AI call log schema补齐 (ai-logging R2 必记字段) ──────
		{
			Version:     "20260704_0001",
			Description: "为 ai_call_logs 补 R2 必记字段: operation/prompt/token_usage/session_id/model 五列 + 索引.",
			Up: func(db *gorm.DB) error {
				stmts := []string{
					"ALTER TABLE ai_call_logs ADD COLUMN IF NOT EXISTS operation VARCHAR(80)",
					"ALTER TABLE ai_call_logs ADD COLUMN IF NOT EXISTS prompt TEXT",
					"ALTER TABLE ai_call_logs ADD COLUMN IF NOT EXISTS token_usage JSONB",
					"ALTER TABLE ai_call_logs ADD COLUMN IF NOT EXISTS session_id VARCHAR(120)",
					"ALTER TABLE ai_call_logs ADD COLUMN IF NOT EXISTS model VARCHAR(100)",
					"CREATE INDEX IF NOT EXISTS idx_call_logs_session ON ai_call_logs(session_id)",
					"CREATE INDEX IF NOT EXISTS idx_call_logs_op_time ON ai_call_logs(operation, created_at)",
				}
				for _, s := range stmts {
					if err := db.Exec(s).Error; err != nil {
						return fmt.Errorf("ai_call_logs schema migration: %w", err)
					}
				}
				// Backfill + SET NOT NULL via the idempotent helper (re-running this
				// migration on an already-NOT-NULL column is a no-op, unlike a bare
				// ALTER COLUMN ... SET NOT NULL which errors on a non-nullable column).
				if err := ensureNotNullDefault(db, "ai_call_logs", "operation", "'unknown'"); err != nil {
					return fmt.Errorf("ai_call_logs.operation NOT NULL: %w", err)
				}
				return nil
			},
		},

		// ── Manual topic lane source column ───────────────────────────
		{
			Version:     "20260702_0001",
			Description: "Add board_persistent_topics.source column (auto/manual) with CHECK constraint.",
			Up: func(db *gorm.DB) error {
				if !tableExists(db, "board_persistent_topics") {
					return nil
				}
				// Add column with default 'auto' (idempotent via IF NOT EXISTS).
				if err := db.Exec(`
					ALTER TABLE board_persistent_topics
					ADD COLUMN IF NOT EXISTS source VARCHAR(10) NOT NULL DEFAULT 'auto'
				`).Error; err != nil {
					return fmt.Errorf("add board_persistent_topics.source column: %w", err)
				}
				// Add CHECK constraint (idempotent: check information_schema first).
				if err := db.Exec(`
					DO $$ BEGIN
						IF NOT EXISTS (
							SELECT 1 FROM information_schema.table_constraints
							WHERE constraint_name = 'chk_board_persistent_topics_source'
							  AND table_name = 'board_persistent_topics'
						) THEN
							ALTER TABLE board_persistent_topics
								ADD CONSTRAINT chk_board_persistent_topics_source
								CHECK (source IN ('auto', 'manual'));
						END IF;
					END $$
				`).Error; err != nil {
					return fmt.Errorf("add board_persistent_topics.source CHECK: %w", err)
				}
				return nil
			},
		},

		// ── Lifeline context: period-archival model change ──────────
		{
			Version:     "20260706_0001",
			Description: "Drop old idx_topic_gran unique index; TRUNCATE topic_lifeline_context old data (pre-period model). Destructive — guarded by MIGRATIONS_ALLOW_DESTRUCTIVE.",
			Up: func(db *gorm.DB) error {
				// Destructive (TRUNCATE): skip unless MIGRATIONS_ALLOW_DESTRUCTIVE=1.
				// Production never enables this; dev/test set the env to clean pre-period data.
				if !IsDestructiveAllowed() {
					logging.Warnf("skipping destructive migration 20260706_0001 (set MIGRATIONS_ALLOW_DESTRUCTIVE=1 to enable)")
					return nil
				}
				// Drop old 2-column unique index (replaced by idx_topic_gran_period).
				if tableExists(db, "topic_lifeline_context") {
					if err := db.Exec(`
						DO $$ BEGIN
							IF EXISTS (
								SELECT 1 FROM pg_indexes
								WHERE indexname = 'idx_topic_gran'
								  AND tablename = 'topic_lifeline_context'
							) THEN
								ALTER TABLE topic_lifeline_context DROP CONSTRAINT IF EXISTS idx_topic_gran;
								DROP INDEX IF EXISTS idx_topic_gran;
							END IF;
						END $$`).Error; err != nil {
						return fmt.Errorf("drop old idx_topic_gran: %w", err)
					}
					// TRUNCATE old data (no period field → cannot satisfy new 3-col UNIQUE).
					if err := db.Exec("TRUNCATE topic_lifeline_context").Error; err != nil {
						return fmt.Errorf("truncate topic_lifeline_context: %w", err)
					}
				}
				return nil
			},
		},

		// ── Data-enrichment §11.5: clear pre-演进定位 schema data ──
		// 旧 result.sectors 用涨跌(direction/trigger)语义、旧 review.verdict 用兑现度(hit/part/miss)，
		// 与 §11.2/§11.3 的演进定位(position/signals、position_change)不兼容，不可复用，清空重跑。
		// stock_debate_result FK 引用 result，一并清。幂等：TRUNCATE 本身幂等 + 框架按 Version 去重。
		{
			Version:     "20260712_0001",
			Description: "TRUNCATE topic_enrichment_result/topic_enrichment_review/stock_debate_result — old 涨跌+兑现 schema incompatible with 演进定位 rewrite (§11.5). Destructive — guarded by MIGRATIONS_ALLOW_DESTRUCTIVE.",
			Up: func(db *gorm.DB) error {
				// Destructive (TRUNCATE): skip unless MIGRATIONS_ALLOW_DESTRUCTIVE=1.
				if !IsDestructiveAllowed() {
					logging.Warnf("skipping destructive migration 20260712_0001 (set MIGRATIONS_ALLOW_DESTRUCTIVE=1 to enable)")
					return nil
				}
				// 三张表由 AutoMigrate 在版本迁移前一并创建；result 表不存在则无需清理。
				if !tableExists(db, "topic_enrichment_result") {
					return nil
				}
				// CASCADE 断开 FK 子表引用；RESTART IDENTITY 复位 BIGSERIAL 序列。TRUNCATE 本身幂等。
				if err := db.Exec("TRUNCATE TABLE topic_enrichment_result, topic_enrichment_review, stock_debate_result RESTART IDENTITY CASCADE").Error; err != nil {
					return fmt.Errorf("truncate enrichment stale data: %w", err)
				}
				return nil
			},
		},

		// ── Board upgrade suggestions table ─────────────────────────────
		{
			Version:     "20260717_0001",
			Description: "Create board_upgrade_suggestions table and seed auxiliary_label_dedupe_sim setting.",
			Up: func(db *gorm.DB) error {
				// Table is created by AutoMigrate (model registered in models/).
				// Add partial unique index on suggestion_hash WHERE status='pending'.
				if err := db.Exec(`
					CREATE UNIQUE INDEX IF NOT EXISTS uq_board_upgrade_suggestions_hash
					ON board_upgrade_suggestions(suggestion_hash) WHERE status = 'pending'
				`).Error; err != nil {
					return fmt.Errorf("create board_upgrade_suggestions partial unique index: %w", err)
				}
				if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_board_upgrade_suggestions_status ON board_upgrade_suggestions(status)`).Error; err != nil {
					return fmt.Errorf("create board_upgrade_suggestions status index: %w", err)
				}

				// Seed auxiliary_label_dedupe_sim default
				var existing models.AISettings
				if err := db.Where("key = ?", "auxiliary_label_dedupe_sim").First(&existing).Error; err != nil {
					if err := db.Create(&models.AISettings{
						Key:         "auxiliary_label_dedupe_sim",
						Value:       "0.95",
						Description: "辅助标签 embedding 去重阈值（cosine similarity，L2 gate），低于此值不合并",
					}).Error; err != nil {
						logging.Warnf("Warning: failed to seed ai_settings key auxiliary_label_dedupe_sim: %v", err)
					}
				}
				return nil
			},
		},

		// ── One-shot auxiliary label text-variant merge ─────────────────
		{
			Version:     "20260717_0002",
			Description: "Merge auxiliary label text variant duplicates by normalize_key grouping, reusing MergeAuxiliaryLabelAlias.",
			Up:          runAuxLabelDupMerge,
		},

		// ── causal-analysis-agent: clear stale 演进定位 enrichment data ──
		// 分析目标从「演进定位」改为「探索判断 agent」(causal-analysis-agent)。
		// 旧 result 的 position/signals、旧 review 的 verdict 语义随之作废，与新
		// agent 产出不兼容，不可复用，清空后由新 agent 重跑。报告追问交互层
		// (topic_enrichment_qa) 为另起新表，不在此清理范围。
		// 幂等：TRUNCATE 本身幂等 + RESTART IDENTITY 复位序列；版本迁移按 Version 去重只跑一次。
		{
			Version:     "20260718_0001",
			Description: "TRUNCATE topic_enrichment_result/topic_enrichment_review — stale 演进定位 (position/signals/verdict) semantics retired for causal-analysis-agent (探索判断). Destructive — guarded by MIGRATIONS_ALLOW_DESTRUCTIVE.",
			Up: func(db *gorm.DB) error {
				// Destructive (TRUNCATE): skip unless MIGRATIONS_ALLOW_DESTRUCTIVE=1.
				if !IsDestructiveAllowed() {
					logging.Warnf("skipping destructive migration 20260718_0001 (set MIGRATIONS_ALLOW_DESTRUCTIVE=1 to enable)")
					return nil
				}
				// 两张表由 AutoMigrate 在版本迁移前一并创建；result 表不存在则无需清理。
				if !tableExists(db, "topic_enrichment_result") {
					return nil
				}
				// 这两张表无 DB 级 FK（模型未声明 GORM relation），无需 CASCADE。
				if err := db.Exec("TRUNCATE TABLE topic_enrichment_result, topic_enrichment_review RESTART IDENTITY").Error; err != nil {
					return fmt.Errorf("truncate enrichment stale data: %w", err)
				}
				return nil
			},
		},

		// ── causal-analysis-agent 阶段2b: qa 沉淀标记列 ──
		// topic_enrichment_qa 加 sedimented 列（用户手动沉淀某轮追问为持久笔记）。
		// 沉淀只翻转 qa 行上的标志，绝不改 topic_enrichment_result 主表（业务约束#2）。
		// 幂等：ADD COLUMN IF NOT EXISTS + 版本迁移按 Version 去重只跑一次。
		{
			Version:     "20260719_0001",
			Description: "Add topic_enrichment_qa.sedimented column (user-pinned Q&A note flag; report itself stays immutable). Idempotent.",
			Up: func(db *gorm.DB) error {
				if !tableExists(db, "topic_enrichment_qa") {
					return nil
				}
				if err := db.Exec(`ALTER TABLE topic_enrichment_qa ADD COLUMN IF NOT EXISTS sedimented BOOLEAN NOT NULL DEFAULT false`).Error; err != nil {
					return fmt.Errorf("add topic_enrichment_qa.sedimented column: %w", err)
				}
				return nil
			},
		},

		// ── model tag 治理：把 NOT NULL/DEFAULT 约束从 GORM model tag 收敛到显式迁移 ──
		// 背景：见 standard/backend/code-style.md「GORM model tag 与迁移」。model tag 写
		// not null/default 会让 AutoMigrate 与显式迁移竞争（ai-call-logging-schema 事故）。
		// 本迁移把 ai_models/topic_graph/semantic_label 三个文件的列级约束落地到 DB，
		// 之后即可从 model tag 安全移除 not null/default（让显式迁移唯一管约束）。
		// 幂等：SET DEFAULT 本身幂等；SET NOT NULL 经 columnIsNullable 检查只在可空时执行。
		{
			Version:     "20260723_0001",
			Description: "Materialize NOT NULL/DEFAULT constraints (previously driven by GORM tags) for ai_models/topic_graph/semantic_label tables, so model tags can be stripped. Idempotent.",
			Up: func(db *gorm.DB) error {
				// helper: set DEFAULT then backfill+NOT NULL for one column.
				constrain := func(table, column, defaultLit string, notNull bool) error {
					if defaultLit != "" {
						if err := db.Exec(fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN %s SET DEFAULT %s`, table, column, defaultLit)).Error; err != nil {
							return fmt.Errorf("set default %s.%s: %w", table, column, err)
						}
						if err := ensureNotNullDefault(db, table, column, defaultLit); err != nil {
							return err
						}
					}
					if notNull && defaultLit == "" {
						// No default: cannot backfill generically; rely on ensureNotNullDefault's
						// empty-string backfill only for text-like columns. Caller passes "" to skip.
						if err := ensureNotNullDefault(db, table, column, "''"); err != nil {
							return err
						}
					}
					return nil
				}

				type col struct {
					table, column, def string
					notNull            bool
				}
				// Columns with a DEFAULT value: backfill uses the default, then SET NOT NULL.
				cols := []col{
					// ── ai_models.go ──
					{"scheduler_tasks", "check_interval", "60", true},
					{"scheduler_tasks", "status", "'idle'", false},
					{"scheduler_tasks", "total_executions", "0", false},
					{"scheduler_tasks", "successful_executions", "0", false},
					{"scheduler_tasks", "failed_executions", "0", false},
					{"scheduler_tasks", "consecutive_failures", "0", false},
					{"ai_providers", "provider_type", "'openai_compatible'", true},
					{"ai_providers", "enabled", "true", true},
					{"ai_providers", "timeout_seconds", "120", true},
					{"ai_providers", "enable_thinking", "false", true},
					{"ai_routes", "enabled", "true", true},
					{"ai_routes", "priority", "100", true},
					{"ai_routes", "strategy", "'ordered_failover'", true},
					{"ai_routes", "max_concurrency", "0", true},
					{"ai_route_providers", "priority", "100", true},
					{"ai_route_providers", "enabled", "true", true},
					{"ai_call_logs", "is_fallback", "false", true},
					// ── topic_graph.go ──
					{"topic_tags", "category", "'keyword'", true},
					{"topic_tags", "is_canonical", "false", false},
					{"topic_tags", "source", "'llm'", false},
					{"topic_tags", "feed_count", "0", false},
					{"topic_tags", "status", "'active'", true},
					{"topic_tags", "is_watched", "false", false},
					{"topic_tags", "quality_score", "0", false},
					{"topic_tags", "metadata", "'{}'::jsonb", false},
					{"topic_tags", "kind", "'keyword'", false},
					{"topic_tag_embeddings", "embedding_type", "'identity'", true},
					{"tag_merge_suggestions", "status", "'pending'", true},
					{"tag_merge_suggestions", "source", "'incremental'", true},
					{"article_topic_tags", "score", "0", false},
					{"article_topic_tags", "source", "'llm'", false},
					// ── semantic_label.go ──
					{"semantic_labels", "ref_count", "0", true},
					{"semantic_labels", "display_order", "0", true},
					{"semantic_labels", "source", "'llm_extract'", true},
					{"semantic_labels", "status", "'active'", true},
					{"semantic_labels", "protected", "false", true},
					{"semantic_labels", "enrichment_enabled", "false", true},
					{"semantic_labels", "window_days", "14", true},
					{"semantic_labels", "aliases", "'[]'::jsonb", false},
					{"semantic_labels", "context_layers", `'["week","month","year","all"]'::jsonb`, false},
					{"topic_tag_board_labels", "score", "0", true},
					{"topic_tag_board_labels", "downgraded", "false", true},
					{"topic_tag_board_labels", "direction_mismatch", "false", true},
				}
				for _, c := range cols {
					if !tableExists(db, c.table) {
						continue
					}
					if err := constrain(c.table, c.column, c.def, c.notNull); err != nil {
						return err
					}
				}

				// Columns with NOT NULL but NO default: string columns backfill '' then NOT NULL.
				// FK/PK uint columns and bool(success)/float(similarity/dimension) cannot be
				// safely backfilled with a blanket value — these tables are populated only via
				// code paths that always set the value, so on a healthy DB they have no NULLs.
				// We still SET NOT NULL (idempotent via columnIsNullable); if NULLs exist the
				// migration fails loudly (preferred over silent constraint vacuum).
				notNullOnly := []col{
					{"scheduler_tasks", "name", "", true},
					{"ai_settings", "key", "", true},
					{"ai_providers", "name", "", true},
					{"ai_providers", "base_url", "", true},
					{"ai_providers", "model", "", true},
					{"ai_routes", "name", "", true},
					{"ai_routes", "capability", "", true},
					{"ai_route_providers", "route_id", "0", true},
					{"ai_route_providers", "provider_id", "0", true},
					{"ai_call_logs", "capability", "", true},
					{"ai_call_logs", "route_name", "", true},
					{"ai_call_logs", "provider_name", "", true},
					{"ai_call_logs", "success", "false", true},
					{"topic_tags", "slug", "", true},
					{"topic_tags", "label", "", true},
					{"topic_tag_embeddings", "topic_tag_id", "0", true},
					{"topic_tag_embeddings", "dimension", "0", true},
					{"topic_tag_embeddings", "model", "", true},
					{"tag_merge_suggestions", "new_tag_id", "0", true},
					{"tag_merge_suggestions", "existing_tag_id", "0", true},
					{"tag_merge_suggestions", "new_label", "", true},
					{"tag_merge_suggestions", "existing_label", "", true},
					{"tag_merge_suggestions", "category", "", true},
					{"tag_merge_suggestions", "similarity", "0", true},
					{"article_topic_tags", "article_id", "0", true},
					{"article_topic_tags", "topic_tag_id", "0", true},
					{"semantic_labels", "label", "", true},
					{"semantic_labels", "slug", "", true},
					{"semantic_labels", "label_type", "", true},
				}
				for _, c := range notNullOnly {
					if !tableExists(db, c.table) {
						continue
					}
					// String columns: backfill '' (empty); numeric/bool: backfill the literal.
					backfill := "''"
					if c.def != "" {
						backfill = c.def
					}
					if err := ensureNotNullDefault(db, c.table, c.column, backfill); err != nil {
						return err
					}
				}
				return nil
			},
		},
	}
}

// InvalidateBoardCache is an optional hook set by the board service to invalidate
// the semantic board cache after the aux-label dup-merge migration (which modifies
// board_composition). Set from semantic_board_cache.go init().
var InvalidateBoardCache func()

// runAuxLabelDupMerge performs a one-shot deduplication of active auxiliary
// labels whose normalizeKey is identical (text-variant duplicates like
// "SK 海力士" / "SK海力士").
//
// For each group: the label with the highest ref_count (ties broken by smallest
// id) is the primary; all others are merged into it by moving aliases,
// topic_tag_semantic_labels, and board_composition references, then disabling
// the source. This mirrors the runtime MergeAuxiliaryLabelAlias semantics.
//
// Idempotent: after a successful run no group will have count>1 (disabled
// labels are excluded from the grouping query).
func runAuxLabelDupMerge(db *gorm.DB) error {
	// Query active auxiliary labels.
	type auxRow struct {
		ID       uint
		Label    string
		RefCount int
	}
	var allRows []auxRow
	if err := db.Model(&models.SemanticLabel{}).
		Select("id, label, ref_count").
		Where("label_type = ? AND status = ?", "auxiliary", "active").
		Order("id ASC").
		Find(&allRows).Error; err != nil {
		return fmt.Errorf("dup-merge: query active auxiliary labels: %w", err)
	}

	// Group by normalizeKey.
	groups := make(map[string][]auxRow)
	for _, r := range allRows {
		nk := textutil.NormalizeLabelKey(r.Label)
		groups[nk] = append(groups[nk], r)
	}

	var mergeCount int
	for nk, group := range groups {
		if len(group) < 2 {
			continue
		}

		// Primary = highest ref_count, ties broken by smallest id.
		primary := group[0]
		for i := 1; i < len(group); i++ {
			if group[i].RefCount > primary.RefCount ||
				(group[i].RefCount == primary.RefCount && group[i].ID < primary.ID) {
				primary = group[i]
			}
		}

		for _, secondary := range group {
			if secondary.ID == primary.ID {
				continue
			}

			logging.Infof("Dup-merge: normalize_key=%q: merging source=%d(%q, ref=%d) → target=%d(%q, ref=%d)",
				nk, secondary.ID, secondary.Label, secondary.RefCount, primary.ID, primary.Label, primary.RefCount)

			if err := mergeOneAuxLabelDup(db, secondary.ID, primary.ID); err != nil {
				return fmt.Errorf("dup-merge: merge source=%d into target=%d: %w", secondary.ID, primary.ID, err)
			}
			mergeCount++
		}

		logging.Infof("Dup-merge: normalized %d duplicates into primary %q (id=%d)", len(group)-1, primary.Label, primary.ID)
	}

	logging.Infof("Dup-merge: complete — %d auxiliary labels merged across all groups", mergeCount)

	// Invalidate board composition cache (board_composition rows may have been reassigned).
	if InvalidateBoardCache != nil {
		InvalidateBoardCache()
	}

	return nil
}

// mergeOneAuxLabelDup merges a single source auxiliary label into a target.
// Mirrors MergeAuxiliaryLabelAlias semantics but uses direct DB operations
// (the service method lives in a package that would create a circular import).
func mergeOneAuxLabelDup(db *gorm.DB, sourceID, targetID uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var source, target models.SemanticLabel
		if err := tx.Where("id = ? AND label_type = ?", sourceID, "auxiliary").First(&source).Error; err != nil {
			return fmt.Errorf("load source: %w", err)
		}
		if err := tx.Where("id = ? AND label_type = ?", targetID, "auxiliary").First(&target).Error; err != nil {
			return fmt.Errorf("load target: %w", err)
		}

		// Merge aliases: source label + source aliases → target aliases (dedup).
		aliasSet := make(map[string]bool)
		for _, a := range target.Aliases {
			aliasSet[strings.ToLower(strings.TrimSpace(a))] = true
		}
		for _, a := range append([]string{source.Label}, source.Aliases...) {
			key := strings.ToLower(strings.TrimSpace(a))
			if !aliasSet[key] && !strings.EqualFold(target.Label, a) {
				target.Aliases = append(target.Aliases, a)
				aliasSet[key] = true
			}
		}
		if err := tx.Save(&target).Error; err != nil {
			return fmt.Errorf("save target aliases: %w", err)
		}

		// Migrate topic_tag_semantic_labels: source → target, ON CONFLICT DO NOTHING.
		var links []models.TopicTagSemanticLabel
		if err := tx.Where("semantic_label_id = ?", sourceID).Find(&links).Error; err != nil {
			return fmt.Errorf("load source links: %w", err)
		}
		for _, link := range links {
			migrated := models.TopicTagSemanticLabel{TopicTagID: link.TopicTagID, SemanticLabelID: targetID}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&migrated).Error; err != nil {
				return fmt.Errorf("migrate link topic_tag=%d: %w", link.TopicTagID, err)
			}
		}
		if err := tx.Where("semantic_label_id = ?", sourceID).Delete(&models.TopicTagSemanticLabel{}).Error; err != nil {
			return fmt.Errorf("delete source links: %w", err)
		}

		// Migrate board_composition: source → target, ON CONFLICT DO NOTHING.
		type boardCompRow struct {
			BoardID          uint `gorm:"column:board_id"`
			AuxiliaryLabelID uint `gorm:"column:auxiliary_label_id"`
		}
		var comps []boardCompRow
		if err := tx.Table("board_composition").Where("auxiliary_label_id = ?", sourceID).Find(&comps).Error; err != nil {
			return fmt.Errorf("load source board_composition: %w", err)
		}
		for _, comp := range comps {
			if err := tx.Exec(`
				INSERT INTO board_composition (board_id, auxiliary_label_id)
				VALUES (?, ?) ON CONFLICT DO NOTHING
			`, comp.BoardID, targetID).Error; err != nil {
				return fmt.Errorf("migrate board_composition board=%d: %w", comp.BoardID, err)
			}
		}
		if err := tx.Where("auxiliary_label_id = ?", sourceID).Delete(&models.BoardComposition{}).Error; err != nil {
			return fmt.Errorf("delete source board_composition: %w", err)
		}

		// Recalculate ref_counts.
		var targetRefCount int64
		if err := tx.Model(&models.TopicTagSemanticLabel{}).Where("semantic_label_id = ?", targetID).Count(&targetRefCount).Error; err != nil {
			return fmt.Errorf("count target refs: %w", err)
		}
		var sourceRefCount int64
		if err := tx.Model(&models.TopicTagSemanticLabel{}).Where("semantic_label_id = ?", sourceID).Count(&sourceRefCount).Error; err != nil {
			return fmt.Errorf("count source refs: %w", err)
		}
		if err := tx.Model(&models.SemanticLabel{}).Where("id = ?", targetID).Update("ref_count", int(targetRefCount)).Error; err != nil {
			return fmt.Errorf("update target ref_count: %w", err)
		}
		if err := tx.Model(&models.SemanticLabel{}).Where("id = ?", sourceID).Updates(map[string]any{
			"ref_count": int(sourceRefCount),
			"status":    "disabled",
		}).Error; err != nil {
			return fmt.Errorf("disable source: %w", err)
		}

		return nil
	})
}

// PruneUnderqualifiedCandidates hard-deletes all candidate topics with
// hit_count < upgradeThreshold. Sections referencing them are unlinked
// (persistent_topic_id / match fields / topic_status_at_report set to NULL).
// Relations pointing to deleted topics are cleaned up. Idempotent — second run
// is a no-op. Returns the number of topics deleted.
func PruneUnderqualifiedCandidates(db *gorm.DB, upgradeThreshold int) (deleted int, err error) {
	var topicIDs []uint
	if err = db.Table("board_persistent_topics").
		Where("status = ? AND hit_count < ?", "candidate", upgradeThreshold).
		Pluck("id", &topicIDs).Error; err != nil {
		return 0, fmt.Errorf("prune: query candidates: %w", err)
	}
	if len(topicIDs) == 0 {
		return 0, nil
	}

	// Collect affected board IDs before any mutation so we can rebuild
	// their relations after topic deletion.
	var boardIDs []uint
	if PruneRelationsRebuild != nil {
		if err = db.Table("board_persistent_topics").
			Where("id IN ?", topicIDs).
			Distinct("semantic_board_id").
			Pluck("semantic_board_id", &boardIDs).Error; err != nil {
			return 0, fmt.Errorf("prune: query board IDs: %w", err)
		}
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		// Unlink sections that reference the to-be-deleted topics.
		if err2 := tx.Table("daily_report_sections").
			Where("persistent_topic_id IN ?", topicIDs).
			Updates(map[string]interface{}{
				"persistent_topic_id":    nil,
				"topic_match_distance":   nil,
				"topic_match_confidence": nil,
				"topic_status_at_report": nil,
			}).Error; err2 != nil {
			return fmt.Errorf("prune: unlink sections: %w", err2)
		}

		// Delete the candidate rows.
		if err2 := tx.Table("board_persistent_topics").
			Where("id IN ?", topicIDs).
			Delete(nil).Error; err2 != nil {
			return fmt.Errorf("prune: delete topics: %w", err2)
		}

		// Rebuild relations for each affected board so stale identity /
		// similarity edges pointing to now-deleted topics are dropped.
		for _, boardID := range boardIDs {
			if err2 := PruneRelationsRebuild(tx, boardID); err2 != nil {
				return fmt.Errorf("prune: rebuild relations for board %d: %w", boardID, err2)
			}
		}

		return nil
	})
	if err != nil {
		return 0, err
	}
	return len(topicIDs), nil
}
