<script setup lang="ts">
/**
 * TopicDetectiveWall — full-screen 3D detective photo wall container.
 *
 * Owns the canvas + CSS2D overlay DOM, initializes the Three.js scene stack
 * (TopicWallScene / DirectorCamera / InteractionLayer), fetches board data, and
 * renders the 2D overlays (board selector, days range, detail panel, chapter
 * transition DOM). 2D overlays use motion-v; 3D animation uses gsap.
 *
 * @see design.md §Architecture Overview
 */
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'
import { Icon } from '@iconify/vue'
import { useDailyReportsApi } from '~/api/dailyReports'
import type { SectionTimelineNode, SectionRelation } from '~/api/dailyReports'
import { TopicWallScene } from './detective-wall/TopicWallScene'
import { DirectorCamera } from './detective-wall/DirectorCamera'
import { InteractionLayer } from './detective-wall/InteractionLayer'
import { ChapterTransition } from './detective-wall/ChapterTransition'
import type { ChapterTransitionData } from './detective-wall/ChapterTransition'
import { SUPPORTED_DAYS } from './detective-wall/types'
import { latestDayX } from './detective-wall/utils'

const props = defineProps<{ boardId: number }>()
const emit = defineEmits<{
  close: []
  openArticle: [articleId: number]
}>()

const { getBoardSectionTimeline } = useDailyReportsApi()

const canvasRef = ref<HTMLCanvasElement | null>(null)
const css2dContainerRef = ref<HTMLDivElement | null>(null)
const wipeRef = ref<HTMLDivElement | null>(null)
const coverRef = ref<HTMLDivElement | null>(null)
const titleRef = ref<HTMLSpanElement | null>(null)

const loading = ref(true)
const error = ref(false)
const days = ref<7 | 14 | 30 | 60>(7)
const sections = ref<SectionTimelineNode[]>([])
const relations = ref<SectionRelation[]>([])

// Detail panel state (plain Vue overlay, not CSS2D).
const showDetailPanel = ref(false)
const focusedNode = ref<SectionTimelineNode | null>(null)
const lifelineNodes = ref<SectionTimelineNode[]>([])

// --- WebGL detection ---
function hasWebGL(): boolean {
  try {
    const c = document.createElement('canvas')
    return !!(c.getContext('webgl2') || c.getContext('webgl'))
  } catch {
    return false
  }
}
const webglOk = hasWebGL()

let scene: TopicWallScene | null = null
let directorCamera: DirectorCamera | null = null
let interaction: InteractionLayer | null = null
let chapterTransition: ChapterTransition | null = null
let resizeObserver: ResizeObserver | null = null

onMounted(async () => {
  if (!webglOk || !canvasRef.value || !css2dContainerRef.value) {
    error.value = true
    return
  }

  scene = new TopicWallScene(canvasRef.value, css2dContainerRef.value)
  directorCamera = new DirectorCamera(scene.camera)
  interaction = new InteractionLayer(scene, directorCamera, canvasRef.value, {
    onCardHover: () => { /* tooltip handled by CSS2D in scene */ },
    onCardClick: (card) => {
      focusedNode.value = card.data
    },
    onStringClick: () => { /* handled internally via re-BFS */ },
    onBackgroundClick: () => {
      showDetailPanel.value = false
      focusedNode.value = null
      lifelineNodes.value = []
    },
    onLifelineReady: (nodes, _edges, start) => {
      lifelineNodes.value = nodes
      focusedNode.value = start
      showDetailPanel.value = true
    },
  })
  interaction.enable()

  chapterTransition = new ChapterTransition(
    wipeRef.value!,
    coverRef.value!,
    titleRef.value!,
    { onReload: () => loadBoardData() },
  )

  scene.startRenderLoop()

  resizeObserver = new ResizeObserver(() => {
    if (!scene || !canvasRef.value) return
    scene.onResize(canvasRef.value.clientWidth, canvasRef.value.clientHeight)
  })
  resizeObserver.observe(canvasRef.value)

  await loadBoardData()
})

