# APPLY_GUIDE — 实现精确指南（apply 执行用）

> 本文件是 tasks.md 的**工程细化**，裁决了 design/tasks 间的措辞冲突，给出确切代码形态。
> 冲突时：spec.md（行为契约）> 本指南 > tasks.md 描述 > design.md 措辞。
> 子线程按本指南实现；主线程按本指南核验。

## 0. 已裁决的冲突点

| 冲突 | 裁决 |
| ---- | ---- |
| 博查 API host（design `open.bocha.cn` vs tasks `api.bochaai.com`） | **`https://api.bochaai.com/v1/web-search`**（tasks 对；`open.bocha.cn` 是文档站）。endpoint 放进 config 可配置兜底 |
| 配置注入点（"照 airouter provider pattern"） | 实际仓库 airouter key 走 DB aisettings，非 config.yaml。本 change 改走 **`platform/config` viper + env override** 模式（与 server/database 一致），新增 `Bocha` 配置段。**不改 `Init(db)` 签名**，在 `Init` 内全局读 `config.AppConfig.Bocha`（与 `runtime.go` 读 `config.AppConfig.Tracing` 同款） |
| 博查 key | **留配置口，不硬编码、不真调**。`config.yaml` 加 `bocha.api_key=""` + 环境变量 `BOCHA_API_KEY`；空 → 回退 `NoopWebSearcher`。单测一律 mock HTTP，不命中真实博查 |

## 1. 配置层（tasks 1.2）

`backend-go/internal/platform/config/config.go`：

```go
// 加到 Config struct
type Config struct {
    // ...既有字段...
    Bocha BochaConfig
}

// 新增类型
type BochaConfig struct {
    APIKey   string `mapstructure:"api_key"`    // 空 → 降级 Noop
    Endpoint string `mapstructure:"endpoint"`   // 默认 https://api.bochaai.com/v1/web-search
}
```

`LoadConfig` 内 `SetDefault`：
```go
viper.SetDefault("bocha.api_key", "")
viper.SetDefault("bocha.endpoint", "https://api.bochaai.com/v1/web-search")
```

`applyEnvOverrides` 内：
```go
if v := strings.TrimSpace(os.Getenv("BOCHA_API_KEY")); v != "" {
    cfg.Bocha.APIKey = v
}
if v := strings.TrimSpace(os.Getenv("BOCHA_ENDPOINT")); v != "" {
    cfg.Bocha.Endpoint = v
}
```

`backend-go/configs/config.yaml` 末尾加：
```yaml
# 博查 Bocha web search（数据增强 web_search 后端；无 key 时优雅降级回 Noop）
bocha:
  api_key: ""        # 在此填入博查 key，或设环境变量 BOCHA_API_KEY
  endpoint: "https://api.bochaai.com/v1/web-search"
```

## 2. BochaWebSearcher（tasks 1.1, 1.3, 1.4）

`backend-go/internal/dataenrichment/service/web_search.go` 追加：

```go
// BochaWebSearcher 是博查（bochaai.com）通搜（原始网页结果模式）实现。
// 只用通搜 raw results，禁用 AI 总结模式（AI summary 有幻觉风险，不可作可核查证据）。
type BochaWebSearcher struct {
    apiKey   string
    endpoint string
    client   *http.Client // 复用 httpclient.New(WithTimeout(10s))
}

func NewBochaWebSearcher(apiKey, endpoint string) *BochaWebSearcher { ... }

// Search 调博查通搜 endpoint：
//   POST {endpoint}  Authorization: Bearer {apiKey}  body: {"query":q,"summary":false,"count":10,"freshness":"noLimit"}
// 响应解析 data.result[]，每条取 {title,url,summary或snippet→Snippet,site_name}，
// 只保留 url 非空的条目，映射为 WebSearchResult{Title,URL,Snippet}。
// HTTP/解析失败返回 error（executeWebSearch 会降级为错误 JSON）。
func (b *BochaWebSearcher) Search(ctx context.Context, query string) ([]WebSearchResult, error) { ... }
```

