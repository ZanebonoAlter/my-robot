package daily_report

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"syntopica-backend/internal/domain/models"
	"syntopica-backend/internal/platform/airouter"
	"syntopica-backend/internal/platform/database"
	"syntopica-backend/internal/platform/logging"

	"gorm.io/gorm"
)

// SaveReport saves a daily report and its sections, replacing any existing
// report for the same board and date.
func SaveReport(report *BoardDailyReport, sections []DailyReportSection, threadBatches [][]DailyReportThread) error {
	report.PeriodDate = normalizeReportDate(report.PeriodDate)
	return database.DB.Transaction(func(tx *gorm.DB) error {
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
			// Delete old relations involving old section IDs
			var oldSectionIDs []uint
			tx.Model(&DailyReportSection{}).Where("report_id = ?", existing.ID).Pluck("id", &oldSectionIDs)
			if len(oldSectionIDs) > 0 {
				tx.Where("from_section_id IN ? OR to_section_id IN ?", oldSectionIDs, oldSectionIDs).Delete(&SectionRelation{})
			}
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

		// Write relations for new sections
		if err := MatchAndSaveRelations(tx, report.SemanticBoardID, report.PeriodDate, sections); err != nil {
			logging.Warnf("SaveReport: relation matching failed: %v", err)
		}

		// Save threads for each section (sections now have IDs after insertion)
		for secIdx, sec := range sections {
			if secIdx < len(threadBatches) && len(threadBatches[secIdx]) > 0 {
				if err := SaveThreads(tx, report.ID, sec.ID, threadBatches[secIdx]); err != nil {
					return fmt.Errorf("save threads for section %d: %w", secIdx, err)
				}
			}
		}

		logging.Infof("daily-report: saved report %d for board %d on %s (%d sections)",
			report.ID, report.SemanticBoardID, report.PeriodDate.Format("2006-01-02"), len(sections))
		return nil
	})
}

// GetReport retrieves a single daily report with its sections.
func GetReport(boardID uint, date time.Time) (*BoardDailyReport, error) {
	reportDate := normalizeReportDate(date)

	var report BoardDailyReport
	err := database.DB.Where("semantic_board_id = ? AND period_date = ?",
		boardID, reportDate.Format("2006-01-02")).
		Preload("Sections", func(db *gorm.DB) *gorm.DB {
			return db.Order("cluster_index ASC")
		}).
		First(&report).Error
	if err != nil {
		return nil, fmt.Errorf("report not found for board %d on %s: %w", boardID, date.Format("2006-01-02"), err)
	}
	return &report, nil
}

// GetReportByID retrieves a single daily report by its primary key.
func GetReportByID(id uint) (*BoardDailyReport, error) {
	var report BoardDailyReport
	err := database.DB.Where("id = ?", id).
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
func ListReports(boardID uint, days int) ([]ReportListItem, error) {
	if days <= 0 {
		days = 7
	}
	if days > 30 {
		days = 30
	}

	now := normalizeReportDate(time.Now())
	rangeStart := now.AddDate(0, 0, -(days - 1))
	rangeEnd := now.AddDate(0, 0, 1)

	var reports []BoardDailyReport
	err := database.DB.Where("semantic_board_id = ? AND period_date >= ? AND period_date < ?",
		boardID, rangeStart.Format("2006-01-02"), rangeEnd.Format("2006-01-02")).
		Order("period_date DESC").
		Find(&reports).Error
	if err != nil {
		return nil, fmt.Errorf("list reports for board %d: %w", boardID, err)
	}

	items := make([]ReportListItem, len(reports))
	for i, r := range reports {
		items[i] = ReportListItem{
			ID:              r.ID,
			SemanticBoardID: r.SemanticBoardID,
			PeriodDate:      r.PeriodDate.Format("2006-01-02"),
			Title:           r.Title,
			Summary:         r.Summary,
			ArticleCount:    r.ArticleCount,
			EventTagCount:   r.EventTagCount,
			ClusterCount:    r.ClusterCount,
			Status:          r.Status,
			CreatedAt:       r.CreatedAt,
		}
	}
	return items, nil
}

// ListReportsForAllBoards returns reports for all boards within a date range.
func ListReportsForAllBoards(days int) ([]BoardDailyReport, error) {
	if days <= 0 {
		days = 7
	}
	if days > 30 {
		days = 30
	}

	now := normalizeReportDate(time.Now())
	rangeStart := now.AddDate(0, 0, -(days - 1))
	rangeEnd := now.AddDate(0, 0, 1)

	var reports []BoardDailyReport
	err := database.DB.Where("period_date >= ? AND period_date < ?",
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

// SetReportStatus updates the status field of a report.
func SetReportStatus(id uint, status string) error {
	return database.DB.Model(&BoardDailyReport{}).Where("id = ?", id).
		Update("status", status).Error
}

func normalizeReportDate(date time.Time) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), 12, 0, 0, 0, time.UTC)
}

