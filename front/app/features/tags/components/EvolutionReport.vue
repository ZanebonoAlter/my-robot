<script setup lang="ts">
import { ref, computed, nextTick } from 'vue'
import type {
  ResultDetailRow,
  ReviewRow,
  EvolutionAnalysis,
  EvolutionEvidence,
  EvolutionPosition,
  PositionChange,
} from '~/api/boardEnrichment'

/**
 * 演进分析报告（报刊式 editorial）—— 消费 result 详情 + reviews。
 *
 * 版式严格照 prototype/evolution-report.html 复刻：
 *  masthead（双线刊头 + 大标题 + byline）/ lede（serif 导语 + 首字下沉）
 *  / article（serif 正文，渲染 signals，跨泳道用 board-cue 标签）
 *  / 双类引用（📰新闻 [n] 红 / 🔧工具 [Tn] 蓝，hover cite-tip，click 闪源）
 *  / position-bar（四档定位可视化 + from→to 迁移）/ sources（新闻/工具分两组清单）
 *  / financial_view（可选，低调小节）。
 *
 * 颜色全部走项目 CSS 变量（var(--color-accent) / --color-info …），明暗主题自动适配。
 * 原型 raw-stone/raw-red 色值即项目 [data-theme] 令牌，故无需硬编码 hex。
 *
 * signals 与 evidence 在后端 schema 里是平行数组（无显式 signal→evidence 链接），
 * 故引用以"支持证据"簇形式内嵌正文末尾（红蓝上标 + hover tooltip），不臆造逐句归属；
 * 完整清单在文末 sources 分组列出。
 */
const props = defineProps<{
  result: ResultDetailRow | null
  reviews: ReviewRow[]
  topicLabel?: string
  loading?: boolean
}>()

// ── 演进定位四档语义 ──────────────────────────────────────────────────────
const POSITION_LABELS: Record<string, string> = {
  reinforcing: '强化',
  turning: '转折',
  expanding: '扩散',
  fading: '衰减',
}
const POSITION_DESC: Record<string, string> = {
  reinforcing: '叙事持续强化，信号热度走升',
  turning: '出现反转，方向切换',
  expanding: '向相邻泳道扩散',
  fading: '信号热度衰减，进入观察',
}
const POSITION_ORDER: EvolutionPosition[] = [
  'reinforcing',
  'turning',
  'expanding',
  'fading',
]

// ── 数据派生 ───────────────────────────────────────────────────────────────
const analysis = computed<EvolutionAnalysis | null>(
  () => props.result?.sectors ?? null,
)
const signals = computed(() => analysis.value?.signals ?? [])
const evidence = computed(() => analysis.value?.evidence ?? [])
const newsEvidence = computed(() =>
  evidence.value.filter((e) => (e.source_type ?? 'news') === 'news'),
)
const toolEvidence = computed(() =>
  evidence.value.filter((e) => e.source_type === 'tool'),
)
const position = computed(() => {
  const p = analysis.value?.position
  return p ? String(p) : null
})
const causalChain = computed(() => props.result?.causal_chain ?? null)
const financialView = computed(() => analysis.value?.financial_view ?? null)

/** 最新 review（reviews 默认最新在前）的定位迁移。 */
const latestReview = computed(() => props.reviews[0] ?? null)
const positionChange = computed<PositionChange | null>(() => {
  const v = latestReview.value?.verdict
  if (v && typeof v === 'object' && 'from' in v && 'to' in v) {
    return v as PositionChange
  }
  return null
})
const changeSummary = computed(
  () => latestReview.value?.deviation_summary ?? positionChange.value?.summary ?? '',
)

/** 跨泳道（去重保序）。 */
const lanes = computed(() => {
  const seen = new Set<string>()
  const out: string[] = []
  for (const s of signals.value) {
    const lane = String(s.lane ?? '').trim()
    if (lane && !seen.has(lane)) {
      seen.add(lane)
      out.push(lane)
    }
  }
  return out
})

