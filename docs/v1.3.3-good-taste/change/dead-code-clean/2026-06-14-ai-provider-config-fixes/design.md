## Context

`AIProvider` 模型已有 `EnableThinking bool` 和 `APIKey string` 字段，前端面板已有 Thinking toggle 与 API Key 输入。问题不在「字段缺失」，而在「语义与校验错误」：

1. **Thinking**：`openai_compatible.go` 在 `!EnableThinking` 时写入 `payload["reasoning_effort"] = "none"`。但 `none` 不是 OpenAI 合法枚举（合法值为 `minimal|low|medium|high`），对 llama.cpp/DeepSeek 也无意义。目标后端的思考机制各不相同：
   - **llama.cpp `/v1`**：请求侧无法控制，思考内容内嵌 `content` 的 `<think>...</think>` 标签，须后处理剥离。
   - **DeepSeek 官方 API**：reasoner 永远思考、chat 不思考，切换靠 model 名而非请求参数；思考内容在独立字段 `reasoning_content`，不污染 `content`。

   请求侧「关闭思考」对这两个后端都做不到，现有的 `reasoning_effort:"none"` 是无效噪音。

2. **API Key**：`store.go` 强制 `ProviderType != Ollama && APIKey == ""` 报错；前端 `useAIRouterSettings.ts` 有三处同类硬校验。但 `openai_compatible.go` 的 client 层早已正确处理空 key（不发 `Authorization` 头），llama.cpp/vLLM/LM Studio 的 `/v1` 端点本就不要求认证。校验层与 client 层矛盾。

3. **清空语义二义**：`UpdateProvider` 现状是「空串 = 沿用已存 key」（`if req.APIKey != "" { provider.APIKey = ... }`）。用户想把已存 cloud key 的 provider 改成本地无 key 服务时，无法表达「清空」。

目标用户场景：本地运行 llama.cpp（OpenAI 兼容格式，无 key，模型为 Qwen3 等），作为 `digest_polish`/`summary` 等 capability 的 provider。

## Goals / Non-Goals

**Goals:**

- 让 llama.cpp 这类无 key 的本地 OpenAI 兼容服务能正确配置并被路由消费。
- `enable_thinking` 开关对目标后端（llama.cpp、DeepSeek）产生真实可观察的效果（输出内容是否剥离 `<think>`）。
- 提供显式的「清空已存密钥」能力，消除更新语义二义。
- 改动最小化：复用现有字段，不引入新的 provider 子类型、不引入 `reasoning_style` 枚举。

**Non-Goals:**

- **不**做请求侧的思考控制——DeepSeek/llama.cpp 本就做不到，不引入 per-provider 的 `reasoning_style` 适配层。
- **不**消费/回传推理内容到前端或 `AICallLog`——推理内容仅作清理，不持久化、不展示（用户明确不需要）。
- **不**引入 thinking budget / `thinking_budget` 字段。
- **不**改变配置粒度（保持 per-provider，不引入 per-capability thinking 配置）。需要差异化时，靠「配不同 provider + 路由选它」实现。
- **不**新增 provider 类型（不引入 `local` 类型，llama.cpp 继续归 `openai_compatible`）。
- **不**做 DB schema 迁移（PostgreSQL 空串本就可写入 NOT NULL 列，仅 GORM 标签表意对齐）。

## Decisions

### D1. `enable_thinking` 重定义为「输出清理标记」，尽力而为语义

**决策**：删除请求侧的 `reasoning_effort:"none"`；`enable_thinking` 仅控制 `stripThinkTags` 是否在响应处理时被调用。

| `enable_thinking` | 请求侧 | 响应侧 |
|---|---|---|
| `true` | 不发送任何 thinking 参数（交给模型默认） | 调用 `stripThinkTags` 剥离 `<think>` |
| `false` | 不发送任何 thinking 参数 | 跳过 `stripThinkTags` |

