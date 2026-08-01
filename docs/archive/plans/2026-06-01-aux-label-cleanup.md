# Aux Label Cleanup 实现计划

> **REQUIRED SUB-SKILL:** Use the executing-plans skill to implement this plan task-by-task.

**Goal:** 修复 auxiliary label ref_count 不递减的 bug，新增 GC 机制清理无活跃引用的辅助标签。

**Architecture:** 在现有的 `AuxiliaryLabelService` 中新增 `RecountRefs` 和 `GC` 方法；在两个 topic_tag 删除入口（`CleanupOrphanedTags`、`HardMergeTags`）注入 ref_count 重算；新建 `AuxLabelCleanupScheduler` 定时任务；新增 `POST /api/auxiliary-labels/gc` 端点；前端新增调度器元数据和 GC API 调用。

**Tech Stack:** Go (Gin/GORM), PostgreSQL, TypeScript (Vue 3/Nuxt)

---

## Task 1: RecountRefs 方法

**Files:**
- Modify: `backend-go/internal/domain/tagging/auxiliary_label_service.go` (在 `MergeAuxiliaryLabelAlias` 方法之后添加)
- Test: `backend-go/internal/domain/tagging/auxiliary_label_service_test.go`

**Step 1: 在 AuxiliaryLabelService 中新增 RecountRefs 方法**

在 `auxiliary_label_service.go` 的 `RemoveBoardComposition` 方法之后（约 line 265）添加：

```go
// RecountRefs recalculates ref_count for the given auxiliary label IDs by counting
// actual topic_tag_semantic_labels rows. This is self-healing: it corrects any
// accumulated drift in ref_count values.
func (s *AuxiliaryLabelService) RecountRefs(ctx context.Context, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	for _, id := range ids {
		var count int64
		if err := s.db.WithContext(ctx).Model(&models.TopicTagSemanticLabel{}).
			Where("semantic_label_id = ?", id).Count(&count).Error; err != nil {
			return fmt.Errorf("recount refs for semantic_label %d: %w", id, err)
		}
		if err := s.db.WithContext(ctx).Model(&models.SemanticLabel{}).
			Where("id = ?", id).Update("ref_count", int(count)).Error; err != nil {
			return fmt.Errorf("update ref_count for semantic_label %d: %w", id, err)
		}
	}
	return nil
}
```

**Step 2: 编写单元测试**

在 `auxiliary_label_service_test.go` 中添加测试（如果文件不存在则创建）：

```go
func TestRecountRefs(t *testing.T) {
	db := setupTestDB(t)
	service := NewAuxiliaryLabelService(db, nil)
	ctx := context.Background()

	// Create two auxiliary labels
	label1 := models.SemanticLabel{Label: "AI", Slug: "ai", LabelType: "auxiliary", Status: "active", RefCount: 10}
	label2 := models.SemanticLabel{Label: "ML", Slug: "ml", LabelType: "auxiliary", Status: "active", RefCount: 5}
	require.NoError(t, db.Create(&label1).Error)
	require.NoError(t, db.Create(&label2).Error)

	// Create topic tags and links for label1 only
	tt := models.TopicTag{Slug: "test", Label: "Test", Category: "keyword", Status: "active"}
	require.NoError(t, db.Create(&tt).Error)
	require.NoError(t, db.Create(&models.TopicTagSemanticLabel{TopicTagID: tt.ID, SemanticLabelID: label1.ID}).Error)

	// Recount both
	err := service.RecountRefs(ctx, []uint{label1.ID, label2.ID})
	require.NoError(t, err)

	// Verify label1 ref_count = 1, label2 ref_count = 0
	var updated1, updated2 models.SemanticLabel
	require.NoError(t, db.First(&updated1, label1.ID).Error)
	require.NoError(t, db.First(&updated2, label2.ID).Error)
	assert.Equal(t, 1, updated1.RefCount)
	assert.Equal(t, 0, updated2.RefCount)
}

func TestRecountRefs_EmptyIDs(t *testing.T) {
	db := setupTestDB(t)
	service := NewAuxiliaryLabelService(db, nil)
	ctx := context.Background()

	err := service.RecountRefs(ctx, []uint{})
	assert.NoError(t, err)
}
```

**Step 3: 运行测试验证**

```bash
cd backend-go && go test ./internal/domain/tagging/... -run TestRecountRefs -v
```

**Step 4: Commit**

```bash
git add backend-go/internal/domain/tagging/auxiliary_label_service.go backend-go/internal/domain/tagging/auxiliary_label_service_test.go
git commit -m "feat(aux-label): add RecountRefs method to AuxiliaryLabelService"
```

---

## Task 2: CleanupOrphanedTags 注入 ref_count 重算

**Files:**
- Modify: `backend-go/internal/domain/tagging/article_tagger.go:415-438` (`CleanupOrphanedTags` 函数)

**Context:** `CleanupOrphanedTags` 当前直接硬删除 orphan TopicTag，CASCADE 删除 `topic_tag_semantic_labels` 行但没更新 `semantic_labels.ref_count`。需要在删除前收集受影响的 aux label IDs，删除后调用 `RecountRefs`。

**Step 1: 修改 CleanupOrphanedTags**

将 `article_tagger.go` 中的 `CleanupOrphanedTags` 函数替换为：

