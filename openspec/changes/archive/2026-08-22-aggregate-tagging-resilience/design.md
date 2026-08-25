# Design: aggregate-tagging-resilience

## Context

aggregate-article-tagging 上线后的实测数据（2026-08-20 ~ 08-21，4 篇 aggregate 文章）：

- 114204/115164（派早报）：7 片全部解析失败 → 整篇 0 标签。聚合路径切片成功后（`handled=true`）无任何兜底，mono 路径的 heuristic 兜底不生效
- 104345/114953（周刊）：15 个标签全 keyword、无 event；18 次片级失败中 aux 校验失败 7 次（全是 event 标签触发，如"华为享界 G9 获批 L3 级自动驾驶测试牌照"），JSON 语法失败（`invalid character ']'` 尾逗号类）11 次
- 首个正文栏目片（tier 0.9、本期主打专题）2/2 零产出：408 期模型返回合法空数组，409 期 JSON 截断 3 连败

现状代码（`backend-go/internal/tagmanagement/service/core/`）：

- `extractor_section.go` `parseSectionTags`：任何标签 aux 校验失败或 keyword 缺 description → `return nil, err` 整片报废，重试 3 次耗尽后由 `tagAggregateArticle` 跳过该片
- `extractor_enhanced.go` `parseEventPersonTags` / `parseKeywordTags`：同样整次 parse 失败（mono 路径靠 heuristic 兜底，损伤较小但同样浪费重试）
- `parseRawTagObjects`：标准 `json.Unmarshal`，对尾逗号零容忍
- `article_tagger.go` `tagArticle`：`tagAggregateArticle` 返回 `handled=true` 但 tags 为空时直接跳过 persist，不回落

## Goals / Non-Goals

**Goals:**

- 聚合文章的 event 标签存活：单标签 aux 不合规不再连坐整片
- 聚合文章零标签绝迹：全片失败回落 mono 路径（含 heuristic 兜底）
- JSON 尾逗号不再导致重试耗尽：解析前容错修复
- 降级行为可观测：每次降级记 warning（含标签名/原因），便于后续统计降级率

**Non-Goals:**

- 不改融合 prompt / 切片器 / score 分层（结构正确，问题在容错）
- 不修"模型返回空数组"的 0.9 缺失（408 期 section 0 是模型行为，不是代码缺陷；observation only）
- 不做 aux 缺失后的异步补齐任务（依赖现有 description 生成链路，本次不新增）
- 不清理存量孤岛标签、不回填历史 aggregate 文章

## Decisions

### D1: 单标签校验失败 → 降级保留标签，丢弃校验不过的部分

**决策**：`parseSectionTags` 中 event/person 的 aux 校验失败 → 该标签保留（AuxiliaryLabels 置 nil）+ warning 日志；keyword 缺 description → 该标签跳过 + warning。**不再返回 error**（error 保留给 JSON 整体解析失败）。

**为什么不是整片重试**：重试 3 次模型大概率仍输出同样的 aux 结构（同一 prompt 同一片文本），实测 7 次 aux 失败全部 3 连败耗尽，重试对这类确定性偏差无效。

**为什么不是放宽校验规则（如 3-5 改 1-5）**：aux 3-5 条约束服务下游匹配质量，放宽规则等于全局降质；丢 aux 保标签的损伤面更小（event 标签本体 + description 仍在，embedding/合并建议链路不依赖 aux）。

**mono 路径同步适用**：`parseEventPersonTags` 同样改为降级保留。理由：同一确定性偏差在 mono 路径同样存在（重试耗尽后走 heuristic，标签质量反而更低），降级保留的 LLM 标签优于 heuristic 猜测。

### D2: 聚合零产出回落 mono 路径

**决策**：`tagArticle` 中 `tagAggregateArticle` 返回 `handled=true, tags 空` 时，继续走 mono 提取分支（双分支 LLM → heuristic 兜底），并在日志记一条 info 说明回落原因（全片失败 or 全部空产出）。

**为什么不是回落 heuristic**：mono 双分支仍是首选（LLM 质量高于 heuristic），heuristic 已是其内置兜底，回落 mono 即自然获得两级兜底。

**为什么零产出而不是片级失败才回落**：全片返回空数组（如 408 期 section 0）也可能是 prompt 与片内容不匹配的信号，此时 mono 双分支对整篇摘要的提取是有效补充；统一"零产出即回落"规则简单且无损。

### D3: 尾逗号修复放在 parseRawTagObjects 入口

**决策**：新增纯函数 `repairTrailingCommas(src string) string`（正则或状态机剥除对象/数组末元素后的逗号），`parseRawTagObjects` 在 Unmarshal 前调用；Unmarshal 仍失败则按原 error 返回。

**为什么不是换 JSON 解析库**：尾逗号是实测主因（11 次语法失败的特征错误），一个 ~30 行的修复函数解决主要矛盾；引入第三方容错 JSON 库（如 json5）增加依赖面，收益不匹配。

**为什么在 parseRawTagObjects 统一做**：mono 双分支与融合路径共用该入口，一处修复三路受益；修复是无损变换（合法 JSON 不含尾逗号，不受影响）。

### D4: 降级不改变重试语义

**决策**：JSON 整体解析失败仍走 3 次重试（模型输出有随机性，重试有效）；降级路径不重试（确定性偏差重试无效，直接入库）。

## Risks / Trade-offs

- **[部分 event 标签无 aux 锚点]** → 下游 SemanticBoard 匹配、升级建议对无 aux 标签覆盖度降低；靠 warning 日志统计降级率，若 >30% 再评估 prompt 修复。embedding 与 description 生成链路不受影响
- **[回落 mono 让全败文章多花 2 次 LLM 调用]** → 仅在全片失败场景触发（罕见），且换来标签从 0 到有，值得
- **[尾逗号修复的正则误伤]** → 修复函数只剥 `,` 后紧跟 `}`/`]`（含空白）的确定模式，单测覆盖嵌套/字符串内逗号/空对象等边界；字符串字面量内的 `,}` 不匹配（正则锚定 JSON 结构字符，用单测验证典型含逗号字符串样例）
- **[keyword 缺 description 被跳过]** → 实测融合路径 keyword description 输出稳定（30/30 全有），此降级纯防御，预期触发率≈0

## Migration Plan

1. 纯代码修复，无迁移；后端发布即生效
2. 存量 2 篇 0 标签派早报（114204/115164）可手动 RetagArticle 补标
3. 回滚：代码回滚即可，无数据影响
