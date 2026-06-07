# Section Embedding 语义匹配 Implementation Plan

> **REQUIRED SUB-SKILL:** Use the executing-plans skill to implement this plan task-by-task.

**Goal:** 用 pgvector embedding 余弦距离替换 section 级别的 tag Jaccard 匹配，建立可靠的跨日 section 传承链。

**Architecture:** 在 `GenerateDailyReport()` 中批量生成 section `cluster_label` 的 embedding；`SaveReport()` 事务内先执行 pgvector 匹配（此时旧 section 还在），再删除旧数据并插入新数据。历史数据通过回填函数补全 embedding 和 `prev_section_id`。

**Tech Stack:** Go, GORM, pgvector, PostgreSQL

---

## Task 1: 数据库迁移 — 添加 embedding 列 (Task 8.1)

**Files:**
- Modify: `backend-go/internal/platform/database/postgres_migrations.go` (末尾追加)

**Step 1: 添加迁移**

在 `postgresMigrations()` 返回的 slice 末尾（最后一个 `}` 之前）追加新迁移：

```go
// ── Section embedding column ────────────────────────────────────
{
    Version:     "20260601_0002",
    Description: "Add embedding vector column to daily_report_sections for semantic section matching.",
    Up: func(db *gorm.DB) error {
        if err := db.Exec(`ALTER TABLE daily_report_sections ADD COLUMN IF NOT EXISTS embedding vector`).Error; err != nil {
            return fmt.Errorf("add daily_report_sections.embedding column: %w", err)
        }
        if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_daily_report_sections_embedding ON daily_report_sections USING hnsw (embedding vector_cosine_ops)`).Error; err != nil {
            // HNSW may fail if dimension > 2000; log but don't block
            logging.Warnf("Failed to create HNSW index on daily_report_sections.embedding: %v", err)
        }
        return nil
    },
},
```

注意：维度不硬编码，`type:vector` 不指定维度。HNSW 索引创建失败时仅 warn，因为维度可能超过 2000（HNSW 限制）。

**Step 2: 验证**

Run: `cd /mnt/d/project/Syntopica/backend-go && go build ./...`
Expected: 编译通过

**Step 3: Commit**

```
feat(daily-report): add embedding vector column migration for daily_report_sections
```

---

## Task 2: 更新 GORM 模型 (Task 8.2)

**Files:**
- Modify: `backend-go/internal/domain/daily_report/models.go`

**Step 1: 在 DailyReportSection 添加 Embedding 字段**

在 `PrevSectionID` 字段之前添加：

```go
Embedding     string    `gorm:"type:vector" json:"-"`
```

JSON tag 用 `"-"` 因为 embedding 不需要暴露给前端。放在 `PrevSectionID` 之前。

**Context — models.go 当前 DailyReportSection 结构体字段顺序：**
```go
type DailyReportSection struct {
    ID            uint
    ReportID      uint
    ClusterIndex  int
    ClusterLabel  string
    ClusterTagIDs JSON
    Threads       []DailyReportThread
    ArticleCount  int
    BestTier      int
    AvgScore      float64
    Status        string
    PrevSectionID *uint     // ← 在这之前插入 Embedding
    CreatedAt     time.Time
}
```

**Step 2: 验证**

Run: `cd /mnt/d/project/Syntopica/backend-go && go build ./...`
Expected: 编译通过

**Step 3: Commit**

```
feat(daily-report): add Embedding field to DailyReportSection GORM model
```

---

## Task 3: 批量生成 Section Embedding (Task 8.3)

**Files:**
- Modify: `backend-go/internal/domain/daily_report/generator.go`

**关键上下文：**

1. `airouter.EmbeddingRequest{Input: []string, ...}` — 批量文本 embedding
2. `airouter.NewRouter().Embed(ctx, req, airouter.CapabilityEmbedding)` — 返回 `(*EmbeddingResult, error)`
3. `EmbeddingResult{Embeddings: [][]float64, Dimensions: int, Model: string}`
4. `tagging` 包有 `floatsToPgVector(v []float64) string` — 但它是包私有的，需要在本包实现或导出

**Step 1: 在 generator.go 添加 floatsToPgVector 辅助函数**

在文件末尾（或 `filterTagsByIDs` 之后）添加：

```go
// floatsToPgVector converts a float64 slice to pgvector string format: [0.1,0.2,0.3]
func floatsToPgVector(v []float64) string {
	parts := make([]string, len(v))
	for i, f := range v {
		parts[i] = fmt.Sprintf("%f", f)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
```

**Step 2: 在 GenerateDailyReport() 中，在 `matchPreviousSections` 调用之前，添加 embedding 生成**

找到这段代码（约 line 380-381）：
```go
	// Section lifecycle matching (must run after all sections are assembled)
	prevSections := findPreviousSections(boardID, startOfDay)
	matchPreviousSections(sections, prevSections)
```

在它**之前**添加：

```go
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
```

这段逻辑：
- 收集所有非空 cluster_label 的文本和索引
- 一次批量 API 调用生成所有 embedding
- 跳过空 cluster_label 的 section
- 失败时仅 warn，不阻塞报告生成

**Step 3: 验证**

Run: `cd /mnt/d/project/Syntopica/backend-go && go build ./...`
Expected: 编译通过

**Step 4: Commit**

```
feat(daily-report): batch-generate section embeddings from cluster_label
```

---

## Task 4: pgvector 匹配 + SaveReport 流程重构 (Tasks 8.4, 8.5, 8.6, 8.7)

这是最核心的 task，涉及 4 个 tasks 合并。

**Files:**
- Modify: `backend-go/internal/domain/daily_report/repository.go`
- Modify: `backend-go/internal/domain/daily_report/generator.go`

### 4a: 添加 MatchSectionsByEmbedding 函数 (Task 8.5)

在 `repository.go` 的 `GetSectionLifecycle` 函数之后添加：

```go
// SectionEmbeddingMatch represents a match result for section embedding lookup.
type SectionEmbeddingMatch struct {
	PrevSectionID uint
	Distance      float64
}

// MatchSectionsByEmbedding finds the nearest existing section for each new section
// using pgvector cosine distance. Runs within SaveReport() transaction BEFORE
// old sections are deleted.
func MatchSectionsByEmbedding(tx *gorm.DB, boardID uint, sections []DailyReportSection) []SectionEmbeddingMatch {
	results := make([]SectionEmbeddingMatch, len(sections))

	for i, sec := range sections {
		if strings.TrimSpace(sec.Embedding) == "" {
			continue
		}
		var match SectionEmbeddingMatch
		err := tx.Raw(`
			SELECT s.id AS prev_section_id, s.embedding <=> ?::vector AS distance
			FROM daily_report_sections s
			JOIN board_daily_reports r ON r.id = s.report_id
			WHERE r.semantic_board_id = ?
			  AND r.status = 'completed'
			  AND s.embedding IS NOT NULL
			ORDER BY s.embedding <=> ?::vector
			LIMIT 1
		`, sec.Embedding, boardID, sec.Embedding).Scan(&match).Error
		if err != nil {
			continue
		}
		if match.Distance < 0.3 {
			results[i] = match
		}
	}

	return results
}
```

需要在 repository.go 顶部添加 import `"strings"`。

### 4b: 重构 SaveReport 流程 (Task 8.4)

**当前 SaveReport 流程**（需要重写）：
1. Upsert report → 2. nullify downstream refs → 3. delete old threads → 4. delete old sections → 5. insert new sections → 6. insert threads

**新流程**：
1. Upsert report → 2. **embedding 匹配**（旧 section 还在）→ 3. nullify downstream refs → 4. delete old threads/sections → 5. insert new sections（含 embedding + prev_section_id）→ 6. insert threads

替换整个 `SaveReport` 函数体为：

```go
func SaveReport(report *BoardDailyReport, sections []DailyReportSection, threadBatches [][]DailyReportThread) error {
	report.PeriodDate = normalizeReportDate(report.PeriodDate)
	return database.DB.Transaction(func(tx *gorm.DB) error {
		// Upsert report: find existing by (semantic_board_id, period_date)
		var existing BoardDailyReport
		err := tx.Where("semantic_board_id = ? AND period_date = ?",
			report.SemanticBoardID,
			report.PeriodDate.Format("2006-01-02")).
			First(&existing).Error

		if err == nil {
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

		// Embedding matching: find prev_section_id BEFORE deleting old data.
		// Old sections are still in DB at this point.
		matches := MatchSectionsByEmbedding(tx, report.SemanticBoardID, sections)
		for i, m := range matches {
			if m.PrevSectionID > 0 {
				sections[i].PrevSectionID = &m.PrevSectionID
				sections[i].Status = "continuing"
			}
		}

		if err == nil {
			// Nullify downstream prev_thread_id references before deleting old threads
			if err := tx.Model(&DailyReportThread{}).
				Where("prev_thread_id IN (SELECT id FROM daily_report_threads WHERE report_id = ?)", existing.ID).
				Update("prev_thread_id", nil).Error; err != nil {
				return fmt.Errorf("nullify downstream prev_thread_id: %w", err)
			}
			// Nullify downstream prev_section_id references before deleting old sections
			if err := tx.Model(&DailyReportSection{}).
				Where("prev_section_id IN (SELECT id FROM daily_report_sections WHERE report_id = ?)", existing.ID).
				Update("prev_section_id", nil).Error; err != nil {
				return fmt.Errorf("nullify downstream prev_section_id: %w", err)
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

		// Insert new sections (with embedding + prev_section_id)
		for i := range sections {
			sections[i].ReportID = report.ID
		}
		if len(sections) > 0 {
			if err := tx.CreateInBatches(sections, 20).Error; err != nil {
				return fmt.Errorf("create sections: %w", err)
			}
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
```

### 4c: 移除 matchPreviousSections 调用和废弃函数 (Tasks 8.6, 8.7)

在 `generator.go` 中：

1. **删除** `GenerateDailyReport()` 末尾这两行：
```go
	prevSections := findPreviousSections(boardID, startOfDay)
	matchPreviousSections(sections, prevSections)
```

2. **删除** `matchPreviousSections` 函数整体（约 line 327-380）

3. **删除** `findPreviousSections` 函数整体（约在 `getPrevThreadSummaries` 之后的 repository.go 中）

**Step: 验证**

Run: `cd /mnt/d/project/Syntopica/backend-go && go build ./...`
Expected: 编译通过

**Commit:**

```
feat(daily-report): replace tag Jaccard with pgvector embedding matching for section lineage

- Add MatchSectionsByEmbedding() using pgvector cosine distance
- Restructure SaveReport() to match BEFORE deleting old data
- Remove findPreviousSections() and matchPreviousSections()
```

---

## Task 5: 历史数据回填 (Tasks 9.1, 9.2, 9.3)

**Files:**
- Modify: `backend-go/internal/domain/daily_report/repository.go` (添加回填函数)
- Modify: `backend-go/internal/domain/daily_report/handler.go` (添加 API 端点)

### 5a: 回填函数 (Task 9.1, 9.2)

在 `repository.go` 末尾添加：

```go
// BackfillSectionEmbeddings generates embeddings for sections that don't have one,
// then runs pgvector matching to set prev_section_id for all sections.
func BackfillSectionEmbeddings(ctx context.Context) (embedded int, matched int, err error) {
	// Phase 1: Generate embeddings for sections without them
	batchSize := 50
	for {
		var sections []DailyReportSection
		if err := database.DB.Where("embedding IS NULL").
			Where("cluster_label != ''").
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

	// Phase 2: Run pgvector matching for ALL sections (overwrite unreliable tag Jaccard results)
	// Group by board
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
		var sections []DailyReportSection
		if err := database.DB.Raw(`
			SELECT s.id, s.embedding
			FROM daily_report_sections s
			JOIN board_daily_reports r ON r.id = s.report_id
			WHERE r.semantic_board_id = ?
			  AND s.embedding IS NOT NULL
			ORDER BY r.period_date ASC, s.id ASC
		`, b.BoardID).Scan(&sections).Error; err != nil {
			continue
		}

		for _, sec := range sections {
			var match struct {
				PrevSectionID uint
				Distance      float64
			}
			err := database.DB.Raw(`
				SELECT s2.id AS prev_section_id, s2.embedding <=> ?::vector AS distance
				FROM daily_report_sections s2
				JOIN board_daily_reports r2 ON r2.id = s2.report_id
				WHERE r2.semantic_board_id = ?
				  AND s2.id != ?
				  AND s2.embedding IS NOT NULL
				ORDER BY s2.embedding <=> ?::vector
				LIMIT 1
			`, sec.Embedding, b.BoardID, sec.ID, sec.Embedding).Scan(&match).Error
			if err != nil || match.PrevSectionID == 0 {
				continue
			}
			if match.Distance < 0.3 {
				status := "continuing"
				database.DB.Model(&DailyReportSection{}).Where("id = ?", sec.ID).
					Updates(map[string]interface{}{
						"prev_section_id": match.PrevSectionID,
						"status":          status,
					})
				matched++
			}
		}
	}

	return embedded, matched, nil
}
```

需要在 repository.go 顶部添加 import `"context"` 和 `"syntopica-backend/internal/platform/airouter"`。

注意：`floatsToPgVector` 已在 Task 3 中添加到 `generator.go`，但 repository.go 也需要它。有两个选择：
- A) 把 `floatsToPgVector` 移到一个共享位置
- B) 在 repository.go 也加一份

推荐方案 A：将其从 `generator.go` 移到 `models.go`（同包内），这样 repository.go 和 generator.go 都能用。Task 3 中添加时就放在 models.go。

### 5b: API 端点 (Task 9.3)

在 `handler.go` 的 `RegisterDailyReportRoutes` 中追加路由：

```go
// POST /api/daily-reports/backfill-embeddings
api.POST("/daily-reports/backfill-embeddings", triggerBackfillEmbeddings)
```

添加 handler：

```go
func triggerBackfillEmbeddings(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	go func() {
		embedded, matched, err := BackfillSectionEmbeddings(ctx)
		if err != nil {
			logging.Errorf("daily-report: backfill failed: %v", err)
			return
		}
		logging.Infof("daily-report: backfill complete: %d sections embedded, %d sections matched", embedded, matched)
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"status": "processing",
		},
	})
}
```

**验证:**

Run: `cd /mnt/d/project/Syntopica/backend-go && go build ./...`
Expected: 编译通过

**Commit:**

```
feat(daily-report): add section embedding backfill with API trigger

- BackfillSectionEmbeddings() generates embeddings for all sections
- Overwrites all prev_section_id with embedding-based matching
- POST /api/daily-reports/backfill-embeddings endpoint
```

---

## Task 6: 验证 (Tasks 10.1, 10.2)

**Step 1: Backend 编译 + 静态检查**

```bash
cd /mnt/d/project/Syntopica/backend-go && go build ./... && go vet ./...
```

**Step 2: 单元测试**

```bash
cd /mnt/d/project/Syntopica/backend-go && go test ./internal/domain/daily_report/...
```

**Step 3: Lint**

```bash
cd /mnt/d/project/Syntopica/backend-go && golangci-lint run ./internal/domain/daily_report/...
```

---

## 实现顺序总结

| Task | 内容 | 涉及文件 | 依赖 |
|------|------|---------|------|
| 1 | DB 迁移 embedding 列 | postgres_migrations.go | 无 |
| 2 | GORM 模型加字段 | models.go | Task 1 |
| 3 | 批量 embedding 生成 | generator.go | Task 2 |
| 4 | pgvector 匹配 + 清理旧代码 | repository.go, generator.go | Task 2, 3 |
| 5 | 回填 + API | repository.go, handler.go | Task 4 |
| 6 | 验证 | - | Task 5 |

Tasks 1-4 串行依赖，Task 5 依赖 Task 4，Task 6 依赖 Task 5。