要点：
- `summary:false`（强制通搜、禁 AI 总结）
- 响应字段防御：`data.result[]` 里 `snippet` 取 `summary` ?? `snippet` ?? `description`
- 别忘了 import `net/http`、`bytes`、`httpclient`、`time`

`wire.go` `Init` 内（全局读 config，不改签名）：
```go
import "syntopica-backend/internal/platform/config"

// 替换 service.WithWebSearcher(service.NoopWebSearcher{}) 为：
var webSearcher service.WebSearcher = service.NoopWebSearcher{}
if cfg := config.AppConfig; cfg != nil && strings.TrimSpace(cfg.Bocha.APIKey) != "" {
    webSearcher = service.NewBochaWebSearcher(cfg.Bocha.APIKey, cfg.Bocha.Endpoint)
}
toolRegistry := service.NewRegistry(
    service.NewDefaultHTTPFetcher(),
    service.WithWebSearcher(webSearcher),
    // ...其余 option 不变
)
```
（注意 `Init` 当前 import 没有 `strings`/`config`，要补。）

`web_search_test.go` 追加：
- `TestBochaWebSearcher_ParsesResults`：用 `httptest.NewServer` mock 博查响应 `{"code":200,"data":{"result":[{"type":"web_page","title":"T","url":"https://x","summary":"S","site_name":"X"}]}}`，断言解析出 1 条 `{Title:T,URL:https://x,Snippet:S}`
- `TestBochaWebSearcher_HttpError`：mock 500 → 返回 error
- 既有 Noop/降级测试保持绿（key 缺失回退 Noop 由 wire 逻辑体现，单测可直接断言 `NewBochaWebSearcher` 需要 key）

## 3. fetch_page 工具（tasks 1.5, 1.6, 1.7）

新增 `backend-go/internal/dataenrichment/service/fetch_page.go`：

```go
package service

import (
    "context"
    "encoding/json"
    "strings"

    reader "syntopica-backend/internal/reader/service"
)

// PageFetcher 抽象正文抓取（复用 reader readability_crawler）。
type PageFetcher interface {
    FetchPage(ctx context.Context, url string) (title, mainText string, err error)
}

// ReaderPageFetcher 适配 reader.ReadabilityCrawler → PageFetcher。
type ReaderPageFetcher struct{ crawler *reader.ReadabilityCrawler }

func NewReaderPageFetcher() *ReaderPageFetcher {
    return &ReaderPageFetcher{crawler: reader.NewReadabilityCrawler()}
}

func (f *ReaderPageFetcher) FetchPage(ctx context.Context, url string) (string, string, error) {
    res, err := f.crawler.ScrapePage(ctx, url)
    if err != nil { return "", "", err }
    return res.Title, truncateRunes(res.Markdown, 4000), nil
}
```

`tool_registry.go`：
- `Registry` 加字段 `pageFetcher PageFetcher`（可空 → 降级）
- 加 option `WithPageFetcher(pf PageFetcher) RegistryOption`
- `register()` 注册 `fetch_page` 工具：
  ```go
  r.tools["fetch_page"] = &Tool{
      Name: "fetch_page",
      Description: "抓取网页正文(readability),返回 {title,url,main_text}。用于给深度层取一手可核查原文。失败返回错误 JSON,可基于已有数据继续。",
      InputSchema: {"type":"object","properties":{"url":{"type":"string"}},"required":["url"]},
      Execute: r.executeFetchPage,
  }
  ```
- `executeFetchPage`：空 url → 参数错误 JSON；`pageFetcher==nil` → "fetch_page 未配置"；调 FetchPage，超时/反爬失败 → 错误 JSON 不阻断（nil go error）；成功 → `{"title","url","main_text"}`。main_text 已在 FetchPage 内截断。

`wire.go`：`toolRegistry` 的 option 加 `service.WithPageFetcher(service.NewReaderPageFetcher())`。

`fetch_page_test.go`（新）：mock `PageFetcher`（用一个测试用 struct 实现 interface），测：正常返回、fetcher==nil 降级、url 空/无效、超时错误返回错误 JSON 且 go error==nil。

