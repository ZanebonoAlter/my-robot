# slim-header-feed-actions

## Why

首页顶栏右侧挤了 11 个无文字图标按钮（只能靠 tooltip 分辨），其中 4 个 feed 管理类操作（添加订阅 / 添加分类 / 导入 OPML / 导出 OPML）属于中低频管理动作，与高频阅读操作（刷新、全部已读）混排，信息密度过高。同时设置页「订阅源」管理区只有"改/删"能力、没有"增/导入/导出"入口，feed 管理入口割裂在两处。

## What Changes

- 首页顶栏移除 4 个 feed 管理按钮：添加订阅（➕）、添加分类（📁+）、导入 OPML（📥）、导出 OPML（📤）。顶栏从 11 个按钮精简到 7 个（暂停分析、AI 健康、刷新、全部已读、设置、新手引导、主题切换）。
- 设置页「订阅源」管理区（`/settings?section=feeds`）新增 4 个入口：添加订阅源、添加分类、导入 OPML、导出 OPML，与订阅源列表同处一屏。
- 复用现有对话框组件 `AddFeedDialog` / `AddCategoryDialog` / `ImportOpmlDialog` 与 `apiStore.exportOpml()`，无新 API、无新组件范式。
- 首页空状态引导（`FeedEmptyGuide`）的"添加订阅"按钮保留不变；侧边栏「发现订阅源」入口保留不变。
- **BREAKING（用户习惯）**：添加订阅源 / 添加分类 / 导入导出 OPML 的操作路径从首页顶栏改为设置页，用户肌肉记忆需要迁移。

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `feed-settings-ui`: 订阅源管理区从"仅查看/编辑/删除"扩展为完整的"增删改 + OPML 导入导出"管理中心；feed 管理类入口聚合于设置工作区，首页顶栏不再承载此类动作。

## Impact

- **前端代码**：
  - `front/app/features/shell/components/AppHeaderView.vue` — 删除 4 个按钮与对应 emits
  - `front/app/features/shell/components/FeedLayoutShell.vue` — 移除顶栏事件接线；保留 `FeedEmptyGuide` 的 `AddFeedDialog` 触发；`AddCategoryDialog` / `ImportOpmlDialog` / `handleExportOpml` 迁出
  - `front/app/features/settings/components/SettingsSectionFeeds.vue`（及可能的 `FeedMasterList.vue`）— 新增管理工具条
- **后端 / API / 数据库**：无变更
- **测试**：受影响的 Vitest 单测（如引用被删 emits 的组件测试）需同步更新
- **文档**：`docs/reference/architecture/ui-navigation.md`（顶栏结构）、`docs/reference/flow/`（feed 管理入口描述，如命中）
