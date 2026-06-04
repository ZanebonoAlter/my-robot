# Relation Skip-Day Filter 实现计划

> **REQUIRED SUB-SKILL:** Use the executing-plans skill to implement this plan task-by-task.

**Goal:** 消除跨天跳跃噪声 relation，保留有价值的隔天续上关系，使 status 推导基于真实的叙事流动

**Architecture:** 改写 `MatchAndSaveRelations` 为两层过滤（相邻天 < 0.35 直接写入，跨天需"无中间天延续" + < 0.25）。新增 `BackfillRelations` / `BackfillAllRelations` 用于回刷。`BackfillSectionEmbeddings` Phase 2 改为调用 `BackfillRelations` 复用统一逻辑。中间天延续检查使用内存邻接表。

**Tech Stack:** Go, GORM, PostgreSQL + pgvector, gin

---

## Task 1: 改写 MatchAndSaveRelations — SQL 查询返回匹配 section 的日期 + 内存邻接表

**Files:**
- Modify: `backend-go/internal/domain/daily_report/repository.go:387-427`（`MatchAndSaveRelations` 函数）

**Step 1: 修改 SQL 查询，返回 match_date 字段**

将原始查询的匿名 struct 改为包含 `MatchDate`：

```go
func MatchAndSaveRelations(tx *gorm.DB, boardID uint, reportDate time.Time, sections []DailyReportSection) error {
	// 预加载该 board 的已有 relation 到内存邻接表
	// from_section_id → []to_section_id
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

	// 预加载该 board 下所有已完成报告的日期列表（按天去重）
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

	// 预加载 section → date 映射（用于中间天检查）
	sectionDateMap := make(map[uint]time.Time)
	var sectionDateRows []struct {
		SectionID uint
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
			  AND r.period_date::date != ?
			  AND s.embedding IS NOT NULL
			  AND s.embedding <=> ?::vector < 0.35
			ORDER BY s.embedding <=> ?::vector
		`, sec.Embedding, boardID, reportDate.Format("2006-01-02"), sec.Embedding, sec.Embedding).Scan(&matches).Error
		if err != nil {
			logging.Warnf("MatchAndSaveRelations: query failed for section %d: %v", sec.ID, err)
			continue
		}

		for _, m := range matches {
			if !shouldWriteRelation(m.MatchID, m.MatchDate, sec.ID, reportDate, m.Distance, adjacency, sectionDateMap, dateSet) {
				continue
			}
			if err := tx.Exec(`
				INSERT INTO daily_report_section_relations (from_section_id, to_section_id, distance)
				VALUES (?, ?, ?)
				ON CONFLICT (from_section_id, to_section_id) DO UPDATE SET distance = EXCLUDED.distance
			`, m.MatchID, sec.ID, m.Distance).Error; err != nil {
				logging.Warnf("MatchAndSaveRelations: save relation failed: %v", err)
			} else {
				// 写入成功，更新邻接表
				adjacency[m.MatchID] = append(adjacency[m.MatchID], sec.ID)
				sectionDateMap[sec.ID] = reportDate
			}
		}
	}
	return nil
}
```

**Step 2: 提取过滤逻辑为独立函数 `shouldWriteRelation`**

在 `repository.go` 中 `MatchAndSaveRelations` 之后添加：

```go
// shouldWriteRelation 判断是否应写入一条 relation。
// 相邻天（from 和 to 之间无其他已完成报告天）: distance < 0.35 → 直接写入
// 跨天（中间有已完成报告天）: from_section 在中间天无延续 + distance < 0.25 → 写入
func shouldWriteRelation(
	fromID uint, fromDate time.Time,
	toID uint, toDate time.Time,
	distance float64,
	adjacency map[uint][]uint,
	sectionDateMap map[uint]time.Time,
	completedDateSet map[string]bool,
) bool {
	// 判断 from 和 to 之间是否有中间天（已完成报告的天）
	hasIntermediate := false
	fromStr := fromDate.Format("2006-01-02")
	toStr := toDate.Format("2006-01-02")
	for dStr := range completedDateSet {
		if dStr > fromStr && dStr < toStr {
			hasIntermediate = true
			break
		}
	}

	if !hasIntermediate {
		// 相邻天匹配：distance < 0.35 直接写入
		return distance < 0.35
	}

	// 跨天匹配：检查 from_section 在中间天是否有延续关系
	if hasContinuationInIntermediateDays(fromID, fromDate, toDate, adjacency, sectionDateMap) {
		return false
	}
	return distance < 0.25
}