async function loadBoardData() {
  if (!scene || !interaction) return
  loading.value = true
  error.value = false
  try {
    const res = await getBoardSectionTimeline(props.boardId, days.value)
    if (res.success && res.data) {
      sections.value = res.data.sections
      relations.value = res.data.relations
      const dates = res.data.sections.map(s => s.period_date.slice(0, 10)).sort()
      const end = dates[dates.length - 1] ?? new Date().toISOString().slice(0, 10)
      const endDate = new Date(end)
      endDate.setDate(endDate.getDate() - (days.value - 1))
      const start = endDate.toISOString().slice(0, 10)
      const dateRange = { start, end }
      scene.loadBoardData(res.data.sections, res.data.relations, dateRange, days.value)
      interaction.setData(res.data.sections, res.data.relations, dateRange, days.value)
      // Snap camera to today focus.
      const tx = latestDayX(res.data.sections)
      directorCamera?.snapTo(directorCamera.todayFocus(tx))
    } else {
      error.value = true
    }
  } catch {
    error.value = true
  } finally {
    loading.value = false
  }
}

watch(() => days.value, () => {
  interaction?.setTimeRange(days.value)
  loadBoardData()
})

watch(() => props.boardId, (_id, oldId) => {
  if (oldId !== _id && chapterTransition) {
    const data: ChapterTransitionData = {
      name: `板块 #${_id}`,
      dateRange: `${days.value}天`,
      topicCount: sections.value.length,
    }
    interaction?.disable()
    chapterTransition.play(data)
    // Re-enable after transition completes.
    setTimeout(() => interaction?.enable(), 1000)
  }
})

function setDays(d: 7 | 14 | 30 | 60) {
  days.value = d
}

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  interaction?.dispose()
  scene?.dispose()
  scene = null
})

function close() {
  emit('close')
}

function openArticle(id: number) {
  emit('openArticle', id)
}
</script>

<template>
  <div class="tdw-root">
    <canvas ref="canvasRef" class="tdw-canvas" />
    <div ref="css2dContainerRef" class="tdw-css2d" />

    <!-- Top bar: close + days range -->
    <div class="tdw-topbar">
      <button class="tdw-btn" @click="close">
        <Icon icon="mdi:arrow-left" width="18" />
        <span>返回</span>
      </button>
      <div class="tdw-days-toggle">
        <button
          v-for="d in SUPPORTED_DAYS"
          :key="d"
          class="tdw-days-btn"
          :class="{ active: days === d }"
          @click="setDays(d)"
        >
          {{ d }}天
        </button>
      </div>
    </div>

    <!-- Detail panel (plain Vue overlay, fixed position) -->
    <Transition name="tdw-panel">
      <div v-if="showDetailPanel && focusedNode" class="tdw-detail-panel">
        <div class="tdw-detail-header">
          <Icon icon="mdi:folder" width="16" />
          <span>案件编号 #{{ focusedNode.id }}</span>
        </div>
        <div class="tdw-detail-title">{{ focusedNode.cluster_label }}</div>
        <div class="tdw-detail-meta">
          <span>{{ focusedNode.article_count }}篇 · {{ focusedNode.thread_count }}线索</span>
          <span class="tdw-detail-status">{{ focusedNode.status }}</span>
        </div>
        <div v-if="lifelineNodes.length > 0" class="tdw-detail-lifeline">
          <div class="tdw-detail-section-label">生命线 ({{ lifelineNodes.length }}节点)</div>
          <div
            v-for="n in lifelineNodes.slice(0, 10)"
            :key="n.id"
            class="tdw-lifeline-item"
            @click="openArticle(n.report_id)"
          >
            <span class="tdw-lifeline-date">{{ n.period_date.slice(0, 10) }}</span>
            <span class="tdw-lifeline-label">{{ n.cluster_label }}</span>
          </div>
        </div>
        <button class="tdw-btn tdw-btn-back" @click="close">返回总览</button>
      </div>
    </Transition>

    <!-- Chapter transition DOM (animated by ChapterTransition.ts) -->
    <div ref="wipeRef" class="tdw-wipe" />
    <div ref="coverRef" class="tdw-cover">
      <span class="tdw-cover-stamp">CONFIDENTIAL</span>
      <span ref="titleRef" data-chapter-title class="tdw-cover-title" />
    </div>

    <!-- Loading / error states -->
    <div v-if="loading" class="tdw-loading">载入中…</div>
    <div v-else-if="error" class="tdw-error">加载失败，请重试</div>
  </div>
</template>

