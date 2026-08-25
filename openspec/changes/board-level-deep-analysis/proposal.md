<!-- constraint-domains: data-enrichment, semantic-board -->

## Why

数据增强（循环 B）当前只有单泳道触发粒度（`EnrichTopic(topicID)`），而用户真实诉求是版块级的跨泳道深度分析：「这 15 条泳道的素材，能立一个什么论」。三个结构性缺陷叠加导致功能「不好用」：① 单泳道粒度对深度剖析通常太窄（`sparse` 形态高频出现自证此点），agent 被迫靠 web_search 硬凑外部素材；② 跨泳道探索是断头路——`list_lanes` 等工具在 prompt 里是「必要时」可选钩子，且 evidence `source_type` 枚举仅 `news|web|page`，agent 查了别的泳道也没有引用槽位，等于白查；③ 分析没有方法论锚——产出质量全靠 LLM 即兴，无立论/论证/升维的结构参考（「内部看美国」式的深度剖析范式）。

## What Changes

- **新增版块级深度分析（EnrichBoard）**：以版块为分析对象、N 条泳道素材为论据的深度剖析。产出 `{thesis, argument, depth, lane_refs}`——一句话命题 + 层级递进的机制层论证 + 深度层（六基因）+ 泳道引用。跨泳道素材是论证的**论据**（织入传导链），不是并列态势条目；这是与「周报式数据堆砌」的本质区别。
- **泳道引用成为 schema 一等公民**：evidence `source_type` 扩 `lane`，digest/版块分析的产物显式挂 lane id，前端可点击下钻泳道/lifeline；不再靠 tool_calls 日志考古。
- **新增全局参考角色库**：独立于版块（不挂 `board_data_sources`）的全局文档库（名称 + markdown 内容 + enabled），分析时注入三角色 prompt 作方法论锚（命题生成模式 / 论证骨架 / 证据纪律 / 反面段）。首份参考角色文档《内部看美国·方法论画像》已由探索阶段产出（见 `docs/research/board-analysis-reference-role/`）。
- **单泳道分析降级为「聚焦放大镜」**：保留现有 EnrichTopic 全链路，但产品定位从「用户盲选话题的主入口」改为「版块分析读后的下钻深挖」——从版块分析的论证段发起，lens 预填自所在论证段。砍掉 3c 的「手动 lens 候选选择 UI」TODO（视角选择下沉到版块级 interpret）。
- **命题生成模式**：版块级 interpret 读全板块素材（态势卡 + 近期 section 事实指纹），按「钩子 × 切角」公式提炼 2-3 个候选命题、自动选最有时效性的深挖；用户不指定分析方向（生成式负担从用户侧移除），只对产出的命题做反应（读 / 追问 / 下钻）。
- **素材装配防 token 爆炸**：版块分析的泳道素材以「态势卡」形态注入（复用循环 A lifeline week 粒度 + 质量加权），需要细节时 agent 经 `get_lane_detail` 下钻——跨泳道工具从「可选钩子」升为「主路径」。
- **泳道质量评估（瘦身版，本期最小实现）**：为探索 agent 提供「哪些泳道值得下钻」的信号（活跃度/信息密度/sparse 历史），仅作素材权重与注入排序，不做治理建议（归档/合并建议后置到独立 change）。

### 明确不做（本期边界）

- 不做泳道治理建议（冗余检测/合并/归档建议状态机）——后置。
- 不做参考角色自动生产线（opencli 拉取 + ASR 转写 + LLM 提炼管线）——工具链已在 research 验证，产品化后置；本期参考角色文档手工录入。
- 不做 FinGenius 辩论清理（遗留代码另行处理）。

## Capabilities

### New Capabilities

- `board-level-analysis`: 版块级深度分析的完整行为——触发与节奏、命题生成（interpret 读全板块素材）、三角色编排的跨泳道主路径、thesis/argument/depth 产物 schema、lane 引用一等公民、态势卡素材装配与质量加权、产物存储与 review 对比复用。
- `reference-role-library`: 全局参考角色库——文档 CRUD、enabled 控制、注入三角色 prompt 的时机与格式、与版块数据源绑定的边界（全局库，非版块绑定）。

### Modified Capabilities

- `data-enrichment`: 单泳道分析的定位与触发关系变化——EnrichTopic 保留但新增「从版块分析下钻触发」入口（lens 预填），独立手动入口保留；evidence `source_type` 枚举扩展 `lane`；「分层上下文驱动的数据增强编排」补充版块级编排的关系声明（单泳道分析从主入口降级为聚焦路径，编排逻辑本身不变）。

## Impact

- **后端**：`internal/dataenrichment/` 为主战场——`service/orchestrator.go`（新增 EnrichBoard 编排 + 版块级 interpret）、`service/exploration.go`（跨泳道工具主路径化）、`service/tool_registry.go`、`repository/`（版块级 result 存储：复用 `topic_enrichment_result` 加 scope 字段或新表，design 决策）、新 reference role 模型与 repository、`handler/`（版块分析触发/查询 + 参考角色 CRUD API）。`ai_call_logs` 新 Operation（`data_enrichment.board_interpret` 等，沿用既有 Operation 注册模式）。
- **前端**：`BoardEnrichmentPanel.vue` 信息架构重排（版块分析为主 tab、单泳道降级为下钻）、版块分析报告渲染组件（thesis/argument/depth/lane_refs 引用点击）、参考角色管理界面（设置页）。
- **数据**：新表或 `topic_enrichment_result` 加列（design 决策）；参考角色库新表；evidence source_type 扩枚举（非破坏性，旧值不受影响）。
- **LLM 成本**：版块级分析单次成本高于单泳道（素材更多、论证更长），靠态势卡压缩 + 质量加权控制；触发节奏 design 定（倾向手动优先 + 可选定时）。
