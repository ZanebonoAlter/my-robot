# userguide-directory (delta)

## REMOVED Requirements

### Requirement: Userguide directory structure

**Reason**: user-guide 目录定位错误且已腐烂——4 个文件是 `reference/api/` 的漂移拷贝，`getting-started.md`/`tagging-flow.md` 带未解决的合并冲突提交。"系统能做什么、怎么用"的说明定位由 `docs/reference/flow/` 五段式模板的「需求说明」节承接；API 参考唯一权威源为 `docs/reference/api/`。整个 `docs/user-guide/` 目录删除。

**Migration**: `getting-started` 有效安装/启动内容并入 `docs/reference/development.md`；`tagging-flow`/`content-processing`/`reading-preferences` 仍有效内容并入对应 flow 文档。

### Requirement: Userguide content orientation

**Reason**: 同上，capability 整体移除。

**Migration**: 面向用户的操作说明以 flow 文档「需求说明」节为载体。

### Requirement: Userguide content source

**Reason**: 同上，capability 整体移除。

**Migration**: 面向用户的操作说明以 flow 文档「需求说明」节为载体。

### Requirement: Userguide update mechanism

**Reason**: 同上，capability 整体移除。

**Migration**: 面向用户的操作说明以 flow 文档「需求说明」节为载体。
