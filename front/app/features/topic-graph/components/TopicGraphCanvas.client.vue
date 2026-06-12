<script setup lang="ts">
import { shallowRef, onMounted, onBeforeUnmount, ref, watch, computed, nextTick } from 'vue'
import type { TopicGraphSceneEdge, TopicGraphSceneNode } from '~/features/topic-graph/utils/buildTopicGraphViewModel'
import { isHighlightedTopicGraphEdge, resolveTopicGraphLinkOpacity } from '~/features/topic-graph/utils/topicGraphCanvasLinks'

// Runtime color bridge: read CSS custom properties for Canvas rendering
function getCSSVar(name: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim()
}

interface TokenColors {
  accent: string
  accentHover: string
  accentSubtle: string
  bgElevated: string
  bgSunken: string
  textPrimary: string
  textSecondary: string
  textMuted: string
  borderSubtle: string
  borderMedium: string
  tagEvent: string
  tagPerson: string
  tagKeyword: string
  feedColor: string
}

// Lazy-resolved color palette from design tokens (resolved once per render cycle)
let _tokenCache: TokenColors | null = null
function tokenColors(): TokenColors {
  if (_tokenCache) return _tokenCache
  _tokenCache = {
    accent: getCSSVar('--color-accent') || 'rgba(240,138,75,0.85)',
    accentHover: getCSSVar('--color-accent-hover') || 'rgba(240,138,75,1)',
    accentSubtle: getCSSVar('--color-accent-subtle') || 'rgba(240,138,75,0.08)',
    bgElevated: getCSSVar('--color-bg-elevated') || 'rgba(17,27,38,0.98)',
    bgSunken: getCSSVar('--color-bg-sunken') || '#0e161d',
    textPrimary: getCSSVar('--color-text-primary') || 'rgba(255,255,255,0.9)',
    textSecondary: getCSSVar('--color-text-secondary') || 'rgba(255,255,255,0.6)',
    textMuted: getCSSVar('--color-text-muted') || 'rgba(255,255,255,0.35)',
    borderSubtle: getCSSVar('--color-border-subtle') || 'rgba(255,255,255,0.05)',
    borderMedium: getCSSVar('--color-border-medium') || 'rgba(255,255,255,0.1)',
    tagEvent: getCSSVar('--color-tag-event') || 'rgba(196,136,60,0.85)',
    tagPerson: getCSSVar('--color-tag-person') || 'rgba(45,138,122,0.85)',
    tagKeyword: getCSSVar('--color-tag-keyword') || 'rgba(74,93,138,0.85)',
    feedColor: getCSSVar('--color-text-secondary') || '#7b8a96',
  }
  return _tokenCache
}
function invalidateTokenCache() { _tokenCache = null }

interface Props {
  nodes: TopicGraphSceneNode[]
  edges: TopicGraphSceneEdge[]
  activeNodeId?: string | null
  focusRequestKey?: number
  featuredNodeIds?: string[]
  selectedCategory?: 'event' | 'person' | 'keyword' | null
  highlightedNodeIds?: string[]
  relatedEdgeIds?: string[]
}

const props = withDefaults(defineProps<Props>(), {
  activeNodeId: null,
  focusRequestKey: 0,
  featuredNodeIds: () => [],
  selectedCategory: null,
  highlightedNodeIds: () => [],
  relatedEdgeIds: () => [],
})

// Watch for theme changes and refresh token cache
const { theme } = useTheme()
watch(theme, () => {
  invalidateTokenCache()
  // Force re-render of the graph with new colors
  nextTick(() => {
    applyGraphData()
  })
})

const emit = defineEmits<{
  nodeClick: [node: TopicGraphSceneNode]
}>()

const containerRef = ref<HTMLDivElement | null>(null)
const graphInstance = shallowRef<any>(null)
const resizeObserver = shallowRef<ResizeObserver | null>(null)
const focusAnimationFrame = ref<number | null>(null)
const spriteTextCtor = shallowRef<any>(null)
const threeLib = shallowRef<any>(null)
const graphReady = ref(false)

// 连线显示模式
const linkDisplayMode = ref<'hidden' | 'selected' | 'all'>('hidden')

