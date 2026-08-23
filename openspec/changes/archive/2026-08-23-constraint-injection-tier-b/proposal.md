## Why

constraint-injection 的注入块当前**无总量预算**：关键词命中只增不减（保前缀缓存，正确），但命中 9 个 flow「业务约束与不变量」节实测合计 43K（daily-report 节 7.8K、discovery 节 7.5K），加上 index/JIT 节/explore-findings/词汇表，最坏 55K+ bytes 常驻 system prompt 每 turn；且 `digestDocThreshold`(6144) 因本仓文档均无「## 硬规则速览」节而从未生效，节级注入（主路径）完全无降级路径。另外 `session_start` 对 resume 也清零档位，apply 做到一半 resume 回来档位掉回未激活，约束注入全部消失，需重新手动触发 `/opsx-apply`。

（判定依据：docs/research/harness-survey/findings.md B 级判定回写——B5 不抄（追加式专属）、B7 暂缓（无 XML 框架与不可信源），本 change 只落 B6 与 B8 衍生需求。）

## What Changes

- **注入预算与降级（B6）**：配置新增 `budgetBytes`（默认 16384）；`planInjection` 注入块超预算时按命中原因分层降级——先降 keyword/jit 命中节（整节 → 标题 + 首行预览 + `read` 路径占位），再收紧 findings/词汇表 digest 阈值；**降级永不真丢**，占位行保证模型知道缺什么、可自行 read 补取
- **模型可见省略通知**：超预算时注入块头部列出已降级路径（模型可见）+ 状态栏 widget 显示「预算 用量/上限」
- **预算记账**：`constraint.inject` payload 新增降级维度（如 `degraded: true` + 降级后字节数），与 A 级 usage 统计形成闭环——常被挤掉的约束节本身就是"该瘦身"的证据
- **档位持久化与 resume 恢复（B8 衍生）**：档位激活时（input/skill 命中设 mode 那一刻）向 events.db 记 `mode.set` 事件（payload 含 mode、boundChange）；`session_start{reason:"resume"}` 时查该会话最近一条 `mode.set` 恢复档位，查不到才回落未激活；`new/fork/reload` 维持清零语义不变
- **事件词汇扩展**：`harness-log.ts` 的 `HarnessEventKind` 增加 `mode.set`（保留期 30 天，与 constraint.inject 同级——档位恢复只需覆盖近期会话）

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `constraint-injection`: 「每 turn system prompt 强制注入」需求增加预算上限与分层降级行为（含模型可见省略通知与记账）；「档位识别与 change 绑定」需求增加 resume 档位恢复（`new/fork/reload` 清零语义不变）
- `harness-fact-log`: 「事件类型词汇与保留期」需求从六类事件扩为七类（新增 `mode.set`，30 天保留）

## Impact

- `.pi/extensions/constraint-injection.ts`：`planInjection` 预算裁剪 + `session_start` resume 恢复 + `mode.set` 记账（改动主战场）
- `.pi/extensions/lib/harness-log.ts`：kind union + RETENTION_DAYS 各加一行；**新增 `queryLatestByKind(kind)` 查询 API**（resume 兑底取数，`idx_ev_kind_ts` 索引现成）
- `.pi/constraint-injection.json`：新增 `budgetBytes`（可选，默认 16384）
- `tests/run-smoke.sh` / 烟测脚本：预算降级（含"永不真丢"占位断言）与 resume 恢复用例
- 既有六类事件语义与 TTL 不变；无 DB schema 迁移（kind 为 TEXT 列，新 kind 零迁移）

**归档依赖**：本 change 的 constraint-injection delta 已 rebase 至 constraint-domain-declaration（cdd）的目标状态，cdd 必须先归档（反序会静默回滚其规范变更，详见 design Migration Plan）