> 注：`internal/reader/service` 导入路径别名。reader 包名是 `service`，会与本包 `service` 冲突，必须用别名 `reader "syntopica-backend/internal/reader/service"`。

## 4. 删除金融方向（tasks 3.1–3.6）

`tool_registry.go`：
- 删 `list_etf_by_keyword`/`get_etf_quote`/`list_sectors` 三个工具注册块
- 删 `executeListETFByKeyword`/`executeGetETFQuote`/`executeListSectors`/`loadETFSpot` 四个方法
- 删 Registry 字段 `etfCacheMu`/`etfCache`/`etfCacheLoaded` 及相关 import（`sync` 若只这处用则删）
- 删 helper `parseFloat`/`toStringSlice`（仅 ETF 工具用；确认 `toUint`/`jsonMarshal` 保留）

`repository/models.go`：
- 删 `SourceTypeETFQuote`/`SourceTypeExchangeRate`/`SourceTypeGDELTEvent` 三个常量
- `validSourceTypes` 清空为 `map[SourceType]bool{}`（保留 SourceType 类型 + ValidateSourceType + CHECK 机制为扩展点；ValidateSourceType 现在对任何值返回 error——这是预期的，内置无金融源）

`board_config.go`：`ToolsForSourceType` 重做——**默认 always-on 集 = 内部导航 + web_search + fetch_page，不再有 per-source_type 条件工具**：
```go
// ToolsForSourceType maps a board_data_sources.source_type to concrete tool names.
// 内置 source_type 已全部移除（金融源删除后无内置源）；返回 nil。
// 机制保留为未来接入结构化外部源的扩展点。
func ToolsForSourceType(sourceType string) []string {
    return nil // 内置无 source_type→工具映射；web_search/fetch_page/导航为 always-on
}
```
（更新函数 doc 注释，删旧的 etf/exchange/gdelt 映射注释）

`orchestrator.go`：
- `explorationToolNames` 改为 `[]string{"list_boards","list_lanes","get_lane_detail","web_search","fetch_page"}`
- `buildAgentAllowedTools` 注释更新（不再提"financial conditional"），逻辑不变（exploration always-on + configured 叠加）
- 删 `import "sync"`（若仅 etfCache 用；check grep 确认）

`board_config_impl.go`：`ToolsForSourceType` 现恒返回 nil → `allowedTools` 恒空；逻辑保留（未来扩展点），但 `GetBoardConfig` 的 `sourceTypes` 查询与 allowedTools 构造保留不动（CHECK 机制在）。

测试更新：
- **删** `financial_tools_test.go`（整文件删——它测的就是被删的 ETF 工具）
- `tool_registry_test.go`：去掉所有 `list_etf_*`/`get_etf_quote`/`list_sectors` 断言；新增 `fetch_page` 在 Tools() 的断言、`web_search` 仍在；"金融工具不可见"改为"未知工具拒绝"断言
- 重写 `financial_tools_test.go` 删除后，把其中 `TestBuildAgentAllowedTools_*` / `TestBuildToolsDesc_*` 这两个**仍然有效**的测试（exploration always-on + 未知工具拒绝）迁到新文件 `allowed_tools_test.go`（package service internal），更新断言：非金融 board 现在也看不到任何金融工具（因为根本没注册），exploration 集含 `fetch_page`

## 5. orchestrator 深度层 + structural 形态（tasks 2.1–2.8）

### 5.1 Depth 结构体（task 2.1）— 加到 orchestrator.go 的 AnalysisBody 区

