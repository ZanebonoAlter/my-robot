package repository

import (
	"context"
	"fmt"
	"time"

	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/airouter"
	"syntopica-backend/internal/platform/logging"

	"gorm.io/gorm"
)

// SaveReport saves a daily report and its sections, replacing any existing
// report for the same board and date.

// GetReportByID retrieves a single daily report by its primary key.

// ReportListItem is a summary view for list endpoints.
type ReportListItem struct {
	ID              uint      `json:"id"`
	SemanticBoardID uint      `json:"semantic_board_id"`
	PeriodDate      string    `json:"period_date"`
	Title           string    `json:"title"`
	Summary         string    `json:"summary"`
	ArticleCount    int       `json:"article_count"`
	EventTagCount   int       `json:"event_tag_count"`
	ClusterCount    int       `json:"cluster_count"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
}

// ListReports returns recent reports for a board.

// ListReportsForAllBoards returns reports for all boards within a date range.

func NormalizeReportDate(date time.Time) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), 12, 0, 0, 0, time.UTC)
}

// collectBoardIDsForDate returns all board IDs that have active event tags on a date.

// SaveThreads persists a batch of DailyReportThread rows.

// SectionTimelineNode represents a section in a timeline view.
type SectionTimelineNode struct {
	ID           uint      `json:"id"`
	ReportID     uint      `json:"report_id"`
	PeriodDate   time.Time `json:"period_date"`
	ClusterLabel string    `json:"cluster_label"`
	Status       string    `json:"status"`
	ArticleCount int       `json:"article_count"`
	ThreadCount  int       `json:"thread_count"`
	ImageURL     string    `json:"image_url"`
}

// GetBoardSectionTimeline fetches all sections and their relations for a board within a date range.

// SectionRelationResult represents a relation record for API responses.
type SectionRelationResult struct {
	FromID   uint    `json:"from_id"`
	ToID     uint    `json:"to_id"`
	Distance float64 `json:"distance"`
}

// SectionTimelineResponse is the response for section timeline/lifecycle APIs.
type SectionTimelineResponse struct {
	Sections  []SectionTimelineNode   `json:"sections"`
	Relations []SectionRelationResult `json:"relations"`
}

// DeriveSectionStatuses computes status for each section based on its relation graph.
//
// Priority: merge > split > continuing > ending > emerging
func DeriveSectionStatuses(sectionIDs []uint, relations []SectionRelationResult, sectionDateMap map[uint]time.Time, latestDate time.Time) map[uint]string {
	statuses := make(map[uint]string, len(sectionIDs))

	// Build degree maps
	outDegree := make(map[uint]int) // from_section_id count
	inDegree := make(map[uint]int)  // to_section_id count
	hasIncoming := make(map[uint]bool)
	hasOutgoing := make(map[uint]bool)

	for _, r := range relations {
		outDegree[r.FromID]++
		inDegree[r.ToID]++
		hasOutgoing[r.FromID] = true
		hasIncoming[r.ToID] = true
	}

	idSet := make(map[uint]bool, len(sectionIDs))
	for _, id := range sectionIDs {
		idSet[id] = true
	}

	for _, id := range sectionIDs {
		switch {
		case !hasIncoming[id]:
			statuses[id] = "emerging"
		case inDegree[id] > 1:
			statuses[id] = "merge"
		default:
			// Check if any of its from-sections has out-degree > 1 (split)
			split := false
			for _, r := range relations {
				if r.ToID == id && outDegree[r.FromID] > 1 {
					split = true
					break
				}
			}
			if split {
				statuses[id] = "split"
			} else {
				statuses[id] = "continuing"
			}
		}

		// Override to ending if no outgoing relations and not on latest date
		if !hasOutgoing[id] {
			if d, ok := sectionDateMap[id]; ok && d.Before(latestDate) {
				statuses[id] = "ending"
			}
		}
	}

	return statuses
}

// BackfillSectionEmbeddings generates embeddings for sections that don't have one,
// then runs pgvector matching to set prev_section_id for all sections.
// It overwrites all prev_section_id values (including those from the old tag Jaccard matching).

// BackfillAllRelations rebuilds relations for all boards that have sections with embeddings.
// Daily Report Repository
// =============================================================================

// SaveReport saves a daily report and its sections, replacing any existing
// report for the same board and date.
func (r *TopicGraphRepository) SaveReport(report *BoardDailyReport, sections []DailyReportSection, threadBatches [][]DailyReportThread) error {
	report.PeriodDate = NormalizeReportDate(report.PeriodDate)
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Upsert report: find existing by (semantic_board_id, period_date)
		var existing BoardDailyReport
		findErr := tx.Where("semantic_board_id = ? AND period_date = ?",
			report.SemanticBoardID,
			report.PeriodDate.Format("2006-01-02")).
			First(&existing).Error

		if findErr == nil {
			// Update existing report
			report.ID = existing.ID
			if err := tx.Model(&existing).Updates(map[string]interface{}{
				"title":                     report.Title,
				"summary":                   report.Summary,
				"highlights":                report.Highlights,
				"dynamics":                  report.Dynamics,
				"article_count":             report.ArticleCount,
				"event_tag_count":           report.EventTagCount,
				"cluster_count":             report.ClusterCount,
				"status":                    report.Status,
				"raw_clusters":              report.RawClusters,
				"prev_report_id":            report.PrevReportID,
				"generation_prompt_version": report.GenerationPromptVersion,
			}).Error; err != nil {
				return fmt.Errorf("update report: %w", err)
			}
		} else {
			// Create new report
			if err := tx.Create(report).Error; err != nil {
				return fmt.Errorf("create report: %w", err)
			}
		}

		if findErr == nil {
			// Delete old threads
			if err := tx.Where("report_id = ?", existing.ID).Delete(&DailyReportThread{}).Error; err != nil {
				return fmt.Errorf("delete old threads: %w", err)
			}
			// Delete old sections
			if err := tx.Where("report_id = ?", existing.ID).Delete(&DailyReportSection{}).Error; err != nil {
				return fmt.Errorf("delete old sections: %w", err)
			}
		}

		// Insert new sections (with embedding)
		for i := range sections {
			sections[i].ReportID = report.ID
		}
		if len(sections) > 0 {
			if err := tx.CreateInBatches(sections, 20).Error; err != nil {
				return fmt.Errorf("create sections: %w", err)
			}
		}

		// Rebuild all relations for this board using bipartite matching
		if err := RebuildBoardRelations(tx, report.SemanticBoardID); err != nil {
			logging.Warnf("SaveReport: relation rebuild failed: %v", err)
		}

		// Save threads for each section (sections now have IDs after insertion)
		for secIdx, sec := range sections {
			if secIdx < len(threadBatches) && len(threadBatches[secIdx]) > 0 {
				if err := r.SaveThreads(tx, report.ID, sec.ID, threadBatches[secIdx]); err != nil {
					return fmt.Errorf("save threads for section %d: %w", secIdx, err)
				}
			}
		}

		logging.Infof("daily-report: saved report %d for board %d on %s (%d sections)",
			report.ID, report.SemanticBoardID, report.PeriodDate.Format("2006-01-02"), len(sections))
		return nil
	})
}

// GetReportByID retrieves a single daily report by its primary key.
func (r *TopicGraphRepository) GetReportByID(id uint) (*BoardDailyReport, error) {
	var report BoardDailyReport
	err := r.db.Where("id = ?", id).
		Preload("Sections.Threads", func(db *gorm.DB) *gorm.DB {
			return db.Order("id ASC")
		}).
		Preload("Sections", func(db *gorm.DB) *gorm.DB {
			return db.Order("cluster_index ASC")
		}).
		First(&report).Error
	if err != nil {
		return nil, fmt.Errorf("report %d not found: %w", id, err)
	}
	return &report, nil
}

// ListReports returns recent reports for a board.
func (r *TopicGraphRepository) ListReports(boardID uint, days int) ([]ReportListItem, error) {
	if days <= 0 {
		days = 7
	}

	now := NormalizeReportDate(time.Now())
	rangeStart := now.AddDate(0, 0, -(days - 1))
	rangeEnd := now.AddDate(0, 0, 1)

	var reports []BoardDailyReport
	err := r.db.Where("semantic_board_id = ? AND period_date >= ? AND period_date < ?",
		boardID, rangeStart.Format("2006-01-02"), rangeEnd.Format("2006-01-02")).
		Order("period_date DESC").
		Find(&reports).Error
	if err != nil {
		return nil, fmt.Errorf("list reports for board %d: %w", boardID, err)
	}

	items := make([]ReportListItem, len(reports))
	for i, rpt := range reports {
		items[i] = ReportListItem{
			ID:              rpt.ID,
			SemanticBoardID: rpt.SemanticBoardID,
			PeriodDate:      rpt.PeriodDate.Format("2006-01-02"),
			Title:           rpt.Title,
			Summary:         rpt.Summary,
			ArticleCount:    rpt.ArticleCount,
			EventTagCount:   rpt.EventTagCount,
			ClusterCount:    rpt.ClusterCount,
			Status:          rpt.Status,
			CreatedAt:       rpt.CreatedAt,
		}
	}
	return items, nil
}

// ListReportsForAllBoards returns reports for all boards within a date range.
func (r *TopicGraphRepository) ListReportsForAllBoards(days int) ([]BoardDailyReport, error) {
	if days <= 0 {
		days = 7
	}
	if days > 30 {
		days = 30
	}

	now := NormalizeReportDate(time.Now())
	rangeStart := now.AddDate(0, 0, -(days - 1))
	rangeEnd := now.AddDate(0, 0, 1)

	var reports []BoardDailyReport
	err := r.db.Where("period_date >= ? AND period_date < ?",
		rangeStart.Format("2006-01-02"), rangeEnd.Format("2006-01-02")).
		Order("period_date DESC, semantic_board_id ASC").
		Preload("Sections", func(db *gorm.DB) *gorm.DB {
			return db.Order("cluster_index ASC")
		}).
		Find(&reports).Error
	if err != nil {
		return nil, fmt.Errorf("list reports: %w", err)
	}

	return reports, nil
}

// CollectBoardIDsForDate returns all board IDs that have active event tags on a date.
func (r *TopicGraphRepository) CollectBoardIDsForDate(date time.Time) ([]uint, error) {
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	type row struct {
		SemanticBoardID uint `json:"semantic_board_id"`
	}
	var rows []row
	err := r.db.Model(&models.TopicTag{}).
		Select("DISTINCT topic_tag_board_labels.semantic_board_id").
		Joins("JOIN topic_tag_board_labels ON topic_tag_board_labels.topic_tag_id = topic_tags.id").
		Joins("JOIN article_topic_tags ON article_topic_tags.topic_tag_id = topic_tags.id").
		Joins("JOIN articles ON articles.id = article_topic_tags.article_id").
		Where("topic_tags.status = ? AND topic_tags.category = ?", "active", models.TagCategoryEvent).
		Where("articles.pub_date >= ? AND articles.pub_date < ?", startOfDay, endOfDay).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	ids := make([]uint, len(rows))
	for i, rw := range rows {
		ids[i] = rw.SemanticBoardID
	}
	return ids, nil
}

// SaveThreads persists a batch of DailyReportThread rows using the given DB handle.
// Accepts *gorm.DB so it can participate in an outer transaction.
func (r *TopicGraphRepository) SaveThreads(tx *gorm.DB, reportID, sectionID uint, threads []DailyReportThread) error {
	for i := range threads {
		threads[i].ReportID = reportID
		threads[i].SectionID = sectionID
	}
	return tx.Create(&threads).Error
}

// GetBoardSectionTimeline fetches all sections and their relations for a board within a date range.
func (r *TopicGraphRepository) GetBoardSectionTimeline(boardID uint, days int) (SectionTimelineResponse, error) {
	if days <= 0 {
		days = 30
	}
	if days > 90 {
		days = 90
	}
	var nodes []SectionTimelineNode
	err := r.db.Raw(`
		SELECT ds.id, ds.report_id, bdr.period_date, ds.cluster_label,
	       ds.article_count,
	       (SELECT COUNT(*) FROM daily_report_threads t WHERE t.section_id = ds.id) AS thread_count,
	       COALESCE(
	         (
	           SELECT a.image_url
	           FROM daily_report_threads t
	           JOIN LATERAL jsonb_array_elements_text(COALESCE(t.related_article_ids, '[]'::jsonb)) aid(article_id) ON true
	           JOIN articles a ON a.id = aid.article_id::bigint
	           WHERE t.section_id = ds.id
	             AND a.image_url IS NOT NULL
	             AND a.image_url != ''
	           ORDER BY t.id ASC, a.pub_date DESC NULLS LAST, a.id ASC
	           LIMIT 1
	         ),
	         (
	           SELECT a.image_url
	           FROM jsonb_array_elements_text(COALESCE(ds.cluster_tag_ids, '[]'::jsonb)) tid(tag_id)
	           JOIN article_topic_tags att ON att.topic_tag_id = tid.tag_id::bigint
	           JOIN articles a ON a.id = att.article_id
	           WHERE a.pub_date >= bdr.period_date
	             AND a.pub_date < bdr.period_date + INTERVAL '1 day'
	             AND a.image_url IS NOT NULL
	             AND a.image_url != ''
	           ORDER BY a.pub_date DESC NULLS LAST, a.id ASC
	           LIMIT 1
	         ),
	         ''
	       ) AS image_url
		FROM daily_report_sections ds
		JOIN board_daily_reports bdr ON bdr.id = ds.report_id
		WHERE bdr.semantic_board_id = ?
		  AND bdr.period_date >= CURRENT_DATE - ? * INTERVAL '1 day'
		  AND bdr.status = 'completed'
		ORDER BY bdr.period_date DESC, ds.id ASC
	`, boardID, days).Scan(&nodes).Error
	if err != nil {
		return SectionTimelineResponse{}, fmt.Errorf("get board section timeline: %w", err)
	}

	if len(nodes) == 0 {
		return SectionTimelineResponse{
			Sections:  []SectionTimelineNode{},
			Relations: []SectionRelationResult{},
		}, nil
	}

	// Collect section IDs and find latest date
	sectionIDs := make([]uint, len(nodes))
	sectionDateMap := make(map[uint]time.Time, len(nodes))
	var latestDate time.Time
	for i, n := range nodes {
		sectionIDs[i] = n.ID
		sectionDateMap[n.ID] = n.PeriodDate
		if n.PeriodDate.After(latestDate) {
			latestDate = n.PeriodDate
		}
	}

	// Query relations involving these sections
	var relations []SectionRelationResult
	if err := r.db.Raw(`
		SELECT from_section_id AS from_id, to_section_id AS to_id, distance
		FROM daily_report_section_relations
		WHERE from_section_id IN ? OR to_section_id IN ?
	`, sectionIDs, sectionIDs).Scan(&relations).Error; err != nil {
		logging.Warnf("GetBoardSectionTimeline: query relations failed: %v", err)
	}

	// Derive statuses
	statuses := DeriveSectionStatuses(sectionIDs, relations, sectionDateMap, latestDate)
	for i := range nodes {
		if s, ok := statuses[nodes[i].ID]; ok {
			nodes[i].Status = s
		}
	}

	if relations == nil {
		relations = []SectionRelationResult{}
	}
	return SectionTimelineResponse{Sections: nodes, Relations: relations}, nil
}

// GetSectionLifecycle fetches the full connected component containing sectionID
// by traversing daily_report_section_relations bidirectionally.
func (r *TopicGraphRepository) GetSectionLifecycle(sectionID uint) (SectionTimelineResponse, error) {
	// Recursive CTE to find the full connected component
	var allIDs []uint
	if err := r.db.Raw(`
		WITH RECURSIVE component AS (
			SELECT ?::bigint AS id
			UNION
			SELECT CASE
				WHEN rel.from_section_id = c.id THEN rel.to_section_id
				ELSE rel.from_section_id
			END AS id
			FROM component c
			JOIN daily_report_section_relations rel
				ON rel.from_section_id = c.id OR rel.to_section_id = c.id
		)
		SELECT DISTINCT id FROM component
	`, sectionID).Scan(&allIDs).Error; err != nil {
		logging.Warnf("GetSectionLifecycle: recursive CTE failed for section %d: %v", sectionID, err)
		// Fallback: return just the section itself
		allIDs = []uint{sectionID}
	}

	if len(allIDs) == 0 {
		allIDs = []uint{sectionID}
	}

	if len(allIDs) == 0 {
		return SectionTimelineResponse{
			Sections:  []SectionTimelineNode{},
			Relations: []SectionRelationResult{},
		}, nil
	}

	// Query section details
	var nodes []SectionTimelineNode
	if err := r.db.Raw(`
		SELECT ds.id, ds.report_id, bdr.period_date, ds.cluster_label,
	       ds.article_count,
	       (SELECT COUNT(*) FROM daily_report_threads t WHERE t.section_id = ds.id) AS thread_count,
	       COALESCE(
	         (
	           SELECT a.image_url
	           FROM daily_report_threads t
	           JOIN LATERAL jsonb_array_elements_text(COALESCE(t.related_article_ids, '[]'::jsonb)) aid(article_id) ON true
	           JOIN articles a ON a.id = aid.article_id::bigint
	           WHERE t.section_id = ds.id
	             AND a.image_url IS NOT NULL
	             AND a.image_url != ''
	           ORDER BY t.id ASC, a.pub_date DESC NULLS LAST, a.id ASC
	           LIMIT 1
	         ),
	         (
	           SELECT a.image_url
	           FROM jsonb_array_elements_text(COALESCE(ds.cluster_tag_ids, '[]'::jsonb)) tid(tag_id)
	           JOIN article_topic_tags att ON att.topic_tag_id = tid.tag_id::bigint
	           JOIN articles a ON a.id = att.article_id
	           WHERE a.pub_date >= bdr.period_date
	             AND a.pub_date < bdr.period_date + INTERVAL '1 day'
	             AND a.image_url IS NOT NULL
	             AND a.image_url != ''
	           ORDER BY a.pub_date DESC NULLS LAST, a.id ASC
	           LIMIT 1
	         ),
	         ''
	       ) AS image_url
		FROM daily_report_sections ds
		JOIN board_daily_reports bdr ON bdr.id = ds.report_id
		WHERE ds.id IN ?
		ORDER BY bdr.period_date ASC, ds.id ASC
	`, allIDs).Scan(&nodes).Error; err != nil {
		return SectionTimelineResponse{}, fmt.Errorf("get section lifecycle: %w", err)
	}

	// Query relations where BOTH endpoints are in the returned sections
	var relations []SectionRelationResult
	if err := r.db.Raw(`
		SELECT from_section_id AS from_id, to_section_id AS to_id, distance
		FROM daily_report_section_relations
		WHERE from_section_id IN ? AND to_section_id IN ?
	`, allIDs, allIDs).Scan(&relations).Error; err != nil {
		logging.Warnf("GetSectionLifecycle: query relations failed: %v", err)
	}

	// Derive statuses
	sectionDateMap := make(map[uint]time.Time, len(nodes))
	var latestDate time.Time
	for _, n := range nodes {
		sectionDateMap[n.ID] = n.PeriodDate
		if n.PeriodDate.After(latestDate) {
			latestDate = n.PeriodDate
		}
	}
	statuses := DeriveSectionStatuses(allIDs, relations, sectionDateMap, latestDate)
	for i := range nodes {
		if s, ok := statuses[nodes[i].ID]; ok {
			nodes[i].Status = s
		}
	}

	if relations == nil {
		relations = []SectionRelationResult{}
	}
	return SectionTimelineResponse{Sections: nodes, Relations: relations}, nil
}

// BackfillSectionEmbeddings generates embeddings for sections that don't have one,
// then runs pgvector matching to set prev_section_id for all sections.
func (r *TopicGraphRepository) BackfillSectionEmbeddings(ctx context.Context) (embedded int, matched int, err error) {
	// Phase 1: Generate embeddings for sections without them
	batchSize := 50
	for {
		var sections []DailyReportSection
		if err := r.db.Where("embedding IS NULL").
			Where("cluster_label != '' AND cluster_label IS NOT NULL").
			Order("id ASC").
			Limit(batchSize).
			Find(&sections).Error; err != nil {
			return embedded, matched, fmt.Errorf("query sections without embedding: %w", err)
		}
		if len(sections) == 0 {
			break
		}

		var texts []string
		for _, sec := range sections {
			texts = append(texts, sec.ClusterLabel)
		}

		result, embedErr := airouter.NewRouter().Embed(ctx, airouter.EmbeddingRequest{
			Input: texts,
			Metadata: map[string]any{
				"operation": "daily_report_section_backfill",
			},
		}, airouter.CapabilityEmbedding)
		if embedErr != nil {
			return embedded, matched, fmt.Errorf("backfill embedding batch: %w", embedErr)
		}

		for i, sec := range sections {
			if i >= len(result.Embeddings) {
				break
			}
			pgVec := FloatsToPgVector(result.Embeddings[i])
			if err := r.db.Model(&DailyReportSection{}).Where("id = ?", sec.ID).
				Update("embedding", pgVec).Error; err != nil {
				logging.Warnf("backfill: failed to update embedding for section %d: %v", sec.ID, err)
				continue
			}
			embedded++
		}
	}

	// Phase 2: Rebuild relations for all boards using the unified filtering logic
	type boardGroup struct {
		BoardID uint
	}
	var boards []boardGroup
	r.db.Raw(`
		SELECT DISTINCT rpt.semantic_board_id AS board_id
		FROM daily_report_sections s
		JOIN board_daily_reports rpt ON rpt.id = s.report_id
		WHERE s.embedding IS NOT NULL
	`).Scan(&boards)

	for _, b := range boards {
		rebuilt, backfillErr := r.BackfillRelations(b.BoardID)
		if backfillErr != nil {
			logging.Warnf("BackfillSectionEmbeddings: backfill board %d failed: %v", b.BoardID, backfillErr)
			continue
		}
		matched += rebuilt
	}

	return embedded, matched, nil
}

// BackfillRelations rebuilds relations for a single board.
func (r *TopicGraphRepository) BackfillRelations(boardID uint) (rebuilt int, err error) {
	tx := r.db.Begin()
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	if err = RebuildBoardRelations(tx, boardID); err != nil {
		return 0, fmt.Errorf("rebuild board relations: %w", err)
	}

	if err = tx.Raw(`
		SELECT COUNT(*) FROM daily_report_section_relations
		WHERE from_section_id IN (
			SELECT s.id FROM daily_report_sections s
			JOIN board_daily_reports rpt ON rpt.id = s.report_id
			WHERE rpt.semantic_board_id = ?
		)
	`, boardID).Scan(&rebuilt).Error; err != nil {
		return 0, fmt.Errorf("count relations: %w", err)
	}

	if err = tx.Commit().Error; err != nil {
		return 0, fmt.Errorf("commit backfill: %w", err)
	}
	return rebuilt, nil
}

// BackfillAllRelations rebuilds relations for all boards that have sections with embeddings.
func (r *TopicGraphRepository) BackfillAllRelations() (map[uint]int, error) {
	type boardEntry struct {
		BoardID uint
	}
	var boards []boardEntry
	if err := r.db.Raw(`
		SELECT DISTINCT rpt.semantic_board_id AS board_id
		FROM daily_report_sections s
		JOIN board_daily_reports rpt ON rpt.id = s.report_id
		WHERE s.embedding IS NOT NULL
	`).Scan(&boards).Error; err != nil {
		return nil, fmt.Errorf("query boards: %w", err)
	}

	results := make(map[uint]int, len(boards))
	for _, b := range boards {
		rebuilt, bErr := r.BackfillRelations(b.BoardID)
		if bErr != nil {
			logging.Warnf("BackfillAllRelations: board %d failed: %v", b.BoardID, bErr)
			continue
		}
		results[b.BoardID] = rebuilt
		logging.Infof("BackfillAllRelations: board %d rebuilt %d relations", b.BoardID, rebuilt)
	}
	return results, nil
}

// =============================================================================
