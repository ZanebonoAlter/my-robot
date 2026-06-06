# Narrative Section Relations 实施计划

> **SUPERSEDED:** 增量贪心匹配算法（`MatchAndSaveRelations`, `shouldWriteRelation`, `competitiveFilter`, `hasContinuationInIntermediateDays`）已被匈牙利二分图匹配（`RebuildBoardRelations`）取代。详见 `docs/plans/2026-06-06-bipartite-relation-matching.md`。

> **REQUIRED SUB-SKILL:** Use the executing-plans skill to implement this plan task-by-task.

**Goal:** 将 Section 间的单链 `prev_section_id` 关系升级为多对多关系表，支持 split/merge；简化 Thread 移除 status 和 prev_thread_id。

**Architecture:** 新增 `daily_report_section_relations` 表存储多对多关系，Section status 改为动态推导（timeline/lifecycle API），Thread 简化为纯事件条目。前端 Gantt 图改为 DAG 渲染，报纸视图移除 section status 徽章和 thread lineage 入口。

**Tech Stack:** Go (Gin/GORM/pgvector), Vue 3 (Nuxt 4), TypeScript, Tailwind CSS v4

---

## 分组策略

本变更分为 **2 条并行工作线**，互不冲突（后端 / 前端分开目录）：

- **Agent A（后端）**: Task 1 → 2 → 3 → 4，顺序执行
- **Agent B（前端）**: Task 5 → 6，顺序执行

两条线完成后进入 Task 7 验证。

---

## Agent A: 后端

### Task 1: 数据库迁移 + 模型变更

**Files:**
- Modify: `backend-go/internal/domain/daily_report/models.go`
- Modify: `backend-go/internal/domain/daily_report/register_models.go`
- Modify: `backend-go/internal/platform/database/postgres_migrations.go`

**Step 1: 新增 `SectionRelation` GORM 模型**

在 `models.go` 中新增：

```go
// SectionRelation represents a many-to-many relation between sections across days.
type SectionRelation struct {
	ID           uint    `gorm:"primarykey" json:"id"`
	FromSectionID uint   `gorm:"not null;index:idx_section_relations_from" json:"from_section_id"`
	ToSectionID   uint   `gorm:"not null;index:idx_section_relations_to" json:"to_section_id"`
	Distance      float64 `gorm:"not null" json:"distance"`
	CreatedAt     time.Time `json:"created_at"`
}

func (SectionRelation) TableName() string {
	return "daily_report_section_relations"
}
```

**Step 2: 修改现有模型**

在 `DailyReportSection` 中：
- 删除 `Status` 字段（`gorm:"size:20;default:emerging" json:"status"`）
- 删除 `PrevSectionID` 字段（`*uint`）

在 `DailyReportThread` 中：
- 删除 `Status` 字段（`gorm:"size:20;default:emerging" json:"status"`）
- 删除 `PrevThreadID` 字段（`*uint`）

在 `Thread`（LLM 输出结构体）中：
- 删除 `Status` 字段
- 删除 `PrevThreadID` 字段

**Step 3: 注册新模型到 AutoMigrate**

在 `register_models.go` 中将 `&SectionRelation{}` 添加到 `database.RegisterModels()` 调用。

**Step 4: 编写版本化迁移**

在 `postgres_migrations.go` 中新增一个 migration（版本号 `20260603_0001`），包含：

