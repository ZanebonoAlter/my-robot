# Delta Spec: frontend-narrative-cleanup (code-cleanup-dead)

## REMOVED Requirements

### Requirement: 死代码叙事 API 模块 useNarrativeApi

移除 `front/app/api/topicGraph.ts` 中 `useNarrativeApi()` 函数及其全部 8 个 API 方法。此函数已定义但从未被任何页面或组件调用。

#### Scenario: useNarrativeApi 不存在
- **WHEN** 检查 `front/app/api/topicGraph.ts` 中的导出函数
- **THEN** 不存在 `useNarrativeApi` 函数

#### Scenario: 死代码 API 方法不存在
- **WHEN** 检查 `front/app/api/topicGraph.ts`
- **THEN** 不存在以下方法：`getNarratives`, `getNarrativeTimeline`, `getNarrativeHistory`, `deleteNarratives`, `getNarrativeScopes`, `regenerateNarratives`, `getBoardTimeline`, `getBoardDetail`

### Requirement: 死代码叙事类型定义

移除 `front/app/api/topicGraph.ts` 中仅被死代码 API 和组件使用的类型定义：`NarrativeItem`, `NarrativeTimelineDay`, `NarrativeScopesResponse`, `BoardNarrativeItem`, `BoardItem` (叙事相关), `TagBrief` (叙事相关), `BoardTimelineDay`。

#### Scenario: 死代码叙事类型不存在
- **WHEN** 检查 `front/app/api/topicGraph.ts` 中的类型定义
- **THEN** 不存在 `NarrativeItem`, `NarrativeTimelineDay`, `NarrativeScopesResponse`, `BoardNarrativeItem` (叙事专用), `BoardTimelineDay` 类型

### Requirement: 死代码组件 NarrativeDetailCard

移除 `front/app/features/topic-graph/components/NarrativeDetailCard.vue` 整个文件。此组件零导入，从未在任何页面或布局中使用。

#### Scenario: NarrativeDetailCard.vue 文件不存在
- **WHEN** 检查 `front/app/features/topic-graph/components/` 目录
- **THEN** 不存在 `NarrativeDetailCard.vue` 文件

### Requirement: 死代码组件 BoardNarrativeTimeline

移除 `front/app/features/tags/components/BoardNarrativeTimeline.vue` 整个文件。此组件零导入，从未在任何页面或布局中使用。

#### Scenario: BoardNarrativeTimeline.vue 文件不存在
- **WHEN** 检查 `front/app/features/tags/components/` 目录
- **THEN** 不存在 `BoardNarrativeTimeline.vue` 文件

### Requirement: 死代码 semanticBoards API 方法

移除 `front/app/api/semanticBoards.ts` 中 `getBoardNarratives` 和 `triggerNarrativeGeneration` 方法。`getBoardNarratives` 仅被已删除的 `BoardNarrativeTimeline.vue` 调用，`triggerNarrativeGeneration` 零调用者。

#### Scenario: getBoardNarratives 不存在
- **WHEN** 检查 `front/app/api/semanticBoards.ts`
- **THEN** 不存在 `getBoardNarratives` 方法

#### Scenario: triggerNarrativeGeneration 不存在
- **WHEN** 检查 `front/app/api/semanticBoards.ts`
- **THEN** 不存在 `triggerNarrativeGeneration` 方法

### Requirement: 死代码 semanticBoards 类型

移除 `front/app/api/semanticBoards.ts` 中仅被已删除方法和组件使用的 `BoardNarrative` 和 `BoardNarrativeTag` 类型。

#### Scenario: 死代码 Board 类型不存在
- **WHEN** 检查 `front/app/api/semanticBoards.ts` 中的类型定义
- **THEN** 不存在 `BoardNarrative` 和 `BoardNarrativeTag` 类型（如果仅被已删除的方法/组件使用）

### Requirement: 未使用的 TopicKind 导入

移除 `front/app/features/topic-graph/pages/TopicGraphPage.vue` 中未使用的 `TopicKind` 导入。

#### Scenario: TopicKind 未被导入
- **WHEN** 检查 `TopicGraphPage.vue` 的 import 语句
- **THEN** 不存在 `TopicKind` 导入（如果未被实际使用）
