## 1. JSON 尾逗号容错

- [x] 1.1 新增 `repairTrailingCommas(src string) string` 纯函数（建议放 `extractor_enhanced.go` 或独立 helper 文件）：剥除对象/数组末元素后的多余逗号（匹配 `,` 后仅空白紧跟 `}`/`]` 的模式）；`parseRawTagObjects` 在 Unmarshal 前调用，Unmarshal 仍失败按原 error 返回
- [x] 1.2 单测：尾逗号对象/数组（含嵌套、多级）、合法 JSON 不变、字符串字面量内 `,}` 不误伤、空对象/空数组、只有尾逗号错误的多标签大 JSON；覆盖 mono 双分支与融合路径共用入口的行为
- [x] 1.3 `go test ./internal/tagmanagement/...` 全绿

## 2. 单标签校验降级

- [x] 2.1 `extractor_section.go` `parseSectionTags`：event/person aux 校验失败 → 保留标签（AuxiliaryLabels 置 nil）+ warning；keyword 缺 description → 跳过该标签 + warning；不再因单标签问题返回 error（JSON 整体解析失败仍返回 error 走重试）
- [x] 2.2 `extractor_enhanced.go` `parseEventPersonTags`：aux 校验失败同样降级保留（与融合路径策略一致）
- [x] 2.3 单测更新：原"aux 校验失败整片报错"断言改为降级断言（标签保留、aux 为空、warning 记录）；新增 keyword 缺 description 跳过用例；混合场景（1 个坏 aux + 2 个好标签同片）产出 3 标签
- [x] 2.4 `go test ./internal/tagmanagement/...` 全绿

## 3. 聚合零产出回落 mono

- [x] 3.1 `article_tagger.go` `tagArticle`：`tagAggregateArticle` 返回 handled=true 且 tags 为空时继续走 mono 提取分支（双分支 LLM → heuristic 兜底），记 info 日志说明回落（含全片失败/空产出区分）
- [x] 3.2 编排单测：全片失败（mock router 全败）→ 走 mono 路径有标签；全片返回空数组 → 同样回落；部分片成功 → 不回落
- [x] 3.3 `go test ./internal/tagmanagement/...` 全绿

## 4. 端到端验证

- [x] 4.1 手动 RetagArticle 114204（派早报，现 0 标签）：验证回落链路生效、产出标签、日志有回落 info 与降级 warning
- [x] 4.2 手动 RetagArticle 104345（周刊）：验证 event 标签存活（对照修复前 30/30 全 keyword）、aux 降级 warning 出现率、尾逗号修复后解析失败率下降

## 5. 门禁与文档

- [x] 5.1 后端门禁：`golangci-lint run ./...` + `go vet ./...` + `go build ./...` + `go test ./internal/tagmanagement/...`
- [x] 5.2 `doc-impact.sh verify` + `check-standards.sh`，按输出补 `docs/reference/flow/reading.md` 打标链路容错行为描述
- [x] 5.3 完工汇报：含部署后影响（event 标签存活率回升、全败文章不再 0 标签、aux 质量轻微下降）、用户需执行的操作（可选 RetagArticle 补标 2 篇派早报）、旧数据降级行为（无）

## 文档

<!-- doc-impact: flow -->
<!-- doc-impact-excuse: api=ai_handler/discovery_handler/scheduler 等 handler 层改动来自并行进行中的其他 change，本 change 未碰 handler/api; database=dataenrichment/migrations 等改动来自并行 change，本 change 零 schema 变更; architecture=runtime.go/pause.go 等改动来自并行 change; configuration=config.yaml 改动来自并行 change，本 change 零配置变更 -->
