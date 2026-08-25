## 1. 修复

- [x] 1.1 `constraint-injection.ts`：`recoverMode` 删第 2 段全局兜底（queryLatestByKind 调用移除，注释同步「多窗口并行下全局最新必属他会话」）；三路径统一仅同 sessionId
- [x] 1.2 烟测断言翻转：「resume 新 sessionId 走全局兜底」→「无自身档位历史的会话 resume/reload 不继承他窗口档位（未激活）」（用户实测场景回归测试）
- [x] 1.3 修掉本日遗留误绑数据不删（mode.set 是审计账本 append-only；错误恢复只在会话内存态，新代码下 reload 即回落）

## 2. 测试

- [x] 2.1 `bash .pi/extensions/tests/run-smoke.sh` → 全绿（含翻转后的回归断言，其余 111+ 断言不回归）
- [x] 2.2 `bash .pi/extensions/tests/run-harness-smoke.sh` → 全绿（queryLatestByKind API 保留，其断言不变）

## 3. 文档

<!-- doc-impact: none（.pi 扩展层恢复语义修正，非产品代码与文档域） -->
<!-- doc-impact-excuse: flow/api/database/architecture/standard/configuration=另一 pi 窗口的 backend-go 脏文件命中启发式（非本 change 改动，本 change 仅碰 .pi/extensions 与 openspec/docs 研究文档） -->

- [x] 3.1 `docs/research/harness-survey/findings.md` B 级落地记录补一行勘误（两段式→同 sessionId 单段，多窗口实测教训）

## 4. 验证

- [x] 4.1 `bash .pi/extensions/tests/run-smoke.sh` → 全部用例 PASS，退出码 0
- [x] 4.2 `bash .pi/extensions/tests/run-harness-smoke.sh` → 全部用例 PASS，退出码 0
- [x] 4.3 `grep -c "queryLatestByKind" .pi/extensions/constraint-injection.ts` → 0（恢复链路不再调用；lib 中保留 API 本身）
- [x] 4.4 用户在受影响探索窗口 `/reload` → widget 显示「未激活（仅索引）」，不再出现他窗口的 change 名（真机会话验证）