// collectBoardIDsForDate returns all board IDs that have active event tags on a date.
func CollectBoardIDsForDate(date time.Time) ([]uint, error) {
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	type row struct {
		SemanticBoardID uint `json:"semantic_board_id"`
	}
	var rows []row
	err := database.DB.Model(&models.TopicTag{}).
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
	for i, r := range rows {
		ids[i] = r.SemanticBoardID
	}
	return ids, nil
}

// SaveThreads persists a batch of DailyReportThread rows.
func SaveThreads(tx *gorm.DB, reportID, sectionID uint, threads []DailyReportThread) error {
	for i := range threads {
		threads[i].ReportID = reportID
		threads[i].SectionID = sectionID
	}
	return tx.Create(&threads).Error
}

// GetThreadsBySection returns all threads for a section, ordered by id.
func GetThreadsBySection(sectionID uint) ([]DailyReportThread, error) {
	var threads []DailyReportThread
	err := database.DB.Where("section_id = ?", sectionID).Order("id ASC").Find(&threads).Error
	return threads, err
}

// GetThreadsByReport returns all threads for a report.
func GetThreadsByReport(reportID uint) ([]DailyReportThread, error) {
	var threads []DailyReportThread
	err := database.DB.Where("report_id = ?", reportID).Order("section_id ASC, id ASC").Find(&threads).Error
	return threads, err
}

// GetThreadByID returns a single thread by its primary key.
func GetThreadByID(id uint) (*DailyReportThread, error) {
	var thread DailyReportThread
	err := database.DB.First(&thread, id).Error
	if err != nil {
		return nil, fmt.Errorf("thread %d not found: %w", id, err)
	}
	return &thread, nil
}

// DeleteThreadsByReport deletes all threads for a report.
func DeleteThreadsByReport(reportID uint) error {
	return database.DB.Where("report_id = ?", reportID).Delete(&DailyReportThread{}).Error
}

// SectionTimelineNode represents a section in a timeline view.
type SectionTimelineNode struct {
	ID           uint      `json:"id"`
	ReportID     uint      `json:"report_id"`
	PeriodDate   time.Time `json:"period_date"`
	ClusterLabel string    `json:"cluster_label"`
	Status       string    `json:"status"`
	ArticleCount int       `json:"article_count"`
	ThreadCount  int       `json:"thread_count"`
}

