/**
 * InteractionLayer — raycasting, BFS lifeline orchestration, time-range switching.
 *
 * Bridges Vue state and the Three.js scene: pointer events drive hover/click,
 * clicks trigger BFS lifelines (date-window constrained, see utils.bfsLifeline),
 * and results animate via GSAP timelines.
 *
 * @see specs/detective-wall-interaction/spec.md
 */
import { Raycaster, Vector2 } from 'three'
import type * as THREE from 'three'
import gsap from 'gsap'
import type {
  InteractionCallbacks, InteractionState, PinCard, RedString,
  SectionRelation, SectionTimelineNode, DateRange,
} from './types'
import type { TopicWallScene } from './TopicWallScene'
import type { DirectorCamera } from './DirectorCamera'
import { bfsLifeline, edgeKey, topicLifelineNodes } from './utils'

const CLICK_MAX_MOVE_PX = 5
/**
 * Extra hit padding (screen px) added to Line2's linewidth when raycasting red
 * strings. Line2's raycast uses `material.linewidth + threshold` as the half-width
 * of the clickable band (see LineSegments2.raycast). Base linewidth is 2 (half-width
 * 1px), which is too narrow to click reliably; +4 makes the band ~3px wide.
 */
const STRING_RAYCAST_PADDING_PX = 4

export class InteractionLayer {
  private readonly raycaster = new Raycaster()
  private readonly pointer = new Vector2()
  private readonly state: InteractionState = {
    mode: 'idle',
    focusedNodeId: null,
    lifelineNodeIds: new Set(),
    lifelineEdgeKeys: new Set(),
    hoveredId: null,
  }

  private enabled = false
  private pendingPointer: { x: number; y: number } | null = null
  private rafScheduled = false
  private downPos: { x: number; y: number } | null = null
  /** When true (e.g. during orbit drag), hover raycasting is skipped. */
  private hoverSuspended = false

  // Cached data for BFS
  private currentSections: SectionTimelineNode[] = []
  private currentRelations: SectionRelation[] = []
  private currentDateRange: DateRange = { start: '', end: '' }
  private currentDays = 7

  constructor(
    private readonly scene: TopicWallScene,
    private readonly directorCamera: DirectorCamera,
    private readonly canvas: HTMLCanvasElement,
    private readonly callbacks: InteractionCallbacks,
  ) {
    // Widen the Line2 click/hover band so thin red strings stay easy to hit.
    // Raycaster.params.Line2 is not defined by default, so create it here.
    this.raycaster.params.Line2 = { threshold: STRING_RAYCAST_PADDING_PX }
  }

  /** Suspend/resume hover (used by orbit controls during drag). */
  setHoverSuspended(suspended: boolean): void {
    this.hoverSuspended = suspended
    if (suspended) this.clearHover()
  }

  /** Cache the latest board data (called by TopicWallScene.loadBoardData). */
  setData(
    sections: SectionTimelineNode[],
    relations: SectionRelation[],
    dateRange: DateRange,
    days: number,
  ): void {
    this.currentSections = sections
    this.currentRelations = relations
    this.currentDateRange = dateRange
    this.currentDays = days
  }

  enable(): void {
    if (this.enabled) return
    this.enabled = true
    this.canvas.addEventListener('pointermove', this.onPointerMove)
    this.canvas.addEventListener('pointerdown', this.onPointerDown)
    this.canvas.addEventListener('pointerup', this.onPointerUp)
    // Tooltip clicks (CSS2D label sits above the mesh) forward to card clicks.
    this.scene.css2d.domElement.addEventListener('click', this.onTooltipClick)
  }

  disable(): void {
    this.enabled = false
    this.canvas.removeEventListener('pointermove', this.onPointerMove)
    this.canvas.removeEventListener('pointerdown', this.onPointerDown)
    this.canvas.removeEventListener('pointerup', this.onPointerUp)
    this.scene.css2d.domElement.removeEventListener('click', this.onTooltipClick)
  }