interface CiteEntry {
  id: string
  kind: 'news' | 'tool'
  label: string
  index: number
  ev: EvolutionEvidence
}
/** 引用编号：新闻 [1..n] + 工具 [T1..m]。 */
const citations = computed<CiteEntry[]>(() => {
  const out: CiteEntry[] = []
  newsEvidence.value.forEach((ev, i) => {
    out.push({ id: `news-${i}`, kind: 'news', label: String(i + 1), index: i + 1, ev })
  })
  toolEvidence.value.forEach((ev, i) => {
    out.push({ id: `tool-${i}`, kind: 'tool', label: `T${i + 1}`, index: i + 1, ev })
  })
  return out
})

const headline = computed(() => {
  if (props.topicLabel && props.topicLabel.trim()) return props.topicLabel.trim()
  const a = props.result?.evolution_assessment ?? ''
  const first = a.split(/[。！？\n]/)[0]?.trim()
  return first || '演进分析'
})
const lede = computed(() => props.result?.evolution_assessment ?? '')
const reportDate = computed(() => {
  const c = props.result?.created_at
  if (!c) return ''
  try {
    return new Date(c).toISOString().slice(0, 10)
  } catch {
    return ''
  }
})
const hasContent = computed(
  () => signals.value.length > 0 || citations.value.length > 0 || causalChain.value,
)

function positionLabel(p?: string | null): string {
  if (!p) return '—'
  return POSITION_LABELS[p] ?? p
}
function isCurrentPosition(p: string): boolean {
  return position.value === p
}

// ── cite-tip tooltip（Teleport to body，fixed 跟随鼠标）────────────────────
const tip = ref<{ visible: boolean; x: number; y: number; entry: CiteEntry | null }>({
  visible: false,
  x: 0,
  y: 0,
  entry: null,
})
function showTip(entry: CiteEntry, e: MouseEvent) {
  tip.value.entry = entry
  tip.value.visible = true
  moveTip(e)
}
function moveTip(e: MouseEvent) {
  tip.value.x = e.clientX
  tip.value.y = e.clientY
}
function hideTip() {
  tip.value.visible = false
}
const tipStyle = computed(() => {
  const x = tip.value.x
  const y = tip.value.y
  const w = 340
  const h = 200
  const winW = typeof window !== 'undefined' ? window.innerWidth : 1280
  const winH = typeof window !== 'undefined' ? window.innerHeight : 800
  let left = x + 14
  let top = y + 14
  if (left + w > winW - 10) left = x - w - 14
  if (top + h > winH - 10) top = y - h - 14
  return { left: `${Math.max(8, left)}px`, top: `${Math.max(8, top)}px` }
})

// ── flash source（点引用 → 滚到文末清单 + 闪烁）────────────────────────────
const flashingId = ref<string | null>(null)
let flashTimer: ReturnType<typeof setTimeout> | null = null
function flashSource(id: string) {
  const el = document.getElementById(`src-${id}`)
  if (el) el.scrollIntoView({ behavior: 'smooth', block: 'center' })
  if (flashTimer) clearTimeout(flashTimer)
  void nextTick(() => {
    flashingId.value = id
    flashTimer = setTimeout(() => {
      flashingId.value = null
    }, 2000)
  })
}

// ── financial_view 方向语义 ────────────────────────────────────────────────
function finDirMeta(d?: string): { cls: 'up' | 'down' | 'flat' | 'unknown'; label: string } {
  if (d === 'up') return { cls: 'up', label: '上行' }
  if (d === 'down') return { cls: 'down', label: '下行' }
  if (d === 'flat') return { cls: 'flat', label: '横盘' }
  return { cls: 'unknown', label: d ? d : '未知' }
}
const finSectors = computed(() => financialView.value?.sectors ?? [])
</script>