```go
// Depth 是结构化深度层（"内部看美国"分析基因映射），非 sparse 形态强制产出。
type Depth struct {
    SystemReframe     string             `json:"system_reframe"`     // ②系统重定位：一句话放进哪个大系统讲
    MechanismLayers   []MechanismLayer   `json:"mechanism_layers"`   // ④多层机制拆解
    HistoricalAnalogy []HistoricalAnalogy `json:"historical_analogy"` // ③历史类比
    RegimeShift       *RegimeShift       `json:"regime_shift"`       // ⑥范式转折（可空）
    Boundary          string             `json:"boundary"`           // ⑤反过度解读边界（非空！）
    EvidenceChain     []EvidenceChainItem `json:"evidence_chain"`    // ⑦可核查证据链
}

type MechanismLayer struct {
    Layer      string `json:"layer"`       // 子机制名
    DeepLogic  string `json:"deep_logic"`  // 深层逻辑
    Basis      string `json:"basis"`       // 依据
}

type HistoricalAnalogy struct {
    Case      string `json:"case"`       // 历史案例
    Mechanism string `json:"mechanism"`  // 机制类比
    Diff      string `json:"diff"`       // 何处不同
}

type RegimeShift struct {
    Judgment string `json:"judgment"` // 范式转折判断
    Evidence string `json:"evidence"` // 依据
}

// EvidenceChainItem 是可核查证据条目。source_type ∈ news|web|page。
type EvidenceChainItem struct {
    SourceType  string `json:"source_type"`            // news|web|page
    Ref         string `json:"ref,omitempty"`          // news 引用 id
    URL         string `json:"url,omitempty"`          // web/page 的可核查 URL
    Quote       string `json:"quote,omitempty"`        // 原文摘录（非 AI 转述）
    Institution string `json:"institution,omitempty"`  // 来源机构
    Date        string `json:"date,omitempty"`         // 日期
}
```

给 **4 个非 sparse Analysis 结构体**（EventChainAnalysis / ThemeVeinAnalysis / SinglePointAnalysis / StructuralAnalysis）加字段 `Depth Depth \`json:"depth"\``。
**SparseAnalysis 不加**（sparse 禁深度层）。

### 5.2 structural 形态（task 2.3）

```go
const FormStructural = "structural"

// isValidForm 加 case FormStructural
func isValidForm(f string) bool {
    switch f {
    case FormEventChain, FormThemeVein, FormSinglePoint, FormStructural, FormSparse:
        return true
    }
    return false
}

// StructuralAnalysis 是 form=structural 的 body：结构演化叙述 + 深度层。
type StructuralAnalysis struct {
    EvolutionNarrative string   `json:"evolution_narrative"` // 结构演化叙述
    Phases             []Phase  `json:"phases"`              // 关键阶段
    Depth              Depth    `json:"depth"`
}
type Phase struct {
    Period string `json:"period"`
    Event  string `json:"event"`
    Ref    *Ref   `json:"ref,omitempty"`
}
func (StructuralAnalysis) isAnalysisBody() {}
```

`UnmarshalJSON`（analyzeOutput）加 `case FormStructural` 分支。
`parseAnalyzeOutput` 的 switch 加 `case FormStructural: body = parseStructuralAnalysis(analysisRaw)`。

### 5.3 depth 解析 + 校验（task 2.2）

新增 `parseDepth(m map[string]any) Depth`（解析 6 字段，mechanism_layers/historical_analogy/evidence_chain 用子解析器）。

**校验**（在 `parseAnalyzeOutput` 内，body 解析后）：
- 非 sparse 形态：`depth.boundary` 必须非空（trim 后 != ""）；`depth.system_reframe` 非空；`mechanism_layers` 至少 1 条；`evidence_chain` 至少 1 条。**任一缺失 → 返回 error**（让上层 analyze 重试一次；retry 机制见下）。
- sparse 形态：禁 depth（即使 LLM 给了也忽略，不解析）。

`analyze` 方法加 **重试一次**：parseAnalyzeOutput 返回 error 时，把"上次错误 + 要求修正"拼进 prompt 再调一次 LLM；仍失败则返回 error。实现：把当前 `parsed, err := ParseJSONResponse(...)` + `parseAnalyzeOutput(parsed)` 包成一个内部调用，失败重试一次。

### 5.4 prompt 重写（tasks 2.4–2.7）— 去"A 股/产业"硬编码 + 注入深度层指令