  /** Forward a click on a CSS2D tooltip element to its card's click handler. */
  private onTooltipClick = (e: MouseEvent) => {
    const target = e.target as HTMLElement | null
    const el = target?.closest<HTMLElement>('[data-card-id]')
    if (!el) return
    const id = Number(el.dataset.cardId)
    if (Number.isNaN(id)) return
    const card = this.scene.cardGroup.getCardById(id)
    if (card) this.handleCardClick(card)
  }

  // --- Event handlers ---

  private onPointerMove = (e: PointerEvent) => {
    this.pendingPointer = { x: e.clientX, y: e.clientY }
    if (!this.rafScheduled) {
      this.rafScheduled = true
      requestAnimationFrame(this.processHover)
    }
  }

  private onPointerDown = (e: PointerEvent) => {
    this.downPos = { x: e.clientX, y: e.clientY }
  }

  private onPointerUp = (e: PointerEvent) => {
    if (!this.downPos) return
    const dx = e.clientX - this.downPos.x
    const dy = e.clientY - this.downPos.y
    const moved = Math.sqrt(dx * dx + dy * dy)
    this.downPos = null
    if (moved > CLICK_MAX_MOVE_PX) return // it was a drag

    this.updatePointer(e.clientX, e.clientY)
    this.processClick()
  }

  private updatePointer(clientX: number, clientY: number): void {
    const rect = this.canvas.getBoundingClientRect()
    this.pointer.x = ((clientX - rect.left) / rect.width) * 2 - 1
    this.pointer.y = -((clientY - rect.top) / rect.height) * 2 + 1
  }

  // --- Hover (rAF-throttled) ---

  private processHover = () => {
    this.rafScheduled = false
    if (!this.enabled || !this.pendingPointer || this.hoverSuspended) return
    this.updatePointer(this.pendingPointer.x, this.pendingPointer.y)
    this.pendingPointer = null

    this.raycaster.setFromCamera(this.pointer, this.scene.camera)
    const card = this.pickCard()
    const prevHovered = this.state.hoveredId

    if (card) {
      if (card.data.id !== prevHovered) {
        this.clearHover()
        this.state.hoveredId = card.data.id
        card.elevate()
        this.highlightNeighborStrings(card)
        this.callbacks.onCardHover(card)
      }
    } else if (prevHovered !== null) {
      this.clearHover()
      this.callbacks.onCardHover(null)
    }
  }

  private clearHover(): void {
    if (this.state.hoveredId !== null) {
      const prev = this.scene.cardGroup.getCardById(this.state.hoveredId)
      prev?.settle()
    }
    this.state.hoveredId = null
    // Reset non-lifeline strings that were hover-highlighted.
    for (const str of this.scene.redStrings.strings) {
      if (!this.state.lifelineEdgeKeys.has(edgeKey(str.fromId, str.toId))) {
        str.reset()
      }
    }
  }

  private highlightNeighborStrings(card: PinCard): void {
    for (const str of this.scene.redStrings.strings) {
      if (str.fromId === card.data.id || str.toId === card.data.id) {
        str.highlight()
      }
    }
  }

  // --- Click ---

  private processClick(): void {
    this.raycaster.setFromCamera(this.pointer, this.scene.camera)
    const card = this.pickCard()
    if (card) {
      this.handleCardClick(card)
      return
    }
    const str = this.pickString()
    if (str) {
      this.handleStringClick(str)
      return
    }
    // Background click → reset (focusing) or notify Vue (lifecycle/idle).
    if (this.state.mode === 'focusing') {
      this.resetToOverview()
    } else {
      // lifecycle & idle: let Vue decide (lifecycle exit requires re-fetch).
      this.callbacks.onBackgroundClick()
    }
  }