```go
func CleanupOrphanedTags(tagIDs []uint) {
	if len(tagIDs) == 0 {
		return
	}

	var orphanIDs []uint
	database.DB.Model(&models.TopicTag{}).
		Where("id IN ?", tagIDs).
		Where("id NOT IN (SELECT topic_tag_id FROM article_topic_tags)").
		Pluck("id", &orphanIDs)

	if len(orphanIDs) == 0 {
		return
	}

	// Collect affected aux label IDs before CASCADE deletes them
	var affectedAuxLabelIDs []uint
	database.DB.Model(&models.TopicTagSemanticLabel{}).
		Where("topic_tag_id IN ?", orphanIDs).
		Distinct("semantic_label_id").
		Pluck("semantic_label_id", &affectedAuxLabelIDs)

	if err := database.DB.Where("topic_tag_id IN ?", orphanIDs).Delete(&models.TopicTagEmbedding{}).Error; err != nil {
		logging.Warnf("Failed to delete embeddings for orphaned topic tags: %v", err)
	}
	if err := database.DB.Where("id IN ?", orphanIDs).Delete(&models.TopicTag{}).Error; err != nil {
		logging.Warnf("Failed to delete %d orphaned topic tags: %v", len(orphanIDs), err)
	} else {
		logging.Infof("Cleaned up %d orphaned topic tags", len(orphanIDs))
	}

	// Recount refs for affected auxiliary labels after CASCADE
	if len(affectedAuxLabelIDs) > 0 {
		auxService := NewAuxiliaryLabelService(database.DB, nil)
		if err := auxService.RecountRefs(context.Background(), affectedAuxLabelIDs); err != nil {
			logging.Warnf("Failed to recount refs after orphan cleanup: %v", err)
		}
	}
}
```

注意需要确保 import 中包含 `"context"` 和 `"syntopica-backend/internal/domain/models"`（检查是否已有）。

**Step 2: 编写测试**

在 `article_tagger_test.go` 中添加：

```go
func TestCleanupOrphanedTags_RecountsRefs(t *testing.T) {
	db := setupTestDB(t)

	// Create article + topic_tag + aux label + link
	article := models.Article{Title: "Test", FeedID: 1}
	require.NoError(t, db.Create(&article).Error)

	auxLabel := models.SemanticLabel{Label: "AI", Slug: "ai", LabelType: "auxiliary", Status: "active", RefCount: 1}
	require.NoError(t, db.Create(&auxLabel).Error)

	tag := models.TopicTag{Slug: "test-tag", Label: "Test Tag", Category: "keyword", Status: "active"}
	require.NoError(t, db.Create(&tag).Error)
	require.NoError(t, db.Create(&models.ArticleTopicTag{ArticleID: article.ID, TopicTagID: tag.ID}).Error)
	require.NoError(t, db.Create(&models.TopicTagSemanticLabel{TopicTagID: tag.ID, SemanticLabelID: auxLabel.ID}).Error)

	// Delete the article_topic_tag link to make the tag orphan
	require.NoError(t, db.Where("topic_tag_id = ?", tag.ID).Delete(&models.ArticleTopicTag{}).Error)

	// Run cleanup
	CleanupOrphanedTags([]uint{tag.ID})

	// Verify tag is deleted
	var tagCount int64
	db.Model(&models.TopicTag{}).Where("id = ?", tag.ID).Count(&tagCount)
	assert.Equal(t, int64(0), tagCount)

	// Verify aux label ref_count is now 0
	var updated models.SemanticLabel
	require.NoError(t, db.First(&updated, auxLabel.ID).Error)
	assert.Equal(t, 0, updated.RefCount)
}
```

**Step 3: 运行测试**

```bash
cd backend-go && go test ./internal/domain/tagging/... -run TestCleanupOrphanedTags -v
```

**Step 4: Commit**

```bash
git add backend-go/internal/domain/tagging/article_tagger.go backend-go/internal/domain/tagging/article_tagger_test.go
git commit -m "fix(aux-label): recount ref_count in CleanupOrphanedTags"
```

---

## Task 3: HardMergeTags 注入 ref_count 重算

**Files:**
- Modify: `backend-go/internal/domain/tagging/hard_merge.go:12-78` (`HardMergeTags` 函数)

**Context:** `HardMergeTags` 在事务中硬删除 source TopicTag。需要在 `tx.Delete(&models.TopicTag{}, sourceID)` 之前收集 source tag 关联的 aux label IDs，在删除之后调用 `RecountRefs`。

**Step 1: 修改 HardMergeTags**

在 `hard_merge.go` 的 `tx.Delete(&models.TopicTag{}, sourceID)` 行（line 61）之前添加收集逻辑，之后添加重算逻辑。替换 line 48 到 line 75：

