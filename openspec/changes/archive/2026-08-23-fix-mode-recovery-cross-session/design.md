# design — fix-mode-recovery-cross-session

## Context

tier-b 的 D6「两段式恢复」第 2 段（全局最新 mode.set 兜底）在真实使用中翻车：多 pi 窗口并行是本仓常态（实测当天同时 3-4 个窗口），无自身档位历史的会话 reload/resume 时，「全局最新」几乎必然是其他窗口的记录，探索窗口被灌入他窗口的 implementation + change 绑定。第 2 段防御的「resume 生成新 sessionId」场景经 pi 文档查证不存在（/resume 切到目标会话文件，id 即目标会话 id）。

## Decisions

### D1 砍第 2 段，三路径统一同 sessionId 取数

`recoverMode` 仅保留第 1 段（`queryBySession(sessionId, ["mode.set"])` 取最近一条）；resume / reload / startup（档位空时）共用之。无本会话记录 → 不恢复（未激活）。这同时消除了「全局最新」在多窗口下的歧义——档位本来就是会话私有状态，恢复也只能是会话私有取数。

### D2 queryLatestByKind API 保留

lib 层通用查询能力（harness 烟测覆盖），仅恢复链路不再调用；未来若有跨会话统计需求（如「最近在哪个 change 上干活」）可直接复用。

## Risks / Trade-offs

- [若未来 pi 行为变化、resume 真的出现新 id 且需恢复] → 届时再评估显式「会话血缘」事件（fork 时记 parent id），不做全局猜测兜底
