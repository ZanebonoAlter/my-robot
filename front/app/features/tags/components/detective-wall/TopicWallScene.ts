/**
 * Three.js scene manager for the detective topic wall.
 *
 * Owns the renderer, camera, scene graph, CSS2D overlay renderer, render loop,
 * and lifecycle. Delegates card/string construction to CardGroup and RedString.
 *
 * @see specs/detective-wall-scene/spec.md
 */
import {
  Scene, PerspectiveCamera, WebGLRenderer, Color, PointLight, Mesh,
  PlaneGeometry, MeshStandardMaterial, CanvasTexture, RepeatWrapping, Vector3,
} from 'three'
import { CSS2DRenderer } from 'three/examples/jsm/renderers/CSS2DRenderer.js'
import type { SectionTimelineNode, SectionRelation, DateRange } from './types'
import { STYLE } from './types'
import { densityForDays } from './utils'
import { CardGroup } from './CardGroup'
import type { PinCard } from './types'
import { RedStringCollection } from './RedString'
import { FogSystem } from './FogSystem'
import { setupLighting } from './lighting'
import { WallPostProcessing } from './WallPostProcessing'

export class TopicWallScene {
  readonly scene = new Scene()
  readonly camera: PerspectiveCamera
  readonly renderer: WebGLRenderer
  readonly css2d: CSS2DRenderer
  readonly cardGroup = new CardGroup()
  readonly redStrings = new RedStringCollection()
  readonly fog: FogSystem
  readonly composer: WallPostProcessing
  /** Camera-following explorer lamp (updated each frame). */
  readonly followLight: PointLight
  /** Red light above the selected card; intensity 0 unless focused. */
  readonly selectionLight: PointLight

  private rafId = 0
  private disposed = false
  private lastTime = 0
  private readonly canvas: HTMLCanvasElement
  private readonly css2dContainer: HTMLElement
  /** Per-frame callbacks (e.g. orbit controls update). */
  private readonly frameCallbacks: Array<() => void> = []
  private wallMesh: Mesh | null = null
  private readonly selectionLightPosition = new Vector3()

