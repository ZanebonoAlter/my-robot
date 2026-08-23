## Why

后端启动时对本地模型（llama-server）做一次性健康探测，但大模型加载可能超过 45 秒轮询窗口（尤其 embedding 走纯 CPU 时），启动探测判定不健康后**快照永不刷新**：健康检查 API 一直显示未就绪、分析任务被健康门持续跳过，即使模型实际已加载完成。手动测试模型是通的，但走独立探测路径不更新快照。用户只能靠「恢复分析」间接触发重探，且无任何 UI 提示可手动重试，体验割裂。

排查还发现一个叠加根因：用户配置的**全局出站代理**（`http_proxy_url`，如 Clash 7897）被注入 httpclient 后，**本地回环地址（localhost:8080/8081 的 llama-server）的探测请求也走了代理**，代理对本地地址返回 502，导致探测永远失败——即使定时重探存在也无法自愈。

## What Changes

- 后端新增**定时重探**：健康快照判定 not healthy 时，后台定时器周期性重新探测（复用现有 RunStartupProbe 与互斥/冷却约束），模型加载完成后自动自愈；快照 healthy 后停止定时重探。
- 后端新增**手动重探 API**：`POST /api/ai/health/reprobe` 立即异步触发一次重探（幂等，探测 in-flight 时直接跳过），返回触发结果。
- 前端**手动恢复入口**：设置页「AI 健康状态」卡片与全局「AI 模型未就绪」banner 增加「重新检测」按钮，点击后触发重探并刷新快照展示。
- **修复全局代理污染本地探测**：httpclient 全局代理对回环地址（localhost / 127.x / ::1）一律直连（NO_PROXY 惯例），外部站点抓取仍走代理；本地 LLM 探测/调用不再被代理 502 拦截。

## Capabilities

### New Capabilities

- `ai-health-reprobe`: 健康快照的自动定时重探与手动触发重探（API + 前端入口），实现健康门自愈；本地回环请求绕过全局出站代理（代理污染修复）

### Modified Capabilities

- `ai-model-health`: 「启动时模型健康检测」要求中「检测 SHALL 仅在后端启动时触发，SHALL NOT 周期性复检」改为允许按条件定时复检；「健康快照与查询 API」增加手动重探端点

## Impact

- 后端：`backend-go/internal/platform/aihealth/`（新增定时重探调度 + reprobe 触发函数）、`backend-go/internal/app/runtime.go`（启动定时器）、`backend-go/internal/app/router.go`（注册 reprobe 路由）
- 前端：`front/app/api/aiAdmin.ts`（reprobe API 封装）、`front/app/features/settings/components/SettingsSectionAiHealth.vue`（重新检测按钮）、`front/app/components/ai/AiHealthBanner.vue`（banner 重新检测按钮）
- 复用现有约束不变：探测互斥（probeMu）、10 分钟拉起冷却、45 秒轮询窗口、宽松健康判定
