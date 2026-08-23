## Why

constraint-injection-tier-b 落地的档位恢复第 2 段「全局最新 mode.set 兜底」在多窗口并行下必然误绑：无自身档位历史的会话（新窗口/reload）会继承**其他窗口**最近的 implementation 档（实测 2026-08-23 13:49:28，探索窗口 reload 后被灌入另一窗口的 scenario-test-mapping-gate 档位）。设计时的「单用户极小概率跨会话误绑」假设错误——本仓库常态即多 pi 窗口并行。且该兜底防御的场景（resume 生成新 sessionId）经 pi 文档查证不存在：/resume 切换到目标会话文件，sessionId 即目标会话自身的 id，第 1 段（同 sessionId）必中。

## What Changes

- `recoverMode` 删除第 2 段全局兜底（`queryLatestByKind` 调用移除）：resume/reload/startup 三条恢复路径统一**仅按同 sessionId** 取最近一条 mode.set，无本会话记录 → 不恢复（回落未激活）
- 事实库 `queryLatestByKind` API 保留（harness 烟测仍覆盖，通用查询能力，仅恢复链路不再使用）

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `constraint-injection`: 「档位识别与 change 绑定」需求的恢复语义从「resume/reload 两段式（同 sessionId → 全局最新）」改为「三路径均仅同 sessionId，无全局兜底」

## Impact

- `.pi/extensions/constraint-injection.ts`：`recoverMode` 删第 2 段（~6 行）+ 注释修正
- `.pi/extensions/tests/constraint-injection.smoke.cjs`：「全局兜底」断言翻转为「新 sessionId 无本会话记录不恢复」（用户实测场景的回归测试）
- 无配置/DB/文档域影响；`queryLatestByKind` 及其烟测保留

<!-- constraint-domains 无（纯工具链 change） -->
