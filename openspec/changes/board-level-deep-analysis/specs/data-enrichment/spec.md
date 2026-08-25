## MODIFIED Requirements

### Requirement: 分层上下文驱动的数据增强编排

数据增强编排的入口 SHALL 是分层上下文（`topic_lifeline_context` + 14天窗口详情 + 历史 applied review），**不是单篇新闻，也不是单一 lifeline**。本条描述单泳道（topic 粒度）编排；版块粒度编排见 `board-level-analysis` capability，两者共享三角色骨架与防御机制，单泳道编排在产品定位上为版块分析的下钻聚焦路径（行为本身不变，独立手动入口保留）。编排 SHALL 由三角色组成：

1. **解读员（结构化分析编辑）**：全层读分层上下文（按板块 `context_layers`，未生成的层跳过），提炼需补数据的**研究方向**（领域自适应：历史机制 / 关键数据 / 可比案例，不限于金融产业方向），输出 JSON。SHALL NOT 硬编码提炼"A 股 ETF 方向"。从版块分析下钻触发时，解读员 SHALL 接收预填 lens（来自版块报告论证段命题，用户可改），SHALL NOT 再要求用户选择视角候选。
2. **研究助理（agent loop）**：对每个研究方向用 `web_search` + `fetch_page` + 内部导航工具链式检索，搜集背景事实、历史 precedents、专家/一手分析，喂给分析员的深度层。
3. **分析员（结构化分析师）**：结合分层上下文 + 检索数据，按形态+视角产出**事实层 + 深度层**（见『分层见解产出』『分析深度层产出』）。SHALL 显式产出反过度解读边界（`boundary`）。

编排 SHALL 对单次 LLM 调用设 max_loops 上限（默认 6）防止无限循环。解读员 SHALL 读取历史 applied review 以避免重蹈已知偏差。**SHALL NOT 产出** 旧主线的走向预测（`direction` / `confidence` / `horizon` / `trigger_up` / `trigger_down` 字段已废弃）。

#### Scenario: 消费分层上下文

- **WHEN** 触发某 topic 的数据增强
- **THEN** 解读员输入 SHALL 含表1 context（按 context_layers）+ 14天窗口详情 + 历史 applied review，不得为单篇 article 原文

#### Scenario: 解读员领域自适应

- **WHEN** 解读员处理非金融结构话题（如"人民币国际化进程"）
- **THEN** 提炼的研究方向 SHALL 覆盖历史机制/关键数据/可比案例，SHALL NOT 强制提炼 A 股 ETF 方向

#### Scenario: 分析员产出深度层而非走向预测

- **WHEN** 分析员产出非 sparse 形态结果
- **THEN** 结果 SHALL 含深度层（`depth` 块），SHALL NOT 含 `direction`/`trigger_up`/`trigger_down` 字段

#### Scenario: 死循环防御

- **WHEN** 研究助理尝试用相同参数重复调用同一工具
- **THEN** 系统 SHALL 拦截并返回"已调用过"提示，不执行重复调用

#### Scenario: 下钻触发预填 lens

- **WHEN** 用户从版块分析报告的论证段泳道引用发起单泳道分析
- **THEN** 解读员以预填 lens（该论证段命题）启动，不要求用户另选视角；用户可在产出前修改

## ADDED Requirements

### Requirement: 泳道证据引用槽位

见解与论证的证据 `source_type` 枚举 SHALL 为 `news | web | page | lane`。`lane` 类证据 MUST 携带泳道标识（lane id）与引用说明，供前端点击下钻；既有三值证据行为不变。

#### Scenario: lane 类证据下钻

- **WHEN** 分析产物中存在 `source_type=lane` 的证据
- **THEN** 该证据携带泳道 id，前端可解析并跳转对应泳道

#### Scenario: 旧枚举不受影响

- **WHEN** 读取仅含 `news|web|page` 证据的历史结果
- **THEN** 解析与渲染行为与扩展前一致
