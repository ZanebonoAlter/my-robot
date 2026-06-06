# Narrative Section Relations - Remaining Tasks Implementation Plan

> **SUPERSEDED:** The incremental greedy matching algorithm described in this plan (`MatchAndSaveRelations`, `shouldWriteRelation`, `competitiveFilter`, `hasContinuationInIntermediateDays`) has been replaced by Hungarian bipartite matching (`RebuildBoardRelations`). See `docs/plans/2026-06-06-bipartite-relation-matching.md`.

> **REQUIRED SUB-SKILL:** Use the executing-plans skill to implement this plan task-by-task.

**Goal:** Complete the remaining 3 task groups for the narrative-section-relations change: fix distance=0.0 bug, implement same-day section merging, and verify everything passes.

**Architecture:** The distance bug fix is a targeted SQL replacement in repository.go. The section merge feature adds a new `MergeSimilarSections` function that runs between embedding generation and `SaveReport`, using union-find for deterministic merging and LLM batch call for gray-zone pairs. Verification ensures both backend and frontend compile cleanly.

**Tech Stack:** Go (Gin/GORM), PostgreSQL with pgvector, LLM via airouter, TypeScript (Nuxt 4)

---

## Task 1: Fix distance=0.0 bug in MatchAndSaveRelations

**Files:**
- Modify: `backend-go/internal/domain/daily_report/repository.go` (lines ~412-419, ~662)

**Step 1: Fix MatchAndSaveRelations (line ~412-419)**

Replace the `FirstOrCreate` block with raw SQL upsert:

```go
// OLD:
relation := SectionRelation{
    FromSectionID: m.MatchID,
    ToSectionID:   sec.ID,
    Distance:      m.Distance,
}
if err := tx.Where("from_section_id = ? AND to_section_id = ?",
    m.MatchID, sec.ID).FirstOrCreate(&relation).Error; err != nil {
    logging.Warnf("MatchAndSaveRelations: save relation failed: %v", err)
}

// NEW:
if err := tx.Exec(`
    INSERT INTO daily_report_section_relations (from_section_id, to_section_id, distance)
    VALUES (?, ?, ?)
    ON CONFLICT (from_section_id, to_section_id) DO UPDATE SET distance = EXCLUDED.distance
`, m.MatchID, sec.ID, m.Distance).Error; err != nil {
    logging.Warnf("MatchAndSaveRelations: save relation failed: %v", err)
}
```

**Step 2: Fix BackfillSectionEmbeddings (line ~662)**

Same pattern — replace `FirstOrCreate` with raw SQL upsert:

```go
// OLD:
relation := SectionRelation{
    FromSectionID: match.MatchID,
    ToSectionID:   sec.ID,
    Distance:      match.Distance,
}
database.DB.Where("from_section_id = ? AND to_section_id = ?",
    match.MatchID, sec.ID).FirstOrCreate(&relation)

// NEW:
database.DB.Exec(`
    INSERT INTO daily_report_section_relations (from_section_id, to_section_id, distance)
    VALUES (?, ?, ?)
    ON CONFLICT (from_section_id, to_section_id) DO UPDATE SET distance = EXCLUDED.distance
`, match.MatchID, sec.ID, match.Distance)
```

**Step 3: Verify backend compiles**

Run: `cd backend-go && go build ./...`
Expected: no errors

**Step 4: Run affected tests**

Run: `cd backend-go && go test ./internal/domain/daily_report/...`
Expected: all pass

**Step 5: Commit**

```bash
git add backend-go/internal/domain/daily_report/repository.go
git commit -m "fix: replace FirstOrCreate with raw SQL upsert to correctly store relation distance"
```

---

## Task 2: Add MergeSimilarSections function - core logic

**Files:**
- Modify: `backend-go/internal/domain/daily_report/generator.go` (add new function after `populateThreadArticles`)

**Step 1: Write the MergeSimilarSections function**

Add after `populateThreadArticles` function (~line 490):

```go
// MergeSimilarSections performs two-stage merging of same-day sections
// to eliminate over-fragmented clusters.
// Stage 1: deterministic merge via embedding distance < 0.20 (union-find for transitive closure)
// Stage 2: LLM arbitration for gray-zone pairs (distance 0.20-0.25)
func MergeSimilarSections(
	ctx context.Context,
	sections []DailyReportSection,
	threadBatches [][]DailyReportThread,
	tags []TagInput,
) ([]DailyReportSection, [][]DailyReportThread, error) {
	// Build tag ID → score lookup for precise avgScore recomputation
	tagScoreMap := make(map[uint]float64)
	for _, t := range tags {
		tagScoreMap[t.ID] = t.Score
	}
	if len(sections) <= 1 {
		return sections, threadBatches, nil
	}

	// Build a tag ID → label lookup for LLM prompts
	tagLabelMap := make(map[uint]string)
	for _, t := range tags {
		tagLabelMap[t.ID] = t.Label
	}

	// Compute pairwise embedding distances
	type pair struct {
		i, j    int
		distance float64
	}
	var grayZonePairs []pair // 0.20 - 0.25
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
				grayZonePairs = append(grayZonePairs, pair{i: i, j: j, distance: dist})
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
	indexMap := make(map[int]int) // old index → new index (for thread batches)

	for root, indices := range groups {
		if len(indices) == 1 {
			idx := len(mergedSections)
			indexMap[indices[0]] = idx
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

		// Merge tag IDs and collect all tag IDs for score recomputation
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

		newIdx := len(mergedSections)
		for _, idx := range indices {
			indexMap[idx] = newIdx
		}
		mergedSections = append(mergedSections, primary)
		mergedThreadBatches = append(mergedThreadBatches, allThreads)
	}

	logging.Infof("MergeSimilarSections: merged %d sections → %d", n, len(mergedSections))
	return mergedSections, mergedThreadBatches, nil
}

// parsePgVector parses "[0.1,0.2,0.3]" into []float64
```