// 当前选中的题材（用于连线动画）
const selectedTopicForLinks = ref<{
  id: string
  category: 'event' | 'person' | 'keyword'
} | null>(null)

// 需要高亮的连线ID列表
const highlightedLinkIds = ref<Set<string>>(new Set())

// 动画状态
const isAnimatingLinks = ref(false)

const highlightedNodeSet = computed(() => new Set(props.highlightedNodeIds || []))
const highlightedEdgeSet = computed(() => new Set(props.relatedEdgeIds || []))
const focusHighlightActive = computed(() => highlightedNodeSet.value.size > 0)

// We compute the adjacency set for the active node to distinguish branch vs peripheral
const activeAdjacency = computed(() => {
  const set = new Set<string>()
  if (!props.activeNodeId) return set
  
  props.edges.forEach(edge => {
    const sourceId = resolveLinkNodeId(edge.source)
    const targetId = resolveLinkNodeId(edge.target)
    if (sourceId === props.activeNodeId) set.add(targetId)
    if (targetId === props.activeNodeId) set.add(sourceId)
  })
  return set
})

function getNodeEmphasis(nodeId: string): 'trunk' | 'branch' | 'peripheral' {
  if (nodeId === props.activeNodeId) return 'trunk'

  if (focusHighlightActive.value) {
    return highlightedNodeSet.value.has(nodeId) ? 'branch' : 'peripheral'
  }

  if (!props.activeNodeId) return 'branch'
  if (activeAdjacency.value.has(nodeId)) return 'branch'
  return 'peripheral'
}

async function setupGraph() {
  if (!containerRef.value) return

  const [{ default: ForceGraph3D }, { default: SpriteText }, threeModule] = await Promise.all([
    import('3d-force-graph'),
    import('three-spritetext'),
    import('three'),
  ])

  spriteTextCtor.value = SpriteText
  threeLib.value = threeModule

  const graph = new (ForceGraph3D as any)(containerRef.value, { controlType: 'orbit' })
    .backgroundColor('rgba(0,0,0,0)')
    .nodeRelSize(4)
    .enableNodeDrag(false)
    .linkOpacity(0) // 默认隐藏所有连线
    .linkWidth((link: TopicGraphSceneEdge) => buildLinkWidth(link))
    .linkColor((link: TopicGraphSceneEdge) => buildLinkColor(link))
    .linkDirectionalParticles((link: TopicGraphSceneEdge) => buildLinkParticles(link))
    .linkDirectionalParticleWidth((link: TopicGraphSceneEdge) => buildLinkParticleWidth(link))
    .linkDirectionalParticleColor((link: TopicGraphSceneEdge) => buildLinkParticleColor(link))
    .linkDirectionalParticleSpeed((link: TopicGraphSceneEdge) => buildLinkParticleSpeed(link))
    .nodeThreeObject((node: TopicGraphSceneNode) => buildNodeObject(node))
    .nodeLabel((node: TopicGraphSceneNode) => `${node.label} · ${node.kind}`)
    .onNodeClick((node: TopicGraphSceneNode) => emit('nodeClick', node))
    .d3VelocityDecay(0.2)
    .cooldownTicks(160)

  graph.d3Force('charge').strength((node: TopicGraphSceneNode) => node.kind === 'feed' ? -260 : -420)
  graph.d3Force('link').distance((link: TopicGraphSceneEdge) => link.kind === 'topic_topic' ? 132 : 184)
  graph.cameraPosition({ z: 200 })
  graphInstance.value = graph
  graphReady.value = true
  applyHighlightStyles()
  applyGraphData()

  resizeObserver.value = new ResizeObserver(() => {
    if (!containerRef.value || !graphInstance.value) return
    graphInstance.value.width(containerRef.value.clientWidth)
    graphInstance.value.height(containerRef.value.clientHeight)
  })
  resizeObserver.value.observe(containerRef.value)
}

function applyGraphData() {
  if (!graphInstance.value || !spriteTextCtor.value || !threeLib.value) return

  applyHighlightStyles()

  graphInstance.value.graphData({
    nodes: props.nodes,
    links: props.edges,
  })
}

