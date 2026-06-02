# SaveReport Nullify Bug 修复计划

> **REQUIRED SUB-SKILL:** Use the executing-plans skill to implement this plan task-by-task.

**Goal:** 修复 SaveReport() 批量 upsert 场景下 nullify 逻辑导致 section status 与 prev_section_id 不一致的 bug，并清理现有脏数据。

**Architecture:** 修改 `SaveReport()` 的 nullify 逻辑——nullify `prev_section_id` 时同步重置 `status` 为 `emerging`。对 thread 的 `prev_thread_id` 做相同处理。然后通过回填 API 清理历史脏数据。

**Tech Stack:** Go, GORM, PostgreSQL

---

## Task 1: 修复 nullify 时同步重置 status

**Files:**
- Modify: `backend-go/internal/domain/daily_report/repository.go:73-83` (SaveReport nullify 块)

**Step 1: 修改 nullify downstream prev_thread_id**

将 `Update("prev_thread_id", nil)` 改为同时重置 status：

```go
// Nullify downstream prev_thread_id references before deleting old threads
if err := tx.Model(&DailyReportThread{}).
    Where("prev_thread_id IN (SELECT id FROM daily_report_threads WHERE report_id = ?)", existing.ID).
    Updates(map[string]interface{}{
        "prev_thread_id": nil,
        "status":         "emerging",
    }).Error; err != nil {
    return fmt.Errorf("nullify downstream prev_thread_id: %w", err)
}
```

**Step 2: 修改 nullify downstream prev_section_id**

将 `Update("prev_section_id", nil)` 改为同时重置 status：

```go
// Nullify downstream prev_section_id references before deleting old sections
if err := tx.Model(&DailyReportSection{}).
    Where("prev_section_id IN (SELECT id FROM daily_report_sections WHERE report_id = ?)", existing.ID).
    Updates(map[string]interface{}{
        "prev_section_id": nil,
        "status":          "emerging",
    }).Error; err != nil {
    return fmt.Errorf("nullify downstream prev_section_id: %w", err)
}
```

**Step 3: 验证**

Run: `cd backend-go && go build ./... && go vet ./...`

**Step 4: Commit**

```bash
git add backend-go/internal/domain/daily_report/repository.go
git commit -m "fix(daily-report): reset status to emerging when nullifying prev_section_id/prev_thread_id in SaveReport upsert"
```

## Task 2: 清理现有脏数据（一次性 SQL + 回填）

**Files:** 无代码修改，纯数据操作

**Step 1: 清理悬空 prev_section_id 引用**

对 `prev_section_id` 指向不存在 section 的记录，清空并重置 status：

```sql
UPDATE daily_report_sections
SET prev_section_id = NULL, status = 'emerging'
WHERE prev_section_id IS NOT NULL
  AND prev_section_id NOT IN (SELECT id FROM daily_report_sections);
```

**Step 2: 清理 status 与 prev_section_id 矛盾的记录**

```sql
-- status=continuing 但 prev_section_id=NULL → 重置为 emerging
UPDATE daily_report_sections
SET status = 'emerging'
WHERE status = 'continuing' AND prev_section_id IS NULL;
```

**Step 3: 调用回填 API 重新生成 embedding 和匹配**

```bash
curl -X POST http://localhost:5000/api/daily-reports/backfill-embeddings
```

这会：
1. 为所有没有 embedding 的 section 生成 embedding
2. 用 pgvector 重新匹配所有 section 的 prev_section_id（覆盖旧值）

**Step 4: 验证**

```sql
-- 应该不再有 status=continuing 但 prev_section_id=NULL 的 section
SELECT COUNT(*) FROM daily_report_sections
WHERE status = 'continuing' AND prev_section_id IS NULL;
-- 预期: 0

-- 应该不再有 prev_section_id 指向不存在 section 的记录
SELECT COUNT(*) FROM daily_report_sections
WHERE prev_section_id IS NOT NULL
  AND prev_section_id NOT IN (SELECT id FROM daily_report_sections);
-- 预期: 0

-- 板块 3639 的数据分布应该合理
SELECT bdr.period_date,
       COUNT(*) AS sections,
       COUNT(s.embedding) AS with_embedding,
       COUNT(s.prev_section_id) AS with_prev,
       SUM(CASE WHEN s.status='continuing' THEN 1 ELSE 0 END) AS continuing,
       SUM(CASE WHEN s.status='emerging' THEN 1 ELSE 0 END) AS emerging
FROM daily_report_sections s
JOIN board_daily_reports bdr ON bdr.id = s.report_id
WHERE bdr.semantic_board_id = 3639
GROUP BY bdr.period_date
ORDER BY bdr.period_date;
```

## Task 3: 更新 tasks.md 标记完成

**Files:**
- Modify: `openspec/changes/thread-independent-storage-and-lineage/tasks.md`

标记 11.1, 11.3, 11.4, 11.5 为完成。11.2（批量 upsert 根本修复）通过 Task 1 的 nullify+status 重置已解决。
