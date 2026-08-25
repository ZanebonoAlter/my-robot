<!-- constraint-domains: topic-graph, daily-report -->

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

两类命中统一写 `topic_watch_hits`。日报时间线的每期记录下显示紧凑命中预告 tag；日报详情以「追踪关键字」「追踪话题」两个优先阅读分区展示可定位索引。keyword 命中不显示 AI 理由，标注实际命中的关键字。

### B. keyword 即时匹配 —— topic-watch

keyword 类 watch SHALL 支持即时匹配：用户建 keyword 关注后，立刻扫最近 14 天 section 命中并写入 `topic_watch_hits`，**不等下一期日报生成**。解决 label 类固有的反馈延迟（keyword 的优势就是即时、确定性，不发挥等于白做）。

### C. 版块级关注管理面板 —— topic-watch

把创建/管理入口从「埋在每一期日报详情里的对话框」收敛到与实体同级的位置：
- 版块工作台内容区 tab 栏（TagsPage tags-content-tabs）右端加「我在追踪 (N)」入口 chip，版块选中后在五个内容 tab（板块内容/话题总览/日报/文章/数据增强）下均常驻可见（注：日报与话题总览是平级 tab，不是同一面板内切换）。
- 点开管理面板（弹层）：新建（类型双选对话框，keyword 态含语法提示 + 实时解析预览 + 回扫说明）+ 全部关注列表（类型标识、暂停/恢复、删除）+ keyword 建后即时回扫反馈（命中数可点查看）。
- 日报详情不再保留独立居中 WatchBar：在「关心的话题」之前加入「追踪关键字」「追踪话题」两个全宽同级分区，每条命中仅为可点击单行索引，定位原 section，不复制正文或常驻理由；无命中分区隐藏。日报时间线每条记录下显示最多两个紧凑命中 tag（余项 `+N`），保持原日期顺序。
- 不做内容流快捷入口（section/话题详情旁「＋关注」）——与文章页标签关注（心形）入口撞心智，用户已否决。

## Capabilities

### Modified Capabilities

- `topic-watch`：watch 实体加 `type` 字段（label/keyword）；命中判定分叉（label 走 AI、keyword 走文本匹配）；keyword 多词逻辑（空格 AND / `|` OR）；管理 API 的 createWatch 加 type 参数。

### Added Capabilities（同 topic-watch capability 内新增 requirement）

- `topic-watch`：keyword 即时匹配（建关注后立刻扫近 14 天）；版块级关注管理面板（创建/管理唯一入口，两类同管）。

## Impact

- **后端**
  - 新增列：`board_topic_watches.type`（label/keyword，CHECK 约束，默认 label）。
  - `BoardTopicWatch.Type` 字段；`CreateWatch` 签名加 type 参数。
  - `EvaluateWatchHits` 分叉：label 类收集走 AI 批量（现有逻辑不变）；keyword 类走纯文本匹配（匹配 threads 标题+摘要，大小写不敏感，多词 AND/OR）。两类命中合并写表。
  - 新增 keyword 即时匹配：建 watch 时（或独立端点）扫近 14 天 section 文本匹配写 hits。
  - 既有 4 端点扩展：createWatch body 加 type；其余 CRUD 对两类一视同仁。
- **前端**
  - `topicWatches.ts`：`createWatch` 加 type 参数；新增即时匹配结果返回。
  - 新建关注对话框（类型双选 label/keyword，keyword 态语法提示 + 实时解析预览 + 回扫说明），挂载于新管理面板。
  - 版块工作台 tab 栏右端「我在追踪 (N)」入口 chip（TagsPage，五 tab 常驻）+ 关注管理面板（新建/列表/暂停/删除/回扫反馈）。
  - 日报时间线列表响应补 active watch 摘要，记录下展示紧凑 tag；以 `DailyReportWatchIndex` 替换 `DailyReportWatchBar`，在详情正文顶部以全宽优先分区呈现两类可定位索引。管理 chip 位置/职责不变，仅防图标与文字换行并调整间距。
- **AI 成本**：keyword 类零 AI 调用（纯文本匹配）；label 类不变。即时匹配零 AI。
- **数据兼容**：`type` 列新增默认 label，历史 watch 全归 label 类，行为不变。

## 依赖与执行顺序

基座现状（2026-08-24 核对）：

- **topic-watchlist-observability 已归档（2026-07-23）**：`board_topic_watches` 表、EvaluateWatchHits AI 单信号、前端 WatchBar 均已在线上代码，可直接开工。其主 spec 归档时遗漏同步到 `openspec/specs/`，本 change 启动前已补回（基线 5 requirement，含被本 change MODIFIED 的 3 个与 REMOVED 的 1 个）。
- **manual-topic-lane 已归档（2026-07-05）**：话题总览工作台已落地；本 change 的管理面板直接落在版块日报面板头部（与话题总览同面），无需另补 lanes 泳道旁入口。

```
topic-watchlist-observability（已归档，基座在线）
  → watch-keyword-and-quickadd（本 change）
```
