# tasks.md — slim-header-feed-actions

## 1. 设置页订阅源管理区新增管理工具条（TDD：先写失败测试）

- [x] 1.1 新增回归测试 `front/app/features/settings/components/SettingsSectionFeeds.test.ts`：断言管理区渲染「添加订阅源」「添加分类」「导入」「导出」4 个入口；点击「添加订阅源」后渲染 `AddFeedDialog`（stub 掉 `FeedMasterList` / `FeedDetailEditor` / `useGlobalSettings` / `useApiStore`，参照 `AppHeaderView.test.ts` 的 mock 模式）。先跑到红。
- [x] 1.2 `SettingsSectionFeeds.vue` 实现顶部工具条（design D1/D2）：4 个文字按钮 + `showAddFeedDialog` / `showAddCategoryDialog` / `showImportDialog` ref + 复用 `AddFeedDialog` / `AddCategoryDialog` / `ImportOpmlDialog`；`handleExportOpml` 从 `FeedLayoutShell.vue` 原样搬入（`apiStore.exportOpml()` + 下载 `feeds-export-<date>.opml`），失败复用 `msg--error` 提示条。跑到 1.1 转绿。
- [x] 1.3 对话框 `@added` / `@imported` 后触发 `apiStore.fetchFeeds({ per_page: 10000 })` 重拉，让新源/新分类立刻出现在左侧列表（design D4）。

## 2. 首页顶栏精简

- [x] 2.1 `AppHeaderView.vue` 删除 4 个按钮（添加订阅 `mdi:plus`、添加分类 `mdi:folder-plus`、导入 `mdi:import`、导出 `mdi:export`）与 `addFeed` / `addCategory` / `importOpml` / `exportOpml` 4 个 emits 声明。
- [x] 2.2 `FeedLayoutShell.vue` 清理：删除 `@add-category` / `@import-opml` / `@export-opml` 接线、`showAddCategoryDialog` / `showImportDialog` ref、`handleExportOpml`、`AddCategoryDialog` / `ImportOpmlDialog` 渲染与 import；**保留** `showAddFeedDialog` + `AddFeedDialog`（`FeedEmptyGuide @add` 仍在用）；同步删除 `AppHeader` 的 `@add-feed` 接线改为只由 `FeedEmptyGuide` 触发。

## 3. 测试

- [x] 3.1 新增 `front/app/features/settings/components/SettingsSectionFeeds.test.ts`（4 个用例）TDD 先红后绿：红=`4 failed`（`.feeds-toolbar__btn` 不存在），绿=`Tests 4 passed`。
- [x] 3.2 影响测试全过：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit"`（SettingsSectionFeeds 4 + AppHeaderView 3 及其余全量用例）。
- [x] 3.3 `pnpm lint` 0 errors（5 warnings 为既有无关文件）。
- [x] 3.4 `pnpm exec nuxi typecheck` / `pnpm build`（cmd 跑）均 EXIT_CODE=0。

## 4. 文档

<!-- doc-impact: architecture -->
<!-- doc-impact-excuse: flow=本 change 仅 UI 入口迁移，flow 文档描述的数据层/业务流程未变，verify 命中系工作树中 ai-model-health-gate change 的后端脏文件干扰; api=同上，无 API 变更，命中为其他 active change 脏文件; database=同上，无 schema 变更，命中为其他 active change 脏文件 -->

> 核实记录：`flow/reading.md` 仅描述 `useApiStore` 数据层职责（含 OPML 导入导出），数据层未变，**无需修改**；`ui-navigation.md` 无顶栏按钮清单段落（只记多步导航链路），故 4.1 修正为给链路 2 补工具条断言锚点。

- [x] 4.1 更新 `docs/reference/architecture/ui-navigation.md`：链路 2「2.3 常用断言锚点」表新增 feeds 管理工具条锚点（`.feeds-toolbar__btn` × 4 + 对话框选择器 + 注明首页顶栏入口已移除）。
- [x] 4.2 `docs/reference/flow/reading.md`：核实后无需更改（OPML 描述在数据层职责表，不受 UI 入口迁移影响）。
- [x] 4.3 归档前跑 `bash scripts/doc-impact.sh verify` + `bash scripts/check-standards.sh`；本 change 不动 flow 文档，§12.2 溯源链接不适用。（实测：verify 通过；check-standards 本 change [OK]，3 个 FAIL 均为其他 active change）

## 5. 验证

- [x] 5.1 `cd front && pnpm lint` → 0 errors（实测 2026-08-04）。
- [x] 5.2 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit 2>&1"` → 全量通过（实测：新增 4 用例 + 既有 AppHeaderView 3 用例绿）。
- [x] 5.3 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck 2>&1"` → EXIT_CODE=0。
- [x] 5.4 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build 2>&1"` → EXIT_CODE=0，`✨ Build complete!`。
- [x] 5.5 opencli 结构化断言（session=syntopica）：
  - 首页 `/`：`document.querySelectorAll('.header-right .header-btn')` → count=7，titles=[暂停分析/AI 模型健康/刷新/全部标为已读/设置/新手引导/切换为浅色模式]，无添加订阅/添加分类/导入/导出；
  - `/settings?section=feeds`：`.feeds-toolbar__btn` → count=4，texts=[添加订阅源/添加分类/导入/导出]；
  - 点击「添加订阅源」（eval click + wait 1s）→ `.app-dialog` 可见，含「RSS 订阅地址」输入与分类下拉（未分类/ai新闻/技术/新闻/游戏相关/论坛）。
- [x] 5.6 空状态引导路径代码核验：`grep '@add="showAddFeedDialog = true"' front/app/features/shell/components/FeedLayoutShell.vue` → 命中（FeedEmptyGuide → AddFeedDialog 保留）。