```go
		if err := tx.Where("topic_tag_id = ?", sourceID).Delete(&models.TopicTagEmbedding{}).Error; err != nil {
			return fmt.Errorf("delete source tag embeddings: %w", err)
		}

		// Collect aux label IDs before CASCADE deletes topic_tag_semantic_labels
		var affectedAuxLabelIDs []uint
		if err := tx.Model(&models.TopicTagSemanticLabel{}).
			Where("topic_tag_id = ?", sourceID).
			Distinct("semantic_label_id").
			Pluck("semantic_label_id", &affectedAuxLabelIDs).Error; err != nil {
			return fmt.Errorf("collect affected aux label ids: %w", err)
		}

		// Clean up queue records that reference the source tag before deletion
		// to avoid foreign key constraint violations.
		if err := tx.Where("tag_id = ?", sourceID).Delete(&models.EmbeddingQueue{}).Error; err != nil {
			return fmt.Errorf("delete source tag embedding queue entries: %w", err)
		}
		if err := tx.Where("source_tag_id = ? OR target_tag_id = ?", sourceID, sourceID).Delete(&models.MergeReembeddingQueue{}).Error; err != nil {
			return fmt.Errorf("delete source tag merge re-embedding queue entries: %w", err)
		}

		if err := tx.Delete(&models.TopicTag{}, sourceID).Error; err != nil {
			return fmt.Errorf("delete source tag %d: %w", sourceID, err)
		}

		// Recount refs for affected auxiliary labels after CASCADE
		if len(affectedAuxLabelIDs) > 0 {
			auxService := NewAuxiliaryLabelService(tx, nil)
			if err := auxService.RecountRefs(ctx, affectedAuxLabelIDs); err != nil {
				return fmt.Errorf("recount refs after hard merge: %w", err)
			}
		}

		if err := tx.Model(&models.TopicTag{}).
			Where("id = ?", targetID).
			Update("feed_count", tx.Model(&models.ArticleTopicTag{}).
				Select("COUNT(DISTINCT a.feed_id)").
				Joins("JOIN articles a ON a.id = article_topic_tags.article_id").
				Where("article_topic_tags.topic_tag_id = ?", targetID),
			).Error; err != nil {
			return fmt.Errorf("recalculate target feed_count: %w", err)
		}

		logging.Infof("HardMergeTags: hard-deleted tag %d into %d", sourceID, targetID)
		return nil
```

**重要**：注意 `HardMergeTags` 是包级函数 `func HardMergeTags(db *gorm.DB, sourceID, targetID uint) error`，没有 `ctx` 参数。`RecountRefs` 需要 `context.Context`。需要用 `context.Background()` 或将 ctx 作为参数传入。

检查调用方式：由于函数签名为 `func HardMergeTags(db *gorm.DB, sourceID, targetID uint) error`，在事务内部调用 `RecountRefs` 时使用 `context.Background()`。

实际代码中应该使用 `context.Background()`：

```go
		if len(affectedAuxLabelIDs) > 0 {
			auxService := NewAuxiliaryLabelService(tx, nil)
			if err := auxService.RecountRefs(context.Background(), affectedAuxLabelIDs); err != nil {
				return fmt.Errorf("recount refs after hard merge: %w", err)
			}
		}
```

同时需要在 import 中添加 `"context"`。

**Step 2: 编写测试**

```go
func TestHardMergeTags_RecountsRefs(t *testing.T) {
	db := setupTestDB(t)

	// Create source and target tags
	source := models.TopicTag{Slug: "source", Label: "Source", Category: "keyword", Status: "active"}
	target := models.TopicTag{Slug: "target", Label: "Target", Category: "keyword", Status: "active"}
	require.NoError(t, db.Create(&source).Error)
	require.NoError(t, db.Create(&target).Error)

	// Create aux label linked to source
	auxLabel := models.SemanticLabel{Label: "AI", Slug: "ai", LabelType: "auxiliary", Status: "active", RefCount: 1}
	require.NoError(t, db.Create(&auxLabel).Error)
	require.NoError(t, db.Create(&models.TopicTagSemanticLabel{TopicTagID: source.ID, SemanticLabelID: auxLabel.ID}).Error)

	// Create article linked to source
	article := models.Article{Title: "Test", FeedID: 1}
	require.NoError(t, db.Create(&article).Error)
	require.NoError(t, db.Create(&models.ArticleTopicTag{ArticleID: article.ID, TopicTagID: source.ID}).Error)

	// Execute hard merge
	err := HardMergeTags(db, source.ID, target.ID)
	require.NoError(t, err)

	// Verify source tag deleted
	var sourceCount int64
	db.Model(&models.TopicTag{}).Where("id = ?", source.ID).Count(&sourceCount)
	assert.Equal(t, int64(0), sourceCount)

	// Verify aux label ref_count is now 0
	var updated models.SemanticLabel
	require.NoError(t, db.First(&updated, auxLabel.ID).Error)
	assert.Equal(t, 0, updated.RefCount)
}
```

**Step 3: 运行测试**

```bash
cd backend-go && go test ./internal/domain/tagging/... -run TestHardMergeTags -v
```

**Step 4: Commit**

```bash
git add backend-go/internal/domain/tagging/hard_merge.go backend-go/internal/domain/tagging/hard_merge_test.go
git commit -m "fix(aux-label): recount ref_count in HardMergeTags"
```

---

## Task 4: GC 服务方法

**Files:**
- Modify: `backend-go/internal/domain/tagging/auxiliary_label_service.go` (添加 GC 方法)
- Modify: `backend-go/internal/domain/tagging/types.go` (添加请求/响应类型，如不存在则在 auxiliary_label_service.go 中定义)

**Step 1: 定义请求和响应类型**

在 `auxiliary_label_service.go` 中（imports 之后、`const auxiliaryLabelMergeThreshold` 之前）添加：

