package repository

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// CreateWatchInput is the full creation payload for a watch (watch-materialized-topic):
// hint tracks (label/keyword) use Label+Type only; keyword_topic adds nothing;
// sentence_topic adds Query (retrieval sentence, may equal Label) and an
// optional EmbeddingCache (empty = lazy-compute at next report generation).
type CreateWatchInput struct {
	SemanticBoardID uint
	Label           string
	Type            string
	Query           string
	EmbeddingCache  *string
}

// CreateWatch creates a new BoardTopicWatch for a semantic board.
// The new watch starts with status=active by default. input.Type selects the
// track: WatchTypeLabel (AI semantic hint, the default when passed "" for
// backward compatibility), WatchTypeKeyword (pure text hint), or the
// materialized tracks WatchTypeKeywordTopic / WatchTypeSentenceTopic.
func (r *TopicGraphRepository) CreateWatch(input CreateWatchInput) (*BoardTopicWatch, error) {
	watchType := input.Type
	if watchType == "" {
		watchType = WatchTypeLabel
	}
	watch := BoardTopicWatch{
		SemanticBoardID: input.SemanticBoardID,
		Label:           input.Label,
		Query:           input.Query,
		Type:            watchType,
		EmbeddingCache:  input.EmbeddingCache,
		Status:          WatchStatusActive,
	}
	if err := r.db.Create(&watch).Error; err != nil {
		return nil, fmt.Errorf("create watch: %w", err)
	}
	return &watch, nil
}

// GetWatchByID returns a single watch by ID (used by the keyword instant
// match to load the watch's expression).
func (r *TopicGraphRepository) GetWatchByID(watchID uint) (*BoardTopicWatch, error) {
	var watch BoardTopicWatch
	if err := r.db.First(&watch, watchID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("watch %d not found", watchID)
		}
		return nil, fmt.Errorf("find watch: %w", err)
	}
	return &watch, nil
}

// ListWatchesByBoard returns all watches for a semantic board (any status).
func (r *TopicGraphRepository) ListWatchesByBoard(boardID uint) ([]BoardTopicWatch, error) {
	var watches []BoardTopicWatch
	if err := r.db.Where("semantic_board_id = ?", boardID).Order("created_at ASC").Find(&watches).Error; err != nil {
		return nil, fmt.Errorf("list watches: %w", err)
	}
	return watches, nil
}

// ListActiveWatchesByBoard returns only active watches for a semantic board.
func (r *TopicGraphRepository) ListActiveWatchesByBoard(boardID uint) ([]BoardTopicWatch, error) {
	var watches []BoardTopicWatch
	if err := r.db.Where("semantic_board_id = ? AND status = ?", boardID, WatchStatusActive).
		Order("created_at ASC").Find(&watches).Error; err != nil {
		return nil, fmt.Errorf("list active watches: %w", err)
	}
	return watches, nil
}

// ListActiveMaterializedWatchesByBoard returns the active materialized-track
// watches (keyword_topic / sentence_topic) for a board — the daily-report
// materialization phase input (watch-materialized-topic). Hint tracks
// (label / keyword) are excluded: they never materialize.
func (r *TopicGraphRepository) ListActiveMaterializedWatchesByBoard(boardID uint) ([]BoardTopicWatch, error) {
	var watches []BoardTopicWatch
	if err := r.db.Where("semantic_board_id = ? AND status = ? AND type IN ?",
		boardID, WatchStatusActive, []string{WatchTypeKeywordTopic, WatchTypeSentenceTopic}).
		Order("created_at ASC").Find(&watches).Error; err != nil {
		return nil, fmt.Errorf("list active materialized watches: %w", err)
	}
	return watches, nil
}

// UpdateWatchEmbeddingCache writes back a sentence_topic watch's recomputed
// query vector (lazy recompute path — the cache was empty/invalidated and the
// daily-report generation just embedded the retrieval sentence).
func (r *TopicGraphRepository) UpdateWatchEmbeddingCache(watchID uint, pgVector string) error {
	if err := r.db.Model(&BoardTopicWatch{}).Where("id = ?", watchID).
		Update("embedding_cache", pgVector).Error; err != nil {
		return fmt.Errorf("update watch embedding cache: %w", err)
	}
	return nil
}

