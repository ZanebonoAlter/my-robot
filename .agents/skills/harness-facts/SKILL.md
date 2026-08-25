---
name: harness-facts
description: Syntopica harness 事实库（.pi/harness/events.db）查询指南。当需要排查"当时为什么注入了某个约束"、constraint.inject / gate.check 历史记录、事件归因（某事件为何落在某 change 名下）、session 档位切换 / reload 重绑问题、pin.write 落盘审计、子线程 Agent 派发历史等"事件考古 / 归因排查"类问题时必须先查本 skill。含 schema、事件类型与保留期、payload 字段、查询配方与归因方法论。
---

# Harness 事实库（events.db）

## 是什么（先分清词撞车）

**harness 事实库** = `.pi/harness/events.db`：pi 扩展自动写入的 SQLite 事件账本（模型零参与），记录约束注入、质量门禁、档位切换、pin 落盘、子线程派发等 harness 层事实。

⚠️ 它**不是** context-mode 的 FTS5 knowledge base（`ctx_search` 那套）。用户说"查事实库/查事件记录"时，指本库，别去调 ctx_search。两者都挂时（比如 CONTEXT_MODE_DIR 报错），本库用 `sqlite3` CLI 直接查，不受影响。

- 位置：`.pi/harness/events.db`（WAL 模式，busy_timeout 5000，CLI 直接读无碍；`.pi/harness/` 已 gitignore，是本机运行数据）
- 设计文档：`docs/research/harness事实库.md`
- 写入方（全部在 `.pi/extensions/`，gitignored；入库代码快照在 `docs/research/`）：
  - `constraint-injection.ts` → `constraint.inject` / `pin.write` / `mode.set`
  - `quality-gate.ts` → `gate.check`
  - `harness-telemetry.ts` → `session.start` / `subagent.dispatch`

## Schema 与事件类型

单表 `events`：`id INTEGER PK, ts TEXT(ISO), session_id TEXT, kind TEXT, change TEXT NULL, payload TEXT(JSON)`。索引：`(session_id,id)`、`(change,id)`、`(kind,ts)`。append-only，除 TTL 清扫外不删。

| kind | 保留期 | payload 关键字段 |
| --- | --- | --- |
| `session.start` | 90 天 | `reason`(new/reload/resume/fork)、`cwd`、`prev`(前一 session 文件) |
| `constraint.inject` | 30 天 | `path`、`mode`(full/section)、`reason`、`bytes` |
| `gate.check` | 30 天 | `cmd`、`phase`(turn_end)、`ok`、`ms`、`diag` |
| `mode.set` | 30 天 | `mode`、`boundChange` |
| `subagent.dispatch` | 30 天 | `type`、`model`、`desc`、`ms`、`tokens`、`status`、`agentId`、`isError` |
| `pin.write` | 永久 | 见 pin_finding 写入方 |

`constraint.inject` 的 `reason` 枚举（实测）：`index`（未激活档的常驻索引）/ `mode-base`（档位激活后的基础注入）/ `declaration`（按 proposal 头 `constraint-domains` 声明拉 flow 约束节）/ `keyword`（对话关键词命中）/ `edit`（编辑路径 JIT 命中）/ `change-file`（change 文档命中）/ `stack-conditional`（栈条件注入）。

## 查询配方

```bash
cd .pi/harness
# 1. 全局账目：各 change 名下有哪些事件、多少条（归因排查第一步）
sqlite3 events.db "SELECT COALESCE(change,'-'), kind, COUNT(*) FROM events GROUP BY change, kind ORDER BY change;"

# 2. 按 change 查时间线（payload 截断看头部）
sqlite3 events.db "SELECT ts, kind, substr(payload,1,200) FROM events WHERE change='X' ORDER BY id;"

# 3. 单条 payload 全文（-line 竖排易读）
sqlite3 -line events.db "SELECT * FROM events WHERE id=1680;"

# 4. 某 session 内多 change 交织（分钟桶，看切换节奏）
sqlite3 events.db "SELECT substr(ts,1,16), COALESCE(change,'-'), COUNT(*) FROM events WHERE session_id='S' AND kind='gate.check' GROUP BY 1,2 ORDER BY 1;"

# 5. session 锚点（reload 清档 / 重绑时点判定）
sqlite3 -line events.db "SELECT ts, payload FROM events WHERE kind='session.start' AND session_id='S';"

# 6. 档位切换史
sqlite3 events.db "SELECT ts, session_id, payload FROM events WHERE kind='mode.set' ORDER BY id;"
```

## 归因方法论（实战教训）

**事件的 `change` 字段 = 写入那一刻绑定的档位**，不是"这个 session 主做哪个 change"。踩坑三件套：

1. **同一 session 交替做多个 change**：gate.check / inject 会按当时档位分散落在不同 change 名下，账目"看着错"其实没错。
2. **reload 清零档位后重绑**：`session.start(reason=reload)` 后重新触发 apply 时，档位可能绑回**上一个 change**（不是当前在做的），此后注入全记在旧 change 头上——恰是 constraint-injection-tier-b 要修的 resume 恢复问题的现场证据形态。
3. **注入内容看 reason 就懂来源**：`declaration` = 机械按 proposal 头 `constraint-domains` 声明拉节，与"当时正在实现什么"无关。

排查顺序：① 全局账目定异常（如某 change 名下 0 条 inject 但同期别处有 declaration）→ ② 涉事 session 分钟桶看交织 → ③ session.start 锚点定位 reload/重绑时点 → ④ 对照归档 proposal 头部声明收口。