**Step 2: Add cosineDistance helper function**

```go
// cosineDistance computes cosine distance between two pgvector strings.
// pgvector stores vectors as [0.1,0.2,0.3] format.
// cosine distance = 1 - cosine similarity.
func cosineDistance(vec1, vec2 string) (float64, error) {
	v1, err := parsePgVector(vec1)
	if err != nil {
		return 1.0, err
	}
	v2, err := parsePgVector(vec2)
	if err != nil {
		return 1.0, err
	}
	if len(v1) != len(v2) || len(v1) == 0 {
		return 1.0, fmt.Errorf("vector dimension mismatch or empty")
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

// parsePgVector parses "[0.1,0.2,0.3]" into []float64
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
```

**Step 3: Add LLM arbitration function**

```go
// llmArbitrateMerges uses LLM to decide whether gray-zone section pairs should be merged.
func llmArbitrateMerges(ctx context.Context, sections []DailyReportSection, pairs []struct{i, j int; distance float64}, tagLabelMap map[uint]string) ([]struct{i, j int}, error) {
	// Build prompt
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

	sb.WriteString("请返回 JSON，格式为 {\"merge_pairs\": [[配对索引, ...]]}，只包含应该合并的配对索引。\n")

	temperature := 0.1
	maxTokens := 2048
	result, err := airouter.NewRouter().Chat(ctx, airouter.ChatRequest{
		Capability: airouter.CapabilityTopicTagging,
		Messages: []airouter.Message{
			{Role: "system", Content: "你是一名专业的新闻叙事分析师。你的任务是判断两个叙事分组是否描述的是同一个更大的故事/叙事框架。"},
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
			"operation": "daily_report_section_merge_arbitration",
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

	var mergeResult []struct{i, j int}
	for _, idx := range response.MergePairs {
		if idx >= 0 && idx < len(pairs) {
			mergeResult = append(mergeResult, struct{i, j int}{pairs[idx].i, pairs[idx].j})
		}
	}
	return mergeResult, nil
}
```

**Step 4: Verify backend compiles**

Run: `cd backend-go && go build ./...`
Expected: no errors

**Step 5: Run affected tests**

Run: `cd backend-go && go test ./internal/domain/daily_report/...`
Expected: all pass

**Step 6: Commit**

```bash
git add backend-go/internal/domain/daily_report/generator.go
git commit -m "feat: add MergeSimilarSections function with two-stage merging logic"
```

---

## Task 3: Integrate MergeSimilarSections into the pipeline

**Files:**
- Modify: `backend-go/internal/domain/daily_report/handler.go` (lines ~103, ~148)

**Step 1: Add MergeSimilarSections call in generateSingleBoard (after line ~103)**

The call should go between `GenerateDailyReport` return and `SaveReport`. Need to pass the tags from generation to merge. But `GenerateDailyReport` already has the tags internally — we should call merge inside `GenerateDailyReport` itself, after embedding generation.

Actually, looking at the design (D6): "时机：在 SaveReport() 事务中，sections 写入后、relations 写入前执行合并。" But this doesn't work because sections get IDs in SaveReport. Better: call merge inside `GenerateDailyReport` after embeddings, before return.

Modify `GenerateDailyReport` in `generator.go` — add merge call right before the return (after the embedding block, ~line 467):

```go
	// Step 7: Merge similar same-day sections (two-stage: embedding + LLM)
	sections, threadBatches, mergeErr := MergeSimilarSections(ctx, sections, threadBatches, tags)
	if mergeErr != nil {
		logging.Warnf("daily-report: section merge failed for board %d: %v", boardID, mergeErr)
		// Non-fatal: continue with unmerged sections
	}

	return report, sections, threadBatches, nil
```

**Step 2: Verify backend compiles**

Run: `cd backend-go && go build ./...`
Expected: no errors

**Step 3: Run affected tests**

Run: `cd backend-go && go test ./internal/domain/daily_report/...`
Expected: all pass

**Step 4: Commit**

```bash
git add backend-go/internal/domain/daily_report/generator.go
git commit -m "feat: integrate MergeSimilarSections into GenerateDailyReport pipeline"
```

---

## Task 4: Mark completed tasks and run full verification

**Files:**
- Modify: `openspec/changes/narrative-section-relations/tasks.md` (mark tasks complete)

**Step 1: Backend verification**

Run: `cd backend-go && golangci-lint run ./internal/domain/daily_report/... && go vet ./internal/domain/daily_report/... && go test ./internal/domain/daily_report/... && go build ./...`

**Step 2: Frontend verification**

```bash
cd front && pnpm lint
cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"
cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"
```

**Step 3: Mark tasks complete in tasks.md**

- [x] 新增 `MergeSimilarSections(...)` 函数
- [x] Stage 1：计算同日 section embedding pairwise distance...
- [x] 合并规则...
- [x] Stage 2...
- [x] `GenerateDailyReport` 流水线中 embedding 生成后、`SaveReport` 前插入 `MergeSimilarSections` 调用
- [x] `MatchAndSaveRelations` 中将 `FirstOrCreate` 替换为 raw SQL...
- [x] 恢复前端 `DailyReportThread` 类型中的 `related_article_ids` 字段（已确认存在，无需操作）
- [x] 后端编译通过、lint 通过、受影响包测试通过
- [x] 前端 lint 通过、typecheck 通过、build 通过

**Step 4: Final commit**

```bash
git add openspec/changes/narrative-section-relations/tasks.md
git commit -m "chore: mark narrative-section-relations tasks 7-9 complete"
```
