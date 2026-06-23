<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch, type ComponentPublicInstance } from 'vue'
import { Icon } from '@iconify/vue'
import {
  buildBezierPath,
  buildLifelineWindow,
  type RequestCacheEntry,
} from './dailyReportMagazine'
import type { TopicLifelineData } from '~/features/tags/composables/useDailyReportReader'

const props = defineProps<{
  topicId: number
  topicColor?: string
  reportDate: string
  entry: RequestCacheEntry<TopicLifelineData>
  selectedDayKey?: string
}>()

const emit = defineEmits<{
  retry: []
  selectDay: [dayKey: string]
  openDetective: [topicId: number]
}>()

const scrollRef = ref<HTMLElement | null>(null)
const trackRef = ref<HTMLElement | null>(null)
const nodeRefs = new Map<string, HTMLElement>()
const paths = ref<Array<{ key: string; d: string; weak: boolean }>>([])
let resizeObserver: ResizeObserver | null = null

const lifeline = computed(() => buildLifelineWindow(
  props.entry.data?.sections ?? [],
  props.entry.data?.relations ?? [],
  props.reportDate,
))
const nodeCount = computed(() => lifeline.value.days.reduce((total, day) => total + day.sections.length, 0))
const selectedDayColumn = computed(() => {
  const index = lifeline.value.days.findIndex(day => day.key === props.selectedDayKey)
  return Math.min(6, Math.max(1, index + 1))
})

function formatLaneDate(date: string): string {
  const [, month = '', day = ''] = date.slice(0, 10).split('-')
  return `${Number(month)}.${Number(day)}`
}

function setNodeRef(key: string, element: Element | ComponentPublicInstance | null) {
  if (element instanceof HTMLElement) nodeRefs.set(key, element)
  else nodeRefs.delete(key)
}

function measurePaths() {
  const track = trackRef.value
  if (!track) return
  const trackRect = track.getBoundingClientRect()
  paths.value = lifeline.value.edges.flatMap((edge) => {
    const from = nodeRefs.get(edge.fromDayKey)
    const to = nodeRefs.get(edge.toDayKey)
    if (!from || !to) return []
    const fromRect = from.getBoundingClientRect()
    const toRect = to.getBoundingClientRect()
    const x1 = fromRect.left - trackRect.left + fromRect.width / 2
    const y1 = fromRect.top - trackRect.top + fromRect.height / 2
    const x2 = toRect.left - trackRect.left + toRect.width / 2
    const y2 = toRect.top - trackRect.top + toRect.height / 2
    return [{ key: edge.key, d: buildBezierPath(x1, y1, x2, y2), weak: edge.weak }]
  })
}

async function scheduleMeasure() {
  await nextTick()
  if (typeof requestAnimationFrame !== 'undefined') requestAnimationFrame(measurePaths)
  else measurePaths()
}

watch([lifeline, () => props.selectedDayKey], scheduleMeasure, { deep: true })

onMounted(() => {
  void scheduleMeasure()
  if (typeof ResizeObserver !== 'undefined' && trackRef.value) {
    resizeObserver = new ResizeObserver(measurePaths)
    resizeObserver.observe(trackRef.value)
  }
  window.addEventListener('resize', measurePaths)
})

onUnmounted(() => {
  resizeObserver?.disconnect()
  window.removeEventListener('resize', measurePaths)
})
</script>