<style scoped>
.tdw-root {
  position: fixed;
  inset: 0;
  background: #0a0f14;
  z-index: 50;
}
.tdw-canvas {
  display: block;
  width: 100%;
  height: 100%;
}
.tdw-css2d {
  position: absolute;
  inset: 0;
  pointer-events: none;
}
.tdw-topbar {
  position: absolute;
  top: 1rem;
  left: 1rem;
  right: 1rem;
  display: flex;
  justify-content: space-between;
  align-items: center;
  pointer-events: none;
}
.tdw-topbar > * { pointer-events: auto; }
.tdw-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.4rem 0.75rem;
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 0.4rem;
  color: rgba(255, 255, 255, 0.85);
  font-size: 0.85rem;
  cursor: pointer;
}
.tdw-btn:hover { background: rgba(255, 255, 255, 0.14); }
.tdw-days-toggle {
  display: flex;
  gap: 0.25rem;
  background: rgba(0, 0, 0, 0.4);
  border-radius: 0.4rem;
  padding: 0.2rem;
}
.tdw-days-btn {
  padding: 0.3rem 0.6rem;
  border: none;
  background: transparent;
  color: rgba(255, 255, 255, 0.6);
  border-radius: 0.3rem;
  cursor: pointer;
  font-size: 0.8rem;
}
.tdw-days-btn.active {
  background: #DC2626;
  color: #fff;
}

/* Detail panel — fixed position Vue overlay */
.tdw-detail-panel {
  position: absolute;
  right: 2rem;
  top: 50%;
  transform: translateY(-50%);
  width: 280px;
  max-height: 70vh;
  overflow-y: auto;
  background: #FFFBEB;
  border: 1px solid rgba(26, 26, 26, 0.15);
  border-radius: 0.5rem;
  padding: 1rem;
  color: #1A1A1A;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
  font-family: 'JetBrains Mono', 'Courier New', monospace;
}
.tdw-detail-header {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.75rem;
  color: #4B5563;
  border-bottom: 1px solid rgba(26, 26, 26, 0.1);
  padding-bottom: 0.5rem;
  margin-bottom: 0.5rem;
}
.tdw-detail-title {
  font-size: 1rem;
  font-weight: 600;
  margin-bottom: 0.4rem;
}
.tdw-detail-meta {
  display: flex;
  justify-content: space-between;
  font-size: 0.75rem;
  color: #4B5563;
  margin-bottom: 0.75rem;
}
.tdw-detail-status {
  color: #DC2626;
  font-weight: 600;
}
.tdw-detail-section-label {
  font-size: 0.7rem;
  color: #4B5563;
  margin-bottom: 0.3rem;
}
.tdw-lifeline-item {
  display: flex;
  gap: 0.5rem;
  padding: 0.25rem 0;
  font-size: 0.75rem;
  cursor: pointer;
  border-bottom: 1px dotted rgba(26, 26, 26, 0.1);
}
.tdw-lifeline-item:hover { background: rgba(220, 38, 38, 0.05); }
.tdw-lifeline-date { color: #D97706; min-width: 5.5rem; }
.tdw-btn-back { margin-top: 0.75rem; width: 100%; justify-content: center; }

/* motion-v-style transition (Vue <Transition>) */
.tdw-panel-enter-active, .tdw-panel-leave-active {
  transition: transform 0.3s ease, opacity 0.3s ease;
}
.tdw-panel-enter-from, .tdw-panel-leave-to {
  transform: translate(50px, -50%);
  opacity: 0;
}
.tdw-panel-enter-to, .tdw-panel-leave-from {
  transform: translate(0, -50%);
  opacity: 1;
}

/* Chapter transition */
.tdw-wipe {
  display: none;
  position: absolute;
  inset: 0;
  background: #DC2626;
  z-index: 60;
}
.tdw-cover {
  display: none;
  position: absolute;
  inset: 0;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  gap: 1rem;
  background: rgba(10, 15, 20, 0.85);
  z-index: 61;
}
.tdw-cover-stamp {
  color: #DC2626;
  font-family: 'JetBrains Mono', monospace;
  font-weight: 700;
  letter-spacing: 0.2em;
  border: 3px solid #DC2626;
  padding: 0.3rem 0.8rem;
  transform: rotate(-8deg);
}
.tdw-cover-title {
  color: #FFFBEB;
  font-family: 'JetBrains Mono', monospace;
  font-size: 1.5rem;
}

.tdw-loading, .tdw-error {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  color: rgba(255, 255, 255, 0.7);
  font-size: 1rem;
}
</style>
