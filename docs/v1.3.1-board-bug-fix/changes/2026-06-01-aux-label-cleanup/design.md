## Context

辅助标签（`semantic_labels` where `label_type='auxiliary'`）数量随时间单调增长。当前系统在以下关键路径加载全部 active 辅助标签进行 O(N) 或 O(N×M) 的 embedding 相似度计算：

1. **`ResolveAuxiliaryLabel`**：每次新辅助标签入库，`loadActiveAuxiliaryLabels` 加载全量 active aux labels，`sqlMergeMatcher` 对所有候选做 Go 侧 cosine 比较
2. **`loadTagAuxiliaries`**：每个 topic_tag 匹配 board 时加载其全部 aux labels
3. **`loadActiveTagAuxiliaries`**：回填 board 模式时加载所有 active topic_tag 的全部 aux labels 关联

`ref_count` 字段在 `AttachAuxiliaryLabels` 中递增，在 `MergeAuxiliaryLabelAlias` 中重算，但从未在 topic_tag 删除时递减。CASCADE 删除了 `topic_tag_semantic_labels` 行，但 `semantic_labels.ref_count` 保持膨胀值。

### 现有 spec 约定

`sematic-label-model` spec 明确要求：
> 系统 SHALL 在 tag 关联或取消关联辅助标签时，自动增减对应辅助标签 semantic_label 的 ref_count。

当前实现只满足了「增」，未实现「减」——这是本次修正的依据。

## Goals / Non-Goals

**Goals:**
- topic_tag 删除时自动递减相关 auxiliary label 的 ref_count
- 提供 GC 机制清理无活跃引用的辅助标签
- GC disable 时同步清理关联的 `board_composition` 行
- 定时自动执行 GC（每小时，mode=disable）
- 支持手动触发 GC（dry_run / disable / delete / recalculate）
- 通过 GC API 完成存量 ref_count 数据校准（无需独立脚本）

**Non-Goals:**
- 不做 delete 模式的自动 GC（仅手动可选，避免误删）
- 不修改 `AttachAuxiliaryLabels` 的核心匹配逻辑
- 不清理 disabled 状态但仍有嵌入向量的历史数据（本次只处理 active → disabled）
- 不优化 `loadActiveAuxiliaryLabels` 的全量加载策略（那是另一个性能优化话题）
- 不在 HardMergeTags 中迁移 source tag 的 aux label 关联到 target tag（现有行为，source 的 aux label 随 CASCADE 删除，GC 会清理无引用的 aux label）

## Decisions

### D1: ref_count 采用重新计算而非减法

**选择**：在 topic_tag 删除后，对受影响的 aux labels 执行 `ref_count = COUNT(topic_tag_semantic_labels)` 重新计算，而非逐条递减。

**理由**：`ref_count` 可能因历史 bug 已经不准，减法会延续错误。重新计算是自修复的，同时兼容存量脏数据。只对受影响的 aux label IDs 重算（而非全表），性能可控。

**替代方案**：逐条 `ref_count = ref_count - 1`。更轻量但无法修复已有偏差。

### D2: GC 默认采取 disable 而非 delete

**选择**：定时任务自动执行时使用 `status='disabled'` 软删除，手动触发时可选择 `delete` 硬删除。

**理由**：disable 可回滚（手动改回 active），delete 不可逆。定时任务无人值守时，安全优先。用户可在确认无误后通过 API 手动执行 delete。

### D3: grace_days 默认 1 天

**选择**：创建 1 天内的 auxiliary label 不参与 GC，与 tag 清理保持一致。

**理由**：新创建的标签可能暂时未被 topic_tag 引用（例如文章正在处理中），1 天保护期足够覆盖正常处理链路。

### D4: 调度器仿照 LogCleanupScheduler 模式

**选择**：新建 `AuxLabelCleanupScheduler` 结构体，实现 `Start/Stop/GetStatus/TriggerNow/ResetStats/UpdateInterval` 接口，通过 `runtimeinfo` 全局变量注册到现有调度器框架。

**理由**：与现有 8 个调度器保持一致的接口和生命周期管理。`GlobalSettingsDialog.vue` 无需改动——调度器列表是自动枚举的。

### D5: ref_count 修正点选择

**选择**：在两个 topic_tag 硬删除入口修正：

1. **`CleanupOrphanedTags`**（`article_tagger.go`）：删除前收集 aux label IDs，删除后调用 `RecountRefs`
2. **`HardMergeTags`**（`hard_merge.go`）：在 `tx.Delete(TopicTag, sourceID)` 前收集，删除后调用 `RecountRefs`