<template>
  <section class="ev-report">
    <!-- ── loading 骨架 ─────────────────────────────────────────────── -->
    <div v-if="loading" class="ev-loading">
      <div class="ev-sk-line w-60" />
      <div class="ev-sk-line w-90" />
      <div class="ev-sk-line w-75" />
      <div class="ev-sk-line w-50" />
      <div class="ev-sk-bar" />
      <div class="ev-sk-line w-80" />
      <div class="ev-sk-line w-65" />
      <span class="ev-loading-note">正在生成演进分析…</span>
    </div>

    <!-- ── 空态 ─────────────────────────────────────────────────────── -->
    <div v-else-if="!result" class="ev-empty">
      <div class="ev-empty-ic">🗞</div>
      <p class="ev-empty-h">暂无演进分析</p>
      <p class="ev-empty-sub">点上方「▶ 重新分析」生成第一份演进报告。</p>
    </div>

    <!-- ── 报告 ─────────────────────────────────────────────────────── -->
    <article v-else class="report">
      <!-- Masthead -->
      <header class="masthead">
        <div class="eyebrow">演进分析<span v-if="reportDate"> · {{ reportDate }}</span></div>
        <h1 class="serif masthead-title">{{ headline }}</h1>
        <div class="byline">
          <span v-for="lane in lanes" :key="lane" class="board-tag">{{ lane }}</span>
          <span v-if="lanes.length" class="sep">·</span>
          <span>{{ signals.length }} 条跨泳道信号</span>
          <span class="sep">·</span>
          <span>{{ newsEvidence.length }} 篇新闻</span>
          <span v-if="toolEvidence.length" class="sep">·</span>
          <span v-if="toolEvidence.length">{{ toolEvidence.length }} 条工具查证</span>
          <span v-if="reviews.length" class="sep">·</span>
          <span v-if="reviews.length">复盘 {{ reviews.length }} 条</span>
        </div>
        <p v-if="lede" class="lede serif">{{ lede }}</p>
      </header>

      <!-- 正文 -->
      <div v-if="hasContent" class="article serif">
        <p v-for="(sig, i) in signals" :key="i">
          <span class="board-cue">{{ sig.lane }}</span>{{ sig.signal }}<span v-if="sig.mechanism">。{{ sig.mechanism }}</span>
        </p>

        <!-- pullquote：因果链 -->
        <div v-if="causalChain" class="pullquote">
          &ldquo;{{ causalChain }}&rdquo;
          <span class="pq-src">— 因果链</span>
        </div>

        <!-- 支持证据引用簇（双类） -->
        <p v-if="citations.length" class="cite-run">
          <span class="cite-run-label">支持证据</span>
          <a
            v-for="c in citations"
            :key="c.id"
            class="cite"
            :class="c.kind"
            :title="c.ev.quote || (c.kind === 'tool' ? '工具查证' : '新闻报道')"
            @mouseenter="showTip(c, $event)"
            @mousemove="moveTip($event)"
            @mouseleave="hideTip"
            @click="flashSource(c.id)"
          >{{ c.label }}</a>
        </p>
      </div>

      <!-- position-bar：四档定位可视化 + 迁移 -->
      <div v-if="position || positionChange" class="position-bar">
        <div class="pb-label">演进定位</div>
        <div v-if="position" class="pb-segments">
          <span
            v-for="p in POSITION_ORDER"
            :key="p"
            class="pb-seg"
            :class="{ on: isCurrentPosition(p) }"
          >{{ POSITION_LABELS[p] }}</span>
        </div>
        <div v-if="position" class="pb-text">
          当前 <b>{{ positionLabel(position) }}</b><span class="pb-desc"> · {{ POSITION_DESC[position] ?? '' }}</span>
        </div>
        <div v-if="positionChange" class="pb-migrate">
          <span class="pb-from">{{ positionLabel(positionChange.from as string) }}</span>
          <span class="pb-arrow">→</span>
          <span class="pb-to">{{ positionLabel(positionChange.to as string) }}</span>
          <span v-if="changeSummary" class="pb-summary">{{ changeSummary }}</span>
        </div>
      </div>

      <!-- 资料来源（新闻 / 工具 分两组） -->
      <div v-if="newsEvidence.length || toolEvidence.length" class="sources">
        <h3>资料来源</h3>

        <div v-if="newsEvidence.length" class="src-group">
          <div class="src-group-h news">
            <span class="gh-icon">📰</span>新闻报道
            <span class="gh-count">{{ newsEvidence.length }} 篇 · 来自订阅源</span>
          </div>
          <ol class="src-list">
            <li
              v-for="(ev, i) in newsEvidence"
              :id="`src-news-${i}`"
              :key="`news-${i}`"
              class="src-item"
              :class="{ flash: flashingId === `news-${i}` }"
            >
              <span class="src-no news">{{ i + 1 }}</span>
              <span v-if="ev.quote" class="src-title">{{ ev.quote }}</span>
              <span v-else class="src-title muted">（无引文）</span>
              <span class="src-meta">
                <span v-if="ev.period" class="src-date">{{ ev.period }}</span>
                <span v-if="ev.context_id != null" class="src-board">ctx#{{ ev.context_id }}</span>
              </span>
            </li>
          </ol>
        </div>

        <div v-if="toolEvidence.length" class="src-group">
          <div class="src-group-h tool">
            <span class="gh-icon">🔧</span>工具查证
            <span class="gh-count">{{ toolEvidence.length }} 次 · agent 自主调用 opencli skill</span>
          </div>
          <ol class="src-list">
            <li
              v-for="(ev, i) in toolEvidence"
              :id="`src-tool-${i}`"
              :key="`tool-${i}`"
              class="src-item tool"
              :class="{ flash: flashingId === `tool-${i}` }"
            >
              <span class="src-no tool">T{{ i + 1 }}</span>
              <span v-if="ev.tool_ref" class="src-query">{{ ev.tool_ref }}</span>
              <div v-if="ev.quote" class="src-result">{{ ev.quote }}</div>
              <span class="src-meta">
                <span v-if="ev.period" class="src-date">{{ ev.period }}</span>
                <span v-if="ev.context_id != null" class="src-board">ctx#{{ ev.context_id }}</span>
              </span>
            </li>
          </ol>
        </div>
      </div>

      <!-- 金融视角（可选 · 非主线） -->
      <details v-if="finSectors.length" class="fin-view">
        <summary>金融视角 · 可选（{{ finSectors.length }} 个板块方向）</summary>
        <div class="fin-grid">
          <div v-for="(fs, i) in finSectors" :key="i" class="fin-row">
            <span class="fin-name">{{ fs.sector }}</span>
            <span class="fin-dir" :class="finDirMeta(fs.direction as string).cls">{{ finDirMeta(fs.direction as string).label }}</span>
            <span v-if="fs.supporting_data" class="fin-data">{{ fs.supporting_data }}</span>
          </div>
        </div>
      </details>

      <div class="editor-note">
        <span class="en-label">关于这份报告</span>
        本报告由演进分析工作流自动生成，定位四档：<b>强化 / 转折 / 扩散 / 衰减</b>。
        <b class="en-news">📰 新闻</b>来自订阅源入库报道，<b class="en-tool">🔧 工具查证</b>为 agent 自主调用 opencli skill 的检索/查询结果；
        所引资料均标注于上方清单。金融视角为可选辅助，非判断依据；非金融话题不采集。
      </div>
    </article>

    <!-- cite-tip tooltip（Teleport to body） -->
    <Teleport to="body">
      <div
        v-if="tip.visible && tip.entry"
        class="cite-tip"
        :style="tipStyle"
      >
        <div class="ct-type" :class="tip.entry.kind">
          {{ tip.entry.kind === 'news' ? '📰 新闻报道' : `🔧 工具查证${tip.entry.ev.tool_ref ? ' · ' + tip.entry.ev.tool_ref : ''}` }}
        </div>
        <div v-if="tip.entry.ev.quote" class="ct-quote">{{ tip.entry.ev.quote }}</div>
        <div v-else class="ct-quote muted">（无引文）</div>
        <div class="ct-meta">
          <span v-if="tip.entry.ev.period">{{ tip.entry.ev.period }}</span>
          <span v-if="tip.entry.ev.context_id != null" class="ct-board">ctx#{{ tip.entry.ev.context_id }}</span>
          <span v-if="tip.entry.kind === 'tool' && tip.entry.ev.tool_ref" class="ct-skill">{{ tip.entry.ev.tool_ref }}</span>
        </div>
      </div>
    </Teleport>
  </section>
