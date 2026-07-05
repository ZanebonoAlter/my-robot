## MODIFIED Requirements

### Requirement: LLM span carries business attributes

Every `Router.Chat` and `Router.Embed` span SHALL include the following business attributes, sourced from the request's first-class fields (**NOT** from `ChatRequest.Metadata`):

- `ai.capability`: from `ChatRequest.Capability` (Chat) / the `capability` parameter (Embed)
- `ai.operation`: from `ChatRequest.Operation` (mandatory, non-empty — enforced by the `ai-logging` capability's forced-Operation requirement)
- `ai.session_id`: from `ChatRequest.SessionID` (set when non-empty; omitted only for non-orchestrated single calls)

`Router.Embed` SHALL be brought to parity with `Router.Chat` — previously (`router.go:191`) Embed spans only carried `ai.capability` and lacked `ai.operation`.

airouter SHALL NOT source these attributes from `ChatRequest.Metadata["operation"]` anymore (deprecates the old `router.go:111-113` weak-read path). Callers transition to passing `Operation`/`SessionID` as first-class fields; this is exercised by the `ai-logging` capability's "现有 AI 调用补齐 Operation 与 SessionID" requirement.

> **为什么 MODIFY 而非 ADD**：现行 spec（`openspec/specs/otel-business-tracing/spec.md`）已要求 Chat span 带 `ai.operation`，只是来源是 `Metadata["operation"]` 且 Operation 非强制。本 delta 把来源改为一等字段、补 session_id、补 Embed parity、并把"漏填静默通过"升级为"airouter 强制拒绝"。属于同一 requirement 的收紧，不新增并列 requirement。

#### Scenario: Chat span carries first-class operation and session_id

- **WHEN** `Router.Chat` is called with `Operation: "daily_report.cluster_tags"` and `SessionID: "daily_report_42_ab12cd34"`
- **THEN** the resulting span has attributes `ai.operation=daily_report.cluster_tags` and `ai.session_id=daily_report_42_ab12cd34`, sourced from the fields, not from Metadata

#### Scenario: Embed span parity with Chat

- **WHEN** `Router.Embed` is called with `Operation: "section.embedding"` and `SessionID` set
- **THEN** the resulting span has `ai.operation` and `ai.session_id` set (previously Embed only carried `ai.capability`)

#### Scenario: Empty operation no longer silently produces untagged span

- **WHEN** `Router.Chat` is called with an empty `Operation`
- **THEN** the call is rejected by airouter (per the `ai-logging` forced-Operation requirement) and no span/LogCall is created — supersedes the prior "LLM call without operation metadata still works" scenario