**理由**：这是 topic_tag 被删除的仅有两个入口（不考虑直接 SQL）。在 CASCADE 删除 `topic_tag_semantic_labels` 之前捕获 IDs 是可行的——只需先查询再删除。

### D6: GC 查询策略

**选择**：单条 SQL 查询无活跃引用的辅助标签：

```sql
SELECT id, label, ref_count
FROM semantic_labels
WHERE label_type = 'auxiliary'
  AND status = 'active'
  AND protected = false
  AND created_at < NOW() - INTERVAL '1 day'
  AND id NOT IN (
    SELECT DISTINCT semantic_label_id
    FROM topic_tag_semantic_labels
  )
  AND id NOT IN (
    SELECT DISTINCT auxiliary_label_id
    FROM board_composition
  )
```

**理由**：系统中不存在 `status='merged'` 的 TopicTag（`HardMergeTags` 直接硬删除，无软删除），因此只需检查 `topic_tag_semantic_labels` 中是否存在关联即可，无需 JOIN `topic_tags`。TopicTag 删除时 CASCADE 已清理其 `topic_tag_semantic_labels` 行，不会留下悬挂关联。

**补充**：查询还需排除挂载在板块下的辅助标签（`board_composition` 中有引用的）。辅助标签可能被手动挂载到板块上作为描述性标签，即使它没有关联任何 topic_tag，也不应被 GC 清理。缺少此条件会导致板块上挂载的辅助标签被误删。

### D7: GC disable 同步清理 board_composition

**选择**：GC disable 时，除了将 aux label 的 status 改为 disabled，同时删除 `board_composition` 中引用该 aux label 的行。

**理由**：`getBoardComposition`（semantic_board_handler.go:924）未过滤 `status='active'`，如果不清理 `board_composition`，board composition 前端仍会显示已 disabled 的 aux label。GC delete 时 CASCADE 会自动清理，但 disable 不会。

**注意**：由于 D6 查询已排除 `board_composition` 中有引用的标签，正常 GC 流程不会 disable 挂载在板块上的标签。D7 的 board_composition 清理是防御性措施，确保万无一失（例如标签在 GC 查询后被手动挂载到板块的极端竞态场景）。

### D8: 存量校准通过 API 而非独立脚本

**选择**：在 GC API 中增加 `mode=recalculate` 模式，对所有 `label_type='auxiliary'` 的 semantic_labels 执行 ref_count 全量重算。部署后通过 `curl POST /api/auxiliary-labels/gc -d '{"mode":"recalculate"}'` 即可完成校准，无需维护独立的 Go 脚本。

**理由**：`RecountRefs` 逻辑已在 Phase 1 实现，recalculate mode 只是全表调用。减少一个需要单独编译和部署的脚本。

## Risks / Trade-offs

- **[风险] 存量 ref_count 可能为负数**（如果已经调用过 CASCADE 删除但 ref_count > 0）：→ 缓解：`RecountRefs` 使用 `COUNT(*)` 直接计算，不会出现负数。recalculate API 可安全执行。
- **[风险] GC 执行期间与 AttachAuxiliaryLabels 并发**：可能 GC 刚查出 label 无引用，AttachAuxiliaryLabels 同时为其创建了新引用。→ 缓解：GC 默认用 disable（软删除），即使并发也不会丢数据；1 天 grace_days 已大幅降低窗口；极端情况下可手动恢复。
- **[取舍] 不删除 disabled 标签的 embedding 向量**：disabled 后 embedding 数据仍占磁盘空间。→ 手动 delete GC 可彻底清理，后续可考虑定期硬删除 disabled 标签。

## Migration Plan

1. 部署新代码（包含 Phase 1 ref_count 修正 + Phase 3 GC 调度器）
2. 调用 `POST /api/auxiliary-labels/gc -d '{"mode":"recalculate"}'` 完成存量 ref_count 校准
3. GC 调度器自动启动（startup delay 10 分钟），开始定期清理
4. 观察 GC 调度器首次执行结果（日志输出清理数量），确认无异常
5. 如需回滚：停止 GC 调度器即可，disabled 标签可手动恢复；ref_count 修正无破坏性影响

## Open Questions

- 是否需要支持 GC 结果透出到前端展示（如「可清理标签数」badge）？当前设计仅通过 GlobalSettings 定时任务面板展示执行统计，不展示待清理预览。
