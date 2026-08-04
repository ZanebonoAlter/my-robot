# tasks.md — remove-dead-global-settings-dialog

## 1. 删除

- [x] 1.1 删除 `front/app/components/dialog/GlobalSettingsDialog.vue` 与 `front/app/components/dialog/FeedSettingsPanel.vue`。
- [x] 1.2 复查零残留：`grep -rn 'GlobalSettingsDialog\|FeedSettingsPanel' front/app front/tests` → 零命中。

## 2. 测试

- [x] 2.1 `cd front && pnpm lint` → 0 errors。
- [x] 2.2 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit 2>&1"` → 全量通过。
- [x] 2.3 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck 2>&1"` → EXIT_CODE=0。
- [x] 2.4 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build 2>&1"` → EXIT_CODE=0。

## 3. 文档

<!-- doc-impact: none(纯死代码删除；docs/reference 无任何文档描述这两个组件) -->

（无文档更新）

## 4. 验证

- [x] 4.1 `grep -rn 'GlobalSettingsDialog\|FeedSettingsPanel' front/app front/tests` → 零命中（实测）。
- [x] 4.2 门禁四条全绿（命令见 §2，实测结果见 §2 勾选记录）。
