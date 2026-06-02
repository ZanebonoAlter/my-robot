## Context

The daily report system generates narrative threads per cluster section, stored as rows in `daily_report_threads` with cross-day lineage via `prev_thread_id`. Section-level lineage uses `prev_section_id` on `daily_report_sections`.

### The Problem: Tag-based Matching is Broken

The original `matchPreviousSections()` used `cluster_tag_ids` Jaccard similarity to link sections across days. This approach **completely fails** in practice because:

1. **Tag IDs are ephemeral**: Each day's clustering generates brand-new tag IDs. For example, board 4393's sections have tag sets `[96329, 100448, 100791]` (05-26), `[102475]` (05-28), `[109520]` (05-31) — zero overlap across days.
2. **Tags get deleted**: Older tags (96329, 100448, etc.) no longer exist in `topic_tags`, so even retrospective matching is impossible.
3. **Thread tag_ids are mostly empty**: Most threads have `tag_ids=[]`, making thread-level tag matching useless too.

Despite visually obvious continuity ("山西沁源留神峪煤矿瓦斯爆炸事故" on 05-26 → "国务院留神峪煤矿事故调查组公告" on 05-31), the system cannot detect any linkage.

### The Solution: Embedding-based Semantic Matching

Using pgvector on `daily_report_sections.embedding` (维度由运行时 `embedding_config` 配置决定，不硬编码):

- Section embedding text: `cluster_label` (short, semantically dense)
- Matching: `embedding <=> $current_embedding` cosine distance, threshold < 0.3
- Scope: ALL sections in the same board (no time restriction)
- Executed: DB-side in `SaveReport()` transaction
- Backfill: Existing 315 sections get embeddings via batch job

Verified on production data: distance < 0.3 correctly links "朗维尤市日资企业工厂爆炸" to "留神峪煤矿瓦斯爆炸" variants, while keeping unrelated events separate.

## Goals / Non-Goals

**Goals:**
- Give every thread a unique, persistent database identity via `daily_report_threads` table
- Populate `prev_thread_id` during generation so threads form a linked list across days
- **Replace tag Jaccard matching with embedding-based semantic matching for section lineage**
- **Add `embedding` column to `daily_report_sections` (维度由模型输出决定) and generate embeddings during report generation**
- **Backfill embeddings for all existing sections**
- Provide API endpoints for thread lineage chain retrieval and board-level thread timeline
- Build frontend views: (A) thread lineage timeline within newspaper modal, (B) board-level Gantt-chart thread browser
- Migrate existing JSON thread data to the new table without data loss

**Non-Goals:**
- Redesigning thread status values (emerging/continuing/splitting/merging/ending remain unchanged)
- Thread-level embedding matching (will use title+summary embedding in future change; tag overlap remains for threads)
- Adding thread editing/merging UI
- Building real-time thread updates via WebSocket
- Changing how sections or clusters are generated

## Decisions

### 1. Independent table vs. adding columns to sections

**Decision**: Create `daily_report_threads` as a separate table with FK to `daily_report_sections`.

**Rationale**: Threads are the primary queryable unit for lineage tracing. Storing them as rows enables:
- `prev_thread_id` self-reference (impossible in JSON arrays)
- Efficient queries: "get all threads for board X across all days" without parsing JSON
- Index on `prev_thread_id` for chain traversal

**Alternative considered**: Add a generated column or `jsonb_path_query` GIN index on the existing `threads` JSONB. Rejected because self-referencing lineage within JSON is awkward, queries are slow, and the current `PrevThreadID` field is never populated anyway.

### 2. Migration strategy: extract JSON → new table, then drop column

**Decision**: Two-phase migration:
1. Create `daily_report_threads` table
2. Extract existing JSON threads: `INSERT INTO daily_report_threads (...) SELECT ... FROM daily_report_sections WHERE threads IS NOT NULL` — `prev_thread_id` left null for historical data
3. Drop `daily_report_sections.threads` column in a separate migration after verification