function applyHighlightStyles() {
  if (!graphInstance.value) return

  const graph = graphInstance.value
  const hasFocusSelection = focusHighlightActive.value

graph
    .nodeOpacity(() => 0.98)
    .linkOpacity((link: TopicGraphSceneEdge) => {
      return resolveTopicGraphLinkOpacity(link, {
        linkDisplayMode: linkDisplayMode.value,
        highlightedLinkIds: highlightedLinkIds.value,
        highlightedNodeIds: highlightedNodeSet.value,
        relatedEdgeIds: highlightedEdgeSet.value,
      })
    })
}

/**
 * 计算与指定题材相关的所有连线
 */
function calculateRelatedLinks(
  topicId: string,
  allLinks: TopicGraphSceneEdge[],
): TopicGraphSceneEdge[] {
  return allLinks.filter(link => {
    const sourceId = resolveLinkNodeId(link.source)
    const targetId = resolveLinkNodeId(link.target)
    return sourceId === topicId || targetId === topicId
  })
}

/**
 * 混合两个颜色（简化版渐变）
 */
function blendColors(color1: string, color2: string): string {
  // 简化实现：返回橙色系渐变
  return tokenColors().accent
}

/**
 * 动态绘制连线（带动画效果）
 */
async function drawLinksAnimated(links: TopicGraphSceneEdge[]) {
  if (!graphInstance.value) return

  isAnimatingLinks.value = true

  const newHighlightedIds = new Set<string>()

  for (let i = 0; i < links.length; i++) {
    const link = links[i]!
    newHighlightedIds.add(link.id)

    highlightedLinkIds.value = new Set(newHighlightedIds)
    applyHighlightStyles()

    await new Promise(resolve => setTimeout(resolve, 80))
  }

  isAnimatingLinks.value = false
}

/**
 * 隐藏所有连线
 */
function hideAllLinks() {
  highlightedLinkIds.value = new Set()
  linkDisplayMode.value = 'hidden'
  applyHighlightStyles()
}

/**
 * 显示选中题材的相关连线
 */
function showLinksForTopic(topicId: string) {
  const relatedLinks = calculateRelatedLinks(topicId, props.edges)

  if (relatedLinks.length === 0) {
    hideAllLinks()
    return
  }

  linkDisplayMode.value = 'selected'
  highlightedLinkIds.value = new Set(relatedLinks.map(link => link.id))
  applyHighlightStyles()
}

