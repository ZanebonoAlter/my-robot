<script setup lang="ts">
import { ref, computed } from 'vue'
import { Icon } from '@iconify/vue'
import type { ResultSector, StockDebateResult, Direction } from '~/api/boardEnrichment'

/**
 * 个股深度辩论（FinGenius）—— 原型 ④。
 *
 * 四态：empty（无 result / 无辩论）/ loading（触发中或 running）/ error / result。
 * result 按 sector 分组（呼应 ② 板块判断），每只标的渲染：
 *   - 头部：code/name/verdict/共识
 *   - 6 agent 网格（name/stance/note）
 *   - 投票柱状图（多头/中性/空头 三条 bar）
 *   - 报告入口（html_content iframe 弹窗）
 *
 * agents / votes 是后端 jsonb（结构未冻结），用 normAgents / normVotes 防御性解析：
 * 兼容 tuple [name,stance,note] 与 object {name,stance,note}；votes 兼容
 * {up,flat,down} 计数与 [{stance}] 计数两种口径。
 *
 * 状态归属在父级（BoardEnrichmentPanel 通过 useBoardEnrichment 持有 debateStage /
 * debates / debateTriggering / debateError），本组件纯展示 + emit trigger/retry。
 */
const props = defineProps<{
  /** 四态：由父级 composable 的 debateStage 传入。 */
  stage: 'empty' | 'loading' | 'error' | 'result'
  debates: StockDebateResult[]
  /** ②最新 result 的 sectors，用于分组头回显「② 判断：↑ 高置信」。 */
  sectors?: ResultSector[] | null
  triggering?: boolean
  errorMsg?: string | null
}>()

const emit = defineEmits<{
  trigger: []
  retry: []
}>()

// ── agent / vote 防御性归一 ──────────────────────────────────────────────
type Stance = 'up' | 'down' | 'flat'
interface NormAgent { name: string; stance: Stance; note: string }
interface NormVotes { up: number; flat: number; down: number; total: number }

function stanceOf(v: unknown): Stance {
  const s = String(v ?? '').toLowerCase()
  if (['up', 'bull', 'long', '多', '看多', 'buy'].includes(s)) return 'up'
  if (['down', 'bear', 'short', '空', '看空', 'sell'].includes(s)) return 'down'
  return 'flat'
}
function str(v: unknown): string {
  if (v == null) return ''
  return String(v)
}
function num(v: unknown): number {
  const n = typeof v === 'number' ? v : Number(v)
  return Number.isFinite(n) && n > 0 ? Math.round(n) : 0
}

function normAgents(raw: unknown): NormAgent[] {
  if (!Array.isArray(raw)) return []
  const out: NormAgent[] = []
  for (const item of raw) {
    let name = '', stance: Stance = 'flat', note = ''
    if (Array.isArray(item)) {
      name = str(item[0]); stance = stanceOf(item[1]); note = str(item[2])
    } else if (item && typeof item === 'object') {
      const o = item as Record<string, unknown>
      name = str(o.name ?? o.role ?? o.agent ?? '')
      stance = stanceOf(o.stance ?? o.view ?? o.verdict ?? o.position ?? '')
      note = str(o.note ?? o.reason ?? o.opinion ?? o.summary ?? o.argument ?? '')
    }
    if (name) out.push({ name, stance, note })
  }
  return out
}

function normVotes(raw: unknown): NormVotes {
  const v: NormVotes = { up: 0, flat: 0, down: 0, total: 0 }
  if (raw && typeof raw === 'object' && !Array.isArray(raw)) {
    const o = raw as Record<string, unknown>
    v.up = num(o.up ?? o.bull ?? o.long ?? o.buy)
    v.flat = num(o.flat ?? o.neutral ?? o.hold)
    v.down = num(o.down ?? o.bear ?? o.short ?? o.sell)
  } else if (Array.isArray(raw)) {
    for (const item of raw) {
      const s = stanceOf(
        (item as Record<string, unknown>)?.stance ??
        (item as Record<string, unknown>)?.view ??
        item,
      )
      v[s]++
    }
  }
  v.total = v.up + v.flat + v.down
  return v
}

