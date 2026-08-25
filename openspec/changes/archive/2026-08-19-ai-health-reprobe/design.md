## Context

后端启动时 `aihealth.RunStartupProbe` 对每条路由主 provider 做一次探测（GET `{base_url}/models`），快照存内存。现有 spec 明确「仅启动时探测、不周期复检」，而本地大模型（llama-server）加载可能超过 45 秒轮询窗口（embedding 实例 -ngl 0 纯 CPU 更慢），启动探测失败后快照无任何刷新路径——健康门持续判不健康、分析任务不跑，但手动测试（独立 TestConnection 路径）是通的，证明模型实际可用。

现有可复用机制：`probeMu` 全局互斥（in-flight 时新探测直接跳过）、`launchCooldown` 10 分钟拉起冷却（冷却内不重复执行 start_command，只轮询）、`pollProbe` 45 秒轮询窗口、`SetSnapshotForTest`/`SetProbeFnForTest` 测试缝。

## Goals / Non-Goals

**Goals:**
- 快照 not healthy 时后台定时重探，模型加载完成后自动自愈，无需用户干预
- 提供手动重探 API（POST /api/ai/health/reprobe），前端设置页与全局 banner 提供「重新检测」入口
- 复用现有互斥/冷却/轮询约束，不改变健康判定口径与自动拉起行为

**Non-Goals:**
- 不监控「已健康后模型又挂掉」的场景（快照 healthy 后定时器保持空闲；模型挂掉仍由分析任务报错暴露）
- 不把快照持久化（仍是内存态）
- 不调整 45 秒轮询窗口 / 10 分钟冷却 / 健康判定口径
- 不托管被拉起进程生命周期（维持现状）

## Decisions

**D1: 定时重探 = 包级 goroutine + Ticker，仅快照 not healthy 时探测**
`aihealth.StartPeriodicReprobe(ctx, store, autoStart)` 在 runtime.go 启动时以 goroutine 运行：Ticker 每 `reprobeInterval`（默认 60s，包级变量供测试缩短）触发，先读快照，healthy 则跳过（空闲）；not healthy 则调 `TryStartProbe`。停止时机 = 后端退出（ctx cancel）。健康后不再探测符合 spec「保持已健康状态」，且避免对远程 provider 做无谓请求。
- 备选：健康后 sleep 长间隔——无实质收益，Ticker + 跳过更简单。
- 备选：做成 scheduler 注册项——需走 SchedulerTask DB 持久化/UI 展示，本功能无状态、无需展示，独立 goroutine 更轻。

**D2: RunStartupProbe 拆出锁内执行体，新增 TryStartProbe 返回是否真的启动**
现有 `RunStartupProbe` 保持签名不变（TryLock 失败打日志跳过），内部逻辑提取为 `runProbeLocked`。新增 `TryStartProbe(ctx, store, autoStart) bool`：TryLock 成功 → go runProbeLocked → 返回 true；失败 → 返回 false。定时重探、手动 reprobe handler、以及前端「恢复分析」触发路径共用它，in-flight 语义一致（返回 false = skipped）。避免 handler 侧自己 TryLock/解锁造成竞态窗口。

**D3: 手动重探 API 返回 triggered/skipped，不等待探测完成**
`POST /api/ai/health/reprobe` handler 调 `TryStartProbe`（异步），返回 `{triggered: bool, skipped: bool}`（skipped = !triggered）。前端随后轮询 `GET /api/ai/health` 直到 `checked_at` 更新或超时（~30s），期间按钮显示「检测中」。端点不读分析暂停状态（spec：不改变暂停状态、不依赖用户点过恢复）。

**D4: 前端两个入口共用同一 reprobe + 轮询刷新逻辑**
设置页健康卡片加「重新检测」AppButton；全局 banner（AiHealthBanner）加「重新检测」小按钮，点击后触发 reprobe 并刷新 scheduler status（banner 可见性由 `useSchedulerStatus().aiHealthy` 驱动，需要重新拉取 status）。逻辑提取为 composable `useHealthReprobe`（reprobe → 轮询快照 → 返回结果），两处复用。

**D5: 全局代理对回环地址直连（代理污染修复）**
`httpclient.SetProxy` 安装的 Proxy 函数对回环目标（`localhost`、`127.0.0.0/8`、`::1`，hostname 为空视为本机）返回 nil（直连），其余目标仍走代理。这是 NO_PROXY 惯例（浏览器/Clash 客户端均如此）：本地 llama-server 探测/推理请求不被代理 502 拦截，feed 抓取/Firecrawl 外部请求继续走代理。
- 备选：仅在 aihealth 探测里用直连 client——无法覆盖 LLM 推理调用（同样会被代理污染），且让代理语义分散；在 httpclient 统一层修复覆盖面最大、行为最符合直觉。

## Risks / Trade-offs

- [定时器间隔内模型仍加载不完] → 下一轮继续重探；10 分钟冷却过期后还会重新拉起一次进程，属预期兜底行为
- [banner 刷新依赖 scheduler status 拉取频率] → reprobe 完成后主动调 status 刷新函数，而不是等下次轮询
- [探测频繁（60s）对远程 provider 的压力] → 仅 not healthy 时探测，healthy 后完全空闲；间隔可调常量
- [手动 reprobe 与启动探测竞态] → 由 probeMu 互斥保证不并发，返回 skipped 让前端知道结果可能来自进行中的探测
