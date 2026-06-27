## Why

PersistentTopic 的 `candidate` 是系统为跨天叙事归属建立的内部观察态，但日报目前把所有非 active 的持久话题直接包装成“突发的新话题”。这把“系统尚未确认归属”误当成“用户值得关注”，每天制造新的注意力入口；同时，长期未复现的 candidate 不会自动退出锚点池，还会持续进入后续聚类上下文。

## What Changes

- 日报取消独立的“突发的新话题”主分区：candidate section 回到普通动态内容流，不再因内部生命周期状态获得高注意力权重。
- 日报的“关心的话题”只承载用户已确认的 active PersistentTopic；candidate、archived 与缺少话题归属的 section 均不得进入该分区。
- 如需表达首次出现，仅使用弱化的“新线索”辅助标记；该标记不得等同于“突发”、不得创建独立泳道，也不得改变内容质量排序。
- 日报分区按报告生成时持久化的阅读语义渲染，避免话题后来归档后反向改变历史日报的分区含义。
- 为 candidate 增加失活归档规则：连续未命中达到配置窗口后自动转 archived，退出可归属锚点池。
- 聚类阶段仅注入 active 话题与仍处于有效观察窗口的 candidate，并对 candidate 数量设置上限；优先保留最近命中、累计命中更高的候选，防止陈旧候选无限占用模型上下文。
- 保持 section → PersistentTopic 的后台归属能力：首次无法稳定归属时仍可创建 candidate，本变更不把持久话题退回每日临时聚类。
- 与 `topic-watchlist-observability` 保持语义边界：用户注意力由明确关注/确认驱动，PersistentTopic candidate 只负责叙事身份观察。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `daily-report-system`: 调整日报内容分区与历史渲染语义，candidate 不再作为“突发的新话题”独立曝光。
- `persistent-topic`: 增加 candidate 失活归档和聚类注入边界，明确内部候选态与用户注意力的隔离。

## Impact

- 前端：`dailyReportMagazine.ts` 的分区模型、日报侧栏和 `DailyReportTopicSection.vue` 的状态文案/弱标记，以及相关 Vitest、E2E 用例。
- 后端：PersistentTopic 生命周期规划、可归属话题查询/聚类注入选择、配置项与相关 repository/service 测试。
- 数据库：需要持久化 section 在报告生成时的阅读分区语义；优先复用现有字段，若不足则新增可空、向后兼容的版本化迁移。
- API：日报详情需返回稳定的阅读分区字段；旧数据缺失时由服务端按保守规则降级，不改变现有字段含义。
- 文档：同步 `docs/reference/flow/`、`architecture/`、`database/`、`api/` 与配置说明中的日报分区和 PersistentTopic 生命周期边界。
- 依赖：不新增第三方依赖，不改变认证、WebSocket 或日报调度方式。
