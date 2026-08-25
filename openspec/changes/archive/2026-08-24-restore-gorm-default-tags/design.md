# Design — restore-gorm-default-tags

## Context

见 proposal.md Why。排查结论（2026-08-24，retire-narrative-legacy 测试期实测 + git 考古 `a0b03bdc`）：两个测试红分别为 GORM 零值显式 INSERT 语义（status）与 default tag 转义错误（context_layers），DB 层约束与迁移账本本身无恙。

## Goals / Non-Goals

**Goals**
- 两个预存测试红转绿（4 个失败用例：TopicTag status ×1、auxlabel ×3）
- 状态机字段 GORM default tag 恢复 + context_layers tag 转义修复 + constrain helper 参数架空修复

**Non-Goals**
- 不重做 a0b03bdc 的 DDL 治理方向（剥除治理整体保留，只修两个例外）
- 不写新迁移（DB DEFAULT/NOT NULL 已由 20260723/20260724_0001 落地）
- 不回滚存量库已 NOT NULL 的 context_layers/aliases 列（无害保留）

## Decisions

### D1: 状态机字段 default tag 属「功能必需」而非风格

GORM 语义：字段无 `default:` tag 时零值（`""`/`0`/`false`）会被显式写入 INSERT，覆盖 DB DEFAULT。对 status 这类「零值=非法状态」的字段，default tag 是唯一治本手段（该结论在 `topic_tag_default_test.go` 注释与 `20260724_0001` 迁移描述中两次独立记录）。故仅恢复两个 status 字段的 tag；`Source`（`default:llm` 剥除后零值 `""`）等非状态机字段维持剥除现状——零值可被业务容忍或显式设置。

### D2: context_layers 修 tag 转义而非剥 tag

`Aliases`（`default:'[]'`）证明 jsonb + serializer + default 组合可用，问题仅在 `\"` 反斜杠转义损坏 GORM 对 default 串的解析。修正为反引号内原生双引号即可。剥 tag 方案被拒绝：jsonb 列的 GORM default tag 属 a0b03bdc 明确保留的 3 个例外之一（"保留 3 个 jsonb serializer 例外"），本字段应同样享受例外待遇。

### D3: constrain helper 修复不产生 schema 不一致问题

修复后新库重放 `20260723_0001` 时 context_layers/aliases 不再被 SET NOT NULL（尊重 notNull:false）——这是 cols 表声明的本意。存量库（本地/生产）已 NOT NULL，不回滚：修复后无人写 NULL（tag 修复治本），NOT NULL 与 nullable 的差异对应用层不可见。黄金 schema 测试（golden schema）若对这两列 nullable 有隐式断言需同步关注（实测 auxlabel 测试链不依赖列 nullability，仅依赖写入成功）。

## Risks / Trade-offs

- [constrain 修复改变新库重放后的列 nullability] → 见 D3，应用层不可见；若后续有测试断言 NOT NULL 再评估
- [default tag 恢复与「DDL 收敛到显式迁移」方向冲突] → D1 已论证状态机字段属功能例外，spec 同步登记（tagging-domain delta）

## Migration Plan

无迁移。部署即代码生效：新进程的 GORM 写入行为恢复，存量数据不动。

## Open Questions

（无）