```go
type AuxLabelGCMode string

const (
	AuxLabelGCModeDryRun      AuxLabelGCMode = "dry_run"
	AuxLabelGCModeDisable     AuxLabelGCMode = "disable"
	AuxLabelGCModeDelete      AuxLabelGCMode = "delete"
	AuxLabelGCModeRecalculate AuxLabelGCMode = "recalculate"
)

type AuxLabelGCRequest struct {
	Mode      AuxLabelGCMode `json:"mode"`
	GraceDays int            `json:"grace_days"`
}

type AuxLabelGCResult struct {
	EligibleCount  int                       `json:"eligible_count"`
	AffectedCount  int                       `json:"affected_count"`
	CorrectedCount int                       `json:"corrected_count,omitempty"`
	TotalCount     int                       `json:"total_count,omitempty"`
	Preview        []AuxLabelGCPreviewItem   `json:"preview,omitempty"`
}

type AuxLabelGCPreviewItem struct {
	ID        uint   `json:"id"`
	Label     string `json:"label"`
	RefCount  int    `json:"ref_count"`
	CreatedAt string `json:"created_at"`
}
```

**Step 2: 实现 GC 方法**

在 `RecountRefs` 之后添加：

```go
// GC performs garbage collection on auxiliary labels based on the requested mode.
func (s *AuxiliaryLabelService) GC(ctx context.Context, req AuxLabelGCRequest) (*AuxLabelGCResult, error) {
	switch req.Mode {
	case AuxLabelGCModeRecalculate:
		return s.gcRecalculate(ctx)
	case AuxLabelGCModeDryRun, AuxLabelGCModeDisable, AuxLabelGCModeDelete:
		return s.gcCleanup(ctx, req)
	default:
		return nil, fmt.Errorf("invalid mode %q, must be one of: dry_run, disable, delete, recalculate", req.Mode)
	}
}

func (s *AuxiliaryLabelService) gcRecalculate(ctx context.Context) (*AuxLabelGCResult, error) {
	var ids []uint
	if err := s.db.WithContext(ctx).Model(&models.SemanticLabel{}).
		Where("label_type = ?", "auxiliary").
		Pluck("id", &ids).Error; err != nil {
		return nil, fmt.Errorf("query auxiliary labels: %w", err)
	}

	if len(ids) == 0 {
		return &AuxLabelGCResult{TotalCount: 0, CorrectedCount: 0}, nil
	}

	// Collect before-state for diff
	beforeCounts := make(map[uint]int, len(ids))
	type refRow struct {
		ID        uint
		RefCount  int
	}
	var before []refRow
	s.db.WithContext(ctx).Model(&models.SemanticLabel{}).
		Select("id, ref_count").
		Where("id IN ?", ids).
		Find(&before)
	for _, r := range before {
		beforeCounts[r.ID] = r.RefCount
	}

	if err := s.RecountRefs(ctx, ids); err != nil {
		return nil, fmt.Errorf("recount refs: %w", err)
	}

	// Count how many actually changed
	var after []refRow
	s.db.WithContext(ctx).Model(&models.SemanticLabel{}).
		Select("id, ref_count").
		Where("id IN ?", ids).
		Find(&after)
	corrected := 0
	for _, r := range after {
		if beforeCounts[r.ID] != r.RefCount {
			corrected++
		}
	}

	return &AuxLabelGCResult{TotalCount: len(ids), CorrectedCount: corrected}, nil
}

func (s *AuxiliaryLabelService) gcCleanup(ctx context.Context, req AuxLabelGCRequest) (*AuxLabelGCResult, error) {
	if req.GraceDays <= 0 {
		req.GraceDays = 1
	}

	// Find eligible labels: active, unprotected, past grace period, no topic_tag_semantic_labels
	var eligible []models.SemanticLabel
	err := s.db.WithContext(ctx).
		Where("label_type = ? AND status = ? AND protected = false", "auxiliary", "active").
		Where("created_at < NOW() - INTERVAL '? days'", req.GraceDays).
		Where("id NOT IN (SELECT DISTINCT semantic_label_id FROM topic_tag_semantic_labels)").
		Find(&eligible).Error
	if err != nil {
		return nil, fmt.Errorf("query eligible labels: %w", err)
	}

	result := &AuxLabelGCResult{EligibleCount: len(eligible)}

	// Preview (first 20)
	previewLimit := 20
	if len(eligible) < previewLimit {
		previewLimit = len(eligible)
	}
	result.Preview = make([]AuxLabelGCPreviewItem, previewLimit)
	for i := 0; i < previewLimit; i++ {
		result.Preview[i] = AuxLabelGCPreviewItem{
			ID:        eligible[i].ID,
			Label:     eligible[i].Label,
			RefCount:  eligible[i].RefCount,
			CreatedAt: eligible[i].CreatedAt.Format(time.RFC3339),
		}
	}

	if req.Mode == AuxLabelGCModeDryRun || len(eligible) == 0 {
		return result, nil
	}

	ids := make([]uint, len(eligible))
	for i, l := range eligible {
		ids[i] = l.ID
	}

	if req.Mode == AuxLabelGCModeDisable {
		// Delete board_composition rows referencing these labels
		if err := s.db.WithContext(ctx).Where("auxiliary_label_id IN ?", ids).
			Delete(&models.BoardComposition{}).Error; err != nil {
			return nil, fmt.Errorf("delete board_composition: %w", err)
		}
		// Soft-delete: update status to disabled
		if err := s.db.WithContext(ctx).Model(&models.SemanticLabel{}).
			Where("id IN ?", ids).
			Update("status", "disabled").Error; err != nil {
			return nil, fmt.Errorf("disable labels: %w", err)
		}
		result.AffectedCount = len(ids)
	} else { // delete
		if err := s.db.WithContext(ctx).Where("id IN ?", ids).
			Delete(&models.SemanticLabel{}).Error; err != nil {
			return nil, fmt.Errorf("delete labels: %w", err)
		}
		result.AffectedCount = len(ids)
	}

	return result, nil
}
```

