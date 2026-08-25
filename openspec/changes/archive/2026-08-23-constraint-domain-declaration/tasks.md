# Tasks — 约束注入业务域显式声明

## 1. Extension 核心改造（constraint-injection.ts）

- [x] 1.1 新增声明解析纯函数：从 proposal.md 文本解析 `<!-- constraint-domains: 域, ... -->` 标记（支持多行合并、容忍空格），返回域名数组；域名合法值取 `docs/reference/flow/*.md` basename
- [x] 1.2 `planInjection` 集成声明注入：档位激活且绑定 change 时读 proposal.md 解析声明域 → 注入对应 flow 文档「业务约束与不变量」节（reason=`declaration`，每回合重解析、不进粘性集合，注入顺序 baseDocs 后）；无声明不注入 + widget 提示「无域声明」；未知域名忽略并提示
- [x] 1.3 命中源收窄：`matchKeywordDocs` 的 matchText 改为仅 `recentInputs.join("\n")`（changeText 退出关键词命中，保留给 `detectStacks` 栈判定）
- [x] 1.4 ASCII 关键词词边界整词匹配：`matchKeywordDocs` 对纯 ASCII 关键词用 `\b${kw}\b` 正则（转义关键词），CJK 关键词保持 `includes`
- [x] 1.5 JIT 映射单一真相源：新增 flow/standard 文档头部 `doc-impact-applies` 标签扫描（mtime 缓存，头部 15 行内 `^doc-impact-applies:`），构建 域文档→pathSignals 映射；`.pi/constraint-injection.json` 的 `jitDocs` 字段删除（airouter 条目已由 ai-logging.md 标签覆盖）
- [x] 1.6 `InjectedDocEntry.reason` 枚举新增 `"declaration"`（事实库记账，design D6）

## 2. 配置与文档标签

- [x] 2.1 `.pi/constraint-injection.json` keywordDocs 泛词清理：删代码标识符类英文词与撞车词（tag/Tag/tags/board/Board/tagmanagement/topic/Topic/topicgraph/digest/Digest/morning/AI/ai-/llm/LLM/模型/摘要），保留 CJK 日常词与强领域英文词（firecrawl/airouter/scheduler/cron/job/discovery/RSS 等），删 `jitDocs` 字段
- [x] 2.2 9 个 flow 文档头部补 `doc-impact-applies` frontmatter 标签：路径前缀取各文档「代码入口」节实际目录（semantic-board→tagmanagement、topic-graph→topicgraph、daily-report→topicgraph+admin/scheduler、content-enrichment→reader+firecrawl 相关、data-enrichment→dataenrichment、ai-summary→platform/airouter、discovery→订阅发现相关目录、reading→reader、scheduler→admin/scheduler），以文档「代码入口」节为准逐一核对
- [x] 2.3 `AGENTS.md` 约束注入段补「业务域声明」一句约定（proposal.md 头部 `<!-- constraint-domains: ... -->`，域名=flow 文档 basename）
- [x] 2.4 `docs/reference/开发执行规范.md` §0.6 编排六步前补「业务域声明」引注
- [x] 2.5 `docs/reference/constraints-index.md` 业务规范节补声明机制说明

## 3. 存量 change 补声明

- [x] 3.1 `watch-keyword-and-quickadd` proposal.md 补 `<!-- constraint-domains: topic-graph, daily-report -->`（watch 评估在 topicgraph 日报管线，两域都沾）
- [x] 3.2 核对存量：`harness-facts-tier-a` 纯工具链不补（widget 提示为预期，V5 已验）；`fix-quality-audit-p0` 实涉业务域，补 `<!-- constraint-domains: reading, content-enrichment, semantic-board -->`（feed 开关/正文补全/tag 计数）

## 4. 测试（烟测，constraint-injection.smoke.cjs 扩展）

- [x] 4.1 声明解析用例：单行多域 / 多行合并 / 未知域名忽略 / 无标记返回空 / 空白容忍
- [x] 4.2 词边界用例：输入含 "stage" 不命中 `tag` 关键词；输入含「标签」命中 semantic-board 域
- [x] 4.3 命中源收窄用例：change 文本含 `digest`/`模型` 不触发关键词命中；recentInputs 含「日报」触发
- [x] 4.4 标签扫描用例：9 个 flow 文档均有 `doc-impact-applies` 标签且所列路径目录在仓库真实存在
- [x] 4.5 声明注入集成用例：mock change 目录带声明 proposal → planInjection 输出含对应 flow 节且 reason=declaration；无声明 → 无 flow 节注入

## 5. 文档

<!-- doc-impact: flow（9 个 flow 文档仅加 frontmatter 元数据行，内容语义不变） -->
<!-- doc-impact-excuse: api=工作区 api 域改动属并行 change（fix-quality-audit-p0 等），本 change 未碰产品代码; database=同上，并行 tagmanagement 工作; architecture=同上，并行工作区脏改动; standard=同上，并行工作区脏改动; configuration=同上，并行工作区脏改动 -->

- [x] D1 上述声明落地：9 个 flow 文档加 `doc-impact-applies` 标签（内容语义不变）；AGENTS.md / 开发执行规范 / constraints-index 补声明约定
- [x] D2 主 spec 同步归档时执行（openspec archive 自动 delta→spec，见验证节）

## 6. 验证

- [x] V1 `bash .pi/extensions/tests/run-smoke.sh` → 91 项断言全过（含 4.1~4.5 新用例），SMOKE OK
- [x] V2 `grep -L "doc-impact-applies" docs/reference/flow/*.md | grep -v README.md` → 无输出（9 文档标签齐全）
- [x] V3 `grep -c "constraint-domains" openspec/changes/watch-keyword-and-quickadd/proposal.md` → 输出 ≥1
- [x] V4 `.pi/constraint-injection.json` 经 `python3 -m json.tool` 解析通过且不含 `jitDocs` 键、不含已删泛词（`模型`/`摘要`/`digest`/`topic` 作为关键词值）
- [x] V5 手动冒烟等效验证（bundle 回放真实 change）：harness-facts-tier-a 零撞车域注入 + 「无域声明」提示；fix-quality-audit-p0 声明三域精确注入：新会话 `/opsx-apply constraint-domain-declaration` → widget 显示「无域声明」（本 change 自身就是纯工具链 change，不涉业务域——这是「无声明=合法」的活例）；再开新会话 `/opsx-apply watch-keyword-and-quickadd` → widget 显示声明域 topic-graph，注入 topic-graph 约束节；对照旧行为：`/opsx-apply harness-facts-tier-a` → 不再注入 semantic-board/topic-graph/daily-report/ai-summary 约束节
- [x] V6 openspec validate：`openspec validate --change constraint-domain-declaration` → 通过