```go
{
    Version:     "20260603_0001",
    Description: "Add section relations table, migrate prev_section_id data, drop legacy columns.",
    Up: func(db *gorm.DB) error {
        // 1. Create daily_report_section_relations table (AutoMigrate already handles this,
        //    but we need the UNIQUE constraint)
        if err := db.Exec(`
            CREATE TABLE IF NOT EXISTS daily_report_section_relations (
                id SERIAL PRIMARY KEY,
                from_section_id INTEGER NOT NULL REFERENCES daily_report_sections(id) ON DELETE CASCADE,
                to_section_id INTEGER NOT NULL REFERENCES daily_report_sections(id) ON DELETE CASCADE,
                distance DOUBLE PRECISION NOT NULL,
                created_at TIMESTAMP DEFAULT NOW()
            )
        `).Error; err != nil {
            return err
        }

        // 2. Migrate existing prev_section_id data into relations
        if err := db.Exec(`
            INSERT INTO daily_report_section_relations (from_section_id, to_section_id, distance)
            SELECT prev_section_id, id, 0.3
            FROM daily_report_sections
            WHERE prev_section_id IS NOT NULL
            ON CONFLICT DO NOTHING
        `).Error; err != nil {
            // Non-fatal: may conflict if data already migrated
            logging.Warnf("section relation data migration: %v", err)
        }

        // 3. Add unique constraint
        if err := db.Exec(`
            ALTER TABLE daily_report_section_relations
            ADD CONSTRAINT daily_report_section_relations_from_to_unique
            UNIQUE (from_section_id, to_section_id)
        `).Error; err != nil {
            // May already exist
            logging.Warnf("unique constraint: %v", err)
        }

        // 4. Create indexes
        for _, idx := range []string{
            "CREATE INDEX IF NOT EXISTS idx_section_relations_from ON daily_report_section_relations(from_section_id)",
            "CREATE INDEX IF NOT EXISTS idx_section_relations_to ON daily_report_section_relations(to_section_id)",
        } {
            if err := db.Exec(idx).Error; err != nil {
                return err
            }
        }

        // 5. Drop prev_section_id column from daily_report_sections
        if err := db.Exec(`ALTER TABLE daily_report_sections DROP COLUMN IF EXISTS prev_section_id`).Error; err != nil {
            logging.Warnf("drop prev_section_id: %v", err)
        }

        // 6. Drop status column from daily_report_sections
        if err := db.Exec(`ALTER TABLE daily_report_sections DROP COLUMN IF EXISTS status`).Error; err != nil {
            logging.Warnf("drop section status: %v", err)
        }

        // 7. Drop status and prev_thread_id from daily_report_threads
        if err := db.Exec(`ALTER TABLE daily_report_threads DROP COLUMN IF EXISTS status`).Error; err != nil {
            logging.Warnf("drop thread status: %v", err)
        }
        if err := db.Exec(`DROP INDEX IF EXISTS idx_daily_report_threads_prev_thread_id`).Error; err != nil {
            logging.Warnf("drop prev_thread_id index: %v", err)
        }
        if err := db.Exec(`ALTER TABLE daily_report_threads DROP COLUMN IF EXISTS prev_thread_id`).Error; err != nil {
            logging.Warnf("drop prev_thread_id: %v", err)
        }

        return nil
    },
},
```

**Step 5: 提交**

```bash
cd backend-go && go build ./...
```

---

### Task 2: 存储层改造（repository.go）

**Files:**
- Modify: `backend-go/internal/domain/daily_report/repository.go`

**Step 1: 新增 `SectionRelationResult` 类型和 `MatchAndSaveRelations` 函数**

替换旧的 `MatchSectionsByEmbedding`。新函数查询同 board 下所有**非当日** section（不再 LIMIT 1），distance < 0.35 的全部写入 relation 表：

```go
// SectionRelationResult represents a relation record for API responses.
type SectionRelationResult struct {
	FromID   uint    `json:"from_id"`
	ToID     uint    `json:"to_id"`
	Distance float64 `json:"distance"`
}

// MatchAndSaveRelations finds all matching sections for new sections via embedding
// and writes relations to daily_report_section_relations.
// Must be called within a transaction, after new sections have been inserted (have IDs).
func MatchAndSaveRelations(tx *gorm.DB, boardID uint, reportDate time.Time, sections []DailyReportSection) error {
	for _, sec := range sections {
		if strings.TrimSpace(sec.Embedding) == "" {
			continue
		}
		// Find ALL matching sections (not just the nearest one), distance < 0.35
		var matches []struct {
			ID       uint
			Distance float64
		}
		err := tx.Raw(`
			SELECT s.id, s.embedding <=> ?::vector AS distance
			FROM daily_report_sections s
			JOIN board_daily_reports r ON r.id = s.report_id
			WHERE r.semantic_board_id = ?
			  AND r.status = 'completed'
			  AND r.period_date::date != ?::date
			  AND s.embedding IS NOT NULL
			  AND s.id != ?
			  AND (s.embedding <=> ?::vector) < 0.35
		`, sec.Embedding, boardID, reportDate.Format("2006-01-02"), sec.ID, sec.Embedding).Scan(&matches).Error
		if err != nil {
			logging.Warnf("MatchAndSaveRelations: query failed for section %d: %v", sec.ID, err)
			continue
		}
		for _, m := range matches {
			relation := SectionRelation{
				FromSectionID: m.ID,
				ToSectionID:   sec.ID,
				Distance:      m.Distance,
			}
			if err := tx.Where("from_section_id = ? AND to_section_id = ?",
				m.ID, sec.ID).FirstOrCreate(&relation).Error; err != nil {
				logging.Warnf("MatchAndSaveRelations: failed to create relation (%d→%d): %v", m.ID, sec.ID, err)
			}
		}
	}
	return nil
}
```

