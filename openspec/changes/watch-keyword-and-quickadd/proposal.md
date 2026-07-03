## Why

关注标记（topic-watch，topic-watchlist-observability 建立）上线后，暴露两个体验缺口：

1. **关键字场景缺位，且现有 watch 全走 AI**：用户想盯死某个具体词（"ASML"、"镓锗"、"霍尔木兹"），这类需求是**确定性抓取**，不该依赖 AI 语义判定（AI 可能漏、有成本、且要等日报生成）。现有 watch 清一色 `label` 类——一句话关切走 AI 单信号——根本没有"关键字文本匹配"这一轨。结果：要么把关键字硬塞进 label 让 AI 模糊匹配（漏召回），要么无解。
2. **创建入口太隐秘、反馈延迟**：关注只能在日报顶部栏的「新建关注」对话框里凭空填一句话，且建完要等**下一期日报生成**才看命中。用户怀疑"我刚点的到底有没有用"，冷启动认知负担高。

本 change 给 watch 加**关键字文本匹配轨**（即时、零 AI 成本、确定性），并补**内容流快捷关注入口**，让关注从"隐秘功能"变成"看得见、点得到、立刻见效"的正经能力。

## What Changes

两块，可在同一 change 内切片交付：

### A. watch 双轨化：新增 keyword 类型 —— topic-watch

`board_topic_watches` 加 `type` 字段（`label` / `keyword`，默认 `label`），命中判定分叉：

- **`label` 类（现有，不变）**：AI 单信号，盯话题语义（"美伊会不会真打起来"），每期日报一轮批量 AI。
- **`keyword` 类（新增）**：纯文本匹配，盯具体词（"ASML"），匹配 section 的 threads 标题+摘要，大小写不敏感；支持多词（空格=AND、`|`=OR）；零 AI 成本。

两类命中统一写 `topic_watch_hits`，统一在日报顶部「我在追踪」栏展示；keyword 命中不显示 AI 理由，标注「含关键字『XX』」。

### B. keyword 即时匹配 —— topic-watch

keyword 类 watch SHALL 支持即时匹配：用户建 keyword 关注后，立刻扫最近 14 天 section 命中并写入 `topic_watch_hits`，**不等下一期日报生成**。解决 label 类固有的反馈延迟（keyword 的优势就是即时、确定性，不发挥等于白做）。

### C. 内容流快捷关注入口 —— topic-watch

把"新建关注"从日报顶部栏的单一对话框，下沉到内容流：
- section 详情 / 话题详情旁加「＋关注」快捷入口。
- 点「＋关注」预填 label（来自 section 的 cluster_label 或 topic.label），默认建 label 类，用户可在对话框切换类型、改 label/关键字。
- 入口**不绑定话题总览工作台**（那是 manual-topic-lane change 的事），先在内容流铺开，等工作台落地后再补泳道旁入口。

## Capabilities

### Modified Capabilities

- `topic-watch`：watch 实体加 `type` 字段（label/keyword）；命中判定分叉（label 走 AI、keyword 走文本匹配）；keyword 多词逻辑（空格 AND / `|` OR）；管理 API 的 createWatch 加 type 参数。

### Added Capabilities（同 topic-watch capability 内新增 requirement）

- `topic-watch`：keyword 即时匹配（建关注后立刻扫近 14 天）；内容流快捷关注入口（一键预填建 watch）。

## Impact

- **后端**
  - 新增列：`board_topic_watches.type`（label/keyword，CHECK 约束，默认 label）。
  - `BoardTopicWatch.Type` 字段；`CreateWatch` 签名加 type 参数。
  - `EvaluateWatchHits` 分叉：label 类收集走 AI 批量（现有逻辑不变）；keyword 类走纯文本匹配（匹配 threads 标题+摘要，大小写不敏感，多词 AND/OR）。两类命中合并写表。
  - 新增 keyword 即时匹配：建 watch 时（或独立端点）扫近 14 天 section 文本匹配写 hits。
  - 既有 5 端点扩展：createWatch body 加 type；其余 CRUD 对两类一视同仁。
- **前端**
  - `topicWatches.ts`：`createWatch` 加 type 参数；新增即时匹配调用。
  - 新建关注 `AppDialog`：加「类型」选择（关注话题 / 关注关键字），label/keyword 输入区随类型切换；keyword 命中展示「含关键字『XX』」无理由。
  - 内容流「＋关注」快捷入口（section 详情 / 话题详情），预填 label 一键建。
- **AI 成本**：keyword 类零 AI 调用（纯文本匹配）；label 类不变。即时匹配零 AI。
- **数据兼容**：`type` 列新增默认 label，历史 watch 全归 label 类，行为不变。

## 依赖与执行顺序

⚠️ 本 change **依赖 topic-watchlist-observability 先归档**（建立 `topic-watch` 主 spec 与 `board_topic_watches` 表）。执行顺序：

```
topic-watchlist-observability（归档，建 topic-watch 基座）
  → watch-keyword-and-quickadd（本 change）
  → manual-topic-lane（话题总览工作台 + 手动建泳道）
```