function buildNodeObject(node: TopicGraphSceneNode) {
  if (!spriteTextCtor.value || !threeLib.value) return null

  const THREE = threeLib.value
  const SpriteText = spriteTextCtor.value
  const group = new THREE.Group()
  
  const emphasis = getNodeEmphasis(node.id)
  const isTrunk = emphasis === 'trunk'
  const isBranch = emphasis === 'branch'
  const isPeripheral = emphasis === 'peripheral'
  const isNeighborHighlighted = focusHighlightActive.value && highlightedNodeSet.value.has(node.id) && !isTrunk

  const isFeatured = isTrunk || props.featuredNodeIds.includes(node.id)
  
  // Base radius calculation
  let radius = node.kind === 'feed'
    ? Math.max(2.4, node.size * 0.12)
    : Math.max(3.2, node.size * 0.14)
    
  // Apply emphasis scaling
  if (isTrunk) {
    radius *= 1.8 // Trunk is significantly larger
  } else if (isNeighborHighlighted) {
    radius *= 1.18
  } else if (isPeripheral) {
    radius *= 0.6 // Peripheral nodes shrink
  }

// Opacity: quality-aware for topic nodes
  const opacity = node.opacity ?? 0.98

// Resolve theme-aware accent color for this node
  let nodeAccent = node.accent
  if (node.kind === 'topic' && node.category) {
    const categoryColors: Record<string, string> = {
      event: tokenColors().tagEvent,
      person: tokenColors().tagPerson,
      keyword: tokenColors().tagKeyword,
    }
    nodeAccent = categoryColors[node.category] || nodeAccent
  } else if (node.kind === 'feed') {
    nodeAccent = tokenColors().feedColor
  }

  const sphere = new THREE.Mesh(
    new THREE.SphereGeometry(radius, 24, 24),
    new THREE.MeshBasicMaterial({
      color: nodeAccent,
      transparent: true,
      opacity,
    }),
  )
  group.add(sphere)

  // Abstract tag glow: emissive halo using lighter version of accent color
  if (node.isAbstract) {
    const glowColor = lightenColor(nodeAccent, 0.35)
    const abstractHalo = new THREE.Mesh(
      new THREE.SphereGeometry(radius * 1.6, 24, 24),
      new THREE.MeshBasicMaterial({
        color: glowColor,
        transparent: true,
        opacity: 0.18,
      }),
    )
    group.add(abstractHalo)

    const abstractOuterGlow = new THREE.Mesh(
      new THREE.SphereGeometry(radius * 2.1, 24, 24),
      new THREE.MeshBasicMaterial({
        color: glowColor,
        transparent: true,
        opacity: 0.08,
      }),
    )
    group.add(abstractOuterGlow)
  }

  // Trunk gets a strong, pulsing-like halo
  if (isTrunk || isNeighborHighlighted) {
    const halo = new THREE.Mesh(
      new THREE.SphereGeometry(radius * (isTrunk ? 1.9 : 1.45), 24, 24),
      new THREE.MeshBasicMaterial({
        color: isTrunk ? tokenColors().accent : nodeAccent,
        transparent: true,
        opacity: isTrunk ? 0.13 : 0.08,
      }),
    )
    group.add(halo)

    if (isTrunk) {
      const focusRing = new THREE.Mesh(
        new THREE.SphereGeometry(radius * 2.55, 32, 32),
        new THREE.MeshBasicMaterial({
          color: tokenColors().accent,
          transparent: true,
          opacity: 0.08,
        }),
      )
      group.add(focusRing)

      const innerHalo = new THREE.Mesh(
        new THREE.SphereGeometry(radius * 1.4, 24, 24),
        new THREE.MeshBasicMaterial({
          color: tokenColors().textPrimary,
          transparent: true,
          opacity: 0.2,
        }),
      )
      group.add(innerHalo)
    }
  }

// Labels: Always show for all nodes, style varies by emphasis
  const label = new SpriteText(compactLabel(node.label, isTrunk ? 40 : 20))
  label.color = tokenColors().textPrimary
  label.textHeight = isTrunk ? 10 : (isBranch ? 6 : 5.5)
  label.backgroundColor = isTrunk ? `${tokenColors().bgSunken}c7` : `${tokenColors().bgSunken}57`
  label.padding = isTrunk ? 4 : 2
  label.borderRadius = 10
  label.position.set(0, radius + (isTrunk ? 12 : 8), 0)
  group.add(label)

  return group
}

function buildLinkWidth(link: TopicGraphSceneEdge) {
  if (focusHighlightActive.value) {
    return isHighlightedEdge(link, highlightedNodeSet.value, highlightedEdgeSet.value)
      ? Math.max(1.1, link.weight * 0.58)
      : 0.12
  }

  if (!props.activeNodeId) {
    return link.kind === 'topic_topic' ? Math.max(0.5, link.weight * 0.34) : Math.max(0.35, link.weight * 0.18)
  }

  return isFocusedEdge(link)
    ? Math.min(4, Math.max(1.8, link.weight * 0.8)) // Cap at 4 to prevent overwhelming lines
    : 0.1 // Thinner unfocused edges
}

function buildLinkColor(link: TopicGraphSceneEdge) {
  if (focusHighlightActive.value) {
    return isHighlightedEdge(link, highlightedNodeSet.value, highlightedEdgeSet.value)
      ? tokenColors().accent
      : `${tokenColors().borderSubtle}`
  }

  if (!props.activeNodeId) {
    return link.kind === 'topic_topic' ? 'rgba(188,206,224,0.08)' : 'rgba(126,151,173,0.08)'
  }

  return isFocusedEdge(link) ? tokenColors().accentHover : `${tokenColors().borderSubtle}`
}

function buildLinkParticles(link: TopicGraphSceneEdge) {
  if (focusHighlightActive.value) {
    return isHighlightedEdge(link, highlightedNodeSet.value, highlightedEdgeSet.value) ? 2 : 0
  }

  if (!props.activeNodeId) return 0
  return isFocusedEdge(link) ? 2 : 0
}

