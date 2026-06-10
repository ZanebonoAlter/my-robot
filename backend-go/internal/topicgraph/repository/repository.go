package repository

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"syntopica-backend/internal/models"
	tagging "syntopica-backend/internal/tagmanagement"
	"syntopica-backend/internal/platform/airouter"
	"syntopica-backend/internal/platform/logging"

	"gorm.io/gorm"
)

// Repo is the package-level repository singleton.
var Repo *TopicGraphRepository

// InitRepository creates and stores the global repository using the given DB connection.
// Must be called once at startup before any handlers use Repo.
func InitRepository(db *gorm.DB) {
	Repo = NewTopicGraphRepository(db)
}

// TopicGraphRepository provides data access methods for topic graph, daily reports,
// and related entities. It replaces direct database.DB access.
type TopicGraphRepository struct {
	db *gorm.DB
}

// NewTopicGraphRepository creates a new repository with the given DB connection.
func NewTopicGraphRepository(db *gorm.DB) *TopicGraphRepository {
	return &TopicGraphRepository{db: db}
}

// DB returns the underlying *gorm.DB for ad-hoc queries.
func (r *TopicGraphRepository) DB() *gorm.DB {
	return r.db
}

// =============================================================================
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
	if days > 30 {
		days = 30
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
		       (SELECT COUNT(*) FROM daily_report_threads t WHERE t.section_id = ds.id) AS thread_count
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

// GetSectionLifecycle fetches the clicked section and its directly connected neighbors (1 hop).
func (r *TopicGraphRepository) GetSectionLifecycle(sectionID uint) (SectionTimelineResponse, error) {
	// Collect only direct neighbors (1 hop)
	visited := map[uint]bool{sectionID: true}

	var connectedIDs []uint
	if err := r.db.Raw(`
		SELECT from_section_id AS id FROM daily_report_section_relations WHERE to_section_id = ?
		UNION
		SELECT to_section_id AS id FROM daily_report_section_relations WHERE from_section_id = ?
	`, sectionID, sectionID).Scan(&connectedIDs).Error; err != nil {
		logging.Warnf("GetSectionLifecycle: query failed for section %d: %v", sectionID, err)
	}

	for _, id := range connectedIDs {
		visited[id] = true
	}

	allIDs := make([]uint, 0, len(visited))
	for id := range visited {
		allIDs = append(allIDs, id)
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
		       (SELECT COUNT(*) FROM daily_report_threads t WHERE t.section_id = ds.id) AS thread_count
		FROM daily_report_sections ds
		JOIN board_daily_reports bdr ON bdr.id = ds.report_id
		WHERE ds.id IN ?
		ORDER BY bdr.period_date ASC, ds.id ASC
	`, allIDs).Scan(&nodes).Error; err != nil {
		return SectionTimelineResponse{}, fmt.Errorf("get section lifecycle: %w", err)
	}

	// Query relations involving these sections
	var relations []SectionRelationResult
	if err := r.db.Raw(`
		SELECT from_section_id AS from_id, to_section_id AS to_id, distance
		FROM daily_report_section_relations
		WHERE from_section_id IN ? OR to_section_id IN ?
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
// Topic Graph Service
// =============================================================================

// BuildTopicGraph builds a topic graph for a given time window.
func (r *TopicGraphRepository) BuildTopicGraph(kind string, anchor time.Time, categoryID, feedID *uint) (*tagging.TopicGraphResponse, error) {
	windowStart, windowEnd, periodLabel, err := tagging.ResolveWindow(kind, anchor)
	if err != nil {
		return nil, err
	}

	articleTags, err := r.fetchArticleTagsData(windowStart, windowEnd, categoryID, feedID)
	if err != nil {
		return nil, err
	}

	nodes, edges, topTopics, articleCount := buildGraphPayloadFromArticles(r.db, articleTags)
	feedCount := 0
	for _, node := range nodes {
		if node.Kind == "feed" {
			feedCount++
		}
	}

	return &tagging.TopicGraphResponse{
		Type:         kind,
		AnchorDate:   windowStart.Format("2006-01-02"),
		PeriodLabel:  periodLabel,
		Nodes:        nodes,
		Edges:        edges,
		TopicCount:   len(topTopics),
		ArticleCount: articleCount,
		FeedCount:    feedCount,
		TopTopics:    topTopics,
	}, nil
}

// BuildTopicDetail builds a detailed view of a topic.
func (r *TopicGraphRepository) BuildTopicDetail(kind string, slug string, anchor time.Time, categoryID, feedID *uint) (*tagging.TopicDetail, error) {
	windowStart, windowEnd, _, err := tagging.ResolveWindow(kind, anchor)
	if err != nil {
		return nil, err
	}

	var topic models.TopicTag
	err = r.db.Where("slug = ?", slug).First(&topic).Error
	if err != nil {
		topic = models.TopicTag{
			ID:       0,
			Slug:     slug,
			Label:    toTitle(slug),
			Category: models.TagCategoryKeyword,
		}
	}

	tagIDs := r.collectAllChildTagIDs(topic.ID)
	ids := make([]uint, 0, len(tagIDs))
	for id := range tagIDs {
		ids = append(ids, id)
	}

	articles, total, err := r.getTopicArticles(ids, windowStart, windowEnd, 1, 15, categoryID, feedID)
	if err != nil {
		return nil, fmt.Errorf("failed to get topic articles: %w", err)
	}

	relatedTags, err := r.getRelatedTags(topic.ID, 20)
	if err != nil {
		logging.Warnf("Warning: failed to get related tags: %v", err)
	}

	canonical := tagging.TopicTag{
		ID:          topic.ID,
		Label:       topic.Label,
		Slug:        topic.Slug,
		Category:    tagging.NormalizeDisplayCategory(topic.Kind, topic.Category),
		Icon:        topic.Icon,
		Description: topic.Description,
		Score:       0,
	}

	history, err := r.buildTopicHistory(kind, slug, anchor, categoryID, feedID)
	if err != nil {
		return nil, err
	}

	related := buildRelatedTopicsFromTags(relatedTags, 8)

	return &tagging.TopicDetail{
		Topic:         canonical,
		Articles:      articles,
		TotalArticles: total,
		RelatedTags:   relatedTags,
		History:       history,
		RelatedTopics: related,
		SearchLinks: map[string]string{
			"youtube_videos": "https://www.youtube.com/results?search_query=" + url.QueryEscape(canonical.Label),
			"youtube_live":   "https://www.youtube.com/results?search_query=" + url.QueryEscape(canonical.Label+" live"),
		},
		AppLinks: map[string]string{
			"digest_view": "/digest/" + kind,
			"topic_graph": "/topics",
		},
	}, nil
}

// FetchTopicArticles retrieves paginated articles for a topic.
func (r *TopicGraphRepository) FetchTopicArticles(slug string, kind string, anchor time.Time, page, pageSize int) ([]tagging.TopicArticleCard, int64, error) {
	windowStart, windowEnd, _, err := tagging.ResolveWindow(kind, anchor)
	if err != nil {
		return nil, 0, err
	}

	var topic models.TopicTag
	err = r.db.Where("slug = ?", slug).First(&topic).Error
	if err != nil {
		return nil, 0, fmt.Errorf("topic not found: %w", err)
	}

	tagIDs := r.collectAllChildTagIDs(topic.ID)
	ids := make([]uint, 0, len(tagIDs))
	for id := range tagIDs {
		ids = append(ids, id)
	}

	return r.getTopicArticles(ids, windowStart, windowEnd, page, pageSize, nil, nil)
}

// BuildTopicsByCategory builds topic lists grouped by category from article tags.
func (r *TopicGraphRepository) BuildTopicsByCategory(kind string, anchor time.Time, categoryID, feedID *uint) (*tagging.TopicsByCategoryResult, error) {
	windowStart, windowEnd, _, err := tagging.ResolveWindow(kind, anchor)
	if err != nil {
		return nil, err
	}

	var articleTags []models.ArticleTopicTag
	query := r.db.
		Joins("JOIN articles ON articles.id = article_topic_tags.article_id").
		Joins("JOIN topic_tags ON topic_tags.id = article_topic_tags.topic_tag_id").
		Where("articles.created_at >= ? AND articles.created_at < ?", windowStart, windowEnd).
		Where("article_topic_tags.source = ?", "llm")

	if feedID != nil {
		query = query.Where("articles.feed_id = ?", *feedID)
	} else if categoryID != nil {
		query = query.Joins("JOIN feeds ON feeds.id = articles.feed_id").
			Where("feeds.category_id = ?", *categoryID)
	}

	err = query.Preload("TopicTag").
		Find(&articleTags).Error
	if err != nil {
		return nil, err
	}

	// Group tags by category and aggregate scores
	eventScores := make(map[string]*tagging.TopicTag)
	personScores := make(map[string]*tagging.TopicTag)
	keywordScores := make(map[string]*tagging.TopicTag)

	for _, at := range articleTags {
		if at.TopicTag == nil {
			continue
		}

		tag := tagging.TopicTag{
			ID:           at.TopicTag.ID,
			Label:        at.TopicTag.Label,
			Slug:         at.TopicTag.Slug,
			Category:     tagging.NormalizeDisplayCategory(at.TopicTag.Kind, at.TopicTag.Category),
			Icon:         at.TopicTag.Icon,
			Description:  at.TopicTag.Description,
			Score:        at.Score,
			QualityScore: at.TopicTag.QualityScore,
			IsLowQuality: at.TopicTag.Source != "abstract" && at.TopicTag.QualityScore < 0.3,
		}

		switch tag.Category {
		case models.TagCategoryEvent:
			if existing, ok := eventScores[tag.Slug]; ok {
				existing.Score += tag.Score
			} else {
				eventScores[tag.Slug] = &tag
			}
		case models.TagCategoryPerson:
			if existing, ok := personScores[tag.Slug]; ok {
				existing.Score += tag.Score
			} else {
				personScores[tag.Slug] = &tag
			}
		default: // keyword
			if existing, ok := keywordScores[tag.Slug]; ok {
				existing.Score += tag.Score
			} else {
				keywordScores[tag.Slug] = &tag
			}
		}
	}

	includeAbstractParents(r.db, eventScores, personScores, keywordScores)
	enrichAbstractTags(r.db, eventScores, personScores, keywordScores)
	finalizeTopicTagQuality(eventScores, personScores, keywordScores)

	result := &tagging.TopicsByCategoryResult{
		Events:   sortTagsByScoreMap(eventScores),
		People:   sortTagsByScoreMap(personScores),
		Keywords: sortTagsByScoreMap(keywordScores),
	}

	return result, nil
}

// GetPendingArticlesByTag retrieves articles that have the given tag but are not in any digest.
func (r *TopicGraphRepository) GetPendingArticlesByTag(tagSlug string, kind string, anchor time.Time) (*tagging.PendingArticlesResponse, error) {
	windowStart, windowEnd, _, err := tagging.ResolveWindow(kind, anchor)
	if err != nil {
		return nil, err
	}

	var topicTag models.TopicTag
	err = r.db.Where("slug = ?", tagSlug).First(&topicTag).Error
	if err != nil {
		return nil, fmt.Errorf("topic tag not found: %w", err)
	}

	tagIDSet := r.collectAllChildTagIDs(topicTag.ID)
	tagIDs := make([]uint, 0, len(tagIDSet))
	for id := range tagIDSet {
		tagIDs = append(tagIDs, id)
	}

	var taggedArticles []models.Article
	err = r.db.
		Joins("JOIN article_topic_tags ON articles.id = article_topic_tags.article_id").
		Where("article_topic_tags.topic_tag_id IN ?", tagIDs).
		Where("articles.created_at >= ? AND articles.created_at < ?", windowStart, windowEnd).
		Preload("Feed").
		Omit("tag_count", "relevance_score").
		Distinct().
		Find(&taggedArticles).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get tagged articles: %w", err)
	}

	pendingArticles := make([]tagging.PendingArticle, 0, len(taggedArticles))
	for _, article := range taggedArticles {
		pa := tagging.PendingArticle{
			ID:    article.ID,
			Title: article.Title,
			Link:  article.Link,
		}

		if article.PubDate != nil {
			pa.PubDate = article.PubDate.In(models.ShanghaiTZ).Format(time.RFC3339)
		}

		if article.Feed.ID != 0 {
			pa.FeedName = article.Feed.Title
			pa.FeedIcon = article.Feed.Icon
			pa.FeedColor = article.Feed.Color
		} else {
			pa.FeedName = "未知订阅源"
		}

		pendingArticles = append(pendingArticles, pa)
	}

	return &tagging.PendingArticlesResponse{
		Articles: pendingArticles,
		Total:    len(pendingArticles),
	}, nil
}

// GetDigestsByArticleTag fetches articles matching a tag for digest display.
func (r *TopicGraphRepository) GetDigestsByArticleTag(tagSlug string, windowKind string, anchor time.Time, limit int, tagKind string) ([]HotspotDigestCard, error) {
	windowStart, windowEnd, _, err := tagging.ResolveWindow(windowKind, anchor)
	if err != nil {
		return nil, err
	}

	var topicTag models.TopicTag
	query := r.db.Where("slug = ?", tagSlug)
	if tagKind != "" {
		query = query.Where("kind = ?", tagKind)
	}
	err = query.First(&topicTag).Error
	if err != nil {
		return nil, fmt.Errorf("topic tag not found: %w", err)
	}

	tagIDSet := r.collectAllChildTagIDs(topicTag.ID)
	tagIDs := make([]uint, 0, len(tagIDSet))
	for id := range tagIDSet {
		tagIDs = append(tagIDs, id)
	}

	var articles []models.Article
	err = r.db.
		Joins("JOIN article_topic_tags ON articles.id = article_topic_tags.article_id").
		Where("article_topic_tags.topic_tag_id IN ?", tagIDs).
		Where("articles.created_at >= ? AND articles.created_at < ?", windowStart, windowEnd).
		Preload("Feed").
		Omit("tag_count", "relevance_score").
		Distinct().
		Order("articles.created_at DESC").
		Find(&articles).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get articles: %w", err)
	}

	if limit > 0 && len(articles) > limit {
		articles = articles[:limit]
	}

	result := make([]HotspotDigestCard, 0, len(articles))
	for _, article := range articles {
		card := HotspotDigestCard{
			ID:    article.ID,
			Title: article.Title,
			Link:  article.Link,
		}

		if article.PubDate != nil {
			card.PublishedAt = article.PubDate.In(models.ShanghaiTZ).Format(time.RFC3339)
		}

		if article.Feed.ID != 0 {
			card.FeedName = article.Feed.Title
			card.FeedIcon = article.Feed.Icon
			card.FeedColor = article.Feed.Color
		} else {
			card.FeedName = "未知订阅源"
		}

		result = append(result, card)
	}

	return result, nil
}

// collectAllChildTagIDs recursively collects all child tag IDs for a parent tag.
func (r *TopicGraphRepository) collectAllChildTagIDs(parentTagID uint) map[uint]bool {
	result := map[uint]bool{parentTagID: true}
	queue := []uint{parentTagID}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		var relations []models.TopicTagRelation
		r.db.Where("parent_id = ? AND relation_type = ?", current, "abstract").
			Find(&relations)

		for _, rel := range relations {
			if !result[rel.ChildID] {
				result[rel.ChildID] = true
				queue = append(queue, rel.ChildID)
			}
		}
	}

	return result
}