const VERDICT_LABEL: Record<Stance, string> = { up: '偏多', down: '偏空', flat: '中性' }
const STANCE_SHORT: Record<Stance, string> = { up: '多', down: '空', flat: '中' }
function pct(n: number, total: number): number {
  return total > 0 ? Math.round((n * 100) / total) : 0
}
function verdictOf(d: StockDebateResult): Stance {
  return stanceOf(d.verdict)
}

// ── sector 回显（从 ② sectors 取方向/置信度做 ref chip） ──────────────────
const sectorEcho = computed<Map<string, { dir: string; conf: string }>>(() => {
  const m = new Map<string, { dir: string; conf: string }>()
  for (const s of props.sectors ?? []) {
    const name = (s.sector || s.name || '') as string
    if (!name) continue
    const conf = typeof s.confidence === 'number'
      ? (s.confidence <= 1 ? Math.round(s.confidence * 100) : Math.round(s.confidence)) + '%'
      : ''
    m.set(name, { dir: dirLabel(s.direction), conf })
  }
  return m
})
function dirLabel(d?: Direction | string): string {
  if (d === 'up') return '↑ 上行'
  if (d === 'down') return '↓ 下行'
  if (d === 'flat') return '↔ 震荡'
  return d ? String(d) : '—'
}
function echoOf(sector: string | undefined): string {
  if (!sector) return '来自 ② 板块判断'
  const e = sectorEcho.value.get(sector)
  if (!e) return `② ${sector}`
  return `② 判断：${e.dir}${e.conf ? ' · ' + e.conf : ''}`
}

// ── 按 sector 分组 ───────────────────────────────────────────────────────
interface DebateGroup { sector: string; ref: string; items: StockDebateResult[] }
const groups = computed<DebateGroup[]>(() => {
  const map = new Map<string, StockDebateResult[]>()
  for (const d of props.debates) {
    const key = d.sector || '未分组'
    if (!map.has(key)) map.set(key, [])
    map.get(key)!.push(d)
  }
  const out: DebateGroup[] = []
  for (const [sector, items] of map) {
    out.push({ sector, ref: echoOf(sector), items })
  }
  return out
})

// ── 完整报告弹窗（html_content iframe） ──────────────────────────────────
const report = ref<StockDebateResult | null>(null)
function openReport(d: StockDebateResult) {
  if (!d.html_content) return
  report.value = d
}
</script>