<template>
  <section
    class="drm-lifeline"
    data-testid="daily-report-lifeline"
    :style="{ '--topic-color': topicColor || 'var(--color-accent)' }"
  >
    <div v-if="entry.status === 'loading'" class="drm-lifeline__state" aria-live="polite">
      <Icon icon="mdi:loading" width="16" class="drm-spin" />
      正在翻阅历史脉络…
    </div>
    <div v-else-if="entry.status === 'error'" class="drm-lifeline__state drm-lifeline__state--error" role="alert">
      <span>{{ entry.error || '生命线加载失败' }}</span>
      <button type="button" @click="emit('retry')">重试</button>
    </div>
    <div v-else-if="entry.status === 'success' && !entry.data?.sections.length" class="drm-lifeline__state">
      这个话题尚无可展示的历史节点。
    </div>
    <template v-else-if="entry.status === 'success'">
      <header class="drm-lifeline__header">
        <strong>话题泳道 <em>最近 7 天 · 同话题连续性</em></strong>
        <span>{{ nodeCount }} 个节点</span>
      </header>

      <div ref="scrollRef" class="drm-lifeline__scroll" @scroll="measurePaths">
        <div ref="trackRef" class="drm-lifeline__track">
          <svg class="drm-lifeline__edges" aria-hidden="true">
            <path
              v-for="path in paths"
              :key="path.key"
              :d="path.d"
              :class="{ 'is-weak': path.weak }"
            />
          </svg>
          <div class="drm-lifeline__days">
            <div
              v-for="day in lifeline.days"
              :key="day.key"
              class="drm-lifeline__day"
              :class="{
                'is-current': day.key === reportDate.slice(0, 10),
                'is-selected': selectedDayKey === day.key,
              }"
            >
              <span class="drm-lifeline__date">{{ formatLaneDate(day.date) }}</span>
              <span class="drm-lifeline__node-row">
                <button
                  v-if="day.sections.length"
                  :ref="element => setNodeRef(day.key, element)"
                  type="button"
                  class="drm-lifeline__node"
                  :aria-pressed="selectedDayKey === day.key"
                  :aria-label="`${day.date}，${day.sections.length} 个节点`"
                  @click="emit('selectDay', day.key)"
                >
                  <span v-if="day.sections.length > 1">{{ day.sections.length }}</span>
                </button>
                <span v-else class="drm-lifeline__empty" aria-label="当日无节点" />
              </span>
            </div>
          </div>
        </div>
      </div>

      <div v-if="selectedDayKey" class="drm-lifeline__detail-grid">
        <div
          class="drm-lifeline__detail-panel"
          :style="{ gridColumn: `${selectedDayColumn} / span 2` }"
        >
          <slot name="details" />
        </div>
      </div>

      <div class="drm-lifeline__legend" aria-label="生命线图例">
        <span><i class="drm-lifeline__legend-line" />同话题连续性</span>
        <span><i class="drm-lifeline__legend-dot" />有动态</span>
        <span>点击节点查看当日详情</span>
      </div>
      <footer class="drm-lifeline__footer">
        <button type="button" @click="emit('openDetective', topicId)">
          在侦探墙打开完整生命线
          <Icon icon="mdi:arrow-top-right" width="14" />
        </button>
      </footer>
    </template>
  </section>
</template>

<style scoped>
.drm-lifeline {
  margin: 1rem 0 1.25rem;
  padding: 1rem 1.125rem 0.75rem;
  border: 1px solid var(--color-border-subtle);
  border-left: 2px solid var(--topic-color);
  background: var(--color-bg-active);
  animation: drmLifelineSlide 0.35s ease both;
}

@keyframes drmLifelineSlide {
  from { opacity: 0; transform: translateY(-6px); }
  to { opacity: 1; transform: translateY(0); }
}

.drm-lifeline__state {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.55rem;
  min-height: 6rem;
  color: var(--color-text-muted);
  font-size: 0.74rem;
}

.drm-lifeline__state--error {
  color: var(--color-error);
}

.drm-lifeline__state button {
  border: 0;
  background: transparent;
  color: var(--color-link);
  cursor: pointer;
}

.drm-lifeline__header {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 1rem;
  color: var(--color-text-secondary);
  font-family: "Noto Serif SC", serif;
  font-size: 0.78rem;
}

.drm-lifeline__header em {
  margin-left: 0.45rem;
  color: var(--color-text-muted);
  font-size: 0.68rem;
  font-weight: 400;
}

.drm-lifeline__header > span {
  color: var(--color-text-muted);
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 0.65rem;
}

.drm-lifeline__scroll {
  overflow-x: auto;
  padding: 0.65rem 0 0.35rem;
  scrollbar-color: var(--color-border-strong) transparent;
  scrollbar-width: thin;
  touch-action: pan-x;
}

.drm-lifeline__scroll::-webkit-scrollbar {
  height: 6px;
}

.drm-lifeline__scroll::-webkit-scrollbar-thumb {
  border: 2px solid transparent;
  border-radius: 3px;
  background: var(--color-border-strong);
  background-clip: padding-box;
}

.drm-lifeline__track {
  position: relative;
  min-width: 49rem;
  height: 9rem;
}

.drm-lifeline__edges {
  position: absolute;
  inset: 0;
  z-index: 0;
  width: 100%;
  height: 100%;
  overflow: visible;
  pointer-events: none;
}

.drm-lifeline__edges path {
  fill: none;
  stroke: var(--color-accent);
  stroke-width: 2;
  stroke-linecap: round;
  opacity: 0.5;
}