**Rationale**: Gradual migration with rollback window. Historical `prev_thread_id` cannot be retroactively determined (the matching function never ran with IDs), so leaving it null is correct.

**JSON field name mapping**: Migration SQL must map the current JSON field names to the new table columns: `related_tag_ids` → `tag_ids`. The `parent_thread_id` and `related_article_ids` fields are not carried over (prev_thread_id=NULL for historical data, article associations not stored in the new table).

### 3. Section lineage: embedding-based semantic matching (replacing tag Jaccard)

**Decision**: Replace `matchPreviousSections()` tag Jaccard logic with pgvector cosine distance matching on `daily_report_sections.embedding`. Embedding text = `cluster_label`. Matching scope = all sections in the same board (not just previous report). Threshold = cosine distance < 0.3.

**Rationale**: Tag-based matching is provably broken — tag IDs have zero cross-day overlap because daily clustering generates fresh tags, and older tags get deleted. Embedding matching directly captures semantic continuity regardless of tag identity. Verified on production data: distance < 0.3 correctly links same-event sections across days.

**Implementation**:
1. Add `embedding vector` column to `daily_report_sections` (migration 声明 `type:vector`，不硬编码维度)。GORM 模型声明 `gorm:"type:vector"`，与 `TopicTagEmbedding.EmbeddingVec` 保持一致
2. After LLM section generation, batch-embed all `cluster_label` texts in one API call. **Skip sections with empty `cluster_label`** — 不生成 embedding，`prev_section_id` 保持 NULL
3. Embedding 维度由模型输出决定（`EmbeddingResult.Dimensions`），`ensureVectorDimension()` 仅确保 DB 列类型与模型输出匹配（在模型切换时 ALTER 列）
4. Store sections with embeddings in `SaveReport()`
5. **Upsert 匹配顺序（关键）**：在 `SaveReport()` 事务内，**先匹配再删除旧数据**：
   - (a) 对每个新 section（带 embedding），用 pgvector `<=>` 查询同 board 内**所有现有 section**（此时旧 section 尚未删除）的最近邻
   - (b) 如果余弦距离 < 0.3，设置 `prev_section_id` 和 `status='continuing'`
   - (c) 然后才删除旧 sections + threads，插入新 sections + threads
   - 这样避免 upsert 场景下丢失与前一天的连续性
6. `findPreviousSections()` and `matchPreviousSections()` in Go are replaced — Go 侧不再做 section 匹配，完全由 DB 侧 pgvector 完成

**Alternative considered**: Use existing `topic_tag_embeddings` to compute section similarity indirectly. Rejected because (a) old tags are deleted so their embeddings are gone, (b) new tags may not have embeddings yet, (c) tag embedding ≠ section semantic.

### 3b. Thread lineage: keep tag-based matching (unchanged)

**Decision**: Thread-level matching continues to use tag overlap via `matchPreviousThreads()`. Thread embedding is deferred to a future change.

**Rationale**: Threads average 326 per report (max 4089). Embedding all threads would significantly increase API cost and storage. Section-level matching already provides the primary Gantt chart data. Thread embedding can be added incrementally later using `title + " " + summary` as embedding text.

### 4. Thread chain retrieval: recursive query vs. iterative API

**Decision**: Use a PostgreSQL recursive CTE (`WITH RECURSIVE`) in a new repository function `GetThreadLineage(threadID)` to fetch the full chain (all ancestors + descendants) in one query.

**Rationale**: Thread chains are short (typically 2-7 days). A single recursive query is simpler than N+1 API calls. The CTE walks both directions: from the given thread backward via `prev_thread_id` to the root, then forward to find all descendants.

### 5. Board thread timeline API: single endpoint returning all threads with prev_thread_id

**Decision**: New endpoint `GET /api/semantic-boards/:id/thread-timeline?days=30` returns all `daily_report_threads` for that board within the date range, including `prev_thread_id` and `period_date` (joined from the report). The frontend assembles the Gantt chart locally.