// SetWatchPersistentTopic binds a sentence_topic watch to its dedicated
// persistent topic (set once at first materialization; watch-materialized-topic
// design D4).
func (r *TopicGraphRepository) SetWatchPersistentTopic(watchID, topicID uint) error {
	if err := r.db.Model(&BoardTopicWatch{}).Where("id = ?", watchID).
		Update("persistent_topic_id", topicID).Error; err != nil {
		return fmt.Errorf("set watch persistent topic: %w", err)
	}
	return nil
}

// UpdateWatch updates a watch's label, query and/or status. Pass nil to
// leave a field unchanged. Updating label or query invalidates the
// sentence_topic embedding cache (the cached vector no longer matches the
// retrieval sentence; the next daily-report generation lazily recomputes).
func (r *TopicGraphRepository) UpdateWatch(watchID uint, label, query, status *string) (*BoardTopicWatch, error) {
	var watch BoardTopicWatch
	if err := r.db.First(&watch, watchID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("watch %d not found", watchID)
		}
		return nil, fmt.Errorf("find watch: %w", err)
	}

	updates := map[string]interface{}{}
	if label != nil {
		updates["label"] = *label
	}
	if query != nil {
		updates["query"] = *query
	}
	if label != nil || query != nil {
		// Cache invalidation: any retrieval-sentence change (explicit query
		// PATCH, or a label PATCH on a watch whose query fell back to label)
		// must drop the stale vector (watch-materialized-topic design D3).
		updates["embedding_cache"] = nil
	}
	if status != nil {
		updates["status"] = *status
	}
	if len(updates) == 0 {
		return &watch, nil
	}

	if err := r.db.Model(&watch).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update watch: %w", err)
	}
	// Reload to return fresh data
	if err := r.db.First(&watch, watchID).Error; err != nil {
		return nil, fmt.Errorf("reload watch after update: %w", err)
	}
	return &watch, nil
}

