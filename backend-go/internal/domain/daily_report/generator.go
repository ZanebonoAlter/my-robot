package daily_report

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"syntopica-backend/internal/domain/models"
	"syntopica-backend/internal/domain/tagging"
	"syntopica-backend/internal/platform/airouter"
	"syntopica-backend/internal/platform/database"
	"syntopica-backend/internal/platform/jsonutil"
	"syntopica-backend/internal/platform/logging"
)

const promptVersion = "3.0"

// ---------------------------------------------------------------------------
// LLM Call A: GenerateHighlights
// ---------------------------------------------------------------------------

const highlightsSystemPrompt = `你是一名专业的新闻分析师。你收到了一个看板的事件标签和聚类分组信息。

你的任务是生成 2-3 条当日要闻（highlights），每条要闻应该：
1. 有一个简洁有力的标题（中文，不超过20字）
2. 有一个简短的理由说明（中文，50-100字）
3. 关联到相关的标签ID

输出要求：
1. 顶层 JSON 对象，只包含 highlights 字段
2. highlights 是数组，每个元素包含 title（字符串）、reason（字符串）、tag_ids（整数数组）
3. 只返回合法 JSON，不要 Markdown 代码块或解释文字`

// GenerateHighlights produces 2-3 highlights for the report.
func GenerateHighlights(ctx context.Context, tags []TagInput, clusters []ClusterGroup) ([]Highlight, error) {
	if len(tags) == 0 {
		return nil, nil
	}

	prompt := buildHighlightsPrompt(tags, clusters)

	temperature := 0.3
	maxTokens := 2000
	result, err := airouter.NewRouter().Chat(ctx, airouter.ChatRequest{
		Capability: airouter.CapabilityTopicTagging,
		Messages: []airouter.Message{
			{Role: "system", Content: highlightsSystemPrompt},
			{Role: "user", Content: prompt},
		},
		Temperature: &temperature,
		MaxTokens:   &maxTokens,
		JSONMode:    true,
		JSONSchema: &airouter.JSONSchema{
			Type: "object",
			Properties: map[string]airouter.SchemaProperty{
				"highlights": {
					Type: "array",
					Items: &airouter.SchemaProperty{
						Type: "object",
						Properties: map[string]airouter.SchemaProperty{
							"title":   {Type: "string", Description: "要闻标题，不超过20字"},
							"reason":  {Type: "string", Description: "要闻理由，50-100字"},
							"tag_ids": {Type: "array", Items: &airouter.SchemaProperty{Type: "integer"}},
						},
						Required: []string{"title", "reason", "tag_ids"},
					},
				},
			},
			Required: []string{"highlights"},
		},
		Metadata: map[string]any{
			"operation":     "daily_report_highlights",
			"tag_count":     len(tags),
			"cluster_count": len(clusters),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("highlights AI call failed: %w", err)
	}

	logging.Infof("daily-report: highlights LLM response length=%d", len(result.Content))
	return parseHighlightsResponse(result.Content, tags)
}

func buildHighlightsPrompt(tags []TagInput, clusters []ClusterGroup) string {
	var sb strings.Builder
	sb.WriteString("## 事件标签\n\n")
	for _, t := range tags {
		fmt.Fprintf(&sb, "- [ID:%d] %s (文章数:%d)\n", t.ID, t.Label, t.ArticleCount)
	}
	if len(clusters) > 0 {
		sb.WriteString("\n## 聚类分组\n\n")
		for i, c := range clusters {
			fmt.Fprintf(&sb, "- 组%d: %s (标签IDs: %v)\n", i+1, c.GroupName, c.TagIDs)
		}
	}
	sb.WriteString("\n请生成 2-3 条当日要闻。\n")
	return sb.String()
}

func parseHighlightsResponse(content string, tags []TagInput) ([]Highlight, error) {
	content = jsonutil.SanitizeLLMJSON(content)

	var raw struct {
		Highlights []Highlight `json:"highlights"`
	}
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, fmt.Errorf("parse highlights JSON: %w", err)
	}

	validTagIDs := make(map[uint]bool, len(tags))
	for _, t := range tags {
		validTagIDs[t.ID] = true
	}

	var result []Highlight
	for _, h := range raw.Highlights {
		if strings.TrimSpace(h.Title) == "" {
			continue
		}
		var validIDs []uint
		for _, id := range h.TagIDs {
			if validTagIDs[id] {
				validIDs = append(validIDs, id)
			}
		}
		h.TagIDs = validIDs
		result = append(result, h)
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// LLM Call C: GenerateClusterThreads (per cluster)
// ---------------------------------------------------------------------------

const threadsSystemPrompt = `你是一名专业的新闻叙事分析师。你收到了一个事件聚类分组及其标签信息。

你的任务是识别该聚类中的叙事线索（threads），每条线索应该：
1. 有一个简洁有力的标题（中文，不超过30字，必须是带判断的短句）
2. 有一段客观的摘要（中文，100-200字）
3. 关联到相关的标签ID
4. 给出置信度分数（0-1）

输出要求：
1. 顶层 JSON 对象，只包含 threads 字段
2. threads 是数组；没有时返回 {"threads":[]}
3. 每个元素包含 title、summary、tag_ids、confidence 字段
4. 只返回合法 JSON，不要 Markdown 代码块或解释文字`

// GenerateClusterThreads produces threads for a single cluster.
func GenerateClusterThreads(ctx context.Context, cluster ClusterGroup, tags []TagInput) ([]Thread, error) {
	clusterTags := filterTagsByIDs(tags, cluster.TagIDs)
	if len(clusterTags) == 0 {
		return nil, nil
	}

	prompt := buildThreadsPrompt(cluster, clusterTags)

	temperature := 0.3
	maxTokens := 2000
	result, err := airouter.NewRouter().Chat(ctx, airouter.ChatRequest{
		Capability: airouter.CapabilityTopicTagging,
		Messages: []airouter.Message{
			{Role: "system", Content: threadsSystemPrompt},
			{Role: "user", Content: prompt},
		},
		Temperature: &temperature,
		MaxTokens:   &maxTokens,
		JSONMode:    true,
		JSONSchema: &airouter.JSONSchema{
			Type: "object",
			Properties: map[string]airouter.SchemaProperty{
				"threads": {
					Type: "array",
					Items: &airouter.SchemaProperty{
						Type: "object",
						Properties: map[string]airouter.SchemaProperty{
							"title":      {Type: "string", Description: "叙事标题"},
							"summary":    {Type: "string", Description: "叙事摘要，100-200字"},
							"tag_ids":    {Type: "array", Items: &airouter.SchemaProperty{Type: "integer"}},
							"confidence": {Type: "number", Description: "0-1 置信度"},
						},
						Required: []string{"title", "summary", "tag_ids", "confidence"},
					},
				},
			},
			Required: []string{"threads"},
		},
		Metadata: map[string]any{
			"operation":    "daily_report_threads",
			"cluster_name": cluster.GroupName,
			"tag_count":    len(clusterTags),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("threads AI call failed for cluster %s: %w", cluster.GroupName, err)
	}

	logging.Infof("daily-report: threads LLM response for cluster '%s' length=%d", cluster.GroupName, len(result.Content))
	return parseThreadsResponse(result.Content, clusterTags)
}

func buildThreadsPrompt(cluster ClusterGroup, tags []TagInput) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "## 聚类: %s\n\n", cluster.GroupName)
	for _, t := range tags {
		fmt.Fprintf(&sb, "- [ID:%d] %s (文章数:%d", t.ID, t.Label, t.ArticleCount)
		if t.Description != "" {
			fmt.Fprintf(&sb, ", 描述:%s", t.Description)
		}
		sb.WriteString(")\n")
	}
	sb.WriteString("\n请识别该聚类中的叙事线索。\n")
	return sb.String()
}

func parseThreadsResponse(content string, tags []TagInput) ([]Thread, error) {
	content = jsonutil.SanitizeLLMJSON(content)

	var raw struct {
		Threads []Thread `json:"threads"`
	}
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, fmt.Errorf("parse threads JSON: %w", err)
	}

	validTagIDs := make(map[uint]bool, len(tags))
	for _, t := range tags {
		validTagIDs[t.ID] = true
	}

	var result []Thread
	for _, th := range raw.Threads {
		if strings.TrimSpace(th.Title) == "" {
			continue
		}
		var validIDs []uint
		for _, id := range th.TagIDs {
			if validTagIDs[id] {
				validIDs = append(validIDs, id)
			}
		}
		th.TagIDs = validIDs
		result = append(result, th)
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// Orchestrator: GenerateDailyReport
// ---------------------------------------------------------------------------

// GenerateDailyReport is the main pipeline that generates a daily report for a board.
func GenerateDailyReport(ctx context.Context, boardID uint, date time.Time) (*BoardDailyReport, []DailyReportSection, [][]DailyReportThread, error) {
	startOfDay := normalizeReportDate(date)

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
		data []Highlight
		err  error
	}
	type threadsResult struct {
		clusterIdx int
		data       []Thread
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
		go func(idx int, c ClusterGroup) {
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
	threadsByCluster := make(map[int][]Thread, len(clusters))
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

	report := &BoardDailyReport{
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
	var sections []DailyReportSection
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

		sections = append(sections, DailyReportSection{
			ClusterIndex:  i,
			ClusterLabel:  cluster.GroupName,
			ClusterTagIDs: tagIDsJSON,
			ArticleCount:  clusterArticleCount,
			BestTier:      bestTier,
			AvgScore:      avgScore,
		})
	}

	// Build thread batches (convert []Thread → []DailyReportThread per cluster)
	var threadBatches [][]DailyReportThread
	for i := range clusters {
		threads := threadsByCluster[i]
		var batch []DailyReportThread
		for _, th := range threads {
			tagIDsJSON, _ := json.Marshal(th.TagIDs)
			articleIDsJSON, _ := json.Marshal(th.RelatedArticleIDs)
			batch = append(batch, DailyReportThread{
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
				sections[idx].Embedding = floatsToPgVector(embedResult.Embeddings[j])
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

// ---------------------------------------------------------------------------
// Continuity matching
// ---------------------------------------------------------------------------

// populateThreadArticles fills RelatedArticleIDs for each thread based on tag→article mapping.
func populateThreadArticles(threads []Thread, tagArticleMap map[uint][]uint) {
	for i := range threads {
		seen := make(map[uint]bool)
		var articleIDs []uint
		for _, tagID := range threads[i].TagIDs {
			for _, artID := range tagArticleMap[tagID] {
				if !seen[artID] {
					seen[artID] = true
					articleIDs = append(articleIDs, artID)
				}
			}
		}
		threads[i].RelatedArticleIDs = articleIDs
	}
}

// ---------------------------------------------------------------------------
// Section merging (same-day de-fragmentation)
// ---------------------------------------------------------------------------

// parsePgVector parses "[0.1,0.2,0.3]" format into []float64.
func parsePgVector(s string) ([]float64, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	if s == "" {
		return nil, fmt.Errorf("empty vector")
	}
	parts := strings.Split(s, ",")
	result := make([]float64, len(parts))
	for i, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return nil, fmt.Errorf("parse vector element %d: %w", i, err)
		}
		result[i] = v
	}
	return result, nil
}

// cosineDistance computes cosine distance (= 1 - cosine similarity) between two pgvector strings.
func cosineDistance(vec1, vec2 string) (float64, error) {
	v1, err := parsePgVector(vec1)
	if err != nil {
		return 1.0, fmt.Errorf("parse vec1: %w", err)
	}
	v2, err := parsePgVector(vec2)
	if err != nil {
		return 1.0, fmt.Errorf("parse vec2: %w", err)
	}
	if len(v1) != len(v2) || len(v1) == 0 {
		return 1.0, fmt.Errorf("vector dimension mismatch or empty: %d vs %d", len(v1), len(v2))
	}

	var dot, norm1, norm2 float64
	for i := range v1 {
		dot += v1[i] * v2[i]
		norm1 += v1[i] * v1[i]
		norm2 += v2[i] * v2[i]
	}
	if norm1 == 0 || norm2 == 0 {
		return 1.0, nil
	}
	similarity := dot / (math.Sqrt(norm1) * math.Sqrt(norm2))
	return 1.0 - similarity, nil
}

// llmMergePair represents a candidate pair for LLM merge arbitration.
type llmMergePair struct {
	i, j     int
	distance float64
}

// llmArbitrateMerges uses LLM to decide whether gray-zone section pairs should be merged.
func llmArbitrateMerges(ctx context.Context, sections []DailyReportSection, pairs []llmMergePair, tagLabelMap map[uint]string) ([]llmMergePair, error) {
	var sb strings.Builder
	sb.WriteString("以下是一些同日生成的叙事分组（section）配对，它们语义相似但不确定是否属于同一叙事框架。\n")
	sb.WriteString("请判断每对是否应该合并为一个 section。合并标准：它们描述的是同一个更大的叙事/故事。\n\n")

	for idx, p := range pairs {
		labelA := sections[p.i].ClusterLabel
		labelB := sections[p.j].ClusterLabel
		var tagIDsA, tagIDsB []uint
		json.Unmarshal(sections[p.i].ClusterTagIDs, &tagIDsA)
		json.Unmarshal(sections[p.j].ClusterTagIDs, &tagIDsB)

		var labelsA, labelsB []string
		for _, id := range tagIDsA {
			if l, ok := tagLabelMap[id]; ok {
				labelsA = append(labelsA, l)
			}
		}
		for _, id := range tagIDsB {
			if l, ok := tagLabelMap[id]; ok {
				labelsB = append(labelsB, l)
			}
		}

		fmt.Fprintf(&sb, "配对 %d:\n", idx)
		fmt.Fprintf(&sb, "  Section A: \"%s\" (标签: %s)\n", labelA, strings.Join(labelsA, ", "))
		fmt.Fprintf(&sb, "  Section B: \"%s\" (标签: %s)\n", labelB, strings.Join(labelsB, ", "))
		fmt.Fprintf(&sb, "  语义距离: %.3f\n\n", p.distance)
	}

	sb.WriteString("请返回 JSON，格式为 {\"merge_pairs\": [配对索引列表]}，只包含应该合并的配对索引（0-based）。\n")

	temperature := 0.1
	maxTokens := 2048
	result, err := airouter.NewRouter().Chat(ctx, airouter.ChatRequest{
		Capability: airouter.CapabilityTopicTagging,
		Messages: []airouter.Message{
			{Role: "system", Content: "你是一名专业的新闻叙事分析师。你的任务是判断两个叙事分组是否描述的是同一个更大的故事/叙事框架。只返回应该合并的配对索引。"},
			{Role: "user", Content: sb.String()},
		},
		Temperature: &temperature,
		MaxTokens:   &maxTokens,
		JSONMode:    true,
		JSONSchema: &airouter.JSONSchema{
			Type: "object",
			Properties: map[string]airouter.SchemaProperty{
				"merge_pairs": {
					Type:  "array",
					Items: &airouter.SchemaProperty{Type: "integer"},
				},
			},
			Required: []string{"merge_pairs"},
		},
		Metadata: map[string]any{
			"operation":  "daily_report_section_merge_arbitration",
			"pair_count": len(pairs),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("LLM merge arbitration failed: %w", err)
	}

	content := jsonutil.SanitizeLLMJSON(result.Content)
	var response struct {
		MergePairs []int `json:"merge_pairs"`
	}
	if err := json.Unmarshal([]byte(content), &response); err != nil {
		return nil, fmt.Errorf("parse merge arbitration response: %w", err)
	}

	var mergeResult []llmMergePair
	for _, idx := range response.MergePairs {
		if idx >= 0 && idx < len(pairs) {
			mergeResult = append(mergeResult, pairs[idx])
		}
	}
	return mergeResult, nil
}

// MergeSimilarSections performs two-stage merging of same-day sections
// to eliminate over-fragmented clusters.
// Stage 1: deterministic merge via embedding distance < 0.20 (union-find for transitive closure)
// Stage 2: LLM arbitration for gray-zone pairs (distance 0.20-0.25)
// Merge failures are non-fatal: the pipeline continues with unmerged sections.
func MergeSimilarSections(
	ctx context.Context,
	sections []DailyReportSection,
	threadBatches [][]DailyReportThread,
	tags []TagInput,
) ([]DailyReportSection, [][]DailyReportThread, error) {
	if len(sections) <= 1 {
		return sections, threadBatches, nil
	}

	// Build tag ID → label and tag ID → score lookups
	tagLabelMap := make(map[uint]string)
	tagScoreMap := make(map[uint]float64)
	for _, t := range tags {
		tagLabelMap[t.ID] = t.Label
		tagScoreMap[t.ID] = t.Score
	}

	var grayZonePairs []llmMergePair // 0.20 - 0.25
	deterministicPairs := make(map[int]map[int]bool) // i → set of j's for dist < 0.20

	for i := 0; i < len(sections); i++ {
		if strings.TrimSpace(sections[i].Embedding) == "" {
			continue
		}
		for j := i + 1; j < len(sections); j++ {
			if strings.TrimSpace(sections[j].Embedding) == "" {
				continue
			}
			dist, err := cosineDistance(sections[i].Embedding, sections[j].Embedding)
			if err != nil {
				continue
			}
			if dist < 0.20 {
				if deterministicPairs[i] == nil {
					deterministicPairs[i] = make(map[int]bool)
				}
				deterministicPairs[i][j] = true
			} else if dist < 0.25 {
				grayZonePairs = append(grayZonePairs, llmMergePair{i: i, j: j, distance: dist})
			}
		}
	}

	// Union-find for deterministic pairs (transitive closure)
	n := len(sections)
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	var find func(x int) int
	find = func(x int) int {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	union := func(x, y int) {
		px, py := find(x), find(y)
		if px != py {
			parent[px] = py
		}
	}

	for i, js := range deterministicPairs {
		for j := range js {
			union(i, j)
		}
	}

	// Stage 2: LLM arbitration for gray-zone pairs
	if len(grayZonePairs) > 0 {
		mergePairs, err := llmArbitrateMerges(ctx, sections, grayZonePairs, tagLabelMap)
		if err != nil {
			logging.Warnf("MergeSimilarSections: LLM arbitration failed: %v", err)
		} else {
			for _, p := range mergePairs {
				union(p.i, p.j)
			}
		}
	}

	// Group sections by root
	groups := make(map[int][]int) // root → indices
	for i := 0; i < n; i++ {
		root := find(i)
		groups[root] = append(groups[root], i)
	}

	// If no merging happened, return as-is
	if len(groups) == n {
		return sections, threadBatches, nil
	}

	// Merge each group: keep highest article_count as primary
	var mergedSections []DailyReportSection
	var mergedThreadBatches [][]DailyReportThread

	for _, indices := range groups {
		if len(indices) == 1 {
			mergedSections = append(mergedSections, sections[indices[0]])
			if indices[0] < len(threadBatches) {
				mergedThreadBatches = append(mergedThreadBatches, threadBatches[indices[0]])
			} else {
				mergedThreadBatches = append(mergedThreadBatches, nil)
			}
			continue
		}

		// Find primary: highest article_count
		primaryIdx := indices[0]
		for _, idx := range indices[1:] {
			if sections[idx].ArticleCount > sections[primaryIdx].ArticleCount {
				primaryIdx = idx
			}
		}

		primary := sections[primaryIdx]

		// Merge tag IDs
		tagIDSet := make(map[uint]bool)
		var primaryTagIDs []uint
		json.Unmarshal(primary.ClusterTagIDs, &primaryTagIDs)
		for _, id := range primaryTagIDs {
			tagIDSet[id] = true
		}

		totalArticles := primary.ArticleCount
		bestTier := primary.BestTier

		// Merge threads from secondary sections
		var allThreads []DailyReportThread
		if primaryIdx < len(threadBatches) {
			allThreads = append(allThreads, threadBatches[primaryIdx]...)
		}

		for _, idx := range indices {
			if idx == primaryIdx {
				continue
			}
			sec := sections[idx]

			var secTagIDs []uint
			json.Unmarshal(sec.ClusterTagIDs, &secTagIDs)
			for _, id := range secTagIDs {
				tagIDSet[id] = true
			}

			totalArticles += sec.ArticleCount
			if sec.BestTier < bestTier {
				bestTier = sec.BestTier
			}

			if idx < len(threadBatches) {
				allThreads = append(allThreads, threadBatches[idx]...)
			}
		}

		// Recompute merged tag IDs
		var mergedTagIDs []uint
		for id := range tagIDSet {
			mergedTagIDs = append(mergedTagIDs, id)
		}
		mergedTagIDsJSON, _ := json.Marshal(mergedTagIDs)

		// Precisely recompute avgScore from tag scores
		totalScore := 0.0
		scoreCount := 0
		for _, id := range mergedTagIDs {
			if s, ok := tagScoreMap[id]; ok {
				totalScore += s
				scoreCount++
			}
		}
		avgScore := 0.0
		if scoreCount > 0 {
			avgScore = totalScore / float64(scoreCount)
		}

		primary.ClusterTagIDs = mergedTagIDsJSON
		primary.ArticleCount = totalArticles
		primary.BestTier = bestTier
		primary.AvgScore = avgScore

		mergedSections = append(mergedSections, primary)
		mergedThreadBatches = append(mergedThreadBatches, allThreads)
	}

	logging.Infof("MergeSimilarSections: merged %d sections → %d", n, len(mergedSections))
	return mergedSections, mergedThreadBatches, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// collectBoardTags loads event tags for a semantic board on a given date.
func collectBoardTags(boardID uint, date time.Time) ([]TagInput, [][]uint, error) {
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
	err := database.DB.Model(&models.TopicTag{}).
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

	var tags []TagInput
	var articleIDSets [][]uint
	for _, row := range rows {
		tags = append(tags, TagInput{
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
		database.DB.Model(&models.ArticleTopicTag{}).
			Select("DISTINCT article_topic_tags.article_id").
			Joins("JOIN articles ON articles.id = article_topic_tags.article_id").
			Where("article_topic_tags.topic_tag_id = ? AND articles.pub_date >= ? AND articles.pub_date < ?",
				row.ID, startOfDay, endOfDay).
			Pluck("article_topic_tags.article_id", &artIDs)
		articleIDSets = append(articleIDSets, artIDs)
	}

	// Fallback: find event tags with auxiliaries but no board labels, compute matches
	var unmatchedTagIDs []uint
	database.DB.Model(&models.TopicTag{}).
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
		matcher := tagging.NewSemanticBoardMatchingService(database.DB)
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
			if err := database.DB.First(&t, tid).Error; err != nil {
				continue
			}
			var artIDs []uint
			database.DB.Model(&models.ArticleTopicTag{}).
				Select("DISTINCT article_topic_tags.article_id").
				Joins("JOIN articles ON articles.id = article_topic_tags.article_id").
				Where("article_topic_tags.topic_tag_id = ? AND articles.pub_date >= ? AND articles.pub_date < ?", tid, startOfDay, endOfDay).
				Pluck("article_topic_tags.article_id", &artIDs)

			tags = append(tags, TagInput{
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

// findPreviousReportBrief finds the most recent report for the board before the given date.
func findPreviousReportBrief(boardID uint, date time.Time) *BoardDailyReport {
	var report BoardDailyReport
	err := database.DB.Where("semantic_board_id = ? AND period_date < ? AND status = ?",
		boardID, normalizeReportDate(date).Format("2006-01-02"), "completed").
		First(&report).Error
	if err != nil {
		return nil
	}
	return &report
}

func filterTagsByQuality(tags []TagInput) []TagInput {
	// Separate by match reason quality
	var kept []TagInput
	var weighted []TagInput
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

func filterTagsByIDs(tags []TagInput, ids []uint) []TagInput {
	idSet := make(map[uint]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}
	var result []TagInput
	for _, t := range tags {
		if idSet[t.ID] {
			result = append(result, t)
		}
	}
	return result
}