**Rationale**: Simpler than a paginated or graph-based API. Board thread counts are modest (typically 20-80 threads across 30 days). The frontend can build lineage chains client-side from the flat list using `prev_thread_id`.

### 6. Frontend architecture: two new components, no routing changes

**Decision**: 
- **View A** (`ThreadLineagePanel.vue`): Side panel within the existing newspaper modal. Triggered by clicking a thread. Fetches lineage via `GET /api/daily-reports/threads/:id/lineage`. Renders vertical timeline.
- **View B** (`BoardThreadBrowser.vue`): New component accessible via a button/link in the `BoardDailyReportTimeline.vue` panel (or a new tab). No new Nuxt route — uses component toggle. Fetches data via `GET /api/semantic-boards/:id/thread-timeline`.

**Rationale**: Keeps the daily report feature self-contained. No route changes needed.

## 实现后发现的 Bug（2026-06-02 探索验证）

对板块 3639（人工智能与大模型技术）数据库实际数据分析，发现以下问题：

### 数据现状

```
日期         section数  有embedding  有prev_section_id  status分布
─────────────────────────────────────────────────────────────────────
05-26   25         ✗ (0/25)     全 NULL            全 emerging
05-27    4         ✗ (0/4)      全 NULL            全 emerging
05-28   47         ✗ (0/47)     全 NULL            全 emerging
05-29   12         ✗ (0/12)     全 NULL            全 emerging
05-30   18         ✗ (0/18)     全 NULL            全 emerging
05-31   20         ✓ (20/20)   全 NULL            18 continuing + 2 emerging ← 矛盾！
06-01   13         ✓ (13/13)   13/13 有值         全 continuing
```

06-01 的 prev_section_id 引用情况：
- 3 个指向 05-31 存活的 section (755, 756, 765) ✓
- 10 个指向已不存在的 section id (701-711) ✗ — 这些是 05-31 被 upsert 删掉的旧 section

### Bug 1：nullify 只清 prev_section_id，不重置 status

**位置**：`repository.go` `SaveReport()` 中 nullify downstream 逻辑

**根因**：批量 upsert 场景下，交叉 nullify 清空了 `prev_section_id` 但没有同步重置 `status`。

```
时序（06-02 批量重新生成）：
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
1. 05-31 upsert (report 33, 06-02 11:11)
   ├── MatchSectionsByEmbedding: 匹配到 06-01 旧 section
   │   → prev_section_id = 指向 06-01 section
   │   → status = "continuing" ✓
   ├── Nullify downstream: 清理引用 05-31 旧 section 的外部引用
   ├── 删除 05-31 旧 section (695-714)
   └── 插入 05-31 新 section (755-774)
       此时 prev_section_id 指向 06-01 旧 section

2. 06-01 upsert (report 43, 06-02 11:13)
   ├── Nullify downstream prev_section_id
   │   → 05-31 新 section (755-774) 的 prev_section_id
   │     指向了 06-01 旧 section → 被清为 NULL ✓
   │   → 但 status="continuing" 没有被重置！✗ ← BUG
   ├── 删除 06-01 旧 section
   ├── MatchSectionsByEmbedding: 匹配到 05-31 新 section
   └── 插入 06-01 新 section (775-787)
```

**后果**：05-31 section `status=continuing` 但 `prev_section_id=NULL`（逻辑矛盾）。

**修复**：nullify downstream prev_section_id 时，同时将对应 section 的 status 重置为 `emerging`。

### Bug 2：nullify 的 scope 不够精确

**位置**：`repository.go` `SaveReport()` nullify downstream prev_section_id

**根因**：nullify 只按 report_id 过滤被引用的旧 section，没有考虑同事务内新 section 可能已经获得了合法的 prev_section_id 指向。当 A 和 B 同批 upsert 时，A 匹配到 B 的旧 section → B upsert 清理时把 A 新 section 的合法引用也 nullify 了。

**后果**：批量 upsert 时 section 链断裂。06-01 的 section 引用了 701-711（05-31 旧 section），这些 section 已被删但 prev_section_id 未被清理（因为 06-01 upsert 先于 05-31 nullify 执行，或者 nullify 时 06-01 的 section 还未指向它们）。