  constructor(canvas: HTMLCanvasElement, css2dContainer: HTMLElement) {
    this.canvas = canvas
    this.css2dContainer = css2dContainer

    this.scene.background = new Color(STYLE.background)

    this.renderer = new WebGLRenderer({ canvas, antialias: true })
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2))
    this.renderer.setSize(canvas.clientWidth, canvas.clientHeight, false)

    this.camera = new PerspectiveCamera(50, canvas.clientWidth / canvas.clientHeight, 0.1, 1000)

    // CSS2D renderer for card tooltips (position: absolute overlay).
    this.css2d = new CSS2DRenderer({ element: document.createElement('div') })
    this.css2d.domElement.style.position = 'absolute'
    this.css2d.domElement.style.top = '0'
    this.css2d.domElement.style.pointerEvents = 'none'
    this.css2d.setSize(canvas.clientWidth, canvas.clientHeight)
    this.css2dContainer.appendChild(this.css2d.domElement)

    this.fog = new FogSystem(this.scene, STYLE.fog)
    const lights = setupLighting(this.scene)
    this.followLight = lights.followLight
    this.selectionLight = lights.selectionLight

    this.composer = new WallPostProcessing(this.renderer, this.scene, this.camera)
  }

  /** Move the selection light above a focused card, or turn it off. */
  setSelectionLight(card: PinCard | null): void {
    if (card) {
      card.group.getWorldPosition(this.selectionLightPosition)
      this.selectionLight.position.set(
        this.selectionLightPosition.x,
        this.selectionLightPosition.y,
        this.selectionLightPosition.z + 1.05,
      )
      this.selectionLight.intensity = 0.65
    } else {
      this.selectionLight.intensity = 0
    }
  }

  /** Register a per-frame callback (e.g. OrbitControls.update). */
  addFrameCallback(fn: () => void): void {
    this.frameCallbacks.push(fn)
  }

  /** Load sections + relations and build all 3D objects. Replaces prior data. */
  loadBoardData(
    sections: SectionTimelineNode[],
    relations: SectionRelation[],
    dateRange: DateRange,
    days: number,
  ): void {
    this.clearScene()
    this.cardGroup.buildCards(sections, relations, dateRange, this.scene)
    this.rebuildWall()
    this.redStrings.build(relations, this.cardGroup, this.scene)
    this.fog.setDensityForDays(days)
  }

  /** Remove all cards and strings from the scene (keeps lights/fog). */
  clearScene(): void {
    this.disposeWall()
    this.cardGroup.clear(this.scene)
    this.redStrings.clear(this.scene)
  }

  private rebuildWall(): void {
    this.disposeWall()
    const cards = this.cardGroup.cards
    if (cards.length === 0) return

    const xs = cards.map(card => card.position.x)
    const ys = cards.map(card => card.position.y)
    const minX = Math.min(...xs)
    const maxX = Math.max(...xs)
    const minY = Math.min(...ys)
    const maxY = Math.max(...ys)
    const width = Math.max(18, maxX - minX + 7)
    const height = Math.max(12, maxY - minY + 5)
    const centerX = (minX + maxX) / 2
    const centerY = (minY + maxY) / 2

    const texture = makeCorkTexture()
    texture.wrapS = RepeatWrapping
    texture.wrapT = RepeatWrapping
    texture.repeat.set(Math.max(2, width / 5), Math.max(2, height / 4))
    const material = new MeshStandardMaterial({
      color: new Color(STYLE.cork),
      map: texture,
      roughness: 0.95,
      metalness: 0,
    })
    this.wallMesh = new Mesh(new PlaneGeometry(width, height), material)
    this.wallMesh.position.set(centerX, centerY, -0.16)
    this.scene.add(this.wallMesh)
  }

  private disposeWall(): void {
    if (!this.wallMesh) return
    this.scene.remove(this.wallMesh)
    this.wallMesh.geometry.dispose()
    const mat = this.wallMesh.material as MeshStandardMaterial & { map?: { dispose: () => void } | null }
    mat.map?.dispose()
    mat.dispose()
    this.wallMesh = null
  }

  startRenderLoop(): void {
    if (this.rafId) return
    this.lastTime = performance.now()
    const tick = () => {
      if (this.disposed) return
      const now = performance.now()
      const dt = (now - this.lastTime) / 1000
      this.lastTime = now
      // Explorer lamp follows the camera so far-away cards stay lit.
      this.followLight.position.copy(this.camera.position)
      // Run registered frame callbacks (orbit pan/zoom, etc.) before rendering.
      for (const fn of this.frameCallbacks) fn()
      this.cardGroup.update(dt)
      this.composer.render(dt)
      this.css2d.render(this.scene, this.camera)
      this.rafId = requestAnimationFrame(tick)
    }
    this.rafId = requestAnimationFrame(tick)
  }

  stopRenderLoop(): void {
    if (this.rafId) {
      cancelAnimationFrame(this.rafId)
      this.rafId = 0
    }
  }

  onResize(width: number, height: number): void {
    this.camera.aspect = width / height
    this.camera.updateProjectionMatrix()
    this.renderer.setSize(width, height, false)
    this.css2d.setSize(width, height)
    this.composer.setSize(width, height)
  }

  dispose(): void {
    this.disposed = true
    this.stopRenderLoop()
    this.clearScene()
    this.composer.dispose()
    this.renderer.dispose()
    this.css2d.domElement.remove()
  }
}

/** Resolve initial fog density helper (re-exported for callers). */
export function initialFogDensity(days: number): number {
  return densityForDays(days)
}