function buildLinkParticleWidth(link: TopicGraphSceneEdge) {
  if (focusHighlightActive.value && isHighlightedEdge(link, highlightedNodeSet.value, highlightedEdgeSet.value)) {
    return 2.2
  }

  return isFocusedEdge(link) ? 2 : 0
}

function buildLinkParticleColor(link: TopicGraphSceneEdge) {
  if (focusHighlightActive.value && isHighlightedEdge(link, highlightedNodeSet.value, highlightedEdgeSet.value)) {
    return tokenColors().accent
  }

  return isFocusedEdge(link) ? tokenColors().accentHover : 'rgba(0,0,0,0)'
}

function buildLinkParticleSpeed(link: TopicGraphSceneEdge) {
  if (focusHighlightActive.value && isHighlightedEdge(link, highlightedNodeSet.value, highlightedEdgeSet.value)) {
    return 0.008
  }

  return isFocusedEdge(link) ? 0.007 : 0
}

function isFocusedEdge(link: TopicGraphSceneEdge) {
  return resolveLinkNodeId(link.source) === props.activeNodeId || resolveLinkNodeId(link.target) === props.activeNodeId
}

function isHighlightedEdge(
  link: TopicGraphSceneEdge,
  highlightedNodes: Set<string>,
  highlightedEdges: Set<string>,
) {
  return isHighlightedTopicGraphEdge(link, highlightedNodes, highlightedEdges)
}

function resolveLinkNodeId(node: string | TopicGraphSceneNode) {
  return typeof node === 'string' ? node : node.id
}

function compactLabel(label: string, maxLength: number) {
  return label.length > maxLength ? `${label.slice(0, maxLength)}…` : label
}

function lightenColor(hex: string, amount: number): string {
  const color = hex.replace('#', '')
  const r = Math.min(255, parseInt(color.slice(0, 2), 16) + Math.round(255 * amount))
  const g = Math.min(255, parseInt(color.slice(2, 4), 16) + Math.round(255 * amount))
  const b = Math.min(255, parseInt(color.slice(4, 6), 16) + Math.round(255 * amount))
  return `#${r.toString(16).padStart(2, '0')}${g.toString(16).padStart(2, '0')}${b.toString(16).padStart(2, '0')}`
}

function cancelFocusAnimation() {
  if (focusAnimationFrame.value === null) return

  cancelAnimationFrame(focusAnimationFrame.value)
  focusAnimationFrame.value = null
  
  // Re-enable controls if they were disabled during animation
  const controls = graphInstance.value?.controls?.()
  if (controls && !controls.enabled) {
    controls.enabled = true
  }
}

function resolveGraphNode(nodeId: string) {
  const graph = graphInstance.value
  const liveGraphData = graph?.graphData?.()
  const liveNode = Array.isArray(liveGraphData?.nodes)
    ? liveGraphData.nodes.find((item: TopicGraphSceneNode) => item.id === nodeId)
    : null

  return liveNode || props.nodes.find(item => item.id === nodeId) || null
}

async function focusActiveNode() {
  if (!graphInstance.value || !props.activeNodeId) return

  await nextTick()
  await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()))

  const graph = graphInstance.value
  const nodeId = props.activeNodeId
  const node = resolveGraphNode(nodeId)
  if (!node) return

  const x = node.x ?? 0
  const y = node.y ?? 0
  const z = node.z ?? 0
  const camera = graph.camera?.()
  const controls = graph.controls?.()
  const currentTarget = controls?.target
    ? {
        x: controls.target.x,
        y: controls.target.y,
        z: controls.target.z,
      }
    : { x: 0, y: 0, z: 0 }

  const focusDistance = node.kind === 'feed' ? 92 : 128

  let direction = { x: 0, y: 0, z: 1 }
  if (camera?.position) {
    const rawOffset = {
      x: camera.position.x - currentTarget.x,
      y: camera.position.y - currentTarget.y,
      z: camera.position.z - currentTarget.z,
    }
    const rawDistance = Math.hypot(rawOffset.x, rawOffset.y, rawOffset.z)

    if (Number.isFinite(rawDistance) && rawDistance > 1) {
      direction = {
        x: rawOffset.x / rawDistance,
        y: rawOffset.y / rawDistance,
        z: rawOffset.z / rawDistance,
      }
    }
  }

