# 日报 / Digest 流程（Daily Report）

> 大功能：Digest 预览/查看/立即生成 + 每日叙事生成。
> 跨端。互补：`flow/topic-graph.md`（日报与话题归属）、`architecture/tracing.md`。

## Digest 预览/查看链路

```text
DigestListView
  → getStatus()
  → getPreview(daily|weekly, date)
  → 左栏分类 + 中栏 summary 列表 + 右栏详情
  → runNow() 可立即生成新版本
  → DigestDetail 按 article_ids 拉关联文章
  → 关联文章在弹窗中复用 ArticleContentView
```

## 每日叙事生成（后端）

```mermaid
flowchart TD
  SCHED[daily_report scheduler 触发] --> JOB[job_daily_report.go 执行]
  JOB --> COLL[daily_report.CollectBoardIDsForDate date]
  COLL --> RA[读取 active SemanticBoard<br/>label_type=board, status=active]
  RA --> EVT[按 date+scope+semantic_board_id<br/>收集 event tags]
  EVT --> GEN{每个有事件的 Board}
  GEN --> GR[GenerateDailyReport ctx, boardID, date]
  GR --> SAVE[SaveReport report, sections, threadBatches]
  SAVE --> FALL[runFallbackAssociations 关联前日叙事]
  FALL --> DERIVE[DeriveBoardConnections 派生 Board 连接]
  DERIVE --> FB[runFeedbackFromTodayNarratives 反馈标签]
  FB --> CLEAN[cleanEmptyBoards 清理空 Board]
```

## 代码入口

- 后端：`internal/admin/`（scheduler, daily_report job）、`internal/topicgraph/`（daily_report service/repository）
- 前端：`front/app/features/ai/`、`front/app/features/articles/`

## 资料来源

迁自原 `architecture/data-flow.md`（Digest 流 / 叙事数据流·每日叙事生成）。