**`interpretPrompt`**（task 2.4）：
- 开头角色 "你是一位资深产业分析师" → **"你是一位结构化分析编辑"**
- 形态判断：四选一 → **五选一**，加 `structural（结构演化）：持续性结构命题、无单一离散事件驱动（如"人民币国际化进程""美元霸权演变"），长时段结构演化`
- 第二步提炼研究方向：删"A 股有对应 ETF 的方向"硬编码 → 改为"提炼需要补数据的**研究方向**（领域自适应：历史机制 / 关键数据 / 可比案例），每个给理由，聚焦 3-5 个"
- 输出 JSON 的 `form` 枚举加 `structural`

**`agentLoopSystemPrompt`**（task 2.5）：
- 角色 "你是一位 A 股数据查询员" → **"你是一位研究助理 / 事实核查员"**
- 删 ETF 工作流（list_etf_by_keyword → get_etf_quote 那套）
- 工作流改为：用 `web_search` 检索背景/历史 precedents/专家分析 → 对关键命中用 `fetch_page` 取一手原文 → 必要时 `list_boards`/`list_lanes`/`get_lane_detail` 查内部脉络。取证支撑分析员的深度层（系统重定位/多层机制/历史类比/可核查证据链）
- 三防御纪律保留（完整结果/不重复调用/取代表性的不要全查）

**`analyzePrompt`**（task 2.6）：
- 角色 "你是一位产业探索判断分析师" → **"你是一位结构化分析师"**
- 开头形态枚举加 structural
- 加 **structural 产出分支**：
  `▶ structural（结构演化）：{"evolution_narrative":"结构演化叙述","phases":[{"period":"...","event":"...","ref":{...}}],"depth":{...}}`
- **非 sparse 形态强制产出 depth 块**：在每个非 sparse 形态的 JSON 示例里加 `"depth":{...}`，并在 prompt 里加【深度层铁律】节：
  ```
  【深度层（非 sparse 形态强制，sparse 不产出）】
  depth 块字段（映射结构化深度分析基因）：
  - system_reframe：一句话——这个话题该放进哪个更大的系统来讲（系统重定位）
  - mechanism_layers：多层子机制拆解，每层给 layer(子机制名)+deep_logic(深层逻辑)+basis(依据)
  - historical_analogy：历史类比，给 case(案例)+mechanism(机制类比)+diff(何处不同)
  - regime_shift：范式转折判断（确实有才填，无则 null）
  - boundary：★反过度解读边界——明确写出"目前还不能下结论的边界"，不可空泛，不可省略
  - evidence_chain：可核查证据链，source_type ∈ news(分层新闻)|web(web_search 网页)|page(fetch_page 正文)；web/page 必须带 url + quote(原文摘录，非转述) + institution(来源机构) + date
  ```
- 见解层铁律保留（evidence 必填、cert 四级、logic）
- 引用格式 source_type 从 `news|tool` 扩展说明 `news|web|page`（深度层用）

**`lensProposePrompt`**（task 2.7, lens_source.go）：
- 角色 "资深产业分析师" → "结构化分析编辑"
- 视角示例从市场事件题 → **结构/系统题**：
  - 好示例："X 为何反复发生"、"X 背后底层结构"、"人民币国际化走到哪一步了"、"美元霸权这次会不会真的动摇"
  - 禁止抽象标签保留
- 加一条：`视角可以是"结构/系统级"问题（长时段机制），不必局限于单一事件`

### 5.5 测试（task 2.8）

`orchestrator_internal_test.go` 更新 + 新增：
- 既有 `TestParseAnalyzeOutput_*`（event_chain/theme_vein/single_point/sparse）：给非 sparse 的测试 JSON **加上 depth 块**（boundary 非空、system_reframe 非空、mechanism_layers≥1、evidence_chain≥1）使其通过新校验；sparse 的不加且断言无 depth
- 新增 `TestParseAnalyzeOutput_Structural`：structural 形态 + depth 解析
- 新增 `TestParseAnalyzeOutput_DepthRequired_NonSparse`：event_chain 无 depth / boundary 空 → 期望 error
- 新增 `TestParseAnalyzeOutput_SparseForbidsDepth`：sparse 带 depth → 忽略，不报错（断言 body 无 depth）
- 新增 `TestParseAnalyzeOutput_EvidenceChainWebPage`：evidence_chain 含 web/page 类带 url+quote
- `TestAnalyzeOutput_JSONRoundTrip`：加 structural case；非 sparse case 补 Depth 字段
- `TestInterpret_FormClassification`：加 structural case

