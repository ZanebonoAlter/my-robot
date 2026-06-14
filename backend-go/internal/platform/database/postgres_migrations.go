package database

import (
	"fmt"

	"gorm.io/gorm"
	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/logging"
)

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
			Description: "Set topic_tag_embeddings.embedding column type to vector(4096).",
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
	}
}