</template>

<style scoped>
/* 项目 main.css 未定义 info/error 的 subtle 变体（仅 success/warning 有），
   此处按 accent-subtle 约定从主题令牌派生 scoped 版，明暗主题自动适配。 */
.ev-report {
  --color-info-subtle: color-mix(in srgb, var(--color-info) 12%, transparent);
  --color-error-subtle: color-mix(in srgb, var(--color-error) 14%, transparent);
}

.serif {
  font-family: Georgia, "Songti SC", "SimSun", "Source Serif 4", "Noto Serif SC", serif;
}

/* ── loading / 空态 ─────────────────────────────────────────────────── */
.ev-loading {
  display: flex;
  flex-direction: column;
  gap: 0.7rem;
  padding: 2rem 1.5rem;
}
.ev-sk-line {
  height: 12px;
  border-radius: 6px;
  background: linear-gradient(
    90deg,
    var(--color-bg-sunken) 0%,
    var(--color-bg-hover) 50%,
    var(--color-bg-sunken) 100%
  );
  background-size: 200% 100%;
  animation: ev-sk 1.4s ease-in-out infinite;
}
.ev-sk-line.w-50 { width: 50%; }
.ev-sk-line.w-60 { width: 60%; }
.ev-sk-line.w-65 { width: 65%; }
.ev-sk-line.w-75 { width: 75%; }
.ev-sk-line.w-80 { width: 80%; }
.ev-sk-line.w-90 { width: 90%; }
.ev-sk-bar { height: 28px; border-radius: 8px; background: var(--color-accent-subtle); }
@keyframes ev-sk {
  0%, 100% { background-position: 200% 0; }
  50% { background-position: -200% 0; }
}
.ev-loading-note {
  font-size: 12px;
  color: var(--color-text-muted);
  text-align: center;
  margin-top: 0.4rem;
}

