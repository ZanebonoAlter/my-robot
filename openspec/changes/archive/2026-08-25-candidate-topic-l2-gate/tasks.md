# Tasks

## 1. 配置开关

- [x] 1.1 `PersistentTopicConfig` 增加 `CandidateL1GateEnabled bool` 字段：`daily_report_topic_repository.go` 的 `DefaultPersistentTopicConfig` 置 true，`LoadPersistentTopicConfig` 解析 ai_settings 键 `persistent_topic_candidate_l1_gate_enabled`（缺省 true，解析失败回落默认）；补 `TestLoadPersistentTopicConfig_CandidateGate` 三态用例（未配置/显式 true/显式 false），`go test ./internal/topicgraph/repository -run TestLoadPersistentTopicConfig` 通过
- [x] 1.2 确认开关读取路径无遗漏：`grep -n CandidateL1GateEnabled backend-go/internal/topicgraph/` 仅出现在 config 定义、BucketTagsByCentroid 调用点与测试（不进 prompt/落库层）——已复核：仅 `daily_report_topic_repository.go`（定义/默认/解析）+ `daily_report_lane.go:194`（分桶分支）

## 2. 观察期门禁（BucketTagsByCentroid）

- [x] 2.1 用例先行（spec Scenario → 测试）：`daily_report_lane_test.go` 新增四例——candidate 近距离降级 L2（对应「candidate 近距离降级 L2」场景：断言落入 L2 bucket 且携带候选集）、开关关闭回退 L1 直挂（「门禁开关关闭回退旧行为」）、active 近距离保持 L1（既有行为回归）、vacuum active 仍降级 L2（「vacuum active 不直挂」回归）；先写测试跑红
- [x] 2.2 实现：`BucketTagsByCentroid` L1 分支增加 `nearestTopic.Status == TopicStatusActive && cfg.CandidateL1GateEnabled` 条件（开关关闭时保持 active/candidate 均可直挂的旧分支）；跑 `go test ./internal/topicgraph/service -run 'TestBucketTagsByCentroid'` 全绿
- [x] 2.3 集成回归：`go test ./internal/topicgraph/service -run 'TestClusterTagsLane'` 确认 lane 管线下游（L2 prompt 构建、assembleLaneGroups、lane_tier 标注）对新增 L2 流量不破契约（FakeLLM/OffShortlist 两例均 PASS）

## 3. briefs 事实化（ListTopicRecentBriefs）

- [x] 3.1 用例先行：`daily_report_repository_test.go` 改造 briefs 用例——注入内容为 section 当天实际 tag 标签（`TestListTopicRecentBriefs_TagLabelInjection`）、candidate 覆盖（`TestListTopicRecentBriefs_CandidateIncluded`，原 CandidateExcluded 语义反转）、已合并/禁用 tag 过滤（`TestListTopicRecentBriefs_MergedDisabledTagsFiltered`）；另补每 section 截 5 个用例（`_PerSectionTagCap`，cluster_tag_ids 原序）
- [x] 3.2 实现：`ListTopicRecentBriefs` SQL 改为 `cluster_tag_ids` jsonb_array_elements_text WITH ORDINALITY join `topic_tags.label`（active 过滤、每 section 截断 5 个，DISTINCT ON 去重数组内重复），查询范围 `status IN (active, candidate)`；`TopicRecentBrief` 字段改为 `TagLabels []string`（移除 SectionLabel/ThreadTitles）；调用方 `buildL2Prompt`/`buildClusterSystemPrompt` 同步适配；`go test ./internal/topicgraph/repository -run TestListTopicRecentBriefs` 全绿（8 例）
- [x] 3.3 降级契约回归：`TestClusterTagsLane_NilBriefsDegradation`——briefs 查询失败（nil）时 lane 管线不失败，L2 prompt 降 label-only（label/状态在、近期标签块无）。注：`GenerateDailyReport` 直连 airouter 无 mock 面，等价 orchestration 测试取 lane 管线层（briefs 流入点），repo 侧 `_NoTopicsNil` 互补
- [x] 3.4 Slice D 渲染适配：`buildClusterSystemPrompt` 「近期实际内容」改渲染 tag 标签列表（`section (日期): tag1 / tag2`），注入范围扩 active+candidate；`buildL2Prompt` 「近期 section 框架」块替换为「近期 section 实际标签」同格式；单测断言新格式与 candidate 覆盖（`_ActiveTopicWithRecentBriefs`/`_CandidateTopicWithBriefs`/`_ActiveAndCandidateMixed`/`_KeepsTagLabelFingerprintAndMeta`）；hygiene 红线测试 `_ExcludesThreadTitles` 保持通过——thread 叙事两处渲染均已移除