// DeleteWatch deletes a watch and cascade-deletes its hits (via FK OnDelete:CASCADE).
func (r *TopicGraphRepository) DeleteWatch(watchID uint) error {
	result := r.db.Delete(&BoardTopicWatch{}, watchID)
	if result.Error != nil {
		return fmt.Errorf("delete watch: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("watch %d not found", watchID)
	}
	return nil
}

// GetWatchHitsByReport returns active watch hits for a daily report, including
// the joined watch descriptor used by the report detail index. Joining here is
// intentional: paused watches and deleted watches must disappear on reread.
func (r *TopicGraphRepository) GetWatchHitsByReport(reportID uint) ([]TopicWatchHit, error) {
	type watchHitRow struct {
		ID         uint      `gorm:"column:id"`
		WatchID    uint      `gorm:"column:watch_id"`
		SectionID  uint      `gorm:"column:section_id"`
		ReportID   uint      `gorm:"column:report_id"`
		PeriodDate time.Time `gorm:"column:period_date"`
		Reason     string    `gorm:"column:reason"`
		CreatedAt  time.Time `gorm:"column:created_at"`
		WatchLabel string    `gorm:"column:watch_label"`
		WatchType  string    `gorm:"column:watch_type"`
	}

	var rows []watchHitRow
	err := r.db.Table("topic_watch_hits AS h").
		Select(`h.id, h.watch_id, h.section_id, h.report_id, h.period_date,
			h.reason, h.created_at, w.label AS watch_label, w.type AS watch_type`).
		Joins("JOIN board_topic_watches AS w ON w.id = h.watch_id").
		Where("h.report_id = ? AND w.status = ?", reportID, WatchStatusActive).
		Order("h.created_at ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("get watch hits: %w", err)
	}

	hits := make([]TopicWatchHit, len(rows))
	for i, row := range rows {
		hits[i] = TopicWatchHit(row)
	}
	return hits, nil
}

// SectionText is the keyword-matching view of one section: its identity
// (section / report / period) plus its threads' title+summary text. Assembled
// by ListWatchSectionTextsByReport / ListWatchSectionTextsSince.
type SectionText struct {
	SectionID  uint
	ReportID   uint
	PeriodDate time.Time
	Threads    []ThreadText
}

// ThreadText carries one thread's text fields for keyword matching. Summary
// may be empty when the thread has only a title (legal degradation).
type ThreadText struct {
	Title   string
	Summary string
}

// watchSectionTextRow is the raw LEFT-JOIN row: one per (section, thread);
// sections without any thread produce a single row with NULL title/summary.
type watchSectionTextRow struct {
	SectionID  uint
	ReportID   uint
	PeriodDate time.Time
	Title      *string
	Summary    *string
}

const watchSectionTextSQL = `
	SELECT s.id AS section_id, r.id AS report_id, r.period_date AS period_date,
	       t.title, t.summary
	FROM daily_report_sections s
	JOIN board_daily_reports r ON r.id = s.report_id
	LEFT JOIN daily_report_threads t ON t.section_id = s.id
	-- Materialized sections are invisible to the keyword hint track (spec:
	-- 物化 section 不被提示轨扫描命中; watch-materialized-topic).
	AND (s.lane_tier IS NULL OR s.lane_tier NOT LIKE 'watch_%%')
	%s
	ORDER BY s.id ASC, t.id ASC
`

// aggregateSectionTextRows groups raw (section, thread) rows into one
// SectionText per section, preserving section id order. Deliberately plain
// SQL + Go aggregation (no PG-only string_agg) so handler-level SQLite tests
// can exercise the same query path.
func aggregateSectionTextRows(rows []watchSectionTextRow) []SectionText {
	byID := make(map[uint]*SectionText, len(rows))
	var order []uint
	for _, rw := range rows {
		st, ok := byID[rw.SectionID]
		if !ok {
			st = &SectionText{
				SectionID:  rw.SectionID,
				ReportID:   rw.ReportID,
				PeriodDate: rw.PeriodDate,
			}
			byID[rw.SectionID] = st
			order = append(order, rw.SectionID)
		}
		if rw.Title == nil && rw.Summary == nil {
			continue // LEFT JOIN miss: section has no thread rows
		}
		th := ThreadText{}
		if rw.Title != nil {
			th.Title = *rw.Title
		}
		if rw.Summary != nil {
			th.Summary = *rw.Summary
		}
		st.Threads = append(st.Threads, th)
	}
	out := make([]SectionText, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	return out
}

// ListWatchSectionTextsByReport returns every section of a report together
// with its threads' text — the keyword-track input at daily-report generation
// time (EvaluateWatchHits runs after SaveReport, so threads are persisted).
func (r *TopicGraphRepository) ListWatchSectionTextsByReport(reportID uint) ([]SectionText, error) {
	var rows []watchSectionTextRow
	q := fmt.Sprintf(watchSectionTextSQL, "WHERE r.id = ?")
	if err := r.db.Raw(q, reportID).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list watch section texts by report: %w", err)
	}
	return aggregateSectionTextRows(rows), nil
}

// ListWatchSectionTextsSince returns the sections (with threads text) of all
// reports of a board whose period_date >= since (date-granular comparison) —
// the keyword instant-match lookback window.
func (r *TopicGraphRepository) ListWatchSectionTextsSince(boardID uint, since time.Time) ([]SectionText, error) {
	var rows []watchSectionTextRow
	q := fmt.Sprintf(watchSectionTextSQL, "WHERE r.semantic_board_id = ? AND r.period_date >= ?")
	if err := r.db.Raw(q, boardID, since).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list watch section texts since: %w", err)
	}
	return aggregateSectionTextRows(rows), nil
}

