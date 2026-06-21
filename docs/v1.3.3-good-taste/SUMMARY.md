# Milestone v1.3.3 — Good Taste

**生成时间:** 2026-06-15
**用途:** 当前活跃里程碑说明
**状态:** 开发进行中

---

## 1. 项目概览

**里程碑名称:** v1.3.3-good-taste
**核心价值:** 架构深化、性能与功能缺陷修复、UI 一致性、死代码清理与测试标准化。不新增大功能，聚焦让现有系统"更好维护、更快、更规范"。
**部署模式:** 个人/单用户，无认证系统，PostgreSQL + pgvector 持久化。

---

## 2. 变更清单

### 顶层 changes（4 个，按主题）

| Change | 主题 | 说明 |
|--------|------|------|
| `2026-06-11-deepen-architecture` | 架构深化 | 模块边界、重复代码分散、接口不规范；后端用 `interface{}` 全局变量打破循环引用，前端四个独立 WebSocket 连接各自为政 |
| `2026-06-14-board-embedding-perf` | 版块 embedding 性能 | `MatchTopicTag` 全量加载 O(N×T×B×d) 暴力比较、HNSW 索引被删未重建、版块升级只支持"发现新版块"模式缺"扩充已有版块方向"、backfill 逐条顺序无法并行 |
| `2026-06-14-scheduler-cron` | 定时任务缺陷 | `next_run` 状态字反了、DailyReport 永不自动执行、4 个调度器执行结果不落库、前端面板几乎不展示信息 |
| `2026-06-14-unify-ui-components` | UI 组件统一 | 三大入口页 + GlobalSettings 的 UI 分裂：四套对话框、三种 toggle/按钮/输入框样式、两套视觉语言、暗色全硬编码 |
| `2026-06-19-persistent-topic` | 持久叙事话题 | 板块下话题关联纯靠匈牙利+embedding 漂移散乱；新增 PersistentTopic 持久层（强制 1:N 归属 + 自动升级 + 身份边），ClusterTags 注入历史框架，侦探墙生命周期升级为话题生命线 |

### 分组 changes

| 分组 | 包含 change 数 | 说明 |
|------|---------------|------|
| `changes/dead-code-clean/` | 4 | 死代码与孤立配置清理（code-cleanup-dead、ai-provider-config-fixes、remove-dead-embedding-config、wire-orphan-capability-routes） |
| `changes/test-standard/` | 2 | 测试标准化与修复（test-standardize、test-tagmanagement-fixes） |

### 零散区 `other/`

| Change | 说明 |
|--------|------|
| `entity-aware-tag-merge` | 标签实体抽取与合并身份保护 |
| `tag-merge-ux-polish` | 标签合并 UI 打磨 |

---

## 3. 目录说明

| 子目录 | 用途 |
|--------|------|
| `changes/` | 从 openspec archive 归类来的 change（顶层 + 二级主题分组） |
| `changes/<分组>/` | 按主题归类的多 change 集合（如 dead-code-clean、test-standard） |
| `other/` | 尚未归入正式分组的零散 change |

> 里程碑目录结构约定见 `docs/reference/开发执行规范.md` §12 文档流转。

---

## 4. 里程碑完成标准

- [ ] 架构深化（deepen-architecture）Phase 2+ 完成
- [ ] 版块 embedding 性能问题修复（board-embedding-perf）
- [ ] 定时任务四项缺陷修复（scheduler-cron）
- [ ] UI 组件统一落地（unify-ui-components）
- [ ] 死代码清理与测试标准化收尾
- [ ] `docs/reference/` 反映本里程碑最终代码状态