// =============================================================================
// Internal helpers (used by repository methods)
// =============================================================================

// fetchArticleTagsData retrieves article-topic associations with feed info for graph building.
func (r *TopicGraphRepository) fetchArticleTagsData(start, end time.Time, categoryID, feedID *uint) ([]ArticleTagData, error) {
	var articleTags []models.ArticleTopicTag
	query := r.db.
		Joins("JOIN articles ON articles.id = article_topic_tags.article_id").
		Joins("JOIN topic_tags ON topic_tags.id = article_topic_tags.topic_tag_id").
		Where("articles.created_at >= ? AND articles.created_at < ?", start, end).
		Where("article_topic_tags.source = ?", "llm")

	if feedID != nil {
		query = query.Where("articles.feed_id = ?", *feedID)
	} else if categoryID != nil {
		query = query.Joins("JOIN feeds ON feeds.id = articles.feed_id").
			Where("feeds.category_id = ?", *categoryID)
	}

	err := query.
		Preload("TopicTag").
		Preload("Article.Feed").
		Find(&articleTags).Error
	if err != nil {
		return nil, err
	}

	data := make([]ArticleTagData, 0, len(articleTags))
	for _, at := range articleTags {
		if at.TopicTag == nil || at.Article == nil {
			continue
		}
		var feedTitle, feedColor string
		if at.Article.Feed.ID != 0 {
			feedTitle = at.Article.Feed.Title
			feedColor = at.Article.Feed.Color
		}
		if strings.TrimSpace(feedTitle) == "" {
			feedTitle = "未知订阅源"
		}
		if strings.TrimSpace(feedColor) == "" {
			feedColor = "#3b6b87"
		}
		data = append(data, ArticleTagData{
			ArticleID: at.ArticleID,
			FeedID:    at.Article.FeedID,
			FeedTitle: feedTitle,
			FeedColor: feedColor,
			TopicTag:  at.TopicTag,
			Score:     at.Score,
		})
	}

	return data, nil
}