// hasContinuationInIntermediateDays 检查 fromSection 是否有指向中间天 section 的 relation
func hasContinuationInIntermediateDays(
	fromSectionID uint, fromDate time.Time, toDate time.Time,
	adjacency map[uint][]uint,
	sectionDateMap map[uint]time.Time,
) bool {
	toTargets, ok := adjacency[fromSectionID]
	if !ok {
		return false
	}
	for _, toID := range toTargets {
		if targetDate, exists := sectionDateMap[toID]; exists {
			targetStr := targetDate.Format("2006-01-02")
			fromStr := fromDate.Format("2006-01-02")
			toStr := toDate.Format("2006-01-02")
			if targetStr > fromStr && targetStr < toStr {
				return true
			}
		}
	}
	return false
}
```

**Step 3: 编译验证**

Run: `cd /mnt/d/project/Syntopica/backend-go && go build ./...`
Expected: 编译成功，无错误

**Step 4: Commit**

```bash
git add backend-go/internal/domain/daily_report/repository.go
git commit -m "feat(daily-report): rewrite MatchAndSaveRelations with two-layer skip-day filter

- Adjacent-day matches (no intermediate completed report days): distance < 0.35 → write directly
- Skip-day matches (intermediate days exist): no intermediate continuation + distance < 0.25 → write
- Pre-load adjacency map and section-date map in memory for efficient intermediate-day checks"
```

---

## Task 2: 新增 BackfillRelations 和 BackfillAllRelations

**Files:**
- Modify: `backend-go/internal/domain/daily_report/repository.go`（在 `BackfillSectionEmbeddings` 之后添加新函数）

**Step 1: 实现 BackfillRelations(boardID)**

```go
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
		ID        uint
		Embedding string
		ReportID  uint
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

	// 4. Process each section in order, building relations incrementally
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
			  AND r.period_date::date != ?
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
```

**Step 2: 实现 BackfillAllRelations()**

```go
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
```

**Step 3: 编译验证**

Run: `cd /mnt/d/project/Syntopica/backend-go && go build ./...`
Expected: 编译成功

**Step 4: Commit**

```bash
git add backend-go/internal/domain/daily_report/repository.go
git commit -m "feat(daily-report): add BackfillRelations and BackfillAllRelations

- BackfillRelations(boardID): delete and rebuild all relations for one board
- BackfillAllRelations(): iterate all boards and rebuild
- Chronological processing ensures intermediate-day checks use already-written relations"
```

---

## Task 3: 重写 BackfillSectionEmbeddings Phase 2

**Files:**
- Modify: `backend-go/internal/domain/daily_report/repository.go:560-659`（`BackfillSectionEmbeddings` 函数）

**Step 1: 重写 Phase 2，替换 LIMIT 1 最近邻为调用 BackfillRelations**

将 Phase 2（从 `// Phase 2:` 注释开始到函数结尾）替换为：

```go
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
```

注意：`BackfillRelations` 使用全局 `database.DB` 而非 `tx`（因为它自己开事务），这在 `BackfillSectionEmbeddings` 的 Phase 2 调用中是安全的，因为 Phase 1 的 embedding 更新已经逐条提交了。

**Step 2: 编译验证**

Run: `cd /mnt/d/project/Syntopica/backend-go && go build ./...`
Expected: 编译成功

**Step 3: Commit**

```bash
git add backend-go/internal/domain/daily_report/repository.go
git commit -m "refactor(daily-report): rewrite BackfillSectionEmbeddings Phase 2

Replace LIMIT 1 nearest-neighbor logic with BackfillRelations call.
Fixes: missing multi-matches, wrong date direction, no skip-day filtering."
```

---

## Task 4: 为新逻辑编写 PostgreSQL 集成测试

**Files:**
- Create: `backend-go/internal/domain/daily_report/match_relations_test.go`

**Step 1: 编写测试辅助函数和核心测试**

```go
package daily_report

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"syntopica-backend/internal/domain/daily_report"
	"syntopica-backend/internal/platform/config"
)

func setupMatchTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	if err := config.LoadConfig("../../../configs"); err != nil {
		t.Skip("config not found, skipping PG integration test")
	}
	dsn := config.AppConfig.Database.DSN
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "connect to postgres")
	t.Cleanup(func() { db.Exec("DROP SCHEMA IF EXISTS test_match_relations CASCADE") })
	return db
}
```