.ev-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  gap: 0.4rem;
  padding: 3rem 1.5rem;
  background: var(--color-bg-elevated);
  border: 1px dashed var(--color-border-medium);
  border-radius: 12px;
}
.ev-empty-ic { font-size: 2.2rem; opacity: 0.5; line-height: 1; }
.ev-empty-h { font-size: 15px; font-weight: 700; margin: 0; color: var(--color-text-primary); }
.ev-empty-sub { font-size: 12.5px; color: var(--color-text-secondary); margin: 0; }

/* ── 报告容器（原型 .report） ────────────────────────────────────────── */
.report { max-width: 680px; margin: 0 auto; padding: 0.5rem 0 1rem; }

/* Masthead：双线刊头 */
.masthead {
  border-bottom: 3px double var(--color-text-primary);
  padding-bottom: 1.1rem;
  margin-bottom: 1.4rem;
}
.masthead .eyebrow {
  font-size: 10.5px;
  letter-spacing: 0.22em;
  text-transform: uppercase;
  color: var(--color-accent);
  font-weight: 700;
  margin-bottom: 0.6rem;
}
.masthead-title {
  font-size: 2rem;
  line-height: 1.22;
  margin: 0 0 0.7rem;
  font-weight: 800;
  letter-spacing: -0.015em;
  color: var(--color-text-primary);
}
.byline {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
  font-size: 11.5px;
  color: var(--color-text-muted);
}
.byline .sep { color: var(--color-border-medium); }
.board-tag {
  font-size: 11px;
  padding: 1px 8px;
  border-radius: 3px;
  background: var(--color-bg-sunken);
  color: var(--color-text-secondary);
  font-weight: 600;
}

