# Design: watch 双轨化（keyword 文本匹配）+ 即时匹配 + 快捷关注

## 1. Context

关注标记（topic-watch）现状：`board_topic_watches(id, semantic_board_id, label, status, ...)`，命中全走 AI 单信号（`EvaluateWatchHits` 批量喂 section+watch 给 AI）。缺口：① 无关键字文本匹配轨（盯具体词只能硬塞 label 让 AI 模糊匹配）；② 入口只在日报顶部栏对话框、建完等下一期日报才见效。

约束：

- 后端 Go/Gin + GORM + PostgreSQL；前端 Nuxt 4 + Vue 3。
- `topic_watch_hits(watch_id, section_id, report_id, period_date, reason)` 复合唯一索引（防重复命中行，已有）。
- `EvaluateWatchHits` 挂在 `GenerateAndSaveReport` 末尾（日报生成流程外，失败 log 吞）。
- `topic-watch` 主 spec 由 topic-watchlist-observability 归档建立；本 change 对其做 MODIFIED。
- section 的文本：`cluster_label`（聚类标题，短）+ threads 的 `title`/`summary`（中长）。底层 articles 不在本 change 匹配范围（开销大）。

## 2. Goals / Non-Goals

**Goals**

- watch 支持 keyword 类型，纯文本匹配，零 AI 成本、即时生效。
- keyword 多词逻辑（空格 AND / `|` OR），大小写不敏感。
- 内容流快捷关注入口（section/话题详情「＋关注」），预填 label 一键建。

**Non-Goals（明确不做）**

- 不动 label 类的 AI 判定逻辑（保持现有批量单信号）。
- 不匹配底层 articles 正文（只匹配 threads 标题+摘要，召回/开销平衡）。
- 不给 keyword 命中补 AI 理由（keyword 是确定性匹配，理由=「含关键字『XX』」，零 AI）。
- 不做话题总览工作台 / 泳道旁入口（那是 manual-topic-lane change 的范围；本 change 入口只铺内容流）。
- 不给快捷关注加 `source_topic_id`/`source_section_id` 来源字段（v1 不统计来源，保持简单；Open Questions 留口）。
- 不改 label 类的"等下一期日报"反馈模式（label 类本质要 AI，无法即时；只有 keyword 类即时）。

## 3. 架构总览

```mermaid
graph TB
    subgraph Create["建关注（API 扩展）"]
      CRT["POST /topic-watches<br/>label + type"]
      KW{type?}
      IMM["keyword 即时匹配<br/>扫近14天 section 文本"]
      DB1[("board_topic_watches<br/>+type")]
    end
    subgraph Daily["日报生成末尾（EvaluateWatchHits 分叉）"]
      SPLIT["遍历 active watches"]
      LBL["label 类<br/>批量 AI 单信号（现有）"]
      KWD["keyword 类<br/>纯文本匹配 threads"]
      MERGE["合并命中"]
      DB2[("topic_watch_hits")]
    end
    subgraph UI["前端"]
      DLG["新建关注对话框<br/>+类型选择"]
      QUICK["内容流＋关注<br/>section/话题详情"]
      BAR["日报顶部·我在追踪<br/>两类命中统一展示"]
    end
    CRT --> KW
    KW -->|keyword| IMM
    KW -->|label| DB1
    IMM --> DB2
    DB1 --> DB2
    SPLIT --> LBL --> MERGE
    SPLIT --> KWD --> MERGE
    MERGE --> DB2
    DB2 --> BAR
    QUICK --> CRT
    DLG --> CRT
    style KWD fill:#2d4a1a,color:#fff
    style IMM fill:#1a2d3a,color:#fff
```

关键：**keyword 的两条命中路径**（建关注时即时 + 日报生成时增量），都写同一张 `topic_watch_hits`，靠复合唯一索引去重。

## 4. 决策点（已与用户确认）

### 4.1 关键字匹配范围 = threads 标题+摘要

**选**：keyword 匹配 section 下各 thread 的 `title` + `summary` 拼接文本（不含底层 articles 正文）。

**理由**：`cluster_label` 太短（常漏，如聚类标题"中东动态"不含"霍尔木兹"）；articles 正文太重（每个 section 数十篇，扫描开销大）。threads 标题+摘要是召回与开销的平衡点——thread 本身已是文章聚类后的代表线索，含关键实体词。