**Rationale**：对 llama.cpp，剥离是唯一有效作用点；对 DeepSeek，`reasoning_content` 独立字段不污染 `content`，剥离调用天然无害（空操作）。请求侧对两者都无法控制，发送参数纯属噪音且 `none` 是非法枚举。

**Alternatives considered**：
- *发 `enable_thinking` bool（Qwen3 风格）*：仅 Qwen3 兼容，DeepSeek/llama.cpp 原生忽略，收益窄且增加分支。拒绝。
- *加 `reasoning_style` 字段做 per-provider 适配*：工程上最干净，但当前仅 2 个目标后端且都做不到请求侧关闭，收益不抵复杂度。未来若引入 OpenAI o 系再做。
- *彻底删掉 `enable_thinking` 字段*：会丢失「这个 provider 输出可能含 think 标签」的语义，导致 llama.cpp 输出污染下游。保留作清理标记更安全。

### D2. API Key 校验放宽：client 层已正确，拆掉校验层即可

**决策**：删除 `store.go` 的 `ProviderType != Ollama && APIKey == ""` 校验；删除前端三处同类校验。client 层 `if provider.APIKey != ""` 已就绪，无需改动。

**Rationale**：校验层与 client 层矛盾，拆校验是零风险操作。误填（cloud 服务漏填 key）靠现有的「测试连接」按钮抓 401 兜底——这是用户已有的工作流。

**Alternatives considered**：
- *按 base_url 启发式（localhost 才允许空 key）*：增加隐式规则，且 docker/k8s 环境下本地服务未必是 localhost。拒绝，信任用户 + 测试连接更简单。
- *新增 `local` provider 类型*：污染类型语义，llama.cpp 本就是 OpenAI 兼容格式。拒绝。

### D3. 清空密钥用显式 `clear_api_key` 字段，而非 sentinel

**决策**：`UpsertProviderRequest` 增 `clear_api_key bool`。`UpdateProvider`：`clear_api_key==true` → `provider.APIKey = ""`；否则沿用「空串=不变」。

**Rationale**：sentinel（如 `"__CLEAR__"`）有碰撞风险且需双方约定；placeholder「留空=沿用」语义必须保留（否则每次保存都得重输 key）。显式布尔字段语义清晰、无歧义。

**Alternatives considered**：
- *sentinel 字符串*：碰撞风险、调试困难。拒绝。
- *改语义为「空串=清空」*：破坏所有现有前端调用，每次保存都得带 key。拒绝。

### D4. 前端「清除密钥」按钮仅出现在编辑备用 provider 表单

**决策**：主模型表单不加（主模型通常不轮换 keyless/local）；新增 provider 表单不加（新建时本就无 key 可清）。仅 `AIRouterBackupProviders.vue` 的编辑表单加，且仅当该 provider 的 `api_key_configured===true` 时显示。

### D5. GORM `not null` 标签对齐

**决策**：`AIProvider.APIKey` 的 `gorm:"type:text;not null"` 改为 `gorm:"type:text"`。PostgreSQL 层面空串本就可写入 NOT NULL 列，无数据迁移，仅标签表意对齐。

## Risks / Trade-offs

- **[cloud 用户漏填 key 不再被前端拦截]** → 「测试连接」按钮 + 后端 401 报错仍是兜底；UI 上保留 API Key 输入框不删除，只是不再 required。
- **[`enable_thinking=false` 时 llama.cpp 输出含 `<think>` 污染下游]** → 这是用户显式选择（关闭清理）。UI 文案需提示「关闭后若模型输出 think 标签将进入结果」。
- **[DeepSeek `enable_thinking` 语义名不副实]**（它关不掉思考）→ 接受尽力而为。spec 明确写「不保证请求侧关闭」。UI 文案调整为「输出清理」语义而非「开关思考」。
- **[DB NOT NULL 标签移除在全新部署时列定义变化]** → PostgreSQL 空串合法，不影响已有数据；新部署的列变为 nullable，语义更宽松，无回滚风险。