**Step 2: 新增 `DeriveSectionStatus` 函数**

```go
// DeriveSectionStatuses computes dynamic status for each section based on relation topology.
// Returns a map of sectionID -> status string.
func DeriveSectionStatuses(sectionIDs []uint, relations []SectionRelationResult, latestDate time.Time, sectionDateMap map[uint]time.Time) map[uint]string {
	statuses := make(map[uint]string)

	// Build lookup maps
	fromCount := make(map[uint]int)    // sectionID -> count of relations where it's from_section (out-degree)
	toCount := make(map[uint]int)      // sectionID -> count of relations where it's to_section (in-degree)
	hasFrom := make(map[uint]bool)     // sectionID -> has at least one relation pointing TO it (it's a to_section)
	hasTo := make(map[uint]bool)       // sectionID -> has at least one relation FROM it (it's a from_section)

	for _, r := range relations {
		fromCount[r.FromID]++
		toCount[r.ToID]++
		hasFrom[r.ToID] = true
		hasTo[r.FromID] = true
	}

	for _, id := range sectionIDs {
		if !hasFrom[id] {
			// No relations point to this section → emerging
			statuses[id] = "emerging"
		} else if toCount[id] > 1 {
			// Multiple from sections point to this → merge
			statuses[id] = "merge"
		} else if fromCount[id] > 1 {
			// This from-section has multiple to-sections → split (for the TO sections)
			// Actually, "split" applies to the to-sections. Need to check if any from-section of this section has out-degree > 1.
			// Let me reconsider...
			statuses[id] = "split"
		} else {
			statuses[id] = "continuing"
		}

		// Check ending: no to relations and not on latest date
		if !hasTo[id] {
			date, ok := sectionDateMap[id]
			if ok && !isSameDay(date, latestDate) {
				statuses[id] = "ending"
			}
		}
	}

	// Second pass: fix split status
	// A section is "split" if its from-section has out-degree > 1
	for _, id := range sectionIDs {
		if statuses[id] == "split" || statuses[id] == "continuing" {
			// Find from-sections for this section
			for _, r := range relations {
				if r.ToID == id && fromCount[r.FromID] > 1 {
					if statuses[id] != "merge" { // merge has priority
						statuses[id] = "split"
					}
					break
				}
			}
		}
	}

	// Priority: merge > split > continuing > emerging
	// (emerging and ending already set correctly above)

	return statuses
}
```

**Step 3: 改造 `SaveReport`**

在 `SaveReport` 中：
- 删除 `MatchSectionsByEmbedding` 调用
- 删除 upsert 中 nullify downstream `prev_thread_id` 和 `prev_section_id` 的代码
- 新增：删除旧 section 前清理 relation 记录
- 新增：新 section 插入后调用 `MatchAndSaveRelations`

关键改动点：

```go
// 替换旧的 embedding matching 代码块
// 删除:
//   matches := MatchSectionsByEmbedding(tx, report.SemanticBoardID, sections)
//   for i, m := range matches { ... }

// upsert 分支中：
// 删除 nullify downstream prev_thread_id / prev_section_id 的代码块
// 替换为：
if findErr == nil {
    // 清理旧 section 相关的 relation 记录
    var oldSectionIDs []uint
    tx.Model(&DailyReportSection{}).Where("report_id = ?", existing.ID).Pluck("id", &oldSectionIDs)
    if len(oldSectionIDs) > 0 {
        tx.Where("from_section_id IN ? OR to_section_id IN ?", oldSectionIDs, oldSectionIDs).Delete(&SectionRelation{})
    }
    // 删除旧 threads
    tx.Where("report_id = ?", existing.ID).Delete(&DailyReportThread{})
    // 删除旧 sections
    tx.Where("report_id = ?", existing.ID).Delete(&DailyReportSection{})
}

// 新 section 插入后：
// 新增:
if err := MatchAndSaveRelations(tx, report.SemanticBoardID, report.PeriodDate, sections); err != nil {
    logging.Warnf("SaveReport: relation matching failed: %v", err)
    // Non-fatal: continue saving
}
```

**Step 4: 改造 `GetBoardSectionTimeline`**

- 返回 `{sections, relations}` 而不是 `{sections}`
- sections 中动态推导 status

新增返回结构体：