注意需要添加 `"time"` 到 import。

**⚠️ 关于 SQL 参数化**：GORM 不支持 `INTERVAL '? days'` 的参数化，需改用 `gorm.Expr`：

```go
		Where("created_at < NOW() - INTERVAL '1 day' * ?", req.GraceDays).
```

或更安全的写法：

```go
	cutoff := time.Now().AddDate(0, 0, -req.GraceDays)
	// ...
		Where("created_at < ?", cutoff).
```

**推荐使用 cutoff 方式**（更清晰且与测试兼容）。

**Step 3: 编写测试**

```go
func TestGC_DryRun(t *testing.T) {
	db := setupTestDB(t)
	service := NewAuxiliaryLabelService(db, nil)
	ctx := context.Background()

	// Create an orphan aux label (no topic_tag link) older than 1 day
	label := models.SemanticLabel{
		Label: "Orphan", Slug: "orphan", LabelType: "auxiliary", Status: "active", RefCount: 0,
		CreatedAt: time.Now().Add(-48 * time.Hour),
	}
	require.NoError(t, db.Create(&label).Error)

	result, err := service.GC(ctx, AuxLabelGCRequest{Mode: AuxLabelGCModeDryRun, GraceDays: 1})
	require.NoError(t, err)
	assert.Equal(t, 1, result.EligibleCount)
	assert.Equal(t, 0, result.AffectedCount)

	// Verify label still active
	var updated models.SemanticLabel
	require.NoError(t, db.First(&updated, label.ID).Error)
	assert.Equal(t, "active", updated.Status)
}

func TestGC_Disable(t *testing.T) {
	db := setupTestDB(t)
	service := NewAuxiliaryLabelService(db, nil)
	ctx := context.Background()

	label := models.SemanticLabel{
		Label: "Orphan", Slug: "orphan", LabelType: "auxiliary", Status: "active", RefCount: 0,
		CreatedAt: time.Now().Add(-48 * time.Hour),
	}
	require.NoError(t, db.Create(&label).Error)

	result, err := service.GC(ctx, AuxLabelGCRequest{Mode: AuxLabelGCModeDisable, GraceDays: 1})
	require.NoError(t, err)
	assert.Equal(t, 1, result.EligibleCount)
	assert.Equal(t, 1, result.AffectedCount)

	var updated models.SemanticLabel
	require.NoError(t, db.First(&updated, label.ID).Error)
	assert.Equal(t, "disabled", updated.Status)
}

func TestGC_SkipsProtected(t *testing.T) {
	db := setupTestDB(t)
	service := NewAuxiliaryLabelService(db, nil)
	ctx := context.Background()

	label := models.SemanticLabel{
		Label: "Protected", Slug: "protected", LabelType: "auxiliary", Status: "active",
		Protected: true, RefCount: 0,
		CreatedAt: time.Now().Add(-48 * time.Hour),
	}
	require.NoError(t, db.Create(&label).Error)

	result, err := service.GC(ctx, AuxLabelGCRequest{Mode: AuxLabelGCModeDryRun, GraceDays: 1})
	require.NoError(t, err)
	assert.Equal(t, 0, result.EligibleCount)
}

func TestGC_SkipsRecentLabels(t *testing.T) {
	db := setupTestDB(t)
	service := NewAuxiliaryLabelService(db, nil)
	ctx := context.Background()

	label := models.SemanticLabel{
		Label: "Recent", Slug: "recent", LabelType: "auxiliary", Status: "active", RefCount: 0,
		CreatedAt: time.Now().Add(-1 * time.Hour), // within grace period
	}
	require.NoError(t, db.Create(&label).Error)

	result, err := service.GC(ctx, AuxLabelGCRequest{Mode: AuxLabelGCModeDryRun, GraceDays: 1})
	require.NoError(t, err)
	assert.Equal(t, 0, result.EligibleCount)
}

func TestGC_SkipsReferencedLabels(t *testing.T) {
	db := setupTestDB(t)
	service := NewAuxiliaryLabelService(db, nil)
	ctx := context.Background()

	label := models.SemanticLabel{
		Label: "Referenced", Slug: "referenced", LabelType: "auxiliary", Status: "active", RefCount: 1,
		CreatedAt: time.Now().Add(-48 * time.Hour),
	}
	require.NoError(t, db.Create(&label).Error)

	tt := models.TopicTag{Slug: "test", Label: "Test", Category: "keyword", Status: "active"}
	require.NoError(t, db.Create(&tt).Error)
	require.NoError(t, db.Create(&models.TopicTagSemanticLabel{TopicTagID: tt.ID, SemanticLabelID: label.ID}).Error)

	result, err := service.GC(ctx, AuxLabelGCRequest{Mode: AuxLabelGCModeDryRun, GraceDays: 1})
	require.NoError(t, err)
	assert.Equal(t, 0, result.EligibleCount)
}

func TestGC_Recalculate(t *testing.T) {
	db := setupTestDB(t)
	service := NewAuxiliaryLabelService(db, nil)
	ctx := context.Background()

	// Create label with incorrect ref_count
	label := models.SemanticLabel{Label: "Stale", Slug: "stale", LabelType: "auxiliary", Status: "active", RefCount: 99}
	require.NoError(t, db.Create(&label).Error)

	result, err := service.GC(ctx, AuxLabelGCRequest{Mode: AuxLabelGCModeRecalculate})
	require.NoError(t, err)
	assert.Equal(t, 1, result.TotalCount)
	assert.Equal(t, 1, result.CorrectedCount)

	var updated models.SemanticLabel
	require.NoError(t, db.First(&updated, label.ID).Error)
	assert.Equal(t, 0, updated.RefCount)
}

func TestGC_InvalidMode(t *testing.T) {
	db := setupTestDB(t)
	service := NewAuxiliaryLabelService(db, nil)
	ctx := context.Background()

	_, err := service.GC(ctx, AuxLabelGCRequest{Mode: "invalid"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid mode")
}
```