  private pickCard(): PinCard | null {
    const meshes: THREE.Object3D[] = []
    for (const card of this.scene.cardGroup.cards) {
      meshes.push(card.group)
    }
    const hits = this.raycaster.intersectObjects(meshes, true)
    if (hits.length === 0) return null
    // Walk up to find the group with cardId userData.
    let obj = hits[0]!.object
    while (obj) {
      if (obj.userData?.cardId != null) {
        return this.scene.cardGroup.getCardById(obj.userData.cardId) ?? null
      }
      obj = obj.parent!
    }
    return null
  }

  private pickString(): RedString | null {
    const lines = this.scene.redStrings.strings.map(s => s.line)
    const hits = this.raycaster.intersectObjects(lines, false)
    if (hits.length === 0) return null
    const line = hits[0]!.object
    return this.scene.redStrings.strings.find(s => s.line === line) ?? null
  }

  private handleCardClick(card: PinCard): void {
    // In lifecycle mode the whole scene IS one lifecycle line — no BFS.
    // Just refocus the selection light + notify Vue (panel updates).
    if (this.state.mode === 'lifecycle') {
      this.state.focusedNodeId = card.data.id
      this.scene.setSelectionLight(card)
      this.callbacks.onCardClick(card)
      return
    }
    // Same card in focusing mode → reset.
    if (this.state.mode === 'focusing' && this.state.focusedNodeId === card.data.id) {
      this.resetToOverview()
      return
    }
    this.triggerLifeline(card)
  }

  private handleStringClick(str: RedString): void {
    // Jump focus to the other endpoint.
    const targetId = str.fromId === this.state.focusedNodeId ? str.toId : str.fromId
    const target = this.scene.cardGroup.getCardById(targetId)
    if (target) this.triggerLifeline(target)
  }

  // --- BFS lifeline ---

  private triggerLifeline(card: PinCard): void {
    const nodeMap = new Map(this.currentSections.map(n => [n.id, n] as const))
    // Fold in every section sharing the focus's persistent topic as BFS roots,
    // so a narrative's cards light up together even when their similarity
    // edges were severed by label drift.
    const preset = topicLifelineNodes(card.data, this.currentSections)
    const result = bfsLifeline(card.data.id, this.currentRelations, nodeMap, this.currentDateRange, preset)

    this.state.mode = 'focusing'
    this.state.focusedNodeId = card.data.id
    this.state.lifelineNodeIds = result.nodes
    this.state.lifelineEdgeKeys = result.edges

    const lifelineNodes = this.currentSections.filter(n => result.nodes.has(n.id))
    const lifelineEdges = this.currentRelations.filter(r =>
      result.edges.has(edgeKey(r.from_id, r.to_id)),
    )

    this.playLifelineAnimation(card, result.nodes, result.depth)
    this.scene.setSelectionLight(card)
    this.callbacks.onCardClick(card)
    this.callbacks.onLifelineReady(lifelineNodes, lifelineEdges, card.data)
  }

  /** GSAP timeline: dim non-lifeline cards → camera focus → stagger highlights by BFS depth. */
  private playLifelineAnimation(
    card: PinCard,
    nodeIds: Set<number>,
    depth: Map<number, number>,
  ): void {
    const tl = gsap.timeline()

    // Dim non-lifeline cards (stagger by index — order is cosmetic here).
    const dimCards = this.scene.cardGroup.cards.filter(c => !nodeIds.has(c.data.id))
    dimCards.forEach((c, i) => {
      tl.add(() => c.dim(), i * 0.02)
    })

    // Camera focus (parallel).
    tl.add(() => {
      const shot = this.directorCamera.topicFocus(card)
      this.directorCamera.transitionTo(shot)
    }, 0)

    // Highlight lifeline nodes + draw their strings, staggered by BFS depth from
    // the focus (depth 0 = immediate, depth N = N * 0.08s). Spec §BFS 动画序列.
    const lifelineCards = this.scene.cardGroup.cards.filter(c => nodeIds.has(c.data.id))
    lifelineCards.forEach((c) => {
      const d = depth.get(c.data.id) ?? 0
      const nodeDelay = 0.1 + d * 0.08
      tl.add(() => c.highlight(), nodeDelay)
      // Draw connected strings slightly before the node.
      for (const str of this.scene.redStrings.strings) {
        if (
          (str.fromId === c.data.id && nodeIds.has(str.toId)) ||
          (str.toId === c.data.id && nodeIds.has(str.fromId))
        ) {
          tl.add(() => str.highlight(), nodeDelay - 0.05)
        }
      }
    })
  }

