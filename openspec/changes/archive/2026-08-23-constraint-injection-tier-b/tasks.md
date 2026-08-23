<!-- doc-impact: none（.pi 扩展层与事实库内部机制，非产品代码与文档域；预算降级与 resume 恢复不改变 flow 约束节的内容与辖区） -->
<!-- doc-impact-excuse: flow=工作区 flow 域启发式命中的是并行 change 的 service/feature 改动，本 change 仅碰 .pi 扩展层与事实库; api=工作区 api 域改动属并行 change，本 change 未碰产品代码; database=同上，并行 tagmanagement 工作; architecture=同上，并行工作区脏改动; standard=同上，并行工作区脏改动; configuration=同上，并行工作区脏改动 -->

## 1. 预算降级（B6）

- [x] 1.1 `harness-log.ts`：`HarnessEventKind` 加 `mode.set`、`RETENTION_DAYS` 加 30 天；新增 `queryLatestByKind(kind)` 查询 API（`idx_ev_kind_ts` 索引现成，ORDER BY id DESC LIMIT 1）
- [x] 1.2 `constraint-injection.ts`：抽 `applyBudget()` 纯函数（两遍装配、D2 降级顺序含域声明层、占位行格式、永不真丢不变量），`planInjection` 接线（`budgetBytes` 配置，缺省 32768、非正数禁用）
- [x] 1.3 降级通知与记账：header 已降级路径行 + widget 预算用量；`InjectedDocEntry` 增 `degraded` 维度，仅降级回合携带
- [x] 1.4 烟测断言组：预算内零降级 / 分层顺序（keyword 先于 jit、findings digest 收紧先于域声明节降级）/ 永不真丢（budgetBytes=2048）/ 确定性（两次装配字节一致）

## 2. 档位持久化与 resume 恢复（B8 衍生）

- [x] 2.1 实测 pi resume 的 sessionId 语义：pi 文档确认 `/resume` 切到目标 session 文件（id 跟随目标会话）→ 常见路径第 1 段命中；烟测 stub 同 id/新 id 两分支均验
- [x] 2.2 `mode.set` 记账三触发点（input 命令 / skill 路径 / change 绑定修正），payload `{mode, boundChange}` + change 列；记账失败不阻断激活
- [x] 2.3 `session_start` resume 分支：两段式取数恢复档位与绑定（new/fork/reload 清零不变；恢复后沿用既有回落规则；粘性命中集合不恢复——有意为之，见 design D6）
- [x] 2.4 烟测断言组：resume 恢复 / 无记录回落 / new 不恢复 / 绑定 change 已归档回落
- [x] 2.5 （6.5 真实链路返工）startup/reload 恢复路径：quit 重启 pi 走 session_start{startup} 而非 resume（07:46:41 事件实证）——startup 双面孔按「档位是否为空」区分（冷启动恢复 vs 子线程派发），档位空时仅同 sessionId 第 1 段恢复（禁全局兜底）；reload 清零后照 resume 两段式恢复；烟测 +4 断言（冷启动恢复/子线程不动/全新会话不继承/reload 恢复）

## 3. 配置与调研文档

- [x] 3.1 `.pi/constraint-injection.json` 补 `budgetBytes`（显式 32768 留档，JSON 无注释、字段本身即文档）
- [x] 3.2 `docs/research/harness-survey/findings.md` 落地记录补一行（B6/B8 衍生已实施 + 指向本 change）

## 4. 测试

- [x] 4.1 `bash .pi/extensions/tests/run-smoke.sh` → 全绿（含 1.4/2.4 新断言组，既有注入主路径/pin.read 去重不回归）
- [x] 4.2 `bash .pi/extensions/tests/run-harness-smoke.sh` → 全绿（安全开库/TTL 含 mode.set 30 天/写入查询含 queryLatestByKind）

## 5. 文档

<!-- doc-impact: none（.pi 扩展层工具链 change，非产品代码与文档域） -->
<!-- doc-impact-excuse: flow/api/database/architecture/standard/configuration=另一 pi 窗口的 backend-go 脏文件命中 doc-impact suggest（非本 change 改动，本 change 仅碰 .pi/extensions 与 openspec/docs 研究文档） -->

- [x] 5.1 `docs/reference/constraints-index.md` 常驻索引不变（本 change 无 flow 业务约束）；3.2 的 findings.md 落地记录即全部文档产出

## 6. 验证

- [x] 6.1 `bash .pi/extensions/tests/run-smoke.sh` → 全部用例 PASS，退出码 0
- [x] 6.2 `bash .pi/extensions/tests/run-harness-smoke.sh` → 全部用例 PASS，退出码 0
- [x] 6.3 `grep -c "mode.set" .pi/extensions/lib/harness-log.ts` → ≥2（kind union + RETENTION_DAYS）
- [x] 6.4 手动触发一次 `/opsx-apply <change>` 后 `sqlite3 .pi/harness/events.db "SELECT kind, payload FROM events WHERE kind='mode.set' ORDER BY id DESC LIMIT 1"` → 一条 mode.set，payload 含 mode 与 boundChange（✅ 2026-08-23 reload 后真实验证：读 verify skill 触发，落库 `{mode:implementation, boundChange:constraint-injection-tier-b}`，后续 constraint.inject reason 变 mode-base）
- [x] 6.5 真实会话中断后 resume/重启 → constraint-injection widget 显示恢复后的档位与 change 绑定（非「未激活」），且 `constraint.inject` 事件照常产生（✅ 2026-08-23 完全重启 pi 实测：13:02:24 session.start{startup}（冷启动路径，首次实测曾失败的同路径）→ 13:02:36 constraint.inject reason=mode-base（零 skill 读取/零命令，档位自动恢复）；对比失败样本 07:46:41 startup → index，行为翻转确认修复生效）
- [x] 6.6 `ls openspec/changes/archive/ | grep constraint-domain-declaration` → 命中（cdd 已归档——本 change 归档顺序依赖满足，见 design Migration Plan；反序归档会回滚 cdd 规范变更）