**Step 4: 运行测试**

```bash
cd backend-go && go test ./internal/domain/tagging/... -run TestGC_ -v
```

**Step 5: Commit**

```bash
git add backend-go/internal/domain/tagging/auxiliary_label_service.go backend-go/internal/domain/tagging/auxiliary_label_service_test.go
git commit -m "feat(aux-label): add GC method with dry_run/disable/delete/recalculate modes"
```

---

## Task 5: GC API 端点

**Files:**
- Modify: `backend-go/internal/domain/tagging/semantic_board_handler.go` (添加 handler 方法 + 路由注册)

**Context:** 路由注册在 `RegisterSemanticBoardRoutes` 函数中，auxiliary-labels 的路由组在 `auxiliary := rg.Group("/auxiliary-labels")` 中。需要在其中添加 `auxiliary.POST("/gc", handler.gcAuxiliaryLabels)`。

**Step 1: 添加路由**

在 `semantic_board_handler.go` 的 `RegisterSemanticBoardRoutes` 函数中，`auxiliary` 路由组内添加：

```go
	auxiliary := rg.Group("/auxiliary-labels")
	{
		auxiliary.GET("", handler.listAuxiliaryLabels)
		auxiliary.GET("/clusters", handler.clusterAuxiliaryLabels)
		auxiliary.POST("/merge-alias", handler.mergeAuxiliaryAlias)
		auxiliary.POST("/:id/disable", handler.disableAuxiliaryLabel)
		auxiliary.POST("/gc", handler.gcAuxiliaryLabels)  // NEW
	}
```

**Step 2: 添加 handler 方法**

在 `semanticBoardHandler` 上添加方法（放在 `disableAuxiliaryLabel` 方法附近）：

```go
func (h *semanticBoardHandler) gcAuxiliaryLabels(c *gin.Context) {
	var req struct {
		Mode      string `json:"mode" binding:"required"`
		GraceDays int    `json:"grace_days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "mode is required"})
		return
	}

	mode := AuxLabelGCMode(req.Mode)
	if mode != AuxLabelGCModeDryRun && mode != AuxLabelGCModeDisable &&
		mode != AuxLabelGCModeDelete && mode != AuxLabelGCModeRecalculate {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "invalid mode, must be one of: dry_run, disable, delete, recalculate",
		})
		return
	}

	result, err := h.auxiliary.GC(c.Request.Context(), AuxLabelGCRequest{
		Mode:      mode,
		GraceDays: req.GraceDays,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}
```

**Step 3: 运行编译验证**

```bash
cd backend-go && go build ./...
```

**Step 4: Commit**

```bash
git add backend-go/internal/domain/tagging/semantic_board_handler.go
git commit -m "feat(aux-label): add POST /api/auxiliary-labels/gc endpoint"
```

---

## Task 6: AuxLabelCleanup 定时任务

**Files:**
- Create: `backend-go/internal/jobs/aux_label_cleanup.go`
- Modify: `backend-go/internal/app/runtimeinfo/schedulers.go` (添加 interface 变量)
- Modify: `backend-go/internal/jobs/handler.go` (添加 descriptor)
- Modify: `backend-go/internal/app/runtime.go` (Runtime struct + StartRuntime + SetupGracefulShutdown)

**Step 1: 创建 aux_label_cleanup.go**

仿照 `blocked_article_recovery.go` 模式，但使用 `LogCleanupScheduler` 的 startup delay 模式：

```go
package jobs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"syntopica-backend/internal/domain/models"
	"syntopica-backend/internal/domain/tagging"
	"syntopica-backend/internal/platform/database"
	"syntopica-backend/internal/platform/logging"
	"syntopica-backend/internal/platform/tracing"
)

const auxLabelCleanupStartupDelay = 10 * time.Minute

type AuxLabelCleanupScheduler struct {
	checkInterval int
	stopChan      chan bool
	wg            sync.WaitGroup
	mu            sync.Mutex
	running       bool
	isExecuting   bool
	nextRun       *time.Time
	lastRun       *time.Time
	lastError     string
	totalRuns     int
	successRuns   int
	failedRuns    int

	lastDisabledCount int
}

func NewAuxLabelCleanupScheduler(intervalSeconds int) *AuxLabelCleanupScheduler {
	return &AuxLabelCleanupScheduler{
		checkInterval: intervalSeconds,
		stopChan:      make(chan bool),
		running:       false,
	}
}