**备选（否决）**：匹配 articles 正文。否决：每期每 board 数百 article 全文扫描，且 keyword 即时匹配要扫近 14 天，开销不可控。

### 4.2 关键字多词逻辑 = 空格 AND / `|` OR

**选**：keyword 文本支持多词——空格分隔 = AND（全含才命中），`|` 分隔 = OR（含任一即命中）。可混用：`ASML|镓锗 出口` = (ASML 或 镓锗) 且 含 出口。

**理由**：盯"半导体管制"用户常想抓多个相关词（ASML/镓锗/锗），单关键字太死，多词 OR 贴合"盯一族词"；AND 用于收紧（"出口 限制"避免误抓"出口增长"）。大小写不敏感（英文如 ASML/asml 等价）。

**备选（否决）**：一 watch 一词。否决：盯一族词要建多条 watch，管理累；顶部栏分组爆炸。

### 4.3 keyword 命中不显示 AI 理由

**选**：keyword 命中的 `reason` 字段写机械文本「含关键字『ASML』」（命中了哪个词），顶部栏展示该机械理由，**不调 AI 生成自然语言理由**。

**理由**：keyword 的价值就是零 AI 成本、确定性。补 AI 理由等于把成本加回来，违背初衷。机械理由「含关键字『XX』」已足够让用户知道为何命中。

### 4.4 keyword 即时匹配（不等日报）

**选**：建 keyword watch 后，立刻扫最近 14 天 section 的 threads 文本匹配，命中即写 `topic_watch_hits`（`report_id` 取 section 所属 report、`period_date` 取 section 日期）。

**理由**：解决 label 类固有的"建完等下一期"反馈延迟。keyword 是纯文本匹配，即时扫历史零成本零风险。label 类无法即时（要 AI 且依赖当期 section 集合），故只有 keyword 类支持即时。

**备选（否决）**：keyword 也只挂日报生成末尾。否决：等于没解决延迟，keyword 的即时优势白费。

### 4.5 快捷关注默认 label、可改可切

**选**：内容流「＋关注」点开后，预填 label（来自 section.cluster_label 或 topic.label），**默认 type=label**；用户可在对话框切换成 keyword、改 label/关键字文本。

**理由**：topic.label/cluster_label 多是一句话（"美伊博弈"），适合 AI 语义；但用户可能想转成盯具体词（"霍尔木兹"），允许切换。预填降低冷启动负担（不用凭空想一句话）。

### 4.6 快捷关注入口铺内容流，不绑工作台

**选**：本 change 的「＋关注」入口放在 section 详情 / 话题详情旁；**不**在话题总览工作台 lanes 旁加入口（那是 manual-topic-lane change 的范围，且依赖工作台落地）。

**理由**：watch 与工作台是两个独立 change，入口不该耦合。先在内容流铺开（用户看到内容就能关注），等工作台落地后再补 lanes 入口，避免阻塞。

### 4.7 架构：keyword 塞进 watch 表（不独立功能）

**选**：keyword 作为 `board_topic_watches.type` 的一个值，与 label 共表共展示，不独立成"关键字监控"功能。

**理由**：用户心智里 label 和 keyword 都是"我在追踪的东西"，顶部栏统一展示比分两个入口清爽。共表共 API 共展示，工程上也最省。`type` 字段只影响"怎么判定命中"，不影响"怎么管理/展示"。

## 5. Risks / Trade-offs

- **[keyword 误召回过多]** → 宽关键字（如"中国"）命中大量 section，顶部栏拥挤。缓解：keyword 多词 AND 收紧；UI 顶部栏已有"每组最多 5 条 + 折叠"（topic-watchlist-observability 既有限制）。
- **[即时匹配与日报匹配重复写]** → 同一 (watch_id, section_id, report_id) 可能被即时匹配和日报匹配各写一次。缓解：复用现有 `topic_watch_hits` 复合唯一索引（watch_id+section_id+report_id）+ OnConflict DoNothing，幂等去重。
- **[keyword 匹配性能]** → 即时匹配扫近 14 天全部 section 的 threads 文本。缓解：14 天 section 量级可控（单 board 数十~百余 section，threads 文本 KB 级），纯内存字符串扫描，无需索引；如后续 article 级匹配再考虑全文索引。
- **[type 字段历史兼容]** → 历史 watch 无 type。缓解：迁移默认 label，历史 watch 行为完全不变（仍走 AI）。
- **[label/keyword 展示混淆]** → 顶部栏两类命中混排，用户可能困惑"这条是 AI 判的还是关键字抓的"。缓解：keyword 命中标注「含关键字『XX』」+ 视觉微区分（如 keyword 分组带标签图标），label 命中带 AI 理由斜体。