  // --- Mode transitions ---

  setTimeRange(days: number): void {
    this.currentDays = days
    // Recompute date window from days relative to the latest section date.
    const dates = this.currentSections.map(s => s.period_date.slice(0, 10)).sort()
    if (dates.length === 0) return
    const end = dates[dates.length - 1]!
    const endDate = new Date(end)
    endDate.setDate(endDate.getDate() - (days - 1))
    const start = endDate.toISOString().slice(0, 10)
    this.currentDateRange = { start, end }

    // In focusing mode, re-run BFS with the new window.
    if (this.state.mode === 'focusing' && this.state.focusedNodeId !== null) {
      const card = this.scene.cardGroup.getCardById(this.state.focusedNodeId)
      if (card) this.triggerLifeline(card)
    }
  }

  resetToOverview(): void {
    this.state.mode = 'idle'
    this.state.focusedNodeId = null
    this.state.lifelineNodeIds.clear()
    this.state.lifelineEdgeKeys.clear()
    this.scene.cardGroup.resetAll()
    this.scene.setSelectionLight(null)
    for (const str of this.scene.redStrings.strings) str.reset()
    this.callbacks.onBackgroundClick()
  }

  /** Focus an already-visible card without recomputing the current lifeline. */
  focusNode(nodeId: number): void {
    const card = this.scene.cardGroup.getCardById(nodeId)
    if (!card) return
    if (this.state.mode === 'idle') {
      this.triggerLifeline(card)
      return
    }
    this.state.focusedNodeId = card.data.id
    this.scene.setSelectionLight(card)
    this.directorCamera.transitionTo(this.directorCamera.topicFocus(card))
    this.callbacks.onCardClick(card)
  }

  /**
   * Enter full lifecycle mode (spec §面板操作 查看完整生命周期).
   * Vue fetches lifecycle data via getSectionLifecycle and passes it here;
   * this rebuilds the scene with only that topic's evolution line, disables
   * fog, and moves the camera to the lifecycleFull shot.
   */
  enterLifecycle(
    sections: SectionTimelineNode[],
    relations: SectionRelation[],
    dateRange: DateRange,
  ): void {
    this.currentSections = sections
    this.currentRelations = relations
    this.currentDateRange = dateRange
    // Distinct day count → column count for layout + camera framing.
    const dayCount = new Set(sections.map(s => s.period_date.slice(0, 10))).size
    this.state.mode = 'lifecycle'
    this.state.focusedNodeId = null
    this.state.lifelineNodeIds.clear()
    this.state.lifelineEdgeKeys.clear()
    this.scene.loadBoardData(sections, relations, dateRange, dayCount)
    this.scene.fog.disable()
    this.scene.setSelectionLight(null)
    this.directorCamera.transitionTo(this.directorCamera.lifecycleFull(dayCount))
  }

  /**
   * Exit lifecycle mode: re-enable fog (for the current timeline window) and
   * reset interaction state. Vue is responsible for reloading the timeline
   * data via loadBoardData afterwards (it owns the fetch).
   */
  exitLifecycle(): void {
    this.state.mode = 'idle'
    this.state.focusedNodeId = null
    this.scene.fog.enable(this.currentDays)
    this.scene.setSelectionLight(null)
  }

  dispose(): void {
    this.disable()
  }
}