## 6. 前端深度层渲染（tasks 4.1–4.4）

### 6.1 类型（task 4.1）`front/app/api/boardEnrichment.ts`

- `AnalyzeForm` 加 `"structural"`
- `AnalyzeRef.source_type` 扩展为 `"news" | "tool" | "web" | "page"`（深度层 evidence_chain 用 web/page）
- 新增 depth 类型：
  ```ts
  export interface MechanismLayer { layer: string; deep_logic: string; basis: string; [k: string]: unknown }
  export interface HistoricalAnalogy { case: string; mechanism: string; diff: string; [k: string]: unknown }
  export interface RegimeShift { judgment: string; evidence: string; [k: string]: unknown }
  export interface EvidenceChainItem {
      source_type: "news" | "web" | "page" | string;
      ref?: string; url?: string; quote?: string; institution?: string; date?: string;
      [k: string]: unknown;
  }
  export interface AnalyzeDepth {
      system_reframe: string;
      mechanism_layers: MechanismLayer[];
      historical_analogy: HistoricalAnalogy[];
      regime_shift?: RegimeShift | null;
      boundary: string;
      evidence_chain: EvidenceChainItem[];
      [k: string]: unknown;
  }
  ```
- 给 `AnalysisEventChain`/`AnalysisThemeVein`/`AnalysisSinglePoint` 加 `depth?: AnalyzeDepth`（可选，旧结果降级）
- 新增 `AnalysisStructural`：`{ evolution_narrative: string; phases: Array<{period:string;event:string;ref?:AnalyzeRef}>; depth?: AnalyzeDepth }`
- `AnalysisBody` 联合加 `AnalysisStructural`
- 新增 `AnalyzeStructuralOutput { form:"structural"; lens:string; analysis: AnalysisStructural }`
- `AnalyzeOutput` 联合加 `AnalyzeStructuralOutput`

### 6.2 渲染（tasks 4.2–4.4）`CausalAnalysisReport.vue`

- 新增「深度层」渲染区块（additive，不改既有事实层渲染）：系统重定位 / 多层机制（列表，每层 deep_logic+basis）/ 历史类比（case+mechanism+diff）/ 范式转折（regime_shift 非 null 才显示）/ 边界限定（boundary，高亮「还不能下结论」）/ 可核查证据链（evidence_chain 列表，web/page 类 url 可点击 🌐/📄，news 类 📰）
- 旧结果无 depth → 不渲染深度层区块，不报错（降级）
- structural 形态：在现有 switch 里加 `case "structural"` 渲染 evolution_narrative + phases（类似 timeline 渲染）+ 深度层
- `BoardEnrichmentPanel.vue`：按需微调深度层区块布局（若有 wrapper）

## 7. 测试/门禁（§5, §7）

- 后端单测仅影响包：`cd backend-go && go test ./internal/dataenrichment/...`（不跑全量）
- 前端：`pnpm lint`（WSL 可跑）/ typecheck+build+test:unit 用 **Windows cmd**（项目铁律）
- 增量门禁（pi quality-gate 自动）会跑 golangci-lint+vet+build；agent 手动跑 go test + 前端 cmd 命令
- 归档门禁（§11）由主线程跑：全量 `go test ./...` + `go build` + 前端 typecheck/build

## 8. 不变量清单（主线程核验用）

