# 前端双主题系统（Theming）

> **权威源**：本文件是双主题/设计系统的唯一权威。`front/AGENTS.md` 顶部「Frontend-Specific Conventions」的红线要点深链指向本文件。
> 互补：`architecture/frontend.md` §设计系统 描述设计系统的架构定位。

## 视觉基调

editorial / magazine（杂志感），**禁止回退到蓝紫色 SaaS 视觉**。

## 三层 Token 架构

| 层 | 命名前缀 | 职责 |
|----|---------|------|
| Primitive（Layer 1） | `--raw-*` | 原始色值调色板 |
| Semantic（Layer 2） | `--color-*` | 语义化颜色，组件**只能引用这一层** |
| Component（Layer 3） | `--dialog-*` 等 | 组件级专用变量 |

**硬约束：组件只引用 Layer 2 语义 token，不直接使用原始色值（`--raw-*`）。**

## 主题切换

- 通过根元素 `data-theme="editorial|dark"` 切换
- 使用 `useTheme()` composable 管理主题状态
- 主题变量集中复用 `app/assets/css/main.css`

## 统一组件（必须复用，禁止各写一套）

| 场景 | 组件 |
|------|------|
| 对话框 | `AppDialog` |
| 按钮 | `AppButton` |
| 开关 | `AppToggle` |
| 输入框 | `AppInput` |
| 区块标题 | `AppSectionHeader` |

## 图标

使用 Iconify，不引入新的图标库。

## 资料来源

本规范源自 v1.3.3 架构深化，内容收敛自原 `front/AGENTS.md` §主题系统 与 `development.md` §前端样式约定。