```go
type SectionTimelineResponse struct {
    Sections   []SectionTimelineNode   `json:"sections"`
    Relations  []SectionRelationResult `json:"relations"`
}
```

修改 `SectionTimelineNode`：删除 `PrevSectionID` 字段。

重写 `GetBoardSectionTimeline` 为返回 `(SectionTimelineResponse, error)`：
- 查询 sections（同现有 SQL 但去掉 `prev_section_id` 和 `status` 列）
- 查询同 board 同时间段的 relations
- 调用 `DeriveSectionStatuses` 推导 status
- 组装返回

**Step 5: 改造 `GetSectionLifecycle`**

返回 `(SectionTimelineResponse, error)` 而不是 `([]SectionTimelineNode, error)`：
- 沿 relation 表双向扩展（不再用 prev_section_id 递归 CTE）
- 使用 BFS/递归查询 relation 表

**Step 6: 删除旧函数**

- 删除 `MatchSectionsByEmbedding`
- 删除 `GetThreadLineage`
- 删除 `GetBoardThreadTimeline`
- 删除 `ThreadLineageNode` 类型
- 删除 `SectionEmbeddingMatch` 类型

**Step 7: 改造 `BackfillSectionEmbeddings`**

Phase 2 的匹配结果改为写 relation 表：
- 不再写 `prev_section_id` + `status`
- 改为调用类似的匹配逻辑，写入 `SectionRelation` 记录
- 阈值保持 0.35（与 `MatchAndSaveRelations` 一致）

**Step 8: 验证编译**

```bash
cd backend-go && go build ./...
```

---

### Task 3: 生成流程改造（generator.go）

**Files:**
- Modify: `backend-go/internal/domain/daily_report/generator.go`

**Step 1: 简化 `Thread` 结构体**

已在 Task 1 的 models.go 中删除 `Status` 和 `PrevThreadID`。

**Step 2: 简化 threadsSystemPrompt 和 JSON Schema**

移除 thread prompt 中的 status 要求。新 prompt：

```
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
```

JSON Schema 中移除 `status` 字段。Required 从 `["title", "summary", "status", "tag_ids", "confidence"]` 改为 `["title", "summary", "tag_ids", "confidence"]`。

**Step 3: 简化 `parseThreadsResponse`**

移除 `validStatuses` 检查和 `th.Status` 默认值设置。

**Step 4: 删除 `matchPreviousThreads` 和 `getPrevThreadSummaries` 函数**

在 `GenerateDailyReport` 中：
- 删除 Step 6 中的 `matchPreviousThreads` 调用循环
- 在 Call C×K goroutine 中删除 `getPrevThreadSummaries` 调用
- `GenerateClusterThreads` 的签名移除 `prevThreadSummaries` 参数
- `buildThreadsPrompt` 移除 `prevThreadSummaries` 参数和相关渲染
- 删除 `findPreviousReport` 函数（不再需要 thread 连续性信息，只需要 prev report 用于 highlights）
  - 实际上 `findPreviousReport` 还被用来设置 `PrevReportID`，简化为只返回 report 不返回 threads

**Step 5: 简化 thread 转换**

在 `GenerateDailyReport` 的 thread batch 构建中，移除 `Status` 和 `PrevThreadID` 赋值：

```go
batch = append(batch, DailyReportThread{
    Title:             th.Title,
    Summary:           th.Summary,
    TagIDs:            tagIDsJSON,
    Confidence:        th.Confidence,
    RelatedArticleIDs: articleIDsJSON,
})
```

**Step 6: 升级 prompt version**

将 `const promptVersion = "2.0"` 改为 `"3.0"`。

**Step 7: 验证编译**

```bash
cd backend-go && go build ./...
```

---

### Task 4: API Handler 改造（handler.go）

**Files:**
- Modify: `backend-go/internal/domain/daily_report/handler.go`

**Step 1: 改造 `getBoardSectionTimeline` handler**

返回 `{sections, relations}` 格式：

```go
func getBoardSectionTimeline(c *gin.Context) {
    // ... parse boardID, days (同现有)
    resp, err := GetBoardSectionTimeline(uint(boardID), days)
    // ...
    c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
        "sections":  resp.Sections,
        "relations": resp.Relations,
    }})
}
```

**Step 2: 改造 `getSectionLifecycle` handler**

返回 `{sections, relations}` 格式（不再是 `{chain}`）：

```go
func getSectionLifecycle(c *gin.Context) {
    // ... parse sectionID (同现有)
    resp, err := GetSectionLifecycle(uint(sectionID))
    // ...
    c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
        "sections":  resp.Sections,
        "relations": resp.Relations,
    }})
}
```

