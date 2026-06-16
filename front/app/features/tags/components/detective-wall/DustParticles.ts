/**
 * DustParticles — motes drifting in the desk-lamp light cone.
 *
 * A single THREE.Points cloud (~150 motes) sampled inside an ellipsoidal cone
 * whose apex is the lamp shade and which opens toward the cork wall. Each frame
 * the motes drift on a slow sine and wrap when they leave the cone. Combined
 * with Bloom + Vignette this produces the "archive room dust in light" feel.
 *
 * One draw call; per-frame CPU cost is ~150×3 floats.
 *
 * @see openspec/changes/detective-wall-ambiance/design.md §DustParticles
 */
import {
  Points, BufferGeometry, BufferAttribute, PointsMaterial,
  CanvasTexture, AdditiveBlending, Color, Vector3, Scene,
} from 'three'
import { STYLE } from './types'

function makeMoteTexture(): CanvasTexture {
  const c = document.createElement('canvas')
  c.width = 32
  c.height = 32
  const ctx = c.getContext('2d')!
  const g = ctx.createRadialGradient(16, 16, 0, 16, 16, 16)
  g.addColorStop(0, 'rgba(255, 250, 230, 1)')
  g.addColorStop(0.4, 'rgba(255, 240, 200, 0.5)')
  g.addColorStop(1, 'rgba(255, 240, 200, 0)')
  ctx.fillStyle = g
  ctx.fillRect(0, 0, 32, 32)
  const tex = new CanvasTexture(c)
  tex.needsUpdate = true
  return tex
}

export class DustParticles {
  readonly points: Points
  private readonly geometry: BufferGeometry
  private readonly material: PointsMaterial
  private readonly texture: CanvasTexture
  private readonly baseX: Float32Array
  private readonly baseY: Float32Array
  private readonly baseZ: Float32Array
  private readonly phase: Float32Array
  private elapsed = 0
  private readonly speed = 0.4
  private readonly amp = 0.18
  private disposed = false

  constructor(origin: Vector3) {
    const count = STYLE.dust.count
    const positions = new Float32Array(count * 3)
    this.baseX = new Float32Array(count)
    this.baseY = new Float32Array(count)
    this.baseZ = new Float32Array(count)
    this.phase = new Float32Array(count)

    // Sample inside an ellipsoidal cone: apex at lamp, opening toward -z (wall),
    // widening and rising gently with distance.
    for (let i = 0; i < count; i++) {
      const t = Math.random() // 0..1 along the cone depth
      const depth = 1 + t * 5.5 // how far toward the wall
      const spread = 0.6 + t * 2.6 // widens with distance
      const x = origin.x + (Math.random() - 0.5) * spread * 2.2
      const y = origin.y - 0.3 + (Math.random() - 0.5) * spread + Math.random() * 1.6
      const z = origin.z - depth

      const i3 = i * 3
      positions[i3] = x
      positions[i3 + 1] = y
      positions[i3 + 2] = z
      this.baseX[i] = x
      this.baseY[i] = y
      this.baseZ[i] = z
      this.phase[i] = Math.random() * Math.PI * 2
    }

    this.geometry = new BufferGeometry()
    this.geometry.setAttribute('position', new BufferAttribute(positions, 3))

    this.texture = makeMoteTexture()
    this.material = new PointsMaterial({
      map: this.texture,
      color: new Color(STYLE.dust.color),
      size: STYLE.dust.size,
      transparent: true,
      opacity: 0.7,
      depthWrite: false,
      blending: AdditiveBlending,
      sizeAttenuation: true,
    })

    this.points = new Points(this.geometry, this.material)
    this.points.frustumCulled = false
  }

  update(dt: number): void {
    if (this.disposed) return
    this.elapsed += dt
    const posAttr = this.geometry.getAttribute('position')!
    const pos = posAttr.array as Float32Array
    const e = this.elapsed * this.speed
    const n = this.baseX.length
    for (let i = 0; i < n; i++) {
      const i3 = i * 3
      const bx = this.baseX[i]!
      const by = this.baseY[i]!
      const ph = this.phase[i]!
      pos[i3] = bx + Math.cos(e * 0.7 + ph) * this.amp * 0.6
      pos[i3 + 1] = by + Math.sin(e + ph) * this.amp
    }
    posAttr.needsUpdate = true
  }

  dispose(_scene: Scene): void {
    if (this.disposed) return
    this.disposed = true
    _scene.remove(this.points)
    this.geometry.dispose()
    this.material.dispose()
    this.texture.dispose()
  }
}
