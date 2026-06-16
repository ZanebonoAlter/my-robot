/**
 * PinCard + CardGroup — topic cards pinned to the cork wall.
 *
 * Each card is a Group: paper box + red pin + title sprite + status bar +
 * meta text. Positions come from layoutCards(); micro-tilt and Z jitter are
 * baked in at build time.
 *
 * @see specs/detective-wall-scene/spec.md §PinCard / §CardGroup / §Layout Algorithm
 */
import {
  Group, Mesh, BoxGeometry, CylinderGeometry, SphereGeometry,
  MeshStandardMaterial, Color, Scene, PlaneGeometry, Vector3,
} from 'three'
import { CSS2DObject } from 'three/examples/jsm/renderers/CSS2DRenderer.js'
import SpriteText from 'three-spritetext'
import gsap from 'gsap'
import type { PinCard as IPinCard, SectionTimelineNode, SectionRelation, DateRange } from './types'
import { STYLE } from './types'
import { layoutCards } from './utils'

/** Status → 中文标签（tooltip 与详情面板共用）。 */
const STATUS_LABELS: Record<string, string> = {
  emerging: '新兴',
  continuing: '持续',
  split: '分化',
  merge: '合并',
  ending: '结束',
}

export class PinCardImpl implements IPinCard {
  readonly data: SectionTimelineNode
  readonly group: Group
  readonly position: Vector3
  private readonly paperMaterial: MeshStandardMaterial
  private readonly pinMaterial: MeshStandardMaterial
  /** CSS2D tooltip that follows the card in screen space (spec §Card Tooltip). */
  readonly tooltip: CSS2DObject
  private baseZ = 0
  private state: 'normal' | 'elevated' | 'highlighted' | 'dimmed' = 'normal'

  constructor(data: SectionTimelineNode) {
    this.data = data
    this.group = new Group()
    this.position = new Vector3()
    this.group.userData.cardId = data.id

    const { card, pin } = STYLE

    // Paper card. emissive base color set so highlight can modulate intensity.
    this.paperMaterial = new MeshStandardMaterial({
      color: new Color(card.paper),
      roughness: card.roughness,
      metalness: card.metalness,
      emissive: new Color('#6b4a1a'),
      emissiveIntensity: 0,
    })
    const paper = new Mesh(
      new BoxGeometry(card.width, card.height, card.depth),
      this.paperMaterial,
    )
    this.group.add(paper)

    // Red pin (cylinder shaft + sphere head), top-center of the card.
    this.pinMaterial = new MeshStandardMaterial({
      color: new Color(pin.color),
      roughness: pin.roughness,
      metalness: pin.metalness,
    })
    const shaft = new Mesh(
      new CylinderGeometry(pin.radius * 0.3, pin.radius * 0.3, 0.15, 8),
      this.pinMaterial,
    )
    shaft.position.set(0, card.height / 2 + 0.075, 0)
    const head = new Mesh(
      new SphereGeometry(pin.radius, 16, 16),
      this.pinMaterial,
    )
    head.position.set(0, card.height / 2 + 0.2, 0)
    this.group.add(shaft, head)

    // Title sprite (canvas-rendered text).
    // Title sprite (canvas-rendered text). Truncate with ellipsis past 14 chars.
    const full = data.cluster_label
    const label = full.length > 14 ? full.slice(0, 14) + '…' : full
    const title = new SpriteText(label, 0.22, '#1A1A1A')
    title.position.set(0, 0.25, card.depth / 2 + 0.01)
    this.group.add(title)

    // Status color bar (thin plane on the card face).
    const statusColor = STYLE.statusColors[data.status] ?? '#9ca3af'
    const barMat = new MeshStandardMaterial({ color: new Color(statusColor) })
    const bar = new Mesh(new PlaneGeometry(card.width * 0.7, 0.08), barMat)
    bar.position.set(0, -0.1, card.depth / 2 + 0.01)
    this.group.add(bar)

    // Meta text ("5篇 · 3线索").
    const meta = new SpriteText(`${data.article_count}篇 · ${data.thread_count}线索`, 0.15, '#4B5563')
    meta.position.set(0, -0.45, card.depth / 2 + 0.01)
    this.group.add(meta)

    // CSS2D tooltip (spec §Card Tooltip): shown on hover, hidden otherwise.
    // Positioned above the card; CSS2DRenderer projects it into screen space.
    // pointer-events are enabled so clicking the tooltip text also selects the
    // card (the label sits above the 3D mesh, which users naturally click).
    const tooltipEl = document.createElement('div')
    tooltipEl.className = 'tdw-card-tooltip'
    tooltipEl.dataset.cardId = String(data.id)
    tooltipEl.style.display = 'none'
    tooltipEl.style.pointerEvents = 'auto'
    tooltipEl.style.cursor = 'pointer'
    const labelLine = document.createElement('div')
    labelLine.className = 'tdw-card-tooltip-label'
    labelLine.textContent = data.cluster_label
    const statusLine = document.createElement('div')
    statusLine.className = 'tdw-card-tooltip-status'
    const statusLabel = STATUS_LABELS[data.status] ?? data.status
    statusLine.textContent = `${statusLabel} · ${data.article_count}篇`
    tooltipEl.append(labelLine, statusLine)
    this.tooltip = new CSS2DObject(tooltipEl)
    this.tooltip.position.set(0, card.height / 2 + 0.5, 0)
    this.group.add(this.tooltip)
  }