func (s *AuxLabelCleanupScheduler) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = true
	s.wg.Add(1)
	nextRun := time.Now().Add(auxLabelCleanupStartupDelay)
	s.nextRun = &nextRun
	s.mu.Unlock()

	go func() {
		defer s.wg.Done()

		timer := time.NewTimer(auxLabelCleanupStartupDelay)
		defer timer.Stop()

		select {
		case <-timer.C:
			s.runCleanupCycle()
		case <-s.stopChan:
			logging.Infof("Aux label cleanup scheduler stopped during startup delay")
			return
		}

		ticker := time.NewTicker(time.Duration(s.checkInterval) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.runCleanupCycle()
				s.updateNextRun(time.Now().Add(time.Duration(s.checkInterval) * time.Second))
			case <-s.stopChan:
				logging.Infof("Aux label cleanup scheduler stopped")
				return
			}
		}
	}()

	logging.Infof("Aux label cleanup scheduler started (interval: %d seconds, first run in %v)", s.checkInterval, auxLabelCleanupStartupDelay)
	return nil
}

func (s *AuxLabelCleanupScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.running = false
	close(s.stopChan)
	s.wg.Wait()
	s.stopChan = make(chan bool)
	s.nextRun = nil
}

func (s *AuxLabelCleanupScheduler) runCleanupCycle() {
	tracing.TraceSchedulerTick("aux_label_cleanup", "cron", func(ctx context.Context) {
		s.mu.Lock()
		if s.isExecuting {
			s.mu.Unlock()
			return
		}
		s.isExecuting = true
		now := time.Now()
		s.lastRun = &now
		s.lastError = ""
		s.mu.Unlock()
		defer func() {
			s.mu.Lock()
			s.isExecuting = false
			s.mu.Unlock()
		}()

		logging.Infof("Running aux label cleanup (mode=disable)...")

		service := tagging.NewAuxiliaryLabelService(database.DB, nil)
		result, err := service.GC(ctx, tagging.AuxLabelGCRequest{
			Mode:      tagging.AuxLabelGCModeDisable,
			GraceDays: 1,
		})
		if err != nil {
			s.mu.Lock()
			s.totalRuns++
			s.failedRuns++
			s.lastError = err.Error()
			s.mu.Unlock()
			logging.Errorf("AuxLabelCleanup: GC failed: %v", err)
			return
		}

		s.mu.Lock()
		s.totalRuns++
		s.successRuns++
		s.lastError = ""
		s.lastDisabledCount = result.AffectedCount
		s.mu.Unlock()

		if result.AffectedCount > 0 {
			logging.Infof("Aux label cleanup completed: disabled %d labels", result.AffectedCount)
		} else {
			logging.Infof("Aux label cleanup completed: no labels to clean")
		}
	})
}

func (s *AuxLabelCleanupScheduler) TriggerNow() map[string]interface{} {
	s.mu.Lock()
	if s.isExecuting {
		s.mu.Unlock()
		return map[string]interface{}{
			"accepted":    false,
			"started":     false,
			"reason":      "already_running",
			"message":     "辅助标签清理正在执行中，稍后再试。",
			"status_code": 409,
		}
	}
	s.mu.Unlock()

	logging.Infof("Manual aux label cleanup triggered")
	s.runCleanupCycle()

	s.mu.Lock()
	defer s.mu.Unlock()

	return map[string]interface{}{
		"accepted":            true,
		"started":             true,
		"message":             "Aux label cleanup triggered",
		"last_disabled_count": s.lastDisabledCount,
	}
}

func (s *AuxLabelCleanupScheduler) GetStatus() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	status := "stopped"
	if s.isExecuting {
		status = "running"
	} else if s.running {
		status = "idle"
	}

	return map[string]interface{}{
		"status":                status,
		"check_interval":        s.checkInterval,
		"is_executing":          s.isExecuting,
		"next_run":              formatOptionalTime(s.nextRun),
		"last_execution_time":   formatOptionalTime(s.lastRun),
		"last_error":            s.lastError,
		"total_executions":      s.totalRuns,
		"successful_executions": s.successRuns,
		"failed_executions":     s.failedRuns,
		"last_disabled_count":   s.lastDisabledCount,
	}
}

func (s *AuxLabelCleanupScheduler) ResetStats() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastRun = nil
	s.lastError = ""
	s.totalRuns = 0
	s.successRuns = 0
	s.failedRuns = 0
	s.lastDisabledCount = 0
	return nil
}

func (s *AuxLabelCleanupScheduler) UpdateInterval(interval int) error {
	if interval <= 0 {
		return fmt.Errorf("interval must be positive")
	}
	wasRunning := false
	s.mu.Lock()
	wasRunning = s.running
	s.mu.Unlock()

	if wasRunning {
		s.Stop()
	}

	s.mu.Lock()
	s.checkInterval = interval
	s.mu.Unlock()

	if wasRunning {
		return s.Start()
	}

	s.updateNextRun(time.Now().Add(time.Duration(interval) * time.Second))
	return nil
}

func (s *AuxLabelCleanupScheduler) updateNextRun(next time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextRun = &next
}
```

**Step 2: 注册到 runtimeinfo/schedulers.go**

在文件末尾添加一行：

```go
var AuxLabelCleanupSchedulerInterface interface{}
```

**Step 3: 注册到 handler.go schedulerDescriptors**

在 `schedulerDescriptors()` 返回的 slice 中，在 `daily_report` entry 之后添加：

```go
		{
			Name:        "aux_label_cleanup",
			DisplayName: "Aux Label Cleanup",
			Description: "Clean up auxiliary labels with no active topic_tag references",
			Get: func() interface{} {
				return runtimeinfo.AuxLabelCleanupSchedulerInterface
			},
		},