注意：由于测试需要 pgvector 向量距离计算，必须在真实 PG 上运行。使用事务回滚做数据隔离。测试辅助需要：
1. 在事务中创建 board + reports + sections（含 embedding）
2. 调用 `MatchAndSaveRelations` 或 `shouldWriteRelation`
3. 断言 relation 表内容
4. 事务回滚

因为测试涉及 pgvector 的 `<=>` 操作符和向量类型，section 的 embedding 字段必须写入合法的 pgvector 格式字符串。可以生成简单的低维向量用于测试（例如 3 维），但实际数据库表的 embedding 列维度可能已经固定。因此更好的策略是：

**测试 `shouldWriteRelation` 纯逻辑函数**（不依赖数据库） + **测试 `BackfillRelations` 端到端**（依赖真实 PG，标记 `testing.Short()` 跳过）。

**Step 2: 编写 shouldWriteRelation 纯逻辑测试**

```go
func TestShouldWriteRelation_AdjacentDay(t *testing.T) {
	dateSet := map[string]bool{"2026-06-01": true, "2026-06-02": true}
	// 6/1 → 6/2，中间无其他天 → 相邻天
	result := shouldWriteRelation(
		100, parseDate("2026-06-01"),
		200, parseDate("2026-06-02"),
		0.30,
		map[uint][]uint{},
		map[uint]time.Time{100: parseDate("2026-06-01")},
		dateSet,
	)
	require.True(t, result, "adjacent day match < 0.35 should be written")
}

func TestShouldWriteRelation_SkipDay_NoIntermediateContinuation(t *testing.T) {
	dateSet := map[string]bool{"2026-06-01": true, "2026-06-02": true, "2026-06-03": true}
	// 6/1 → 6/3，有 6/2 作为中间天，from section 100 在 6/2 无延续
	result := shouldWriteRelation(
		100, parseDate("2026-06-01"),
		300, parseDate("2026-06-03"),
		0.094,
		map[uint][]uint{}, // section 100 无出边
		map[uint]time.Time{100: parseDate("2026-06-01")},
		dateSet,
	)
	require.True(t, result, "skip-day match with no intermediate continuation and dist < 0.25 should be written")
}

func TestShouldWriteRelation_SkipDay_HasIntermediateContinuation(t *testing.T) {
	dateSet := map[string]bool{"2026-06-01": true, "2026-06-02": true, "2026-06-03": true}
	// section 100 有出边指向 6/2 的 section 200
	adjacency := map[uint][]uint{100: {200}}
	sectionDateMap := map[uint]time.Time{
		100: parseDate("2026-06-01"),
		200: parseDate("2026-06-02"),
	}
	result := shouldWriteRelation(
		100, parseDate("2026-06-01"),
		300, parseDate("2026-06-03"),
		0.213,
		adjacency,
		sectionDateMap,
		dateSet,
	)
	require.False(t, result, "skip-day match with intermediate continuation should be filtered")
}

func TestShouldWriteRelation_SkipDay_DistanceTooHigh(t *testing.T) {
	dateSet := map[string]bool{"2026-06-01": true, "2026-06-02": true, "2026-06-03": true}
	result := shouldWriteRelation(
		100, parseDate("2026-06-01"),
		300, parseDate("2026-06-03"),
		0.27,
		map[uint][]uint{},
		map[uint]time.Time{100: parseDate("2026-06-01")},
		dateSet,
	)
	require.False(t, result, "skip-day match with dist >= 0.25 should be filtered even without continuation")
}

func TestShouldWriteRelation_DiscontinuousDates_TreatedAsAdjacent(t *testing.T) {
	// board 只有 6/1 和 6/3，没有 6/2 的报告
	dateSet := map[string]bool{"2026-06-01": true, "2026-06-03": true}
	result := shouldWriteRelation(
		100, parseDate("2026-06-01"),
		300, parseDate("2026-06-03"),
		0.30,
		map[uint][]uint{},
		map[uint]time.Time{100: parseDate("2026-06-01")},
		dateSet,
	)
	require.True(t, result, "discontinuous dates (no 6/2 report) should be treated as adjacent")
}

func TestShouldWriteRelation_MultipleAdjacentMatches_Split(t *testing.T) {
	dateSet := map[string]bool{"2026-06-01": true, "2026-06-02": true}
	// 两个相邻天旧 section 都匹配
	result1 := shouldWriteRelation(
		80, parseDate("2026-06-01"),
		200, parseDate("2026-06-02"),
		0.20,
		map[uint][]uint{},
		map[uint]time.Time{80: parseDate("2026-06-01")},
		dateSet,
	)
	result2 := shouldWriteRelation(
		85, parseDate("2026-06-01"),
		200, parseDate("2026-06-02"),
		0.30,
		map[uint][]uint{},
		map[uint]time.Time{85: parseDate("2026-06-01")},
		dateSet,
	)
	require.True(t, result1, "first adjacent match should be written")
	require.True(t, result2, "second adjacent match should be written (split)")
}

func parseDate(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}
```