// GetBoardSectionTimeline fetches all sections and their relations for a board within a date range.
func GetBoardSectionTimeline(boardID uint, days int) (SectionTimelineResponse, error) {
	if days <= 0 {
		days = 30
	}
	if days > 90 {
		days = 90
	}
	var nodes []SectionTimelineNode
	err := database.DB.Raw(`
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
	if err := database.DB.Raw(`
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

// MatchAndSaveRelations finds and persists section relations based on embedding similarity.
// For each section with a non-empty embedding, it queries ALL matching sections in the
// same board with distance < 0.35, excluding same-day sections, then applies two-layer
// filtering: adjacent-day matches write directly, skip-day matches require no intermediate
// continuation and distance < 0.25.
func MatchAndSaveRelations(tx *gorm.DB, boardID uint, reportDate time.Time, sections []DailyReportSection) error {
	// Pre-load existing relations into in-memory adjacency map (from_section_id → []to_section_id)
	adjacency := make(map[uint][]uint)
	var existingRelations []SectionRelation
	if err := tx.Raw(`
		SELECT r.from_section_id, r.to_section_id
		FROM daily_report_section_relations r
		JOIN daily_report_sections s1 ON s1.id = r.from_section_id
		JOIN board_daily_reports b1 ON b1.id = s1.report_id
		WHERE b1.semantic_board_id = ?
	`, boardID).Scan(&existingRelations).Error; err == nil {
		for _, r := range existingRelations {
			adjacency[r.FromSectionID] = append(adjacency[r.FromSectionID], r.ToSectionID)
		}
	}

	// Pre-load completed report dates for this board
	var completedDates []time.Time
	if err := tx.Raw(`
		SELECT DISTINCT period_date::date
		FROM board_daily_reports
		WHERE semantic_board_id = ? AND status = 'completed'
		ORDER BY period_date::date
	`, boardID).Scan(&completedDates).Error; err != nil {
		logging.Warnf("MatchAndSaveRelations: query completed dates failed: %v", err)
	}
	dateSet := make(map[string]bool, len(completedDates))
	for _, d := range completedDates {
		dateSet[d.Format("2006-01-02")] = true
	}

	// Pre-load section → date mapping for intermediate-day checks
	sectionDateMap := make(map[uint]time.Time)
	var sectionDateRows []struct {
		SectionID  uint
		PeriodDate time.Time
	}
	tx.Raw(`
		SELECT s.id AS section_id, r.period_date::date AS period_date
		FROM daily_report_sections s
		JOIN board_daily_reports r ON r.id = s.report_id
		WHERE r.semantic_board_id = ?
	`, boardID).Scan(&sectionDateRows)
	for _, row := range sectionDateRows {
		sectionDateMap[row.SectionID] = row.PeriodDate
	}

	for _, sec := range sections {
		if strings.TrimSpace(sec.Embedding) == "" {
			continue
		}
		var matches []struct {
			MatchID   uint
			MatchDate time.Time
			Distance  float64
		}
		err := tx.Raw(`
			SELECT s.id AS match_id, r.period_date::date AS match_date, s.embedding <=> ?::vector AS distance
			FROM daily_report_sections s
			JOIN board_daily_reports r ON r.id = s.report_id
			WHERE r.semantic_board_id = ?
			  AND r.status = 'completed'
			  AND r.period_date::date < ?
			  AND s.embedding IS NOT NULL
			  AND s.embedding <=> ?::vector < 0.35
			ORDER BY s.embedding <=> ?::vector
		`, sec.Embedding, boardID, reportDate.Format("2006-01-02"), sec.Embedding, sec.Embedding).Scan(&matches).Error
		if err != nil {
			logging.Warnf("MatchAndSaveRelations: query failed for section %d: %v", sec.ID, err)
			continue
		}

		// Collect candidates that pass time-dimension filtering
		var candidates []matchCandidate
		for _, m := range matches {
			if !shouldWriteRelation(m.MatchID, m.MatchDate, sec.ID, reportDate, m.Distance, adjacency, sectionDateMap, dateSet) {
				continue
			}
			candidates = append(candidates, matchCandidate{
				FromID:   m.MatchID,
				FromDate: m.MatchDate,
				Distance: m.Distance,
			})
		}

		// Apply competitive filtering
		survivors := competitiveFilter(candidates)

		// Write surviving relations
		for _, c := range survivors {
			if err := tx.Exec(`
				INSERT INTO daily_report_section_relations (from_section_id, to_section_id, distance)
				VALUES (?, ?, ?)
				ON CONFLICT (from_section_id, to_section_id) DO UPDATE SET distance = EXCLUDED.distance
			`, c.FromID, sec.ID, c.Distance).Error; err != nil {
				logging.Warnf("MatchAndSaveRelations: save relation failed: %v", err)
			} else {
				adjacency[c.FromID] = append(adjacency[c.FromID], sec.ID)
				sectionDateMap[sec.ID] = reportDate
			}
		}
	}
	return nil
}

// shouldWriteRelation determines whether a relation should be written based on two-layer filtering:
//   - Adjacent-day matches (no intermediate completed report days): distance < 0.35 → write directly
//   - Skip-day matches (intermediate days exist): no intermediate continuation + distance < 0.25 → write
func shouldWriteRelation(
	fromID uint, fromDate time.Time,
	toID uint, toDate time.Time,
	distance float64,
	adjacency map[uint][]uint,
	sectionDateMap map[uint]time.Time,
	completedDateSet map[string]bool,
) bool {
	fromStr := fromDate.Format("2006-01-02")
	toStr := toDate.Format("2006-01-02")

	// Check if any completed report days exist between from and to
	hasIntermediate := false
	for dStr := range completedDateSet {
		if dStr > fromStr && dStr < toStr {
			hasIntermediate = true
			break
		}
	}

	if !hasIntermediate {
		// Adjacent-day match: distance < 0.35 → write directly
		return distance < 0.35
	}

	// Skip-day match: check if from_section has continuation in intermediate days
	if hasContinuationInIntermediateDays(fromID, fromDate, toDate, adjacency, sectionDateMap) {
		return false
	}
	return distance < 0.25
}

// hasContinuationInIntermediateDays checks if fromSection has any outgoing relation
// pointing to a section on a day between fromDate and toDate.
func hasContinuationInIntermediateDays(
	fromSectionID uint, fromDate time.Time, toDate time.Time,
	adjacency map[uint][]uint,
	sectionDateMap map[uint]time.Time,
) bool {
	toTargets, ok := adjacency[fromSectionID]
	if !ok {
		return false
	}
	fromStr := fromDate.Format("2006-01-02")
	toStr := toDate.Format("2006-01-02")
	for _, tid := range toTargets {
		if targetDate, exists := sectionDateMap[tid]; exists {
			targetStr := targetDate.Format("2006-01-02")
			if targetStr > fromStr && targetStr < toStr {
				return true
			}
		}
	}
	return false
}

type matchCandidate struct {
	FromID   uint
	FromDate time.Time
	Distance  float64
}

func competitiveFilter(candidates []matchCandidate) []matchCandidate {
	if len(candidates) <= 1 {
		return candidates
	}

	slices.SortFunc(candidates, func(a, b matchCandidate) int {
		if a.Distance < b.Distance {
			return -1
		}
		if a.Distance > b.Distance {
			return 1
		}
		return 0
	})

	best := candidates[0].Distance
	gap := candidates[1].Distance - best

	if gap >= 0.03 {
		return candidates[:1]
	}

	threshold := best + 0.03
	i := 1
	for i < len(candidates) && candidates[i].Distance <= threshold {
		i++
	}
	return candidates[:i]
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

// GetSectionLifecycle fetches the clicked section and its directly connected neighbors (1 hop).
func GetSectionLifecycle(sectionID uint) (SectionTimelineResponse, error) {
	// Collect only direct neighbors (1 hop)
	visited := map[uint]bool{sectionID: true}

	var connectedIDs []uint
	if err := database.DB.Raw(`
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
	if err := database.DB.Raw(`
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
	if err := database.DB.Raw(`
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
// It overwrites all prev_section_id values (including those from the old tag Jaccard matching).
func BackfillSectionEmbeddings(ctx context.Context) (embedded int, matched int, err error) {
	// Phase 1: Generate embeddings for sections without them
	batchSize := 50
	for {
		var sections []DailyReportSection
		if err := database.DB.Where("embedding IS NULL").
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
			pgVec := floatsToPgVector(result.Embeddings[i])
			if err := database.DB.Model(&DailyReportSection{}).Where("id = ?", sec.ID).
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
	database.DB.Raw(`
		SELECT DISTINCT r.semantic_board_id AS board_id
		FROM daily_report_sections s
		JOIN board_daily_reports r ON r.id = s.report_id
		WHERE s.embedding IS NOT NULL
	`).Scan(&boards)

	for _, b := range boards {
		rebuilt, backfillErr := BackfillRelations(b.BoardID)
		if backfillErr != nil {
			logging.Warnf("BackfillSectionEmbeddings: backfill board %d failed: %v", b.BoardID, backfillErr)
			continue
		}
		matched += rebuilt
	}

	return embedded, matched, nil
}

// BackfillRelations deletes all relations for a board and rebuilds them using
// the two-layer filtering logic, processing sections in chronological order.
func BackfillRelations(boardID uint) (rebuilt int, err error) {
	tx := database.DB.Begin()
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// 1. Delete all existing relations for this board
	if err = tx.Exec(`
		DELETE FROM daily_report_section_relations
		WHERE from_section_id IN (
			SELECT s.id FROM daily_report_sections s
			JOIN board_daily_reports r ON r.id = s.report_id
			WHERE r.semantic_board_id = ?
		) OR to_section_id IN (
			SELECT s.id FROM daily_report_sections s
			JOIN board_daily_reports r ON r.id = s.report_id
			WHERE r.semantic_board_id = ?
		)
	`, boardID, boardID).Error; err != nil {
		return 0, fmt.Errorf("delete old relations: %w", err)
	}

	// 2. Load all sections with embeddings, ordered by date ascending
	var sections []struct {
		ID         uint
		Embedding  string
		ReportID   uint
		PeriodDate time.Time
	}
	if err = tx.Raw(`
		SELECT s.id, s.embedding, s.report_id, r.period_date::date AS period_date
		FROM daily_report_sections s
		JOIN board_daily_reports r ON r.id = s.report_id
		WHERE r.semantic_board_id = ?
		  AND s.embedding IS NOT NULL
		  AND s.cluster_label != '' AND s.cluster_label IS NOT NULL
		ORDER BY r.period_date ASC, s.id ASC
	`, boardID).Scan(&sections).Error; err != nil {
		return 0, fmt.Errorf("query sections: %w", err)
	}

	// 3. Load completed report dates
	var completedDates []time.Time
	tx.Raw(`
		SELECT DISTINCT period_date::date
		FROM board_daily_reports
		WHERE semantic_board_id = ? AND status = 'completed'
		ORDER BY period_date::date
	`, boardID).Scan(&completedDates)
	dateSet := make(map[string]bool, len(completedDates))
	for _, d := range completedDates {
		dateSet[d.Format("2006-01-02")] = true
	}

	// 4. Process each section in chronological order, building relations incrementally
	adjacency := make(map[uint][]uint)
	sectionDateMap := make(map[uint]time.Time, len(sections))
	for _, sec := range sections {
		sectionDateMap[sec.ID] = sec.PeriodDate
	}

	for _, sec := range sections {
		var matches []struct {
			MatchID   uint
			MatchDate time.Time
			Distance  float64
		}
		qErr := tx.Raw(`
			SELECT s.id AS match_id, r.period_date::date AS match_date, s.embedding <=> ?::vector AS distance
			FROM daily_report_sections s
			JOIN board_daily_reports r ON r.id = s.report_id
			WHERE r.semantic_board_id = ?
			  AND r.status = 'completed'
			  AND r.period_date::date < ?
			  AND s.embedding IS NOT NULL
			  AND s.embedding <=> ?::vector < 0.35
			ORDER BY s.embedding <=> ?::vector
		`, sec.Embedding, boardID, sec.PeriodDate.Format("2006-01-02"), sec.Embedding, sec.Embedding).Scan(&matches).Error
		if qErr != nil {
			logging.Warnf("BackfillRelations: query failed for section %d: %v", sec.ID, qErr)
			continue
		}

		for _, m := range matches {
			if !shouldWriteRelation(m.MatchID, m.MatchDate, sec.ID, sec.PeriodDate, m.Distance, adjacency, sectionDateMap, dateSet) {
				continue
			}
			if wErr := tx.Exec(`
				INSERT INTO daily_report_section_relations (from_section_id, to_section_id, distance)
				VALUES (?, ?, ?)
				ON CONFLICT (from_section_id, to_section_id) DO UPDATE SET distance = EXCLUDED.distance
			`, m.MatchID, sec.ID, m.Distance).Error; wErr != nil {
				logging.Warnf("BackfillRelations: write relation failed: %v", wErr)
			} else {
				adjacency[m.MatchID] = append(adjacency[m.MatchID], sec.ID)
				rebuilt++
			}
		}
	}

	if err = tx.Commit().Error; err != nil {
		return 0, fmt.Errorf("commit backfill: %w", err)
	}
	return rebuilt, nil
}

// BackfillAllRelations rebuilds relations for all boards that have sections with embeddings.
func BackfillAllRelations() (map[uint]int, error) {
	type boardEntry struct {
		BoardID uint
	}
	var boards []boardEntry
	if err := database.DB.Raw(`
		SELECT DISTINCT r.semantic_board_id AS board_id
		FROM daily_report_sections s
		JOIN board_daily_reports r ON r.id = s.report_id
		WHERE s.embedding IS NOT NULL
	`).Scan(&boards).Error; err != nil {
		return nil, fmt.Errorf("query boards: %w", err)
	}

	results := make(map[uint]int, len(boards))
	for _, b := range boards {
		rebuilt, err := BackfillRelations(b.BoardID)
		if err != nil {
			logging.Warnf("BackfillAllRelations: board %d failed: %v", b.BoardID, err)
			continue
		}
		results[b.BoardID] = rebuilt
		logging.Infof("BackfillAllRelations: board %d rebuilt %d relations", b.BoardID, rebuilt)
	}
	return results, nil
}
