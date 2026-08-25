# Proposal: fix-section-merge-blackhole

## Why

`fix-section-embedding-content-based`（8-22 上线）把 section embedding 从标题文本改为内容长文后，同日无关叙事的 section 间距从旧几何的 0.4+ 压缩到 0.11~0.25，但 `MergeSimilarSections` 的 0.20/0.25 阈值未重标定。8-22 21:00 首次实战：全部 7 个 board 触发确定性合并 + union-find 传递闭包，section 从 51 塌到 19（单板 6+ → 1），乌克兰/以色列等无关线索顶着 `l1_direct` 外衣挂进美伊 topic 1151，L3 新叙事 section 落库前被吞导致当日 0 个新 persistent topic 诞生（常态 9~16/天）。实证无关叙事可贴近 0.11，**距离阈值在新几何下不存在可用判别值**，必须用结构性约束替代阈值信任。

## What Changes

- **锚定边界（方案 A，核心）**：同日两阶段合并引入归因边界约束——`MatchedTopicID` 不同的两个 section SHALL NOT 合并；L3 新叙事 section（`MatchedTopicID=NULL` 且 `LaneTier=l3_new`）SHALL NOT 被任何锚定 section 吸收；两个 L3 section 合并仍允许（同属新叙事池，共享空锚定）。lane 管线的 keep/switch/new 裁决是系统记录，展示层合并不得跨越。
- **合并开关（方案 B，保险丝）**：新增 `ai_settings` 配置项 `daily_report_section_merge_enabled`（默认 false，关）。开关关闭时跳过两阶段合并（section 原样落库），线上出问题时可零代码回退；本 change 落地后默认关闭，观察 lane 管线单独运行的 section 粒度，后续另行评估是否重启合并。
- **确定性合并审计（方案 E，顺带）**：Stage 1 确定性合并对当前零日志，补记 audit log（合并对双方 label、距离、边界拦截与否），与 Stage 2 灰区仲裁一致可回放。
- 不做：不重标定 0.20/0.25 阈值（已被 0.11 数据点证伪不可行）；不改 lane 分桶 / L2 裁决 / 质心计算；不改跨日关系重建（`RebuildBoardRelations` 另行评估）；不自动重跑 8-22 日报（修复后由用户手动 runNow，`SaveReport` 幂等覆盖式重建自动复原）。

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `section-relations`: 「同日 Section 两阶段合并」requirement 变化——合并候选对先过锚定边界校验（不同 `MatchedTopicID` 或 L3↔锚定跨界 → 拒绝合并，且该 pair 不进 LLM 仲裁）；两阶段合并整体可由 `daily_report_section_merge_enabled` 配置开关跳过；Stage 1 增加审计日志。

## Impact

- **后端**：
  - `backend-go/internal/topicgraph/service/daily_report_merge.go`：`MergeSimilarSections` 增加锚定边界过滤（确定性对与灰区对两侧）、kill switch 短路、审计日志
  - `backend-go/internal/topicgraph/service/daily_report_orchestrator.go`：Step 7 调用点读取配置开关
  - `backend-go/internal/platform/aisettings/`（或现有配置读取路径）：注册 `daily_report_section_merge_enabled` 配置项（默认 false）
- **前端**：无
- **行为影响**：
  - 默认关闭合并后，section 粒度回到 lane 管线原始输出（更细，可能偏碎——这是接受的回归，观察后再定）
  - L3 新叙事不再被吞，恢复 candidate topic 供给
  - 8-22 已生成的 11 份 mega-section 日报保持污染状态，直到用户手动重跑当日（可选）
- **部署影响**：合并即生效，无需迁移；`daily_report_section_merge_enabled` 默认 false 等价于直接禁用旧合并路径；对 8-22 污染数据可用各 board `runNow` 重跑复原（幂等覆盖重建），无需数据手术