**修复方案**：nullify 时增加条件——只 nullify `prev_section_id` 指向被删 section **且**该 section 确实属于当前 report 的引用。或者更根本地，在 nullify 后对被影响的 section 重新跑一次 embedding 匹配。

### Bug 3：回填未执行

**位置**：任务 10.3/10.4 未完成

**根因**：`BackfillSectionEmbeddings()` 函数已实现但从未被调用。所有历史 section（05-26 到 05-30）没有 embedding，无法参与匹配。

**后果**：139 个 section 中只有 33 个有 embedding（23.7%），历史 section 全部无法匹配。

### Bug 4：prev_section_id 无 FK 约束

**根因**：`prev_section_id` 列没有外键约束，可以指向已删除的 section。

**后果**：06-01 的 10 个 section 的 `prev_section_id` 指向不存在的 section id (701-711)。前端 `buildChains` 通过 `nodeMap.has()` 容忍了这种情况，但数据不干净。

**修复方案**：在 nullify 逻辑正确后（Bug 1+2 修复后），添加 FK 约束 `prev_section_id REFERENCES daily_report_sections(id) ON DELETE SET NULL`。或者保持无 FK 但确保 nullify 逻辑完整。

## Risks / Trade-offs

- **[Risk] Migration data loss if threads JSON has unexpected shapes** → Mitigation: Migration uses `jsonb_array_elements` with error handling; column drop is in a separate migration that can be deferred. JSON field names (`related_tag_ids` not `tag_ids`) are correctly mapped in migration SQL.
- **[Risk] Upsert invalidates downstream prev_thread_id references** → Mitigation: `SaveReport` sets `prev_thread_id=NULL` on any threads that reference the report's threads before deleting them. Report regeneration implies content has changed, so broken lineage is expected.
- **[Risk] Upsert 时旧 section 被删导致 section 链断裂** → Mitigation: `SaveReport()` 事务内**先执行 pgvector 匹配（此时旧 section 还在），再删除旧 section 并插入新 section**。确保新 section 能匹配到被替换的旧 section。
- **[Risk] `cluster_label` 为空时生成无效 embedding** → Mitigation: 跳过 `cluster_label` 为空的 section，不调用 Embedding API，`embedding` 和 `prev_section_id` 均保持 NULL。
- **[Risk] 回填覆盖已有的错误 tag Jaccard 匹配** → 回填 SHALL 用 embedding 结果覆盖**所有** `prev_section_id`（不仅限于 NULL 的），因为现有 tag Jaccard 匹配结果不可靠。
- **[Risk] Section embedding threshold may need tuning** → Mitigation: Verified threshold 0.3 on production data (same-event sections at 0.22-0.29, unrelated events at >0.4). Can be adjusted via constant. Historical sections with no embedding will be backfilled.
- **[Risk] Embedding API cost increase** → Mitigation: ~9 sections per report average, minimal cost. Batch API reduces HTTP overhead. Max case (47 sections) still reasonable.
- **[Risk] Thread matching quality — tag overlap may produce false lineage links** → Mitigation: Thread embedding is planned for a future change. Current tag-based matching is a known limitation.
- **[Risk] Board thread timeline query performance on boards with long history** → Mitigation: `days` parameter capped at 30. Indexes on `report_id`, `prev_thread_id`, and `board_daily_reports.semantic_board_id` keep queries fast.
- **[Trade-off] Historical sections will have `prev_section_id` populated after backfill** → The backfill job will embed all existing 315 sections and run the same pgvector matching, establishing lineage chains for historical data.
- **[Trade-off] Historical threads have no `prev_thread_id`** → Accepted. Thread embedding matching is a separate future change. The UI will simply show these as chain-starting nodes.
- **[Trade-off] Frontend Gantt chart is a custom component, not a charting library** → Accepted for now. Thread counts per board are small enough that a CSS grid/Flexbox solution works. Can upgrade to a library later if needed.
