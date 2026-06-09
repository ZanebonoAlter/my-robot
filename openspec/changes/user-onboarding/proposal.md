## Why

Syntopica 目前没有任何用户指引机制——首次使用的用户直接面对完整的应用界面，没有引导流程、功能提示或空状态说明。对于信息聚合 + AI 标签系统这种复杂产品，缺少用户引导会显著增加学习成本和流失率。

## What Changes

- 首次使用引导流程（First-run experience）：
  - 检测用户是否首次访问（基于 localStorage 标记）
  - 分步引导用户完成核心操作：添加 RSS 源 → 等待内容抓取 → 查看标签/话题图 → 查看日报
  - 使用 overlay + 高亮步骤式引导（参考 driver.js 或类似方案）
- 功能发现提示：
  - 关键功能入口增加首次点击时的简短说明 tooltip
  - 如：话题图的筛选控件、版块管理入口、日报生命周期查看等
- 空状态引导：
  - 当用户尚未添加 RSS 源时，显示引导卡片
  - 当日报为空时，显示"等待数据积累"的说明
  - 当话题图为空时，提示可能的原因和操作建议

## Capabilities

### New Capabilities

- `user-onboarding`: 首次使用引导流程，包括分步教程、功能发现提示和空状态引导

### Modified Capabilities

（无现有能力的需求变更）

## Impact

- `front/app/composables/`：新增 `useOnboarding` composable
- `front/app/features/feeds/`：空状态引导卡片
- `front/app/features/topic-graph/`：功能发现提示
- `front/app/features/tags/`：功能发现提示
- 新增 npm 依赖（driver.js 或类似引导库）
- 后端无需改动