## 4. L2 裁决从严指令

- [x] 4.1 用例先行 + 实现：`buildL2Prompt` system 追加观察期从严段（D5 措辞），`TestBuildL2Prompt_CandidateStrictAdjudication` 断言 system 含「观察中」「从严」、user 含 candidate 近期 tag 标签注入；`go test ./internal/topicgraph/service -run TestBuildL2Prompt` 通过（3 例）

## 5. 测试

- [x] 5.1 影响包全量：`change-scope.sh` 判定（含并行 change 脏文件噪音，本 change 实际=topicgraph/service+repository，admin/tagmanagement 同包编译依赖一并跑）；`go test -short ./internal/topicgraph/... ./internal/admin/... ./internal/tagmanagement/...` 全绿
- [x] 5.2 场景映射对账（Scenario↔测试锚点，无孤儿）：
  - candidate 近距离降级 L2 → TestBucketTagsByCentroid_CandidateNearDistanceDowngradesL2
  - 门禁开关关闭回退旧行为 → TestBucketTagsByCentroid_GateOffRestoresLegacyL1 + TestLoadPersistentTopicConfig_CandidateGate(explicit false)
  - L1 直挂命中（active 保持）→ TestBucketTagsByCentroid_ActiveKeepsL1WithGateOn + _L1L2L3 + TestClusterTagsLane_FakeLLM
  - vacuum active 不直挂 → TestBucketTagsByCentroid_VacuumActiveStillL2WithGateOn + _VacuumDowngrade
  - L2 裁决中观察中话题从严判断 → TestBuildL2Prompt_CandidateStrictAdjudication
  - L2 弱区留/换/新 / 超候选集降级 → TestParseL2Response_* + TestClusterTagsLane_FakeLLM/_L2OnlySwitchOffShortlist（既有，未破坏）
  - candidate 话题近期内容注入 → TestListTopicRecentBriefs_CandidateIncluded + TestBuildClusterSystemPrompt_CandidateTopicWithBriefs + TestBuildL2Prompt_CandidateStrictAdjudication
  - L2 候选预筛注入 → TestBuildL2Prompt_KeepsTagLabelFingerprintAndMeta
  - briefs 查询失败降级 label-only → TestClusterTagsLane_NilBriefsDegradation（+ 既有 TestClusterTags_NilBriefsDegradation）
  - L3 新建/embedding 空/手动改写（既有场景，本 change 未动，既有测试保持绿）
- [x] 5.3 门禁静态检查：`golangci-lint run ./...`（0 issues）+ `go vet ./...` + `go build ./...` 全绿

## 6. 文档

<!-- doc-impact: flow configuration -->
<!-- doc-impact-excuse: api=watch-keyword 并行 change 脏文件（topic_watch_handler.go）; database=narrative 清退并行 change 脏文件（models/narrative*.go）; standard=test-design 文档 change 脏文件（standard/shared/test-design.md）——均非本 change 改动 -->

