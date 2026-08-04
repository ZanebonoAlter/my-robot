# design.md — slim-header-feed-actions

## Context

首页顶栏（`AppHeaderView.vue`）右侧 11 个图标按钮中，4 个 feed 管理类（addFeed / addCategory / importOpml / exportOpml）通过 emits 上抛给 `FeedLayoutShell.vue`，由后者持有 `showAddFeedDialog` / `showAddCategoryDialog` / `showImportDialog` 状态并渲染 3 个独立对话框组件；导出直接调 `apiStore.exportOpml()` 触发浏览器下载。设置页订阅源管理区（`SettingsSectionFeeds.vue` + `FeedMasterList.vue`）目前只有搜索、分类分组列表与单源编辑，无"增/导入/导出"入口。

约束：
- 3 个对话框组件（`app/components/dialog/AddFeedDialog.vue` / `AddCategoryDialog.vue` / `ImportOpmlDialog.vue`）是独立通用组件，不依赖首页状态，可直接在设置页复用。
- `mdi:plus` / `mdi:folder-plus` / `mdi:import` / `mdi:export` 已在 iconify 本地子集（顶栏在用），设置页沿用同图标**无需重新 `pnpm generate:icons`**（reading.md 约束 #5：图标必须来自 `app/assets/iconify-subset.json`）。
- `FeedEmptyGuide`（首页空状态）的 `@add` 仍触发 `AddFeedDialog`，**该路径必须保留**，不能误删。

## Goals / Non-Goals

**Goals:**
- 顶栏 11 → 7，只留阅读高频 + 系统状态类按钮。
- feed 管理动作（增/导入/导出）全部聚合到 `/settings?section=feeds` 一屏。
- 零后端变更、零新组件范式、零新图标。

**Non-Goals:**
- 不动「新手引导」按钮（可选项，用户未选）。
- 不重构 `feed-settings-ui` 旧 spec 中已过时的"GlobalSettingsDialog 卡片"措辞（与本 change 无关）。
- 不改侧边栏 / 发现订阅源页的任何入口。
- 不清理死组件 `GlobalSettingsDialog.vue`（与本次无关，另立 change）。

## Decisions

### D1: 入口放在 `SettingsSectionFeeds` 顶部工具条，而非塞进 `FeedMasterList` 搜索行

在 `feeds-section` 顶部（msg 提示条之上、`feeds-layout` 之上）加一条工具条：4 个带文字标签的按钮（添加订阅源 / 添加分类 / 导入 / 导出）。

备选：塞进 `FeedMasterList` 的搜索行右侧（图标按钮）。否决理由：搜索行只有 40px 高、宽度 280px，塞 4 个图标太挤且无文字不可发现；设置页是"管理中心"心智，带文字的按钮更符合该页面语境（页面其余操作如「删除订阅源」也是文字按钮）。

### D2: 对话框状态由 `SettingsSectionFeeds` 持有

`showAddFeedDialog` / `showAddCategoryDialog` / `showImportDialog` 3 个 ref + 对话框渲染直接放 `SettingsSectionFeeds.vue`，导出逻辑（`handleExportOpml`，约 12 行）原样搬入，成功/失败复用该组件已有的 `msg--success` / `msg--error` 提示条（`useGlobalSettings` 的 `error`/`success` 或本地 ref，按现有模式）。

备选：新建 `FeedManagementToolbar.vue` 子组件。否决理由：4 个按钮 + 3 个 v-if 对话框约 60 行，单文件内聚即可，抽象一个单用途组件属于过度设计（Simplicity First）。若未来工具条变复杂再拆。

### D3: 顶栏删除后的事件接线清理

`FeedLayoutShell.vue`：
- 删 `@add-category` / `@import-opml` / `@export-opml` 接线与 `showAddCategoryDialog` / `showImportDialog` / `handleExportOpml`。
- **保留** `showAddFeedDialog` 与 `AddFeedDialog` 渲染——`FeedEmptyGuide @add` 仍在用。
- `AppHeaderView.vue` 删 4 个按钮 + 4 个 emits 声明。

### D4: 导入成功后的列表刷新

迁移前 `ImportOpmlDialog @imported` 在首页是空 handler（`() => {}`），依赖对话框内"请稍后刷新查看"提示。设置页同样先挂空 handler 或触发 `apiStore.fetchFeeds` 重拉（成本一行，体验更好，采用后者）；`AddFeedDialog @added` / `AddCategoryDialog @added` 同理触发重拉，让新源/新分类立刻出现在左侧列表。

## Risks / Trade-offs

- [用户肌肉记忆：习惯顶栏加源] → 设置页入口带文字标签可发现性更强；首页空状态引导与「发现订阅源」页两条添加路径兜底；完工汇报明确告知路径变化（§部署后影响）。
- [设置页工具条与 `msg` 提示条样式冲突] → 复用页面既有 CSS 变量与 `feeds-section` flex 布局，工具条放 msg 之下、`feeds-layout` 之上。
- [Vitest 组件测试引用被删 emits] → tasks 中含"同步更新受影响单测"子任务，门禁 `pnpm test:unit` 兜底。

## Migration Plan

纯前端 UI 搬迁，无数据迁移。合并后用户唯一变化是操作路径（见上方 Risk 1）。回滚 = revert 本 change 提交。

## Open Questions

（无）
