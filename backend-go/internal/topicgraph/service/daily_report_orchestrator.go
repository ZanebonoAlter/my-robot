package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/airouter"
	"syntopica-backend/internal/platform/logging"
	tagging "syntopica-backend/internal/tagmanagement"
	"syntopica-backend/internal/topicgraph/repository"
)

func GenerateDailyReport(ctx context.Context, boardID uint, date time.Time) (*repository.BoardDailyReport, []repository.DailyReportSection, [][]repository.DailyReportThread, error) {
	startOfDay := repository.NormalizeReportDate(date)

	// Step 1: Collect board event tags
	tags, articleCounts, err := collectBoardTags(boardID, date)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("collect board tags: %w", err)
	}
	if len(tags) == 0 {
		return nil, nil, nil, nil
	}

	// Build tag→articleIDs map (before dedup/filter changes the tag slice)
	tagArticleMap := make(map[uint][]uint, len(tags))
	for i, t := range tags {
		tagArticleMap[t.ID] = articleCounts[i]
	}

	// Step 2: Deduplicate
	tags = DeduplicateTags(tags, articleCounts)

	// Step 2.5: Quality filter
	tags = filterTagsByQuality(tags)
	if len(tags) == 0 {
		return nil, nil, nil, nil
	}

	// Step 3: Cluster via LLM
	clusters, err := ClusterTags(ctx, tags)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("cluster tags: %w", err)
	}

	// Step 4: Query yesterday's report for continuity
	prevReport := findPreviousReportBrief(boardID, startOfDay)

	// Step 5: Generate in parallel (A + C×K)
	type highlightsResult struct {
		data []repository.Highlight
		err  error
	}
	type threadsResult struct {
		clusterIdx int
		data       []repository.Thread
		err        error
	}

	highlightsCh := make(chan highlightsResult, 1)
	threadsCh := make(chan threadsResult, len(clusters))

	// Call A: Highlights
	go func() {
		data, err := GenerateHighlights(ctx, tags, clusters)
		highlightsCh <- highlightsResult{data: data, err: err}
	}()

	// Call C×K: Threads per cluster
	for i, cluster := range clusters {
		go func(idx int, c repository.ClusterGroup) {
			data, err := GenerateClusterThreads(ctx, c, tags)
			threadsCh <- threadsResult{clusterIdx: idx, data: data, err: err}
		}(i, cluster)
	}

	// Collect highlights
	hr := <-highlightsCh
	if hr.err != nil {
		return nil, nil, nil, fmt.Errorf("generate highlights: %w", hr.err)
	}

	// Collect threads
	threadsByCluster := make(map[int][]repository.Thread, len(clusters))
	for i := 0; i < len(clusters); i++ {
		tr := <-threadsCh
		if tr.err != nil {
			logging.Warnf("daily-report: threads failed for cluster %d: %v", tr.clusterIdx, tr.err)
			continue
		}
		threadsByCluster[tr.clusterIdx] = tr.data
	}

	// Step 6: Assemble report
	highlightsJSON, _ := json.Marshal(hr.data)

	// Calculate article count
	totalArticles := 0
	for _, t := range tags {
		totalArticles += t.ArticleCount
	}

	// Title: use first highlight if available, else board name + date
	title := fmt.Sprintf("日报 %s", startOfDay.Format("2006-01-02"))
	if len(hr.data) > 0 {
		title = hr.data[0].Title
	}

	// Summary: use first highlight reason as summary fallback
	summary := ""
	if len(hr.data) > 0 {
		summary = hr.data[0].Reason
	}

	clustersJSON, _ := json.Marshal(clusters)

	report := &repository.BoardDailyReport{
		SemanticBoardID:         boardID,
		PeriodDate:              startOfDay,
		Title:                   title,
		Summary:                 summary,
		Highlights:              highlightsJSON,
		Dynamics:                "",
		ArticleCount:            totalArticles,
		EventTagCount:           len(tags),
		ClusterCount:            len(clusters),
		Status:                  "completed",
		RawClusters:             clustersJSON,
		GenerationPromptVersion: promptVersion,
	}
	if prevReport != nil {
		report.PrevReportID = &prevReport.ID
	}

	// Build sections
	var sections []repository.DailyReportSection
	for i, cluster := range clusters {
		threads := threadsByCluster[i]
		// Populate related article IDs
		populateThreadArticles(threads, tagArticleMap)
		clusterTags := filterTagsByIDs(tags, cluster.TagIDs)
		clusterArticleCount := 0
		for _, t := range clusterTags {
			clusterArticleCount += t.ArticleCount
		}

		tagIDsJSON, _ := json.Marshal(cluster.TagIDs)

		// Calculate best_tier and avg_score
		tagIDSet := make(map[uint]bool)
		for _, tid := range cluster.TagIDs {
			tagIDSet[tid] = true
		}
		bestTier := 4 // worst possible
		totalScore := 0.0
		matchCount := 0
		for _, t := range tags {
			if tagIDSet[t.ID] {
				tier := tagging.MatchTier(t.MatchReason, false)
				if tier < bestTier {
					bestTier = tier
				}
				totalScore += t.Score
				matchCount++
			}
		}
		avgScore := 0.0
		if matchCount > 0 {
			avgScore = totalScore / float64(matchCount)
		}

		sections = append(sections, repository.DailyReportSection{
			ClusterIndex:  i,
			ClusterLabel:  cluster.GroupName,
			ClusterTagIDs: tagIDsJSON,
			ArticleCount:  clusterArticleCount,
			BestTier:      bestTier,
			AvgScore:      avgScore,
		})
	}

	// Build thread batches (convert []repository.Thread → []repository.DailyReportThread per cluster)
	var threadBatches [][]repository.DailyReportThread
	for i := range clusters {
		threads := threadsByCluster[i]
		var batch []repository.DailyReportThread
		for _, th := range threads {
			tagIDsJSON, _ := json.Marshal(th.TagIDs)
			articleIDsJSON, _ := json.Marshal(th.RelatedArticleIDs)
			batch = append(batch, repository.DailyReportThread{
				Title:             th.Title,
				Summary:           th.Summary,
				TagIDs:            tagIDsJSON,
				Confidence:        th.Confidence,
				RelatedArticleIDs: articleIDsJSON,
			})
		}
		threadBatches = append(threadBatches, batch)
	}

	// Generate section embeddings from cluster_label texts
	var embedTexts []string
	var embedIndices []int
	for i, sec := range sections {
		if strings.TrimSpace(sec.ClusterLabel) != "" {
			embedTexts = append(embedTexts, sec.ClusterLabel)
			embedIndices = append(embedIndices, i)
		}
	}
	if len(embedTexts) > 0 {
		embedResult, embedErr := airouter.NewRouter().Embed(ctx, airouter.EmbeddingRequest{
			Input: embedTexts,
			Metadata: map[string]any{
				"operation": "daily_report_section_embedding",
				"board_id":  boardID,
			},
		}, airouter.CapabilityEmbedding)
		if embedErr != nil {
			logging.Warnf("daily-report: section embedding failed for board %d: %v", boardID, embedErr)
		} else if len(embedResult.Embeddings) >= len(embedTexts) {
			for j, idx := range embedIndices {
				sections[idx].Embedding = repository.FloatsToPgVector(embedResult.Embeddings[j])
			}
		}
	}

	// Step 7: Merge similar same-day sections (two-stage: embedding + LLM)
	sections, threadBatches, mergeErr := MergeSimilarSections(ctx, sections, threadBatches, tags)
	if mergeErr != nil {
		logging.Warnf("daily-report: section merge failed for board %d: %v", boardID, mergeErr)
		// Non-fatal: continue with unmerged sections
	}

	return report, sections, threadBatches, nil
}
func collectBoardTags(boardID uint, date time.Time) ([]repository.TagInput, [][]uint, error) {
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	type tagRow struct {
		ID           uint    `json:"id"`
		Label        string  `json:"label"`
		Category     string  `json:"category"`
		Description  string  `json:"description"`
		Source       string  `json:"source"`
		MatchReason  string  `json:"match_reason"`
		Score        float64 `json:"score"`
		ArticleCount int     `json:"article_count"`
	}

	var rows []tagRow
	err := repository.Repo.DB().Model(&models.TopicTag{}).
		Select(`topic_tags.id AS id,
			topic_tags.label AS label,
			topic_tags.category AS category,
			topic_tags.description AS description,
			topic_tags.source AS source,
			topic_tag_board_labels.match_reason AS match_reason,
			topic_tag_board_labels.score AS score,
			COUNT(DISTINCT articles.id) AS article_count`).
		Joins("JOIN topic_tag_board_labels ON topic_tag_board_labels.topic_tag_id = topic_tags.id").
		Joins("JOIN article_topic_tags ON article_topic_tags.topic_tag_id = topic_tags.id").
		Joins("JOIN articles ON articles.id = article_topic_tags.article_id").
		Where("topic_tag_board_labels.semantic_board_id = ?", boardID).
		Where("NOT COALESCE(topic_tag_board_labels.direction_mismatch, false)").
		Where("topic_tags.status = ? AND topic_tags.category = ?", "active", models.TagCategoryEvent).
		Where("articles.pub_date >= ? AND articles.pub_date < ?", startOfDay, endOfDay).
		Group("topic_tags.id, topic_tags.label, topic_tags.category, topic_tags.description, topic_tags.source, topic_tag_board_labels.match_reason, topic_tag_board_labels.score").
		Order("article_count DESC, topic_tags.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, nil, fmt.Errorf("query board event tags: %w", err)
	}

	if len(rows) == 0 {
		return nil, nil, nil
	}

	var tags []repository.TagInput
	var articleIDSets [][]uint
	for _, row := range rows {
		tags = append(tags, repository.TagInput{
			ID:           row.ID,
			Label:        row.Label,
			Category:     row.Category,
			Description:  row.Description,
			ArticleCount: row.ArticleCount,
			Source:       row.Source,
			MatchReason:  row.MatchReason,
			Score:        row.Score,
		})

		// Get article IDs for this tag on this date
		var artIDs []uint
		repository.Repo.DB().Model(&models.ArticleTopicTag{}).
			Select("DISTINCT article_topic_tags.article_id").
			Joins("JOIN articles ON articles.id = article_topic_tags.article_id").
			Where("article_topic_tags.topic_tag_id = ? AND articles.pub_date >= ? AND articles.pub_date < ?",
				row.ID, startOfDay, endOfDay).
			Pluck("article_topic_tags.article_id", &artIDs)
		articleIDSets = append(articleIDSets, artIDs)
	}

	// Fallback: find event tags with auxiliaries but no board labels, compute matches
	var unmatchedTagIDs []uint
	repository.Repo.DB().Model(&models.TopicTag{}).
		Select("DISTINCT topic_tags.id").
		Joins("JOIN article_topic_tags ON article_topic_tags.topic_tag_id = topic_tags.id").
		Joins("JOIN articles ON articles.id = article_topic_tags.article_id").
		Joins("JOIN topic_tag_semantic_labels ON topic_tag_semantic_labels.topic_tag_id = topic_tags.id").
		Where("topic_tags.status = ? AND topic_tags.category = ?", "active", models.TagCategoryEvent).
		Where("articles.pub_date >= ? AND articles.pub_date < ?", startOfDay, endOfDay).
		Where("NOT EXISTS (SELECT 1 FROM topic_tag_board_labels WHERE topic_tag_board_labels.topic_tag_id = topic_tags.id)").
		Limit(50).
		Pluck("topic_tags.id", &unmatchedTagIDs)

	if len(unmatchedTagIDs) > 0 {
		logging.Infof("[daily-report] fallback: found %d unmatched event tags for board %d, computing matches", len(unmatchedTagIDs), boardID)
		matcher := tagging.NewSemanticBoardMatchingService(repository.Repo.DB())
		for _, tid := range unmatchedTagIDs {
			matches, matchErr := matcher.MatchTopicTag(context.Background(), tid)
			if matchErr != nil {
				logging.Warnf("[daily-report] fallback match failed for tag %d: %v", tid, matchErr)
				continue
			}
			var boardMatch *tagging.SemanticBoardMatchResult
			for i := range matches {
				if matches[i].SemanticBoardID == boardID && !matches[i].DirectionMismatch {
					boardMatch = &matches[i]
					break
				}
			}
			if boardMatch == nil {
				continue
			}

			var t models.TopicTag
			if err := repository.Repo.DB().First(&t, tid).Error; err != nil {
				continue
			}
			var artIDs []uint
			repository.Repo.DB().Model(&models.ArticleTopicTag{}).
				Select("DISTINCT article_topic_tags.article_id").
				Joins("JOIN articles ON articles.id = article_topic_tags.article_id").
				Where("article_topic_tags.topic_tag_id = ? AND articles.pub_date >= ? AND articles.pub_date < ?", tid, startOfDay, endOfDay).
				Pluck("article_topic_tags.article_id", &artIDs)

			tags = append(tags, repository.TagInput{
				ID:           t.ID,
				Label:        t.Label,
				Category:     t.Category,
				Description:  t.Description,
				Source:       t.Source,
				ArticleCount: len(artIDs),
				MatchReason:  boardMatch.MatchReason,
				Score:        boardMatch.Score,
			})
			articleIDSets = append(articleIDSets, artIDs)
		}
	}

	return tags, articleIDSets, nil
}
func findPreviousReportBrief(boardID uint, date time.Time) *repository.BoardDailyReport {
	var report repository.BoardDailyReport
	err := repository.Repo.DB().Where("semantic_board_id = ? AND period_date < ? AND status = ?",
		boardID, repository.NormalizeReportDate(date).Format("2006-01-02"), "completed").
		First(&report).Error
	if err != nil {
		return nil
	}
	return &report
}
func filterTagsByQuality(tags []repository.TagInput) []repository.TagInput {
	// Separate by match reason quality
	var kept []repository.TagInput
	var weighted []repository.TagInput
	for _, t := range tags {
		switch t.MatchReason {
		case "direct_hit", "hit_rate", "max_sim":
			kept = append(kept, t)
		default: // "weighted" or unknown
			weighted = append(weighted, t)
		}
	}

	// If kept < 10, pull back weighted tags (保底)
	if len(kept) < 10 {
		kept = append(kept, weighted...)
	}

	// If kept > 30, truncate by quality (截断)
	if len(kept) > 30 {
		sort.SliceStable(kept, func(i, j int) bool {
			ti := tagging.MatchTier(kept[i].MatchReason, false)
			tj := tagging.MatchTier(kept[j].MatchReason, false)
			if ti != tj {
				return ti < tj
			}
			return kept[i].Score > kept[j].Score
		})
		kept = kept[:30]
	}

	return kept
}
func filterTagsByIDs(tags []repository.TagInput, ids []uint) []repository.TagInput {
	idSet := make(map[uint]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}
	var result []repository.TagInput
	for _, t := range tags {
		if idSet[t.ID] {
			result = append(result, t)
		}
	}
	return result
}