  /** Apply a layout result (position + rotation). */
  place(position: Vector3, rotationZ: number): void {
    this.baseZ = position.z
    this.position.copy(position)
    this.group.position.copy(position)
    this.group.rotation.z = rotationZ
  }

  elevate(): void {
    if (this.state === 'elevated') return
    this.state = 'elevated'
    gsap.to(this.group.position, { z: this.baseZ + 0.2, duration: 0.2, ease: 'power2.out' })
    this.tooltip.element.style.display = 'block'
  }

  settle(): void {
    if (this.state !== 'elevated') return
    this.state = 'normal'
    gsap.to(this.group.position, { z: this.baseZ, duration: 0.2, ease: 'power2.out' })
    this.tooltip.element.style.display = 'none'
  }

  highlight(): void {
    this.state = 'highlighted'
    // Keep the glow subtle (0.2) — a higher intensity washes out the title/meta
    // text rendered on the card face.
    gsap.to(this.paperMaterial, { emissiveIntensity: 0.2, duration: 0.3 })
    // Hide tooltip in lifeline mode (detail panel shows the info instead).
    this.tooltip.element.style.display = 'none'
  }

  dim(): void {
    this.state = 'dimmed'
    gsap.to(this.paperMaterial, { opacity: 0.35, transparent: true, duration: 0.3 })
    this.tooltip.element.style.display = 'none'
  }

  reset(): void {
    this.state = 'normal'
    gsap.to(this.group.position, { z: this.baseZ, duration: 0.2, ease: 'power2.out' })
    gsap.to(this.paperMaterial, { emissiveIntensity: 0, opacity: 1, transparent: false, duration: 0.3 })
    this.tooltip.element.style.display = 'none'
  }
}

export class CardGroup {
  readonly cards: PinCardImpl[] = []
  private readonly byId = new Map<number, PinCardImpl>()

  buildCards(
    sections: SectionTimelineNode[],
    _relations: SectionRelation[],
    _dateRange: DateRange,
    scene: Scene,
  ): void {
    const layout = layoutCards(sections)
    for (const section of sections) {
      const card = new PinCardImpl(section)
      const pos = layout.get(section.id)
      if (pos) {
        card.place(pos.position, pos.rotationZ)
      }
      this.cards.push(card)
      this.byId.set(section.id, card)
      scene.add(card.group)
    }
  }

  getCardById(id: number): PinCardImpl | undefined {
    return this.byId.get(id)
  }

  /** Stagger entrance: cards rise from below the wall. */
  staggerEntrance(intervalSec: number): gsap.core.Timeline {
    const tl = gsap.timeline()
    this.cards.forEach((card, i) => {
      const targetY = card.group.position.y
      card.group.position.y = targetY - 3
      card.group.visible = false
      tl.set(card.group, { visible: true }, i * intervalSec)
      tl.to(card.group.position, { y: targetY, duration: 0.4, ease: 'back.out(1.4)' }, i * intervalSec)
    })
    return tl
  }

  /** Stagger exit: cards fall away. */
  staggerExit(intervalSec: number): gsap.core.Timeline {
    const tl = gsap.timeline()
    this.cards.forEach((card, i) => {
      tl.to(card.group.position, { y: card.group.position.y - 3, duration: 0.3, ease: 'power2.in' }, i * intervalSec)
    })
    return tl
  }

  highlightSet(ids: Set<number>): void {
    for (const card of this.cards) {
      if (ids.has(card.data.id)) card.highlight()
      else card.dim()
    }
  }

  dimAll(): void {
    for (const card of this.cards) card.dim()
  }

  resetAll(): void {
    for (const card of this.cards) card.reset()
  }

  /** Per-frame update hook (reserved for future idle animation). */
  update(_dt: number): void {
    // no-op for now
  }

  clear(scene: Scene): void {
    for (const card of this.cards) {
      scene.remove(card.group)
      // Remove the CSS2D tooltip element from the DOM (it lives outside the canvas).
      card.tooltip.element.remove()
      card.group.traverse((obj) => {
        const mesh = obj as Mesh
        if (mesh.geometry) mesh.geometry.dispose()
        const mat = mesh.material
        if (Array.isArray(mat)) mat.forEach(m => m.dispose())
        else if (mat) (mat as MeshStandardMaterial).dispose()
      })
    }
    this.cards.length = 0
    this.byId.clear()
  }
}