**Step 3: `getDailyReportDetail` handler 无需改动**

GORM Preload 自动跟随模型字段变化，Thread 响应已不含被删除的字段。Section 响应同理。

**Step 4: 移除 thread lineage 和 thread timeline**

删除 `getThreadLineage` 和 `getBoardThreadTimeline` handler 函数。

**Step 5: 更新路由注册**

删除两条路由：

```go
// 删除:
// api.GET("/daily-reports/threads/:id/lineage", getThreadLineage)
// api.GET("/semantic-boards/:id/thread-timeline", getBoardThreadTimeline)
```

**Step 6: 验证**

```bash
cd backend-go && go build ./... && golangci-lint run ./internal/domain/daily_report/...
```

---

## Agent B: 前端

### Task 5: API 类型层更新

**Files:**
- Modify: `front/app/api/dailyReports.ts`

**Step 1: 更新类型定义**

```typescript
// 移除 prev_section_id，status 改为可选（仅 timeline/lifecycle 返回）
export interface SectionTimelineNode {
  id: number
  report_id: number
  period_date: string
  cluster_label: string
  status: string  // emerging / continuing / split / merge / ending
  article_count: number
  thread_count: number
  // prev_section_id 已移除
}

// 新增关系类型
export interface SectionRelation {
  from_id: number
  to_id: number
  distance: number
}

// SectionLifecycleNode 同 SectionTimelineNode
export type SectionLifecycleNode = SectionTimelineNode

// DailyReportThread 移除 status 和 prev_thread_id
export interface DailyReportThread {
  id: number
  report_id: number
  section_id: number
  title: string
  summary: string
  tag_ids: number[]
  confidence: number
  related_article_ids: number[]
  created_at: string
}

// DailyReportSection 移除 status 和 prev_section_id
export interface DailyReportSection {
  id: number
  cluster_index: number
  cluster_label: string
  cluster_tag_ids: number[]
  threads: DailyReportThread[]
  article_count: number
  best_tier: number
  avg_score: number
}

// 删除 ThreadLineageNode 类型
```

**Step 2: 更新 API 函数**

```typescript
// getBoardSectionTimeline 返回 sections + relations
async function getBoardSectionTimeline(boardId: number, days?: number): Promise<ApiResponse<{ sections: SectionTimelineNode[], relations: SectionRelation[] }>> {
  const query = days ? `?days=${days}` : ''
  return apiClient.get(`/semantic-boards/${boardId}/section-timeline${query}`)
}

// getSectionLifecycle 返回 sections + relations
async function getSectionLifecycle(sectionId: number): Promise<ApiResponse<{ sections: SectionLifecycleNode[], relations: SectionRelation[] }>> {
  return apiClient.get(`/daily-reports/sections/${sectionId}/lifecycle`)
}

// 删除 getThreadLineage
// 删除 getBoardThreadTimeline
```

**Step 3: 更新 return 对象**

```typescript
return {
  generateDailyReport,
  getBoardDailyReports,
  getDailyReportDetail,
  getBoardSectionTimeline,
  getSectionLifecycle,
  // 已移除: getThreadLineage, getBoardThreadTimeline
}
```

---

### Task 6: 前端组件改造

**Files:**
- Modify: `front/app/features/tags/components/BoardThreadBrowser.vue`
- Modify: `front/app/features/tags/components/BoardDailyReportTimeline.vue`
- Modify: `front/app/features/tags/components/SectionLifecyclePanel.vue`
- Delete: `front/app/features/tags/components/ThreadLineagePanel.vue`

**Step 1: 改造 `BoardThreadBrowser.vue`**

核心变化：
- API 返回新增 `relations` 数组
- `buildChains` 改为基于 relations 的 DAG 构建
- 连线基于 relations 而非 prev_section_id
- 新增 split=橙、merge=紫 颜色

数据加载：
```typescript
const relations = ref<SectionRelation[]>([])

// loadData 中:
sections.value = res.data.sections || []
relations.value = res.data.relations || []
```

`buildChains` 改为基于 relations：
```typescript
function buildChains(flatNodes: SectionTimelineNode[], rels: SectionRelation[]): LineageChain[] {
  // 1. Build nodeMap (id -> node)
  // 2. Build childrenMap using relations: from_id -> [to_id nodes]
  // 3. Build parentMap using relations: to_id -> [from_id nodes]
  // 4. Find roots: nodes that have no incoming relation (not a to_id in any relation)
  //    OR whose from_id is not in nodeMap
  // 5. BFS from roots via childrenMap
  // 6. Handle orphans (no relations at all)
}
```