<template>
  <div class="debate-stage">
    <!-- 态1：空（无 result / 未辩论） -->
    <div v-if="stage === 'empty'" class="debate-empty">
      <div class="debate-empty-ic">⚖</div>
      <p class="debate-empty-h">还没辩论个股</p>
      <p class="debate-empty-sub">
        先跑 <b>② 会往哪走</b> 拿到板块方向 + 代表标的池，再对个股做 6 角色深度辩论（调 FinGenius）。
      </p>
      <button type="button" class="btn btn-primary" :disabled="triggering" @click="emit('trigger')">
        <Icon icon="mdi:play" width="13" />
        {{ triggering ? '触发中…' : '开始辩论' }}
      </button>
      <p class="debate-empty-note">基于 ② 的板块代表标的 · 约 2-3 分钟，6 个分析师各自分析后多轮博弈</p>
    </div>

    <!-- 态2：loading（触发中 / running） -->
    <div v-else-if="stage === 'loading'" class="debate-loading">
      <div class="dl-spin">⟳</div>
      <div class="dl-h">FinGenius 6 角色辩论中…</div>
      <div class="dl-roles">
        <span class="dl-role">舆情</span><span class="dl-role">游资</span><span class="dl-role">风控</span>
        <span class="dl-role">技术</span><span class="dl-role">筹码</span><span class="dl-role">大单</span>
      </div>
      <p class="dl-note">约 2-3 分钟，6 个分析师各自分析后进入多轮辩论博弈</p>
    </div>

    <!-- 态3：error -->
    <div v-else-if="stage === 'error'" class="debate-error">
      <div class="de-ic">⚠</div>
      <div class="de-h">连接 FinGenius 失败</div>
      <div class="de-msg">{{ errorMsg || '请检查 FinGenius 服务是否启动，配置见 ⑤ 数据源与参数。' }}</div>
      <button type="button" class="btn btn-ghost btn-sm" @click="emit('retry')">
        <Icon icon="mdi:refresh" width="12" /> 重试
      </button>
    </div>

    <!-- 态4：result（按板块分组） -->
    <div v-else class="debate-result">
      <div v-for="g in groups" :key="g.sector" class="stk-group">
        <div class="stk-group-h">
          <span class="stk-g-name">{{ g.sector }}</span>
          <span class="stk-g-ref">{{ g.ref }}</span>
        </div>

        <div v-for="d in g.items" :key="d.id ?? d.code" class="stk">
          <div class="stk-head">
            <span class="stk-code">{{ d.code }}</span>
            <span v-if="d.name" class="stk-name">{{ d.name }}</span>
            <span class="stk-verdict" :class="verdictOf(d)">{{ VERDICT_LABEL[verdictOf(d)] }}</span>
            <span class="stk-spacer" />
            <span v-if="d.consensus" class="stk-consensus">共识 <b>{{ d.consensus }}</b></span>
            <span v-if="d.distill_status === 'running'" class="stk-running">
              <Icon icon="mdi:loading" width="11" /> 进行中…
            </span>
          </div>

          <!-- 6 agent 网格 -->
          <div v-if="normAgents(d.agents).length" class="agent-grid">
            <div v-for="(a, i) in normAgents(d.agents)" :key="i" class="agent-cell">
              <span class="ag-name">{{ a.name }}</span>
              <span class="ag-stance" :class="a.stance">{{ STANCE_SHORT[a.stance] }}</span>
              <span v-if="a.note" class="ag-note">{{ a.note }}</span>
            </div>
          </div>

          <!-- 投票柱状图 -->
          <div class="vote-bar">
            <div class="vote-row up">
              <span class="vote-lbl">多头</span>
              <div class="vote-track"><div class="vote-fill" :style="{ width: pct(normVotes(d.votes).up, normVotes(d.votes).total) + '%' }" /></div>
              <span class="vote-num">{{ normVotes(d.votes).up }}</span>
            </div>
            <div class="vote-row flat">
              <span class="vote-lbl">中性</span>
              <div class="vote-track"><div class="vote-fill" :style="{ width: pct(normVotes(d.votes).flat, normVotes(d.votes).total) + '%' }" /></div>
              <span class="vote-num">{{ normVotes(d.votes).flat }}</span>
            </div>
            <div class="vote-row down">
              <span class="vote-lbl">空头</span>
              <div class="vote-track"><div class="vote-fill" :style="{ width: pct(normVotes(d.votes).down, normVotes(d.votes).total) + '%' }" /></div>
              <span class="vote-num">{{ normVotes(d.votes).down }}</span>
            </div>
          </div>

          <div v-if="d.html_content" class="stk-foot">
            <button type="button" class="link-btn" @click="openReport(d)">
              <Icon icon="mdi:file-document-outline" width="12" /> 查看完整辩论报告 ↗
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- 完整报告弹窗（iframe） -->
    <AppDialog
      :model-value="report !== null"
      :title="report ? `完整报告 · ${report.code}` : '完整报告'"
      width="90%"
      @update:model-value="(v) => { if (!v) report = null }"
    >
      <iframe
        v-if="report?.html_content"
        class="debate-iframe"
        :srcdoc="report.html_content"
        sandbox="allow-same-origin"
      />
      <template #footer>
        <AppButton variant="ghost" size="sm" @click="report = null">关闭</AppButton>
      </template>
    </AppDialog>
  </div>
</template>