// ListWatchScanArticles returns the keyword-materialization scan pool: all
// unarchived articles published in [start, end), each with its topic-tag IDs
// and the best available summary-layer text. Summary precedence (AI content
// summary > Firecrawl content > raw content > description) is coalesced in
// SQL with empty strings NULLIF'd away, mirroring buildArticleContextForTag's
// Go-side precedence. limit caps the scan (extreme-volume guard, design §D2);
// 0 means unlimited.
func (r *TopicGraphRepository) ListWatchScanArticles(start, end time.Time, limit int) (articles []WatchScanArticle, err error) {
	q := r.db.Table("articles").
		Select(`articles.id AS id,
			articles.title AS title,
			COALESCE(NULLIF(articles.ai_content_summary, ''),
			         NULLIF(articles.firecrawl_content, ''),
			         NULLIF(articles.content, ''),
			         NULLIF(articles.description, '')) AS summary,
			CASE WHEN COUNT(article_topic_tags.id) = 0 THEN NULL
			     ELSE (SELECT json_agg(t ORDER BY t) FROM (
			         SELECT DISTINCT article_topic_tags.topic_tag_id AS t
			         FROM article_topic_tags
			         WHERE article_topic_tags.article_id = articles.id) sub) END AS tag_ids`).
		Joins("LEFT JOIN article_topic_tags ON article_topic_tags.article_id = articles.id").
		Where("articles.archived = ? AND articles.pub_date >= ? AND articles.pub_date < ?", false, start, end).
		Group("articles.id, articles.title, articles.ai_content_summary, articles.firecrawl_content, articles.content, articles.description").
		Order("articles.id ASC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Scan(&articles).Error; err != nil {
		return nil, fmt.Errorf("list watch scan articles: %w", err)
	}
	return articles, nil
}

// WatchScanArticle is the keyword-materialization view of one article:
// identity plus the best summary-layer text and the article's topic-tag IDs
// as a raw JSON array string (service-side DNF matching runs over
// Title+"\n"+Summary; TagIDs JSON is parsed by the caller when needed).
type WatchScanArticle struct {
	ID      uint    `gorm:"column:id"`
	Title   string  `gorm:"column:title"`
	Summary string  `gorm:"column:summary"`
	TagIDs  *string `gorm:"column:tag_ids"`
}

// ListBoardAuxLabelEmbeddings returns the board's auxiliary-label pool with
// parsed embeddings — the sentence-track retrieval input (design §D3).
// BoardComposition links a board (SemanticLabel row) to its auxiliary
// labels; only labels carrying an embedding participate (NULL is skipped).
func (r *TopicGraphRepository) ListBoardAuxLabelEmbeddings(boardID uint) ([]WatchSentenceLabel, error) {
	type row struct {
		ID        uint    `gorm:"column:id"`
		Label     string  `gorm:"column:label"`
		Embedding *string `gorm:"column:embedding"`
	}
	var rows []row
	if err := r.db.Table("board_composition bc").
		Select("l.id AS id, l.label AS label, l.embedding AS embedding").
		Joins("JOIN semantic_labels l ON l.id = bc.auxiliary_label_id").
		Where("bc.board_id = ? AND l.embedding IS NOT NULL", boardID).
		Order("l.id ASC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list board aux label embeddings: %w", err)
	}
	out := make([]WatchSentenceLabel, 0, len(rows))
	for _, rw := range rows {
		vec, err := repoParsePgVector(*rw.Embedding)
		if err != nil || len(vec) == 0 {
			continue // malformed vector: skip the label (legal degradation)
		}
		out = append(out, WatchSentenceLabel{ID: rw.ID, Label: rw.Label, Embedding: vec})
	}
	return out, nil
}

// WatchSentenceLabel is one auxiliary label with its parsed embedding
// (service-side cosine retrieval input).
type WatchSentenceLabel struct {
	ID        uint
	Label     string
	Embedding []float64
}

// ListActiveEventTagsBySemanticLabels resolves hit auxiliary labels to the
// active event tags linked via topic_tag_semantic_labels that have at least
// one article published in [start, end) — the sentence-track tag set. The
// same tag reached via several labels dedupes in the caller (or naturally by
// the DISTINCT here).
func (r *TopicGraphRepository) ListActiveEventTagsBySemanticLabels(labelIDs []uint, start, end time.Time) ([]uint, error) {
	if len(labelIDs) == 0 {
		return nil, nil
	}
	var tagIDs []uint
	err := r.db.Table("topic_tag_semantic_labels tsl").
		Select("DISTINCT tt.id").
		Joins("JOIN topic_tags tt ON tt.id = tsl.topic_tag_id").
		Joins("JOIN article_topic_tags att ON att.topic_tag_id = tt.id").
		Joins("JOIN articles a ON a.id = att.article_id").
		Where("tsl.semantic_label_id IN ? AND tt.status = ? AND tt.category = ?", labelIDs, "active", "event").
		Where("a.archived = ? AND a.pub_date >= ? AND a.pub_date < ?", false, start, end).
		Order("tt.id ASC").
		Pluck("tt.id", &tagIDs).Error
	if err != nil {
		return nil, fmt.Errorf("list event tags by semantic labels: %w", err)
	}
	return tagIDs, nil
}