function makeCorkTexture(): CanvasTexture {
  const canvas = document.createElement('canvas')
  canvas.width = 512
  canvas.height = 512
  const ctx = canvas.getContext('2d')!
  const rnd = seededWallNoise(42)

  const base = ctx.createLinearGradient(0, 0, canvas.width, canvas.height)
  base.addColorStop(0, '#3A2116')
  base.addColorStop(0.46, STYLE.cork)
  base.addColorStop(1, '#140D0B')
  ctx.fillStyle = base
  ctx.fillRect(0, 0, canvas.width, canvas.height)

  for (let i = 0; i < 7600; i++) {
    const warm = 18 + Math.floor(rnd() * 42)
    const alpha = 0.04 + rnd() * 0.16
    ctx.fillStyle = `rgba(${warm + 38}, ${warm + 18}, ${warm}, ${alpha})`
    ctx.fillRect(rnd() * canvas.width, rnd() * canvas.height, 1 + rnd() * 3, 1 + rnd() * 2)
  }

  const frost = ctx.createRadialGradient(132, 120, 10, 132, 120, 360)
  frost.addColorStop(0, 'rgba(255, 244, 214, 0.11)')
  frost.addColorStop(0.54, 'rgba(255, 244, 214, 0.035)')
  frost.addColorStop(1, 'rgba(255, 244, 214, 0)')
  ctx.fillStyle = frost
  ctx.fillRect(0, 0, canvas.width, canvas.height)

  ctx.strokeStyle = 'rgba(255, 244, 214, 0.05)'
  ctx.lineWidth = 1
  for (let x = 32; x < canvas.width; x += 32) {
    ctx.beginPath()
    ctx.moveTo(x + rnd() * 2, 0)
    ctx.lineTo(x + rnd() * 2, canvas.height)
    ctx.stroke()
  }

  ctx.strokeStyle = 'rgba(255, 244, 214, 0.045)'
  ctx.lineWidth = 1
  for (let y = 32; y < canvas.height; y += 32) {
    ctx.beginPath()
    ctx.moveTo(0, y + rnd() * 2)
    ctx.lineTo(canvas.width, y + rnd() * 2)
    ctx.stroke()
  }

  // Faint visual-novel chapter-route decoration behind the actual red strings.
  const routePoints = [
    [58, 394], [116, 324], [184, 352], [248, 272],
    [324, 296], [392, 216], [456, 250],
  ] as const
  ctx.strokeStyle = 'rgba(255, 238, 190, 0.12)'
  ctx.lineWidth = 3
  ctx.setLineDash([12, 9])
  ctx.beginPath()
  routePoints.forEach(([x, y], index) => {
    if (index === 0) ctx.moveTo(x, y)
    else ctx.lineTo(x, y)
  })
  ctx.stroke()
  ctx.setLineDash([])

  routePoints.forEach(([x, y], index) => {
    ctx.fillStyle = 'rgba(18, 11, 8, 0.55)'
    ctx.beginPath()
    ctx.arc(x + 3, y + 4, 13, 0, Math.PI * 2)
    ctx.fill()
    ctx.fillStyle = 'rgba(235, 203, 139, 0.72)'
    ctx.beginPath()
    ctx.arc(x, y, 11, 0, Math.PI * 2)
    ctx.fill()
    ctx.fillStyle = '#2A1710'
    ctx.font = '800 12px "Courier New", monospace'
    ctx.fillText(String(index + 1).padStart(2, '0'), x - 8, y + 4)
  })

  ctx.fillStyle = 'rgba(255, 238, 190, 0.08)'
  ctx.font = '900 54px "Courier New", monospace'
  ctx.rotate(-0.08)
  ctx.fillText('CASE MAP', 278, 88)
  ctx.setTransform(1, 0, 0, 1, 0, 0)

  ctx.fillStyle = 'rgba(255, 250, 230, 0.045)'
  for (let i = 0; i < 80; i++) {
    ctx.beginPath()
    ctx.arc(rnd() * canvas.width, rnd() * canvas.height, 0.5 + rnd() * 1.8, 0, Math.PI * 2)
    ctx.fill()
  }

  const texture = new CanvasTexture(canvas)
  texture.needsUpdate = true
  return texture
}

function seededWallNoise(seed: number): () => number {
  let t = seed + 0x6D2B79F5
  return () => {
    t += 0x6D2B79F5
    let r = Math.imul(t ^ (t >>> 15), 1 | t)
    r ^= r + Math.imul(r ^ (r >>> 7), 61 | r)
    return ((r ^ (r >>> 14)) >>> 0) / 4294967296
  }
}