- [x] 6.1 `docs/reference/flow/daily-report.md`：三层分桶两处（§链路设计 Lane blockquote + 业务约束 2）加 active-only 门禁与开关名；L2 prompt 注入内容块改 tag 标签事实指纹+从严指令；约束 7 补开关键；约束 10 措辞同步（section_label→实际 tag 标签）；新增约束 14（candidate 不享有直挂资格、退场靠 decay 窗口自然滑出、全人工归档主权不变）+ 约束 15（briefs 事实指纹规范）；顺手修正 §概念表 embedding 来源过时表述（标题文本→内容化文本，08-22 遗留）
- [x] 6.2 `docs/reference/configuration.md`：ai_settings 表新增 `persistent_topic_candidate_l1_gate_enabled` 行（默认 true、语义、在线回滚用法、flow 约束 14 引用）

## 7. 验证

- [x] 7.1 `go test ./internal/topicgraph/service -run 'TestBucketTagsByCentroid|TestClusterTagsLane|TestBuildL2Prompt' -v` → 18 个测试全 PASS（含 candidate 降级/开关回退/active 保持/vacuum 回归/降级契约）
- [x] 7.2 `go test ./internal/topicgraph/repository -run 'TestListTopicRecentBriefs|TestLoadPersistentTopicConfig' -v` → 13 个（子）测试全 PASS（tag 标签注入/candidate 覆盖/禁用过滤/截断/开关三态+非法值）
- [x] 7.3 `golangci-lint run ./...`（0 issues）+ `go vet ./...` + `go build ./...` → 零 error；另 `doc-impact.sh verify` 通过（声明 flow configuration + 并行 change 脏文件豁免）
- [x] 7.4 部署后观察（用户操作项，已写入完工汇报）：见完工汇报「部署后影响与需要你做的事」节

## 8. 卡里巴夫复发修复（08-25 上线后诊断，D7/D8）

背景：上线首日观察发现门禁已生效（tag 全走 l2_llm、LLM 裁决语义正确 keep→1151），但 #1032 仍现最新日报——根因见 docs/research/candidate-topic-l2-gate-rootcause/（parseL2Response keep 吞 target + 同日 briefs 自证回路）。

- [x] 8.1 用例先行（D7）：`TestParseL2Response_KeepHonorsInShortlistTarget`（keep+集内非最近 target → 尊重归属）、`TestParseL2Response_KeepOffShortlistTargetFallsBackNearest`（keep+集外 target → 安全网回最近候选）；先跑红确认田行为复现
- [x] 8.2 实现 D7：`parseL2Response` case "keep" 增加 `rd.TargetTopicID != nil && != nearest && inCandidateSet(...)` 时尊重指定；既有 `_KeepUsesNearest`（无 target）/switch/new/降级用例全绿
- [x] 8.3 用例先行（D8）：`TestListTopicRecentBriefs_ExcludesTodaySections`（昨日 section 注入、当日 section 排除）；先跑红（当日被注入）确认自证回路复现
- [x] 8.4 实现 D8：`ListTopicRecentBriefs` SQL 加 `period_date < today` 上界（`NormalizeReportDate(now)`）；4 处旧用例 seed 日期 today→-1 适配（PerSectionTagCap/MergedDisabled/CandidateIncluded/MultipleTopics）
- [x] 8.5 场景映射对账（新增 Scenario↔测试锚点）：
  - L2 keep 显式指定候选集内话题尊重归属 → TestParseL2Response_KeepHonorsInShortlistTarget
  - L2 keep 集外/空 target 维持最近候选 → TestParseL2Response_KeepOffShortlistTargetFallsBackNearest
  - briefs 排除当日 section（同日重跑防自证） → TestListTopicRecentBriefs_ExcludesTodaySections
- [x] 8.6 全量验证：`go test ./internal/topicgraph/...`（handler/repository/service 三包全 ok）+ `golangci-lint run ./internal/topicgraph/...`（0 issues）+ `go vet` + `go build ./...` 全绿
