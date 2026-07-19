# docs-reference-layer Specification

## Purpose

TBD
## Requirements
### Requirement: Reference directory as single truth source
`docs/reference/` SHALL 作为跨里程碑活文档的唯一权威位置。架构文档、API 参考、数据库文档、开发规范 SHALL 只在 reference/ 下维护一份，不在其他位置保留副本。

#### Scenario: Architecture documentation location
- **WHEN** 需要查阅后端架构
- **THEN** 唯一位置为 `docs/reference/architecture/backend.md`，不存在 `backend-go/ARCHITECTURE.md`

#### Scenario: Frontend architecture documentation location
- **WHEN** 需要查阅前端架构
- **THEN** 唯一位置为 `docs/reference/architecture/frontend.md`，不存在 `front/ARCHITECTURE.md`

### Requirement: Reference directory structure
docs/reference/ SHALL 包含以下子目录和文件：
- architecture/ — 系统总览、后端架构、前端架构、数据流、运行时、链路追踪
- api/ — API 参考文档（按路由前缀拆分）
- database/ — 数据库字段参考
- development.md — 开发规范（构建、测试、代码风格、目录约定、提交检查）
- 其他跨里程碑功能指南（configuration.md、deployment.md、testing.md 等）

以下功能说明文档 SHALL NOT 出现在 docs/reference/ 中：
- frontend-features.md — 已移至 docs/archive/，内容拆分到 docs/userguide/
- content-processing.md — 已移至 docs/archive/，用户可见部分拆分到 docs/userguide/reading.md
- reading-preferences.md — 已移至 docs/archive/，用户可见部分拆分到 docs/userguide/reading.md

#### Scenario: Reference directory listing
- **WHEN** 列出 docs/reference/
- **THEN** 可见 architecture/、api/、database/ 目录和 development.md、configuration.md、deployment.md、testing.md、开发执行规范.md 等文件
- **THEN** 不存在 frontend-features.md、content-processing.md、reading-preferences.md

### Requirement: Reference docs are living documents
docs/reference/ 下的文档 SHALL 反映当前系统真实状态。architecture/ 下的代码路径引用 SHALL 与实际代码目录一致。

#### Scenario: Post-milestone reference update
- **WHEN** v1.3 完成并引入新的后端模块
- **THEN** `docs/reference/architecture/backend.md` 更新以反映新模块

#### Scenario: Backend architecture paths are correct
- **WHEN** 阅读 docs/reference/architecture/backend.md
- **THEN** 引用 backend-go/internal/domain/content/ 而非 contentprocessing/
- **THEN** 引用 backend-go/internal/domain/tagging/ 而非 topicanalysis/ 或 topicextraction/

#### Scenario: Frontend architecture routes are correct
- **WHEN** 阅读 docs/reference/architecture/frontend.md
- **THEN** 引用 front/app/pages/tags.vue 作为标签管理页面
- **THEN** 不引用已删除的 pages/digest/ 路由

### Requirement: Getting started promotion
`docs/guides/getting-started.md` SHALL 提升到 `docs/getting-started.md`（docs 根目录），作为新用户入口。

#### Scenario: New user entry point
- **WHEN** 新用户打开 docs 目录
- **THEN** `docs/getting-started.md` 直接可见，无需进入子目录

### Requirement: Flow directory positioning

`docs/reference/flow/` SHALL 定位为「需求说明 + 链路设计 + 业务约束 + 代码索引 + 变更溯源」五位一体的大功能活文档，并承接原 user-guide 的"系统能做什么"说明职责。

每个 flow 文档 SHALL 遵循五段式固定结构：

1. `## 需求说明` — 功能解决什么问题（面向使用视角）
2. `## 链路设计` — mermaid 流程图 + 状态流转
3. `## 业务约束与不变量` — 状态机/幂等/去重/限额等业务红线
4. `## 代码入口` — 后端 handler/service + 前端 feature 入口
5. `## 变更溯源` — archive change 链接表（见《开发执行规范》§12.2）

#### Scenario: Flow 文档五段式结构

- **WHEN** check-standards.sh A 段校验任一 `docs/reference/flow/*.md`（README 除外）
- **THEN** 该文档存在「需求说明」「链路设计」「业务约束与不变量」「代码入口」「变更溯源」五个二级标题

### Requirement: Business constraint ownership

业务约束 SHALL 按类型归属固定位置，不散落多处：

| 约束类型 | 归属 |
| --- | --- |
| 业务不变量（状态机、去重、限额） | flow 文档「业务约束与不变量」节 |
| 跨功能传导耦合（改 A 影响 B） | `architecture/coupling-map.md` |
| 代码写法约束 | `standard/` |
| 数据/测试安全红线 | `standard/backend/testing.md` |

#### Scenario: 新增业务不变量

- **WHEN** 一个 change 引入新的业务不变量（如状态机新状态约束）
- **THEN** 该约束记录在受影响 flow 文档的「业务约束与不变量」节，而非仅存在代码注释中

#### Scenario: apply 时作为约束上下文注入

- **WHEN** 一个 change 的 apply 阶段跑 `doc-impact.sh context` 且其代码改动命中某 flow 的 domain
- **THEN** 该 flow 文档「业务约束与不变量」节内容被 dump 为 apply 上下文（见 doc-impact-gate capability），约束从「被动记录」转为「主动注入」