- [ ] `grep -rn "list_etf_by_keyword\|get_etf_quote\|list_sectors\|etf_quote\|exchange_rate\|gdelt_event" backend-go/internal/` → 仅剩 board_config_impl 的 sourceTypes 查询（表机制保留）/ 注释 / 已删测试，**无残留工具注册或 source_type 常量**
- [ ] `grep -rn "NoopWebSearcher" backend-go/internal/dataenrichment/wire.go` → key 空时仍走 Noop（降级在）
- [ ] `grep -n "fetch_page" backend-go/internal/dataenrichment/service/tool_registry.go backend-go/internal/dataenrichment/service/fetch_page.go` → 工具注册 + 实现都在
- [ ] `grep -n "FormStructural\|StructuralAnalysis" backend-go/internal/dataenrichment/service/orchestrator.go` → 形态+结构体+解析都在
- [ ] 非 sparse Analysis 结构体都有 `Depth` 字段；SparseAnalysis 没有
- [ ] 博查 key 未硬编码；config.yaml 有 `bocha:` 段；env `BOCHA_API_KEY` override 在

## 9. 博查 key 界面可配置（追加 task §8，照 Firecrawl 模式）

> 用户追加需求：博查 key 从 config.yaml/env 升级为**界面可配**。照 Firecrawl 已验证的 `ai_settings` 模式。优先级 **DB(UI) > env > config.yaml > 空(Noop 降级)**，动态读（界面改即时生效不重启）。

### 9.1 后端 aisettings 存取（task 8.1）

`backend-go/internal/platform/aisettings/config_store.go` 照 firecrawl 加：
```go
const bochaConfigKey = "bocha_config"

// LoadBochaConfig 读取 bocha_config（数据增强 web_search 后端 key + endpoint + enabled）。
func LoadBochaConfig() (map[string]interface{}, *models.AISettings, error) {
    return loadConfigByKey(bochaConfigKey)
}

// SaveBochaConfig 写入 bocha_config。
func SaveBochaConfig(config map[string]interface{}, description string) error {
    return saveConfigByKey(bochaConfigKey, config, description)
}
```
jsonb 形状：`{"api_key":"sk-xxx","endpoint":"https://api.bochaai.com/v1/web-search","enabled":true}`。

### 9.2 后端 API（task 8.2）

照 `GetRSSHubSettings`/`SaveRSSHubSettings`（在 `admin/handler/`，查 `routes.go:68-71` 的 `/settings/rsshub` 注册）写 `GetBochaSettings`/`SaveBochaSettings`：
- `GET /api/settings/bocha` → 读 `LoadBochaConfig()`，返回 `{api_key(脱敏:有则返回"已配置"或末4位,空则""), endpoint, enabled}`
- `POST /api/settings/bocha` → body `{api_key, endpoint, enabled}`，调 `SaveBochaConfig`，返回成功
- `admin/routes.go` `settings.Group` 加 `settings.GET/POST("/bocha", GetBochaSettings/SaveBochaSettings)`
- `ai_handler.go:308` 的 `GET /settings` 全量返回白名单 `s.Key == "summary_config" || s.Key == "firecrawl_config"` 加 `|| s.Key == "bocha_config"`

> **脱敏**：GET 返回时**不要回显完整 key**（跟其他 secret 设置一致；若 firecrawl/rsshub handler 有脱敏先例就照做，否则返回布尔"已配置" + 末 4 位）。POST 时若 `api_key` 字段为空字符串视为"不改 key"（保留原值），非空才覆盖——避免表单回填空覆盖掉已有 key。

### 9.3 BochaWebSearcher 改动态读（task 8.3）

`service/web_search.go`：`BochaWebSearcher` 从启动时注入 key 改为持 provider：
```go
// BochaConfigProvider 返回当前博查配置（DB 优先 + env/config.yaml 兜底），每次 Search 现读。
// 让界面修改即时生效，无需重启（对齐 Firecrawl 每次 job 读 DB）。
type BochaConfigProvider func() (apiKey, endpoint string)

type BochaWebSearcher struct {
    config BochaConfigProvider
    client *http.Client
}
func NewBochaWebSearcher(provider BochaConfigProvider) *BochaWebSearcher { ... }

func (b *BochaWebSearcher) Search(ctx context.Context, query string) ([]WebSearchResult, error) {
    apiKey, endpoint := b.config()
    if strings.TrimSpace(apiKey) == "" {
        return nil, errors.New("web_search not configured")  // executeWebSearch 降级错误 JSON，同 Noop
    }
    if strings.TrimSpace(endpoint) == "" { endpoint = "https://api.bochaai.com/v1/web-search" }
    // ... 原有 POST + 解析逻辑，用 endpoint/apiKey
}
```

