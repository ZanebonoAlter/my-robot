/**
 * RedString + RedStringCollection — stylized red lines connecting related cards.
 *
 * Uses Line2 (fat lines) for controllable screen-space width. Straight lines
 * (no bezier / physics sag) per design.md §RedString. Distance attenuates
 * opacity (near = solid, far = faint).
 *
 * @see specs/detective-wall-scene/spec.md §RedString
 */
import { Scene, Color, Vector3, Vector2 } from 'three'
import { Line2 } from 'three/examples/jsm/lines/Line2.js'
import { LineMaterial } from 'three/examples/jsm/lines/LineMaterial.js'
import { LineGeometry } from 'three/examples/jsm/lines/LineGeometry.js'
import gsap from 'gsap'
import type { RedString as IRedString, SectionRelation } from './types'
import { STYLE } from './types'
import type { CardGroup } from './CardGroup'

const MAX_OPACITY_DISTANCE = 10

export class RedStringImpl implements IRedString {
  readonly fromId: number
  readonly toId: number
  readonly distance: number
  readonly line: Line2
  private readonly material: LineMaterial
  private baseOpacity: number

  constructor(from: Vector3, to: Vector3, rel: SectionRelation) {
    this.fromId = rel.from_id
    this.toId = rel.to_id
    this.distance = from.distanceTo(to)

    // Opacity falls off with distance: near → base, far → base * 0.3.
    const falloff = Math.max(0.3, 1 - this.distance / MAX_OPACITY_DISTANCE)
    this.baseOpacity = STYLE.string.baseOpacity * falloff

    const geometry = new LineGeometry()
    geometry.setPositions([from.x, from.y, from.z, to.x, to.y, to.z])

    this.material = new LineMaterial({
      color: new Color(STYLE.string.color),
      linewidth: STYLE.string.baseLinewidth,
      transparent: true,
      opacity: this.baseOpacity,
      // resolution must be set or linewidth renders as 0.
      resolution: new Vector2(window.innerWidth, window.innerHeight),
      dashed: false,
    })

    this.line = new Line2(geometry, this.material)
    this.line.computeLineDistances()
  }

  draw(progress: number): void {
    // Line2 supports dashed draw via 'dashSize' / 'gapSize'; simplest visible
    // draw effect: fade opacity in along progress.
    this.material.opacity = this.baseOpacity * progress
  }

  highlight(): void {
    gsap.to(this.material, {
      opacity: STYLE.string.highlightOpacity,
      linewidth: STYLE.string.highlightLinewidth,
      duration: 0.3,
    })
  }

  dim(): void {
    gsap.to(this.material, {
      opacity: this.baseOpacity * 0.2,
      duration: 0.3,
    })
  }

  reset(): void {
    gsap.to(this.material, {
      opacity: this.baseOpacity,
      linewidth: STYLE.string.baseLinewidth,
      duration: 0.3,
    })
  }

  setResolution(width: number, height: number): void {
    this.material.resolution.set(width, height)
  }

  dispose(): void {
    this.line.geometry.dispose()
    this.material.dispose()
  }
}

export class RedStringCollection {
  readonly strings: RedStringImpl[] = []

  build(relations: SectionRelation[], cardGroup: CardGroup, scene: Scene): void {
    for (const rel of relations) {
      const fromCard = cardGroup.getCardById(rel.from_id)
      const toCard = cardGroup.getCardById(rel.to_id)
      if (!fromCard || !toCard) continue
      const str = new RedStringImpl(fromCard.position, toCard.position, rel)
      this.strings.push(str)
      scene.add(str.line)
    }
  }

  getByEndpoints(a: number, b: number): RedStringImpl | undefined {
    return this.strings.find(s =>
      (s.fromId === a && s.toId === b) || (s.fromId === b && s.toId === a),
    )
  }

  setResolution(width: number, height: number): void {
    for (const s of this.strings) s.setResolution(width, height)
  }

  clear(scene: Scene): void {
    for (const s of this.strings) {
      scene.remove(s.line)
      s.dispose()
    }
    this.strings.length = 0
  }
}
