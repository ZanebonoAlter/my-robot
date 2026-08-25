/**
 * keyword 关注表达式纯函数（前端解析 / 预览 / 校验）。
 *
 * 语法与后端 parseKeywordExpr 对齐（openspec/changes/watch-keyword-and-quickadd
 * design.md §4.2 + test-cases.md 白盒 B3-B7）：
 * - 空格 = AND（槽位之间须全含）
 * - `|` = OR（槽位内任一备选词命中即算）
 * - 混用例：`ASML|镓锗 出口` = (ASML 或 镓锗) 且 含 出口
 * - 大小写不敏感由匹配侧（后端 threads 文本匹配）负责；解析保留原词形
 *
 * 边界（与后端 400 校验一致，前端提前禁提交；B3/V3）：
 * - 空串 / 纯空白（含全角空格、tab）→ 无效
 * - 纯分隔符（`|` / `||` / `| |`）→ 无效
 * - 独立 `|` token（如 `| 出口` 的首个 token）→ 视为冗余分隔符，容忍丢弃
 * - 前导/中间空分支（`|出口` / `A||B`）→ 忽略空分支；尾随 `|`（`ASML|`）→ 无效
 */

/** AND 槽位解析：token（按空白切）→ 槽（按 `|` 切备选词）。无效表达式返回 null。
 *  空白集为显式清单（不含 ZWJ/ZWNJ：emoji 序列是合法词，JS 的 \s 会误判 U+200D；
 *  与 Go strings.Fields/unicode.IsSpace 语义对齐）。 */
const WS_RE = /[ \t\u00a0\u1680\u180e\u2000-\u200a\u2028\u2029\u202f\u205f\u3000\ufeff]+/

function parseSlotsOrNull(expr: string): string[][] | null {
  const slots: string[][] = []
  for (const token of expr.split(WS_RE)) {
    if (!token) continue
    const parts = token.split('|')
    if (parts.every(p => !p)) continue // 整个 token 都是 `|`：冗余分隔符，丢弃
    // 后端 parseKeywordExpr 契约：前导/中间空分支忽略，但尾随 |
    // 表示尚未写完表达式，创建时必须拒绝。
    if (token.endsWith('|')) return null
    slots.push(parts.filter(Boolean))
  }
  return slots
}

/** 解析为 OR-of-ANDs 分支（笛卡尔展开，每分支 = 每槽取一词的 AND 组）。
 *  `ASML|镓锗 出口` → [['ASML', '出口'], ['镓锗', '出口']]；空/无效 → []。 */
export function parseKeywordExpr(expr: string): string[][] {
  const slots = parseSlotsOrNull(expr)
  if (!slots || slots.length === 0) return []
  let branches: string[][] = [[]]
  for (const slot of slots) {
    const next: string[][] = []
    for (const b of branches) {
      for (const alt of slot) next.push([...b, alt])
    }
    branches = next
  }
  return branches
}

/** 解析预览分组（对话框 chips 展示用）：AND 槽位列表，槽内为 OR 备选词。
 *  `ASML|镓锗 出口` → [['ASML', '镓锗'], ['出口']]（展示为 `[ASML|镓锗] × [出口]`）；空/无效 → []。 */
export function parseKeywordSlots(expr: string): string[][] {
  const slots = parseSlotsOrNull(expr)
  return slots ?? []
}

/** 表达式是否有效（空串 / 纯空白 / 纯分隔符 / 槽内空洞均无效，与后端 400 对齐）。 */
export function isValidKeywordExpr(expr: string): boolean {
  return parseKeywordExpr(expr).length > 0
}
