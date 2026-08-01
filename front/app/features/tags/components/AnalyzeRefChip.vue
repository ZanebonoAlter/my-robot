<script setup lang="ts">
import { ref, computed } from 'vue'
import type { AnalyzeRef } from '~/api/boardEnrichment'

/**
 * 双类引用芯片（news📰 / tool🔧）+ hover tooltip —— 因果分析报告通用证据标记。
 *
 * AnalyzeRef.source_type 区分两类来源：
 *  - news：订阅源新闻报道（📰，accent 红）
 *  - tool：agent 自主调用 opencli skill 查证（🔧，info 蓝）
 *
 * hover 弹 tooltip 显示 ref（引用键）+ quote（引文）。自带 Teleport tooltip
 * （fixed 跟随鼠标，视口边缘自动翻转）。因果报告的事实层/时间线/见解层各处复用。
 *
 * 颜色全部走 CSS 变量；--color-info-subtle 本地派生（main.css 仅 success/warning
 * 有 subtle 变体），明暗主题自动适配。
 */
const props = defineProps<{
  r: AnalyzeRef
  /** 紧凑模式：只显图标（用于时间线节点等密集场景），tooltip 仍显全量。 */
  compact?: boolean
}>()

const isNews = computed(() => (props.r.source_type ?? 'news') === 'news')
const icon = computed(() => (isNews.value ? '📰' : '🔧'))
const kindLabel = computed(() => (isNews.value ? '新闻报道' : '工具查证'))

// ── tooltip（Teleport to body，fixed 跟随鼠标）────────────────────────────
const tip = ref(false)
const x = ref(0)
const y = ref(0)
function enter(e: MouseEvent) {
  tip.value = true
  move(e)
}
function move(e: MouseEvent) {
  x.value = e.clientX
  y.value = e.clientY
}
function leave() {
  tip.value = false
}
const tipStyle = computed(() => {
  const w = 320
  const h = 180
  const winW = typeof window !== 'undefined' ? window.innerWidth : 1280
  const winH = typeof window !== 'undefined' ? window.innerHeight : 800
  let left = x.value + 14
  let top = y.value + 14
  if (left + w > winW - 8) left = x.value - w - 14
  if (top + h > winH - 8) top = y.value - h - 14
  return { left: `${Math.max(8, left)}px`, top: `${Math.max(8, top)}px` }
})
</script>

<template>
  <a
    class="ref-chip"
    :class="isNews ? 'news' : 'tool'"
    :title="r.quote || kindLabel"
    @mouseenter="enter($event)"
    @mousemove="move($event)"
    @mouseleave="leave"
  >
    <span class="rc-ic">{{ icon }}</span>
    <span v-if="!compact" class="rc-ref">{{ r.ref }}</span>
  </a>

  <Teleport to="body">
    <div
      v-if="tip"
      class="ref-tip"
      :class="isNews ? 'news' : 'tool'"
      :style="tipStyle"
    >
      <div class="rt-type">{{ icon }} {{ kindLabel }}</div>
      <div class="rt-ref">{{ r.ref }}</div>
      <div v-if="r.quote" class="rt-quote">&ldquo;{{ r.quote }}&rdquo;</div>
    </div>
  </Teleport>
</template>

<style scoped>
/* info/error subtle 本地派生（main.css 仅 success/warning/accent 有 subtle 变体）。 */
.ref-chip {
  --color-info-subtle: color-mix(in srgb, var(--color-info) 12%, transparent);
  display: inline-flex;
  align-items: center;
  gap: 3px;
  max-width: 200px;
  padding: 1px 7px;
  border-radius: 5px;
  font-size: 11px;
  font-weight: 600;
  line-height: 1.5;
  cursor: help;
  text-decoration: none;
  vertical-align: middle;
  transition: background 0.12s, filter 0.12s;
}
.ref-chip .rc-ic {
  font-size: 11px;
  line-height: 1;
}
.ref-chip .rc-ref {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: ui-monospace, Menlo, monospace;
  font-size: 10.5px;
}
.ref-chip.news {
  color: var(--color-accent);
  background: var(--color-accent-subtle);
}
.ref-chip.news:hover {
  background: color-mix(in srgb, var(--color-accent) 20%, transparent);
}
.ref-chip.tool {
  color: var(--color-info);
  background: var(--color-info-subtle);
}
.ref-chip.tool:hover {
  background: color-mix(in srgb, var(--color-info) 20%, transparent);
  filter: brightness(1.05);
}

/* tooltip（Teleport to body，用全局令牌，不依赖组件 scoped 变量） */
.ref-tip {
  position: fixed;
  z-index: 200;
  max-width: 320px;
  background: var(--color-bg-elevated);
  border: 1px solid var(--color-border-strong);
  border-radius: 8px;
  padding: 0.7rem 0.85rem;
  font-size: 12.5px;
  line-height: 1.6;
  color: var(--color-text-secondary);
  box-shadow: 0 6px 24px rgba(0, 0, 0, 0.18);
  font-family: -apple-system, "PingFang SC", "Microsoft YaHei", sans-serif;
  pointer-events: none;
}
.ref-tip .rt-type {
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  margin-bottom: 0.35rem;
}
.ref-tip.news .rt-type {
  color: var(--color-accent);
}
.ref-tip.tool .rt-type {
  color: var(--color-info);
}
.ref-tip .rt-ref {
  font-family: ui-monospace, Menlo, monospace;
  font-size: 11px;
  color: var(--color-text-primary);
  word-break: break-all;
}
.ref-tip .rt-quote {
  font-style: italic;
  color: var(--color-text-primary);
  margin-top: 0.4rem;
  padding-top: 0.4rem;
  border-top: 1px solid var(--color-border-subtle);
}

/* ── 窄屏适配（≤720px，对齐 daily-report 家族断点） ───────────────────── */
@media (max-width: 720px) {
  /* chip 更紧凑：长 ref 键更早截断，不挤爆窄行；图标+tooltip 仍全量 */
  .ref-chip { max-width: 140px; padding: 2px 7px; }
  /* tooltip 硬钳制在视口内（JS 翻转按 320px 估算，320 屏上会溢出） */
  .ref-tip { max-width: calc(100vw - 16px); }
}
</style>