```

**Step 4: 注册到 runtime.go**

4a. 在 `Runtime` struct 中添加字段：

```go
	AuxLabelCleanup       *jobs.AuxLabelCleanupScheduler
```

4b. 在 `StartRuntime()` 中，daily_report 启动之后添加：

```go
	runtime.AuxLabelCleanup = jobs.NewAuxLabelCleanupScheduler(3600)
	if err := runtime.AuxLabelCleanup.Start(); err != nil {
		logging.Warnf("Failed to start aux label cleanup scheduler: %v", err)
	} else {
		logging.Infoln("Aux label cleanup scheduler started successfully")
	}
```

4c. 在 runtimeinfo 赋值区域添加：

```go
	runtimeinfo.AuxLabelCleanupSchedulerInterface = runtime.AuxLabelCleanup
```

4d. 在 `SetupGracefulShutdown` 的 done goroutine 中，daily_report stop 之后添加：

```go
			if runtime.AuxLabelCleanup != nil {
				logging.Infoln("Stopping aux label cleanup scheduler...")
				runtime.AuxLabelCleanup.Stop()
			}
```

**Step 5: 编译验证**

```bash
cd backend-go && go build ./...
```

**Step 6: 编写调度器测试**

```go
// aux_label_cleanup_test.go
func TestAuxLabelCleanupScheduler_StartStop(t *testing.T) {
	s := NewAuxLabelCleanupScheduler(3600)
	require.NoError(t, s.Start())
	time.Sleep(100 * time.Millisecond)

	status := s.GetStatus()
	assert.Equal(t, "idle", status["status"])

	s.Stop()
	status = s.GetStatus()
	assert.Equal(t, "stopped", status["status"])
}

func TestAuxLabelCleanupScheduler_TriggerNow(t *testing.T) {
	s := NewAuxLabelCleanupScheduler(86400)
	require.NoError(t, s.Start())
	defer s.Stop()

	result := s.TriggerNow()
	assert.True(t, result["accepted"].(bool))
}
```

**Step 7: 运行测试**

```bash
cd backend-go && go test ./internal/jobs/... -run TestAuxLabelCleanup -v
```

**Step 8: Commit**

```bash
git add backend-go/internal/jobs/aux_label_cleanup.go backend-go/internal/jobs/aux_label_cleanup_test.go backend-go/internal/app/runtimeinfo/schedulers.go backend-go/internal/jobs/handler.go backend-go/internal/app/runtime.go
git commit -m "feat(aux-label): add AuxLabelCleanupScheduler with hourly GC"
```

---

## Task 7: 前端集成

**Files:**
- Modify: `front/app/utils/schedulerMeta.ts` (添加 aux_label_cleanup 元数据)
- Modify: `front/app/api/auxiliaryLabels.ts` (添加 triggerGc API)

**Step 1: 更新 schedulerMeta.ts**

在 `getSchedulerDisplayName` 的 `names` Record 中添加：

```typescript
    'aux_label_cleanup': '辅助标签清理',
```

在 `getSchedulerIcon` 的 `icons` Record 中添加：

```typescript
    'aux_label_cleanup': 'mdi:tag-minus-outline',
```

在 `getSchedulerColor` 的 `colors` Record 中添加：

```typescript
    'aux_label_cleanup': 'from-teal-500 to-emerald-500',
```

**Step 2: 更新 auxiliaryLabels.ts**

在 `useAuxiliaryLabelsApi()` 的返回值中添加 `triggerGc` 方法：

```typescript
  async function triggerGc(mode: 'dry_run' | 'disable' | 'delete' | 'recalculate', graceDays?: number): Promise<ApiResponse<{
    eligible_count?: number
    affected_count?: number
    corrected_count?: number
    total_count?: number
    preview?: Array<{ id: number; label: string; ref_count: number; created_at: string }>
  }>> {
    return apiClient.post('/auxiliary-labels/gc', { mode, grace_days: graceDays })
  }

  return {
    getLabels,
    getClusters,
    disableLabel,
    mergeAlias,
    triggerGc,
  }
```

**Step 3: 验证**

```bash
cd front && pnpm lint
cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"
```

**Step 4: Commit**

```bash
git add front/app/utils/schedulerMeta.ts front/app/api/auxiliaryLabels.ts
git commit -m "feat(aux-label): add frontend scheduler meta and GC API"
```

---

## Task 8: 全量验证

**Step 1: 后端 lint + vet + build**

```bash
cd backend-go && golangci-lint run ./... && go vet ./... && go build ./...
```

**Step 2: 后端测试（只跑修改的包）**

```bash
cd backend-go && go test ./internal/domain/tagging/... ./internal/jobs/... -v
```

**Step 3: 前端验证**

```bash
cd front && pnpm lint
cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"
cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"
```

**Step 4: 标记 tasks.md 中所有任务完成**

将 `openspec/changes/aux-label-cleanup/tasks.md` 中的所有 `- [ ]` 更新为 `- [x]`。

**Step 5: Final commit**

```bash
git add openspec/changes/aux-label-cleanup/tasks.md
git commit -m "chore(aux-label): mark all tasks complete"
```
