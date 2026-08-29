# Tasks: section 展示标题内容化

## 1. LLM 产出扩展（daily_report_llm.go）

- [x] 1.1 threads JSON schema 顶层增加可选 `section_title` 字段（description 含事实锚与「不得复述候选话题名」约束），`buildThreadsPrompt` 补对应指令；promptVersion 常量升至 "5.0"。验证：`go test ./internal/topicgraph/service -run TestBuildThreadsPrompt`（或新增用例断言 prompt 含 section_title 指令）。
- [x] 1.2 `parseThreadsResponse` 解析 `section_title` 并随返回值带出（实现时按现有风格选择返回结构，见 design Open Question）。验证：新增表驱动用例——含 section_title / 缺失 / 空串三种响应，断言解析结果与降级为空的事实。

## 2. 标题解析与兜底链（daily_report_orchestrator.go）

- [x] 2.1 section 构建处实现兜底链：threads `section_title` → 首条 thread `title` → 话题 label → `cluster.GroupName`；删除 `clusterLabel = topic.Label` 的无条件覆盖。验证：新增单测覆盖链上每一级（LLM 标题命中 / threads 空 / 无 thread 且命中话题 / 纯 L3），断言 `ClusterLabel` 取值符合 spec Scenario。
- [x] 2.2 确认 `buildSectionEmbedText` 传入的 `clusterLabel` 为新标题（无需改函数，仅验证调用点传值正确）。验证：现有 embedding 相关测试全绿。

## 3. 用例先行与回归（按 standard/shared/test-design.md 故事锚点）

- [x] 3.1 spec Scenario 映射单测：「命中既有话题的 section 标题反映当天内容」「标题生成失败时降级兜底」「L3 新话题标题行为不变」「话题归属字段不受标题影响」「标题遵守事实锚约束」。验证：`go test ./internal/topicgraph/service -short` 全绿且新用例名与 Scenario 一一对应。
- [x] 3.2 回归验证 lane 归属不受影响：现有 lane/归属相关测试（`daily_report_lane*_test.go`、orchestrator 测试）全绿。验证：`bash scripts/change-scope.sh` 判定影响包后按范围跑 `go test`。

## 4. 门禁与文档

<!-- doc-impact: flow -->

- [x] 4.1 质量门禁：`golangci-lint run ./...`、`go vet ./...`、影响包 `go test`、`go build ./...` 全绿。验证：命令实跑输出为证。
- [x] 4.2 更新 `docs/reference/flow/daily-report.md`：STEP3/约束节补「section 展示标题内容化」表述 + 变更溯源表登记（promptVersion 5.0）。验证：`doc-impact.sh verify` 通过、文档与 spec 表述一致。
- [ ] 4.3 按 §11 归档门禁完成收尾（含「部署后影响」汇报：下一期日报起标题语义变化、历史不回刷、无用户手动操作）。

## 5. 验证（Scenario → 测试映射，scenario-trace 对账）

| Scenario | 测试文件 |
| 命中既有话题的 section 标题反映当天内容 | backend-go/internal/topicgraph/service/daily_report_orchestrator_test.go |
| 标题生成失败时降级兜底 | backend-go/internal/topicgraph/service/daily_report_orchestrator_test.go |
| L3 新话题标题行为不变 | backend-go/internal/topicgraph/service/daily_report_orchestrator_test.go |
| 话题归属字段不受标题影响 | backend-go/internal/topicgraph/service/daily_report_orchestrator_test.go |
| 标题遵守事实锚约束 | backend-go/internal/topicgraph/service/daily_report_llm_test.go |
| 历史数据不回刷 | 人工：部署后抽查历史日报标题未变（无回刷代码即语义保证） |
| 时间线跨天串联不依赖标题一致 | 人工：时间线按 persistent_topic_id 归并同话题多日 section |