// getTopicArticles retrieves articles associated with one or more topic tags.
func (r *TopicGraphRepository) getTopicArticles(tagIDs []uint, startDate, endDate time.Time, page, pageSize int, categoryID, feedID *uint) ([]tagging.TopicArticleCard, int64, error) {
	if len(tagIDs) == 0 {
		return []tagging.TopicArticleCard{}, 0, nil
	}

	var articles []models.Article
	var total int64

	offset := (page - 1) * pageSize

	base := r.db.Model(&models.Article{}).
		Joins("JOIN article_topic_tags ON articles.id = article_topic_tags.article_id").
		Where("article_topic_tags.topic_tag_id IN ?", tagIDs).
		Where("articles.created_at >= ? AND articles.created_at < ?", startDate, endDate)

	countQuery := base
	dataQuery := r.db.Model(&models.Article{}).
		Joins("JOIN article_topic_tags ON articles.id = article_topic_tags.article_id").
		Where("article_topic_tags.topic_tag_id IN ?", tagIDs).
		Where("articles.created_at >= ? AND articles.created_at < ?", startDate, endDate)

	if feedID != nil {
		countQuery = countQuery.Where("articles.feed_id = ?", *feedID)
		dataQuery = dataQuery.Where("articles.feed_id = ?", *feedID)
	} else if categoryID != nil {
		countQuery = countQuery.Joins("JOIN feeds ON feeds.id = articles.feed_id").Where("feeds.category_id = ?", *categoryID)
		dataQuery = dataQuery.Joins("JOIN feeds ON feeds.id = articles.feed_id").Where("feeds.category_id = ?", *categoryID)
	}

	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count articles: %w", err)
	}

	err := dataQuery.
		Preload("Feed").
		Order("articles.created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Omit("tag_count", "relevance_score").
		Find(&articles).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query articles: %w", err)
	}

	// Batch fetch tags for all returned articles
	articleIDs := make([]uint, 0, len(articles))
	for _, a := range articles {
		articleIDs = append(articleIDs, a.ID)
	}

	tagMap := make(map[uint][]tagging.TopicTagSummary)
	if len(articleIDs) > 0 {
		type tagRow struct {
			ArticleID uint
			Slug      string
			Label     string
			Category  string
		}
		var rows []tagRow
		dbErr := r.db.Raw(`
			SELECT att.article_id, tt.slug, tt.label, tt.category
			FROM article_topic_tags att
			JOIN topic_tags tt ON att.topic_tag_id = tt.id
			WHERE att.article_id IN ?
		`, articleIDs).Scan(&rows).Error
		if dbErr != nil {
			return nil, 0, fmt.Errorf("failed to fetch article tags: %w", dbErr)
		}
		for _, row := range rows {
			tagMap[row.ArticleID] = append(tagMap[row.ArticleID], tagging.TopicTagSummary{
				Slug:     row.Slug,
				Label:    row.Label,
				Category: row.Category,
			})
		}
	}

	// Convert to cards
	cards := make([]tagging.TopicArticleCard, 0, len(articles))
	for _, article := range articles {
		card := tagging.TopicArticleCard{
			ID:       article.ID,
			Title:    article.Title,
			Link:     article.Link,
			FeedID:   article.FeedID,
			ImageURL: article.ImageURL,
			Summary:  article.AIContentSummary,
			Content:  article.Content,
		}

		if article.PubDate != nil {
			card.PubDate = article.PubDate
		}

		if article.Feed.ID != 0 {
			card.FeedName = article.Feed.Title
		}

		if t, ok := tagMap[article.ID]; ok {
			card.Tags = t
		} else {
			card.Tags = []tagging.TopicTagSummary{}
		}

		cards = append(cards, card)
	}

	return cards, total, nil
}