/* Lede：serif 导语 + 首字下沉 */
.lede {
  font-size: 15.5px;
  line-height: 1.78;
  color: var(--color-text-secondary);
  padding: 0.8rem 0 0.4rem;
  border-top: 1px solid var(--color-border-subtle);
  margin-top: 0.7rem;
  margin-bottom: 0;
}
.lede::first-letter {
  font-size: 3.2rem;
  font-weight: 800;
  float: left;
  line-height: 0.9;
  padding: 0.1rem 0.5rem 0.1rem 0;
  color: var(--color-accent);
  font-family: Georgia, serif;
}

/* ── 正文 ───────────────────────────────────────────────────────────── */
.article { font-size: 16px; line-height: 1.9; color: var(--color-text-primary); }
.article p { margin: 0 0 1.1rem; }
.article b, .article strong { color: var(--color-text-primary); font-weight: 700; }

/* 跨泳道标签（board-cue） */
.board-cue {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  font-weight: 700;
  padding: 1px 8px;
  border-radius: 3px;
  background: var(--color-info-subtle);
  color: var(--color-info);
  vertical-align: middle;
  margin-right: 4px;
  letter-spacing: 0.02em;
}

/* pullquote：因果链大字引语 */
.pullquote {
  margin: 1.5rem 0;
  padding: 0.4rem 0 0.4rem 1.1rem;
  border-left: 3px solid var(--color-accent);
  font-size: 15px;
  font-style: italic;
  color: var(--color-text-secondary);
  line-height: 1.7;
  font-family: Georgia, "Songti SC", serif;
}
.pullquote .pq-src {
  display: block;
  font-style: normal;
  font-size: 11px;
  color: var(--color-text-muted);
  margin-top: 0.3rem;
}

/* 支持证据引用簇 */
.cite-run {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0.4rem;
  padding: 0.6rem 0.8rem;
  background: var(--color-bg-elevated);
  border-radius: 8px;
  font-size: 13px;
}
.cite-run-label {
  font-size: 10.5px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--color-text-muted);
}

/* 引用标记（两类 · 核心） */
.cite {
  display: inline;
  cursor: pointer;
  font-weight: 700;
  font-size: 0.8em;
  vertical-align: super;
  line-height: 0;
  padding: 0 1px;
  text-decoration: none;
  transition: color 0.1s;
}
.cite::before { content: '['; }
.cite::after { content: ']'; }
.cite.news { color: var(--color-accent); }
.cite.news:hover { color: var(--color-accent-hover); text-decoration: underline; }
.cite.tool { color: var(--color-info); }
.cite.tool:hover { text-decoration: underline; filter: brightness(1.15); }

/* ── position-bar：四档定位可视化 ───────────────────────────────────── */
.position-bar {
  margin: 1.8rem 0 1rem;
  padding: 1rem 1.2rem;
  background: var(--color-accent-subtle);
  border-left: 3px solid var(--color-accent);
  border-radius: 0 8px 8px 0;
}
.position-bar .pb-label {
  font-size: 10.5px;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--color-accent);
  margin-bottom: 0.5rem;
}
.pb-segments {
  display: flex;
  gap: 4px;
  margin-bottom: 0.6rem;
}
.pb-seg {
  flex: 1;
  text-align: center;
  font-size: 12px;
  font-weight: 600;
  padding: 4px 6px;
  border-radius: 5px;
  background: var(--color-bg-sunken);
  color: var(--color-text-muted);
  transition: background 0.15s, color 0.15s;
}
.pb-seg.on {
  background: var(--color-accent);
  color: #fff;
}
.pb-text { font-size: 14.5px; color: var(--color-text-primary); line-height: 1.6; }
.pb-text b { color: var(--color-accent); }
.pb-desc { color: var(--color-text-secondary); }
.pb-migrate {
  margin-top: 0.6rem;
  padding-top: 0.6rem;
  border-top: 1px solid var(--color-border-subtle);
  font-size: 13px;
  color: var(--color-text-secondary);
  line-height: 1.6;
  display: flex;
  align-items: baseline;
  gap: 0.4rem;
  flex-wrap: wrap;
}
.pb-from { color: var(--color-text-muted); text-decoration: line-through; }
.pb-arrow { color: var(--color-accent); font-weight: 700; }
.pb-to { color: var(--color-accent); font-weight: 700; }
.pb-summary { width: 100%; margin-top: 0.25rem; color: var(--color-text-primary); }