<style scoped>
/* 四态容器 */
.debate-stage { min-height: 120px; }

.debate-empty, .debate-loading, .debate-error {
  display: flex; flex-direction: column; align-items: center; justify-content: center;
  text-align: center; gap: 0.45rem; padding: 2.2rem 1.5rem;
  background: var(--color-bg-elevated); border: 1px dashed var(--color-border-medium);
  border-radius: 12px;
}
.debate-empty-ic { font-size: 2.2rem; opacity: 0.5; line-height: 1; }
.debate-empty-h { font-size: 15px; font-weight: 700; margin: 0; color: var(--color-text-primary); }
.debate-empty-sub { font-size: 12.5px; color: var(--color-text-secondary); margin: 0; max-width: 420px; line-height: 1.6; }
.debate-empty-sub b { color: var(--color-text-primary); }
.debate-empty-note { font-size: 11.5px; color: var(--color-text-muted); margin: 0.7rem 0 0; }

.debate-loading { border-style: solid; }
.dl-spin { font-size: 1.6rem; color: var(--color-accent); animation: dl-pulse 1.2s infinite; }
.dl-h { font-size: 14px; font-weight: 600; margin-top: 0.3rem; color: var(--color-text-primary); }
.dl-roles { display: flex; gap: 0.4rem; flex-wrap: wrap; justify-content: center; margin-top: 0.6rem; }
.dl-role { font-size: 11px; padding: 2px 9px; border-radius: 999px; background: var(--color-bg-sunken); color: var(--color-text-secondary); }
.dl-note { font-size: 11.5px; color: var(--color-text-muted); margin: 0.5rem 0 0; }
@keyframes dl-pulse { 0%,100%{opacity:1;} 50%{opacity:.35;} }

.debate-error { border-style: solid; border-color: var(--color-error); background: var(--color-error-subtle); }
.de-ic { font-size: 1.6rem; }
.de-h { font-size: 14px; font-weight: 600; color: var(--color-error); }
.de-msg { font-size: 12px; color: var(--color-text-secondary); max-width: 380px; line-height: 1.5; }

/* result：板块分组 */
.debate-result { display: flex; flex-direction: column; gap: 1.1rem; }
.stk-group {
  background: var(--color-bg-elevated); border: 1px solid var(--color-border-medium);
  border-left: 3px solid var(--color-accent); border-radius: 0 10px 10px 0;
  padding: 0.9rem 1.05rem 0.8rem; box-shadow: var(--shadow-print);
}
.stk-group-h { display: flex; align-items: center; gap: 0.6rem; padding-bottom: 0.6rem; margin-bottom: 0.7rem; border-bottom: 1px solid var(--color-border-subtle); }
.stk-g-name { font-size: 15px; font-weight: 700; color: var(--color-text-primary); }
.stk-g-ref { font-size: 11.5px; color: var(--color-text-secondary); background: var(--color-bg-sunken); padding: 2px 9px; border-radius: 999px; }

/* 单标的卡片 */
.stk { border: 1px solid var(--color-border-subtle); border-radius: 9px; background: var(--color-bg-base); padding: 0.75rem 0.9rem; }
.stk + .stk { margin-top: 0.55rem; }
.stk-head { display: flex; align-items: center; gap: 0.55rem; flex-wrap: wrap; margin-bottom: 0.65rem; }
.stk-code { font-family: ui-monospace, "Cascadia Code", Menlo, monospace; font-size: 12px; color: var(--color-text-muted); background: var(--color-bg-sunken); padding: 1px 7px; border-radius: 5px; }
.stk-name { font-weight: 700; font-size: 13.5px; color: var(--color-text-primary); }
.stk-verdict { font-size: 11.5px; font-weight: 700; padding: 2px 9px; border-radius: 999px; }
.stk-verdict.up { background: var(--color-success-subtle); color: var(--color-success); }
.stk-verdict.down { background: var(--color-error-subtle); color: var(--color-error); }
.stk-verdict.flat { background: var(--color-bg-sunken); color: var(--color-text-muted); }
.stk-spacer { margin-left: auto; }
.stk-consensus { font-size: 11px; color: var(--color-text-secondary); }
.stk-consensus b { color: var(--color-text-primary); }
.stk-running { display: inline-flex; align-items: center; gap: 3px; font-size: 11px; color: var(--color-text-muted); }

