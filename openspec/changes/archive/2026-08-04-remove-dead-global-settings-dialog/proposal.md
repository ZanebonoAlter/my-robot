# remove-dead-global-settings-dialog

## Why

`GlobalSettingsDialog.vue` 是 `settings-workspace`（独立 `/settings` 设置工作区）改造前的旧设置对话框，自设置工作区上线后零引用（grep 全仓含测试零命中）；其独占子组件 `FeedSettingsPanel.vue` 仅被它 import。共 350 行死代码，徒增图标子集扫描、依赖分析与阅读噪音。

## What Changes

- 删除 `front/app/components/dialog/GlobalSettingsDialog.vue`（139 行）
- 删除 `front/app/components/dialog/FeedSettingsPanel.vue`（211 行，唯一引用者是 GlobalSettingsDialog）
- 无行为变化、无 API 变化、无 spec 需求变化（纯死代码删除）

引用尸检结论（删除前已核实）：
- `FirecrawlConfigPanel.vue` — **保留**（`SettingsSectionFirecrawl` 在用）
- `SchedulerStatusPanel.vue` — **保留**（`SettingsSectionSchedulers` 在用）
- `TopicManageDialog.vue` — **保留**（`TopicDetectiveWall.client.vue` 模板在用）
- `FeedSettingsPanel` 依赖仅 `FeedIcon`（多方在用）+ `Icon`，无独占深层依赖

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

（无——纯死代码删除，不改变任何需求行为）

## Impact

- 前端：-350 行死代码
- 门禁证明：lint / typecheck / test:unit 全绿即证明无引用断裂