**Step 3: 运行测试**

Run: `cd /mnt/d/project/Syntopica/backend-go && go test ./internal/domain/daily_report -run TestShouldWriteRelation -v`
Expected: 全部 PASS

**Step 4: Commit**

```bash
git add backend-go/internal/domain/daily_report/match_relations_test.go
git commit -m "test(daily-report): add shouldWriteRelation tests for two-layer filter

Cover: adjacent day, skip-day with/without continuation, distance threshold,
discontinuous dates, multiple adjacent matches (split)."
```

---

## Task 5: （可选）新增回刷 API 端点

**Files:**
- Modify: `backend-go/internal/domain/daily_report/handler.go`
- Modify: `backend-go/internal/app/router.go`

**Step 1: 在 handler.go 添加触发函数**

在 `triggerBackfillEmbeddings` 之后添加：

```go
// triggerBackfillRelations handles POST /api/daily-reports/backfill-relations
func triggerBackfillRelations(c *gin.Context) {
	boardIDStr := c.Query("board_id")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	if boardIDStr != "" {
		boardID, err := strconv.ParseUint(boardIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid board_id"})
			return
		}
		go func() {
			rebuilt, err := BackfillRelations(uint(boardID))
			if err != nil {
				logging.Errorf("daily-report: backfill relations for board %d failed: %v", boardID, err)
				return
			}
			logging.Infof("daily-report: backfill relations for board %d complete: %d relations rebuilt", boardID, rebuilt)
		}()
	} else {
		go func() {
			results, err := BackfillAllRelations()
			if err != nil {
				logging.Errorf("daily-report: backfill all relations failed: %v", err)
				return
			}
			for bid, cnt := range results {
				logging.Infof("daily-report: board %d: %d relations rebuilt", bid, cnt)
			}
		}()
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "processing"}})
}
```

注意：需要在 handler.go 的 import 中添加 `"strconv"`（如果还没有）。

**Step 2: 在 router.go 注册路由**

在 `triggerBackfillEmbeddings` 路由附近添加：

```go
dailyReports.POST("/backfill-relations", triggerBackfillRelations)
```

**Step 3: 编译验证**

Run: `cd /mnt/d/project/Syntopica/backend-go && go build ./...`
Expected: 编译成功

**Step 4: Commit**

```bash
git add backend-go/internal/domain/daily_report/handler.go backend-go/internal/app/router.go
git commit -m "feat(daily-report): add POST /api/daily-reports/backfill-relations endpoint

Optional board_id query param: with board_id rebuilds single board,
without rebuilds all boards."
```

---

## Task 6: 编译 + lint + 测试通过

**Step 1: 编译**

Run: `cd /mnt/d/project/Syntopica/backend-go && go build ./...`
Expected: 成功

**Step 2: 受影响包测试**

Run: `cd /mnt/d/project/Syntopica/backend-go && go test ./internal/domain/daily_report/... -v`
Expected: 全部 PASS

**Step 3: Lint**

Run: `cd /mnt/d/project/Syntopica/backend-go && golangci-lint run ./internal/domain/daily_report/...`
Expected: 无错误

**Step 4: Vet**

Run: `cd /mnt/d/project/Syntopica/backend-go && go vet ./internal/domain/daily_report/...`
Expected: 无错误

---

## Summary

| Task | 描述 | 依赖 |
|------|------|------|
| 1 | 改写 `MatchAndSaveRelations` + 提取 `shouldWriteRelation` | 无 |
| 2 | 新增 `BackfillRelations` + `BackfillAllRelations` | Task 1 |
| 3 | 重写 `BackfillSectionEmbeddings` Phase 2 | Task 2 |
| 4 | 编写 `shouldWriteRelation` 纯逻辑测试 | Task 1 |
| 5 | （可选）新增回刷 API 端点 | Task 2 |
| 6 | 编译 + lint + 测试 | Task 1-5 |

**并行机会**: Task 1-2 是顺序的（2 依赖 1 的 `shouldWriteRelation`）。Task 4 可与 Task 2 并行。Task 3 依赖 Task 2。Task 5 依赖 Task 2。Task 6 是最终验证。