/* ── 资料来源（分两组） ─────────────────────────────────────────────── */
.sources {
  margin-top: 2rem;
  padding-top: 1.1rem;
  border-top: 2px solid var(--color-text-primary);
}
.sources h3 {
  font-size: 11px;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: var(--color-text-muted);
  font-weight: 700;
  margin: 0 0 0.9rem;
}
.src-group { margin-bottom: 1.3rem; }
.src-group-h {
  font-size: 12px;
  font-weight: 700;
  color: var(--color-text-secondary);
  margin-bottom: 0.5rem;
  display: flex;
  align-items: center;
  gap: 0.4rem;
  padding-bottom: 0.3rem;
  border-bottom: 1px solid var(--color-border-subtle);
}
.src-group-h .gh-icon { font-size: 13px; }
.src-group-h .gh-count {
  font-size: 10.5px;
  color: var(--color-text-muted);
  font-weight: 600;
  margin-left: auto;
}
.src-group-h.news .gh-icon { color: var(--color-accent); }
.src-group-h.tool .gh-icon { color: var(--color-info); }

.src-list { list-style: none; padding: 0; margin: 0; }
.src-item {
  position: relative;
  padding: 0.5rem 0.7rem 0.5rem 2rem;
  margin-bottom: 0.3rem;
  border-radius: 6px;
  transition: background 0.15s;
  font-size: 13px;
  line-height: 1.6;
  color: var(--color-text-secondary);
}
.src-item:hover { background: var(--color-bg-hover); }
.src-item.flash { animation: srcFlash 2s ease; }
.src-item.flash.tool { animation: srcFlashTool 2s ease; }
@keyframes srcFlash {
  0%, 70% { background: var(--color-accent-subtle); }
  100% { background: transparent; }
}
@keyframes srcFlashTool {
  0%, 70% { background: var(--color-info-subtle); }
  100% { background: transparent; }
}
.src-item .src-no {
  position: absolute;
  left: 0.5rem;
  top: 0.55rem;
  font-size: 10.5px;
  font-weight: 700;
  padding: 1px 5px;
  border-radius: 4px;
  min-width: 20px;
  text-align: center;
}
.src-item .src-no.news { color: var(--color-accent); background: var(--color-accent-subtle); }
.src-item .src-no.tool { color: var(--color-info); background: var(--color-info-subtle); }
.src-item .src-title { color: var(--color-text-primary); }
.src-item .src-query {
  font-family: ui-monospace, Menlo, monospace;
  font-size: 11.5px;
  color: var(--color-text-secondary);
  background: var(--color-bg-sunken);
  padding: 1px 6px;
  border-radius: 4px;
}
.src-item .src-result {
  color: var(--color-text-primary);
  margin-top: 0.2rem;
}
.src-meta {
  display: inline-flex;
  gap: 0.35rem;
  align-items: center;
  margin-left: 0.4rem;
  flex-wrap: wrap;
}
.src-date { color: var(--color-text-primary); font-weight: 600; font-size: 11.5px; }
.src-board {
  font-size: 10px;
  padding: 1px 6px;
  border-radius: 3px;
  background: var(--color-info-subtle);
  color: var(--color-info);
  font-weight: 700;
}