// getRelatedTags retrieves tags that co-occur with the given topic.
func (r *TopicGraphRepository) getRelatedTags(topicID uint, limit int) ([]tagging.RelatedTag, error) {
	var relatedTags []tagging.RelatedTag

	err := r.db.Raw(`
		SELECT 
			t.id,
			t.label,
			t.slug,
			t.category,
			t.kind,
			COUNT(*) as cooccurrence
		FROM topic_tags t
		JOIN article_topic_tags at1 ON t.id = at1.topic_tag_id
		JOIN article_topic_tags at2 ON at1.article_id = at2.article_id
		WHERE at2.topic_tag_id = ?
		  AND t.id != ?
		GROUP BY t.id, t.label, t.slug, t.category
		ORDER BY cooccurrence DESC
		LIMIT ?
	`, topicID, topicID, limit).Scan(&relatedTags).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get related tags: %w", err)
	}

	for i := range relatedTags {
		relatedTags[i].Category = tagging.NormalizeDisplayCategory(relatedTags[i].Kind, relatedTags[i].Category)
	}

	return relatedTags, nil
}

// buildTopicHistory builds the topic history over time windows.
func (r *TopicGraphRepository) buildTopicHistory(kind string, slug string, anchor time.Time, categoryID, feedID *uint) ([]tagging.TopicHistoryPoint, error) {
	history := make([]tagging.TopicHistoryPoint, 0, 7)
	for i := 6; i >= 0; i-- {
		var pointAnchor time.Time
		if kind == "weekly" {
			pointAnchor = anchor.AddDate(0, 0, -7*i)
		} else {
			pointAnchor = anchor.AddDate(0, 0, -i)
		}

		start, end, label, err := tagging.ResolveWindow(kind, pointAnchor)
		if err != nil {
			return nil, err
		}

		articleTags, err := r.fetchArticleTagsData(start, end, categoryID, feedID)
		if err != nil {
			return nil, err
		}

		articleSet := make(map[uint]bool)
		for _, at := range articleTags {
			if at.TopicTag != nil && at.TopicTag.Slug == slug {
				articleSet[at.ArticleID] = true
			}
		}
		count := len(articleSet)

		history = append(history, tagging.TopicHistoryPoint{
			AnchorDate: start.Format("2006-01-02"),
			Count:      count,
			Label:      label,
		})
	}

	return history, nil
}
