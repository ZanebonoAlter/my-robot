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
import SpriteText from 'three-spritetext'
import gsap from 'gsap'
import type { PinCard as IPinCard, SectionTimelineNode, SectionRelation, DateRange } from './types'
import { STYLE } from './types'
import { layoutCards } from './utils'

export class PinCardImpl implements IPinCard {
  readonly data: SectionTimelineNode
  readonly group: Group
  readonly position: Vector3
  private readonly paperMaterial: MeshStandardMaterial
  private readonly pinMaterial: MeshStandardMaterial
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
    const label = data.cluster_label.slice(0, 14)
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
  }

  settle(): void {
    if (this.state !== 'elevated') return
    this.state = 'normal'
    gsap.to(this.group.position, { z: this.baseZ, duration: 0.2, ease: 'power2.out' })
  }

  highlight(): void {
    this.state = 'highlighted'
    gsap.to(this.paperMaterial, { emissiveIntensity: 0.5, duration: 0.3 })
  }

  dim(): void {
    this.state = 'dimmed'
    gsap.to(this.paperMaterial, { opacity: 0.35, transparent: true, duration: 0.3 })
  }

  reset(): void {
    this.state = 'normal'
    gsap.to(this.group.position, { z: this.baseZ, duration: 0.2, ease: 'power2.out' })
    gsap.to(this.paperMaterial, { emissiveIntensity: 0, opacity: 1, transparent: false, duration: 0.3 })
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