Status 颜色扩展：
```typescript
const statusColors: Record<string, string> = {
  emerging: 'bg-green-500',
  continuing: 'bg-blue-500',
  split: 'bg-orange-500',
  merge: 'bg-purple-500',
  ending: 'bg-gray-500',
}
const statusLabels: Record<string, string> = {
  emerging: '新兴',
  continuing: '持续',
  split: '分化',
  merge: '合并',
  ending: '结束',
}
```

连线改造：
```typescript
function connectorSegments(chain: LineageChain, rels: SectionRelation[], colWidth: number = 36) {
  // For each pair of consecutive nodes in chain, find the relation
  // Actually better: for each relation where both from_id and to_id are in chain.nodes
  // draw a line from from_id's column to to_id's column
  const chainNodeIds = new Set(chain.nodes.map(n => n.id))
  const segments = []
  for (const r of rels) {
    if (chainNodeIds.has(r.from_id) && chainNodeIds.has(r.to_id)) {
      const fromNode = nodeMap.get(r.from_id)!
      const toNode = nodeMap.get(r.to_id)!
      segments.push({ x1: ..., y1: ..., x2: ..., y2: ... })
    }
  }
  return segments
}
```

**Step 2: 改造 `BoardDailyReportTimeline.vue`**

移除项：
- 删除 `import ThreadLineagePanel` 和所有 `ThreadLineagePanel` 相关代码（变量、函数、模板、样式）
- 删除 `lineageThreadId`、`lineageVisible`、`openThreadLineage`、`closeThreadLineage`
- 在 thread 列表中删除 sitemap 图标按钮 (`mdi:sitemap-outline`)
- 在 section 卡片 header 中删除 status 徽章
  - 删除 `sectionStatusColor`、`sectionStatusLabel` 对象
  - 删除模板中的 `np-section-status` span
  - 删除对应的 CSS class（`.np-section-emerging` 等）

保留的：
- `SectionLifecyclePanel` 保留（点击 section header 打开）
- thread 的 document 图标按钮保留（打开文章列表）

**Step 3: 改造 `SectionLifecyclePanel.vue`**

核心变化：API 返回从 `{chain}` 改为 `{sections, relations}`

```typescript
const sections = ref<SectionLifecycleNode[]>([])
const relations = ref<SectionRelation[]>([])

async function fetchLifecycle() {
  // ...
  const res = await getSectionLifecycle(props.sectionId)
  if (res.success && res.data) {
    sections.value = res.data.sections || []
    relations.value = res.data.relations || []
  }
}
```

Status 颜色扩展（新增 split/merge）：
```typescript
const statusColor: Record<string, string> = {
  emerging: 'bg-emerald-500/20 text-emerald-400 ring-emerald-500/40',
  continuing: 'bg-blue-500/20 text-blue-400 ring-blue-500/40',
  split: 'bg-orange-500/20 text-orange-400 ring-orange-500/40',
  merge: 'bg-purple-500/20 text-purple-400 ring-purple-500/40',
  ending: 'bg-gray-500/20 text-gray-400 ring-gray-500/40',
}
// dotColor 和 statusLabel 同理
```

渲染改造：从线性链改为树状结构（基于 relations 构建父子关系）。

**Step 4: 删除 `ThreadLineagePanel.vue`**

```bash
rm front/app/features/tags/components/ThreadLineagePanel.vue
```

**Step 5: 验证**

```bash
# lint（WSL 可用）
cd front && pnpm lint

# typecheck / build（必须用 Windows cmd）
cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"
cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"
```

---

## Task 7: 集成验证

**Step 1: 后端验证**

```bash
cd backend-go && golangci-lint run ./internal/domain/daily_report/... && go vet ./internal/domain/daily_report/... && go test ./internal/domain/daily_report/... && go build ./...
```

**Step 2: 前端验证**

```bash
cd front && pnpm lint
cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"
cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"
```

**Step 3: 手动冒烟测试**

1. 启动数据库 + 后端 + 前端
2. 确认 migration 运行成功（检查 `daily_report_section_relations` 表存在，`prev_section_id` 列已删除）
3. 触发一次日报生成，确认新 relation 写入
4. 打开前端，验证：
   - 报纸视图 section 卡片无 status 徽章
   - Thread 列表无 sitemap 图标
   - 话题总览 Gantt 图正常渲染
   - SectionLifecyclePanel 正常加载