// ListArticlesByTagsForDay returns the deduplicated article union (id-ordered)
// across the given tags for [start, end), each with the best summary-layer
// text (same coalesce precedence as ListWatchScanArticles) and its raw tag-id
// JSON. The sentence-track article pool.
func (r *TopicGraphRepository) ListArticlesByTagsForDay(tagIDs []uint, start, end time.Time) ([]WatchScanArticle, error) {
	if len(tagIDs) == 0 {
		return nil, nil
	}
	var articles []WatchScanArticle
	err := r.db.Table("articles").
		Select(`articles.id AS id,
			articles.title AS title,
			COALESCE(NULLIF(articles.ai_content_summary, ''),
			         NULLIF(articles.firecrawl_content, ''),
			         NULLIF(articles.content, ''),
			         NULLIF(articles.description, '')) AS summary,
			(SELECT json_agg(t ORDER BY t) FROM (
			     SELECT DISTINCT article_topic_tags.topic_tag_id AS t
			     FROM article_topic_tags
			     WHERE article_topic_tags.article_id = articles.id) sub) AS tag_ids`).
		Joins("JOIN article_topic_tags ON article_topic_tags.article_id = articles.id").
		Where("article_topic_tags.topic_tag_id IN ? AND articles.archived = ? AND articles.pub_date >= ? AND articles.pub_date < ?",
			tagIDs, false, start, end).
		Group("articles.id, articles.title, articles.ai_content_summary, articles.firecrawl_content, articles.content, articles.description").
		Order("articles.id ASC").
		Scan(&articles).Error
	if err != nil {
		return nil, fmt.Errorf("list articles by tags for day: %w", err)
	}
	return articles, nil
}

// CreateWatchTopic creates a sentence_topic watch's dedicated persistent
// topic: status=active, source=manual (a full-citizen lane anchor — the
// persistent-topic spec treats manual+active exactly like any active topic).
// Both Embedding and Centroid are set to pgVec: Centroid is the lane anchor
// and must never lag behind on creation day (design §D4).
// Hit counters seed at 0: the very same SaveReport that persists this run's
// materialized section advances them to 1 via planLifecycle (mirrors how
// auto_new candidates end their creating day at 1 — no day-1 double count).
func (r *TopicGraphRepository) CreateWatchTopic(boardID uint, label, pgVec string, firstSeen time.Time) (uint, error) {
	topic := BoardPersistentTopic{
		SemanticBoardID: boardID,
		Label:           label,
		Status:          TopicStatusActive,
		Source:          TopicSourceManual,
		Embedding:       pgVec,
		Centroid:        pgVec,
		FirstSeenDate:   firstSeen,
		LastSeenDate:    firstSeen,
		HitCount:        0,
		ConsecutiveHits: 0,
	}
	if err := r.db.Create(&topic).Error; err != nil {
		return 0, fmt.Errorf("create watch topic: %w", err)
	}
	// The model tag defaults hit_count to 1 and GORM skips zero-valued int
	// fields on Create (Select with field names did not override it in this
	// GORM version) — an explicit UPDATE is the only reliable way to seed 0s.
	// Seeding matters: the same SaveReport's planLifecycle advances the day's
	// hit (+1), so seeded 0s end day one at exactly 1 (no double count).
	if err := r.db.Model(&BoardPersistentTopic{}).Where("id = ?", topic.ID).
		Updates(map[string]interface{}{"hit_count": 0, "consecutive_hits": 0}).Error; err != nil {
		return 0, fmt.Errorf("seed watch topic counters: %w", err)
	}
	return topic.ID, nil
}