`wire.go`：不再 key 空切 Noop，统一注入 `BochaWebSearcher` + provider：
```go
bochaProvider := func() (string, string) {
    // 1. DB (UI) 优先
    if cfg, _, err := aisettings.LoadBochaConfig(); err == nil && cfg != nil {
        if en, _ := cfg["enabled"].(bool); en {  // enabled 缺省视为 true? 用 v, ok := cfg["enabled"]; !ok || v
            if k, _ := cfg["api_key"].(string); strings.TrimSpace(k) != "" {
                ep, _ := cfg["endpoint"].(string)
                return k, ep  // DB 命中
            }
        }
    }
    // 2. env/config.yaml 兜底
    if c := config.AppConfig; c != nil { return c.Bocha.APIKey, c.Bocha.Endpoint }
    return "", ""
}
toolRegistry := service.NewRegistry(
    service.NewDefaultHTTPFetcher(),
    service.WithWebSearcher(service.NewBochaWebSearcher(bochaProvider)),
    service.WithPageFetcher(service.NewReaderPageFetcher()),
    // ...
)
```
> `enabled` 缺省语义：DB 无 `enabled` 字段或 `enabled=true` 都视为启用（仅 `enabled` 显式为 false 才跳过 DB）。即 `v, ok := cfg["enabled"]; !ok || v.(bool)`。
> `NoopWebSearcher` 保留仅作测试桩（单测直接用它，不依赖 DB/config）。

### 9.4 前端（task 8.5）

照 `SettingsSectionFirecrawl.vue` + `FirecrawlConfigPanel.vue` 模式：
- 新建 `front/app/features/settings/components/SettingsSectionBocha.vue`（薄壳，包 `BochaConfigPanel`）
- 新建 `front/app/components/dialog/BochaConfigPanel.vue`（照 `FirecrawlConfigPanel.vue`，表单：`api_key`(password 输入) + `endpoint`(默认 `https://api.bochaai.com/v1/web-search`) + `enabled`(开关)；onMounted 拉 `GET /api/settings/bocha`；保存调 `POST`）
- `front/app/pages/settings.vue`：`sectionComponents` 加 `'bocha': SettingsSectionBocha` + import
- `front/app/features/settings/components/SettingsWorkspace.vue`：左侧导航 section 列表加 `{ key:'bocha', label:'博查搜索', ... }`（照 firecrawl 项）

### 9.5 测试（task 8.4）

- `config_store_test.go`：`SaveBochaConfig({api_key,endpoint,enabled})` 后 `LoadBochaConfig` 读回字段一致
- `web_search_test.go`：provider 优先级——mock DB 返回 key → 用 DB key；DB 空/err → 用 env/config；全空 → Search 返回 not configured error
- handler 测试：`GET /api/settings/bocha` 返回脱敏 key；`POST` 写入后 GET 读回

### 9.6 文档（task 8.6）

- `configuration.md`：博查 key 主路径改「设置界面」（env 表 `BOCHA_API_KEY` 标注为「部署兜底」）
- `flow/data-enrichment.md` 业务约束 #10：补「DB/UI 可配 + 动态读，优先级 DB>env>config.yaml」

### 9.7 不变量（核验用）

- [ ] `grep -rn "bocha_config" backend-go/internal/platform/aisettings/config_store.go` → key + Load/Save 在
- [ ] `grep -rn "/settings/bocha" backend-go/internal/admin/` → 路由注册 + handler 在
- [ ] `grep -n "BochaConfigProvider" backend-go/internal/dataenrichment/service/web_search.go` → 动态读在
- [ ] `grep -n "LoadBochaConfig" backend-go/internal/dataenrichment/wire.go` → DB 优先读取在
- [ ] 前端设置页有「博查搜索」section；GET 不回显完整 key；POST 空 key 不覆盖原值