## 6. Migration Plan

1. 后端：版本化迁移新增 `board_topic_watches.type` 列（CHECK label/keyword，默认 label），幂等。历史行 type=label。
2. 后端：`BoardTopicWatch.Type` 字段；`CreateWatch(boardID, label, watchType)` 签名扩展（默认 label 保持向后兼容）。
3. 后端：`EvaluateWatchHits` 内部分叉——收集 label 类走 AI 批量（现有逻辑）、keyword 类走 `matchKeywordSections`（纯函数）；合并命中写表。
4. 后端：keyword 即时匹配——`CreateWatch` 当 type=keyword 时，建表后同步扫近 14 天 section 写 hits（OnConflict DoNothing 幂等）。
5. 后端：createTopicWatch handler body 加 type 字段解析（缺省 label）。
6. 前端：`topicWatches.ts` createWatch 加 type；新建关注对话框加类型切换。
7. 前端：内容流「＋关注」快捷入口（section/话题详情）。
8. 部署：迁移幂等；历史 watch type=label 不报错；keyword 即时匹配失败 SHALL NOT 阻断建关注（非致命，记日志跳过，watch 仍建成功）。
9. 回滚：DROP type 列可逆；keyword 逻辑可独立 revert（label 类不受影响）。

## 8. 项目执行规范约束（实现期强制遵循）

> 与 manual-topic-lane change §8 同源（§4 后端 / §5 前端 / §7 架构体检 / §10 数据兼容 / §12 文档），此处只列本次强相关差异，不全文复述。

### 8.1 后端

- `matchKeywordSections(keywords string, sections []Section) []Hit` 为**纯函数**（多词解析 + 大小写不敏感 + threads 文本拼接），SQLite 单测覆盖（单词/多词AND/多词OR/混用/大小写/无命中）。
- keyword 即时匹配 + EvaluateWatchHits 分叉涉及真实 section 文本 → testcontainer pgvector 集成测试（断言两类命中分别写表、复合唯一索引去重）。
- `CreateWatch` 签名变更波及调用方（handler + 即时匹配），跑 `codegraph impact CreateWatch`。
- 新增/改 handler grep 路由注册二次确认（codegraph 追不到 group.POST）。

### 8.2 前端

- 新建关注对话框类型切换 + 内容流快捷入口，复用 `AppDialog`/`AppInput`/`AppButton`，禁 `window.*`。
- keyword 命中「含关键字『XX』」+ label/keyword 视觉微区分，全语义 token。
- API 经 `ApiClient`，snake→camel normalizer。

### 8.3 数据兼容性（§10）

- `type` 列默认 label，历史行无行为变化。
- keyword 即时匹配写 hits 用 OnConflict DoNothing，与日报匹配幂等共存。
- JSON 响应 type 为新增可选字段（默认 label），向后兼容。

### 8.4 文档流转（§12）

- `docs/reference/`（api / database）里程碑收尾统一更新。
- 本 change tasks 文档节列待更新清单，标注「里程碑收尾」。

## 9. Open Questions

- **快捷关注是否记来源**：v1 不加 `source_topic_id`/`source_section_id`（不统计来源）。若后续要分析"哪些 watch 是一键建的 vs 手填的"，再加来源字段。
- **keyword 即时匹配的时间窗口**：默认扫近 14 天（与话题总览窗口对齐）。是否允许用户自定义即时匹配窗口——倾向 v1 固定 14 天，避免参数膨胀。
- **多词解析的边界**：`ASML|镓锗 出口` 的解析优先级（先拆 `|` 再拆空格），tasks 阶段在纯函数单测里固化。
- **入口与工作台的衔接**：manual-topic-lane 落地后，是否在 lanes 泳道旁也加「＋关注」——作为后续增量，不在本 change。