/* ── 金融视角（可选 · 折叠低调） ────────────────────────────────────── */
.fin-view {
  margin-top: 1.5rem;
  background: var(--color-bg-elevated);
  border: 1px solid var(--color-border-subtle);
  border-radius: 8px;
  padding: 0.6rem 0.9rem;
}
.fin-view > summary {
  cursor: pointer;
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-muted);
  list-style: none;
}
.fin-view > summary::-webkit-details-marker { display: none; }
.fin-view > summary::before { content: '▸ '; color: var(--color-text-muted); }
.fin-view[open] > summary::before { content: '▾ '; }
.fin-grid { display: flex; flex-direction: column; gap: 0.3rem; padding: 0.6rem 0 0; }
.fin-row {
  display: flex;
  align-items: baseline;
  gap: 0.6rem;
  font-size: 12.5px;
  padding: 0.25rem 0;
  border-bottom: 1px solid var(--color-border-subtle);
}
.fin-row:last-child { border-bottom: none; }
.fin-name { font-weight: 600; color: var(--color-text-primary); min-width: 80px; }
.fin-dir { font-size: 11.5px; font-weight: 700; padding: 1px 8px; border-radius: 999px; }
.fin-dir.up { background: var(--color-success-subtle); color: var(--color-success); }
.fin-dir.down { background: var(--color-error-subtle); color: var(--color-error); }
.fin-dir.flat { background: var(--color-bg-sunken); color: var(--color-text-muted); }
.fin-dir.unknown { background: var(--color-bg-sunken); color: var(--color-text-muted); }
.fin-data { color: var(--color-text-secondary); font-size: 11.5px; margin-left: auto; text-align: right; }

/* ── 编者按 ─────────────────────────────────────────────────────────── */
.editor-note {
  margin-top: 1.6rem;
  padding: 0.9rem 1.1rem;
  background: var(--color-bg-sunken);
  border-radius: 8px;
  font-size: 12.5px;
  line-height: 1.7;
  color: var(--color-text-muted);
}
.editor-note .en-label {
  font-weight: 700;
  color: var(--color-text-secondary);
  font-size: 11px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  display: block;
  margin-bottom: 0.3rem;
}
.editor-note .en-news { color: var(--color-accent); }
.editor-note .en-tool { color: var(--color-info); }

/* ── cite-tip tooltip（Teleport to body，fixed） ─────────────────────── */
.cite-tip {
  position: fixed;
  z-index: 200;
  max-width: 340px;
  background: var(--color-bg-elevated);
  border: 1px solid var(--color-border-strong);
  border-radius: 8px;
  padding: 0.8rem 0.95rem;
  font-size: 13px;
  line-height: 1.6;
  color: var(--color-text-secondary);
  box-shadow: 0 6px 24px rgba(0, 0, 0, 0.18);
  font-family: -apple-system, "PingFang SC", "Microsoft YaHei", sans-serif;
  pointer-events: none;
}
.cite-tip .ct-type {
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  margin-bottom: 0.4rem;
}
.cite-tip .ct-type.news { color: var(--color-accent); }
.cite-tip .ct-type.tool { color: var(--color-info); }
.cite-tip .ct-quote { font-style: italic; color: var(--color-text-primary); }
.cite-tip .ct-quote.muted { color: var(--color-text-muted); font-style: normal; }
.cite-tip .ct-meta {
  font-size: 11px;
  color: var(--color-text-muted);
  margin-top: 0.5rem;
  padding-top: 0.4rem;
  border-top: 1px solid var(--color-border-subtle);
  display: flex;
  gap: 0.5rem;
  align-items: center;
  flex-wrap: wrap;
}
.cite-tip .ct-skill {
  font-family: ui-monospace, Menlo, monospace;
  font-size: 10.5px;
  background: var(--color-info-subtle);
  color: var(--color-info);
  padding: 1px 7px;
  border-radius: 4px;
  font-weight: 700;
}
.cite-tip .ct-board {
  font-size: 10px;
  padding: 1px 6px;
  border-radius: 3px;
  background: var(--color-info-subtle);
  color: var(--color-info);
  font-weight: 700;
}

.muted { color: var(--color-text-muted); }
</style>