/* 6 agent 网格 */
.agent-grid { display: grid; grid-template-columns: repeat(6, 1fr); gap: 0.4rem; margin-bottom: 0.65rem; }
@media (max-width: 680px) { .agent-grid { grid-template-columns: repeat(3, 1fr); } }
.agent-cell { display: flex; flex-direction: column; gap: 0.2rem; padding: 0.45rem 0.5rem; background: var(--color-bg-elevated); border: 1px solid var(--color-border-subtle); border-radius: 7px; min-width: 0; }
.ag-name { font-size: 10.5px; font-weight: 700; color: var(--color-text-muted); letter-spacing: 0.02em; }
.ag-stance { font-size: 11.5px; font-weight: 700; padding: 1px 7px; border-radius: 5px; display: inline-block; width: fit-content; }
.ag-stance.up { background: var(--color-success-subtle); color: var(--color-success); }
.ag-stance.down { background: var(--color-error-subtle); color: var(--color-error); }
.ag-stance.flat { background: var(--color-bg-sunken); color: var(--color-text-muted); }
.ag-note { font-size: 11px; color: var(--color-text-secondary); line-height: 1.4; word-break: break-word; }

/* 投票柱状图 */
.vote-bar { display: flex; flex-direction: column; gap: 0.28rem; padding: 0.55rem 0.65rem; background: var(--color-bg-sunken); border-radius: 7px; margin-bottom: 0.5rem; }
.vote-row { display: grid; grid-template-columns: 42px 1fr 18px; align-items: center; gap: 0.5rem; font-size: 11.5px; }
.vote-lbl { color: var(--color-text-secondary); font-weight: 600; }
.vote-track { height: 8px; background: var(--color-bg-base); border-radius: 999px; overflow: hidden; }
.vote-fill { height: 100%; border-radius: 999px; transition: width 0.5s ease; }
.vote-row.up .vote-fill { background: var(--color-success); }
.vote-row.flat .vote-fill { background: var(--color-text-muted); }
.vote-row.down .vote-fill { background: var(--color-error); }
.vote-num { text-align: right; font-weight: 700; color: var(--color-text-primary); }

.stk-foot { display: flex; align-items: center; gap: 0.5rem; padding-top: 0.4rem; border-top: 1px dashed var(--color-border-subtle); }

/* 通用 btn（呼应原型 .btn，但作用域内自给自足） */
.btn { display: inline-flex; align-items: center; gap: 6px; font-family: inherit; cursor: pointer; border: none; border-radius: 8px; font-weight: 600; font-size: 13px; padding: 7px 14px; transition: background 0.15s, opacity 0.15s, transform 0.1s; }
.btn:active { transform: translateY(1px); }
.btn-primary { background: var(--color-accent); color: #fff; }
.btn-primary:hover { background: var(--color-accent-hover); }
.btn-ghost { background: transparent; color: var(--color-text-secondary); border: 1px solid var(--color-border-medium); }
.btn-ghost:hover { background: var(--color-bg-hover); color: var(--color-text-primary); }
.btn-sm { padding: 4px 10px; font-size: 12px; }
.btn:disabled { opacity: 0.4; cursor: not-allowed; }

.link-btn { display: inline-flex; align-items: center; gap: 3px; padding: 2px 7px; border: none; background: none; color: var(--color-text-muted); font-size: 11.5px; cursor: pointer; border-radius: 5px; font-family: inherit; }
.link-btn:hover { color: var(--color-accent); background: var(--color-accent-subtle); }

.debate-iframe { width: 100%; height: 70vh; border: 1px solid var(--color-border-subtle); border-radius: 8px; background: #fff; }
</style>