cancelFocusAnimation()

  if (camera?.position && controls?.target?.set && typeof controls.update === 'function') {
    // Disable controls during animation to prevent interference
    const wasEnabled = controls.enabled
    controls.enabled = false

    const startCameraPosition = {
      x: camera.position.x,
      y: camera.position.y,
      z: camera.position.z,
    }
    const startTarget = {
      x: controls.target.x,
      y: controls.target.y,
      z: controls.target.z,
    }
    const duration = 900
    const startedAt = performance.now()

    const step = (now: number) => {
      const currentNode = resolveGraphNode(nodeId) || node
      const currentLookAt = {
        x: currentNode.x ?? x,
        y: currentNode.y ?? y,
        z: currentNode.z ?? z,
      }
      const currentCameraPosition = {
        x: currentLookAt.x + direction.x * focusDistance,
        y: currentLookAt.y + direction.y * focusDistance,
        z: currentLookAt.z + direction.z * focusDistance,
      }
      const progress = Math.min(1, (now - startedAt) / duration)
      const eased = 1 - Math.pow(1 - progress, 3)

      camera.position.set(
        startCameraPosition.x + (currentCameraPosition.x - startCameraPosition.x) * eased,
        startCameraPosition.y + (currentCameraPosition.y - startCameraPosition.y) * eased,
        startCameraPosition.z + (currentCameraPosition.z - startCameraPosition.z) * eased,
      )
      controls.target.set(
        startTarget.x + (currentLookAt.x - startTarget.x) * eased,
        startTarget.y + (currentLookAt.y - startTarget.y) * eased,
        startTarget.z + (currentLookAt.z - startTarget.z) * eased,
      )
      controls.update()

      if (progress < 1) {
        focusAnimationFrame.value = requestAnimationFrame(step)
        return
      }

const finalNode = resolveGraphNode(nodeId) || currentNode
      controls.target.set(finalNode.x ?? currentLookAt.x, finalNode.y ?? currentLookAt.y, finalNode.z ?? currentLookAt.z)
      controls.update()
      controls.enabled = wasEnabled
      focusAnimationFrame.value = null
    }

    focusAnimationFrame.value = requestAnimationFrame(step)
    return
  }

  const lookAt = { x, y, z }
  const targetCameraPosition = {
    x: x + direction.x * focusDistance,
    y: y + direction.y * focusDistance,
    z: z + direction.z * focusDistance,
  }

  graph.cameraPosition(
    targetCameraPosition,
    lookAt,
    900,
  )
}

watch(() => [props.nodes, props.edges, props.featuredNodeIds, props.highlightedNodeIds, props.relatedEdgeIds, props.selectedCategory], applyGraphData)
watch(() => [props.activeNodeId, props.focusRequestKey] as const, async ([newId]) => {
  if (newId) {
    await focusActiveNode()
    await showLinksForTopic(newId)
  } else {
    hideAllLinks()
  }
})

onMounted(() => {
  void setupGraph()
  invalidateTokenCache()
})

onBeforeUnmount(() => {
  cancelFocusAnimation()
  resizeObserver.value?.disconnect()
})
</script>

<template>
  <div
    ref="containerRef"
    class="topic-canvas w-full rounded-[34px]"
    data-testid="topic-graph-canvas"
    :data-state="graphReady ? 'ready' : 'initializing'"
    :data-selected-category="props.selectedCategory || 'none'"
    :data-highlight-count="String((props.highlightedNodeIds || []).length)"
  />
</template>

<style scoped>
.topic-canvas {
  position: relative;
  width: 100%;
  height: clamp(34rem, 68vh, 58rem);
  min-height: 34rem;
  overflow: hidden;
  background:
    radial-gradient(circle at 18% 20%, var(--color-accent-subtle), transparent 28%),
    radial-gradient(circle at 76% 18%, var(--color-link-subtle), transparent 26%),
    linear-gradient(180deg, var(--color-bg-sunken), var(--color-bg-elevated));
}

.topic-canvas :deep(canvas) {
  display: block;
}

@media (max-width: 1024px) {
  .topic-canvas {
    height: clamp(28rem, 62vh, 44rem);
    min-height: 28rem;
  }
}
</style>