.drm-lifeline__edges path.is-weak {
  opacity: 0.28;
}

.drm-lifeline__days {
  position: relative;
  z-index: 1;
  display: grid;
  grid-template-columns: repeat(7, minmax(7rem, 1fr));
  min-width: 100%;
}

.drm-lifeline__day {
  position: relative;
  min-width: 0;
  text-align: center;
}

.drm-lifeline__day::before {
  position: absolute;
  top: 1.9rem;
  left: 50%;
  height: 5.6rem;
  border-left: 1px dashed var(--color-border-subtle);
  content: '';
  transform: translateX(-50%);
}

.drm-lifeline__date {
  display: block;
  min-height: 1.7rem;
  color: var(--color-text-muted);
  font-size: 0.66rem;
}

.drm-lifeline__day.is-current .drm-lifeline__date {
  color: var(--color-accent);
  font-weight: 700;
}

.drm-lifeline__day.is-current .drm-lifeline__date::after {
  display: block;
  margin-top: 0.1rem;
  content: '今';
  font-size: 0.55rem;
}

.drm-lifeline__node-row {
  position: relative;
  display: grid;
  height: 7.5rem;
  place-items: center;
}

.drm-lifeline__node,
.drm-lifeline__empty {
  position: relative;
  z-index: 2;
  display: grid;
  width: 0.9rem;
  height: 0.9rem;
  padding: 0;
  border-radius: 50%;
  place-items: center;
}

.drm-lifeline__node {
  border: 2.5px solid var(--color-bg-base);
  background: var(--color-accent);
  color: var(--color-bg-base);
  cursor: pointer;
  transition: box-shadow 160ms ease, transform 160ms ease;
}

.drm-lifeline__node:hover,
.drm-lifeline__node:focus-visible,
.drm-lifeline__day.is-selected .drm-lifeline__node {
  box-shadow: 0 0 0 4px color-mix(in srgb, var(--color-accent) 22%, transparent);
  outline: none;
  transform: scale(1.18);
}

.drm-lifeline__node span {
  position: absolute;
  top: -0.65rem;
  right: -0.65rem;
  display: grid;
  min-width: 0.95rem;
  height: 0.95rem;
  border-radius: 50%;
  background: var(--color-text-primary);
  color: var(--color-bg-base);
  font-size: 0.55rem;
  place-items: center;
}

.drm-lifeline__empty {
  border: 1px dashed var(--color-text-muted);
  background: transparent;
}

.drm-lifeline__detail-grid {
  display: grid;
  grid-template-columns: repeat(7, minmax(0, 1fr));
  margin-top: 0.35rem;
}

.drm-lifeline__detail-panel {
  min-width: 0;
}

.drm-lifeline__detail-panel :deep(.drm-history) {
  height: 100%;
  margin: 0;
}

.drm-lifeline__legend {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: center;
  gap: 0.65rem 1.25rem;
  margin-top: 0.85rem;
  color: var(--color-text-muted);
  font-size: 0.62rem;
}

.drm-lifeline__legend span {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
}

.drm-lifeline__legend-line {
  width: 1.2rem;
  border-top: 2px solid var(--color-accent);
  opacity: 0.55;
}

.drm-lifeline__legend-dot {
  width: 0.5rem;
  height: 0.5rem;
  border-radius: 50%;
  background: var(--color-accent);
}

.drm-lifeline__footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 0.7rem;
  padding-top: 0.65rem;
  border-top: 1px solid var(--color-border-subtle);
}

.drm-lifeline__footer button {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  border: 0;
  border-bottom: 1px dotted currentColor;
  background: transparent;
  color: var(--color-link);
  font-size: 0.66rem;
  cursor: pointer;
}

.drm-spin {
  animation: drmSpin 0.9s linear infinite;
}

@keyframes drmSpin {
  to { transform: rotate(1turn); }
}

@media (max-width: 720px) {
  .drm-lifeline {
    padding-inline: 0.75rem;
  }

  .drm-lifeline__header {
    align-items: start;
  }

  .drm-lifeline__header em {
    display: block;
    margin: 0.2rem 0 0;
  }

  .drm-lifeline__detail-grid {
    grid-template-columns: 1fr;
  }

  .drm-lifeline__detail-panel {
    grid-column: 1 !important;
  }
}

@media (prefers-reduced-motion: reduce) {
  .drm-lifeline,
  .drm-lifeline__node,
  .drm-spin {
    transition: none;
    animation: none;
  }
}
</style>
