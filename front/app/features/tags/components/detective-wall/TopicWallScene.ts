/**
 * Three.js scene manager for the detective topic wall.
 *
 * Owns the renderer, camera, scene graph, CSS2D overlay renderer, render loop,
 * and lifecycle. Delegates card/string construction to CardGroup and RedString.
 *
 * @see specs/detective-wall-scene/spec.md
 */
import { Scene, PerspectiveCamera, WebGLRenderer, Color } from 'three'
import { CSS2DRenderer } from 'three/examples/jsm/renderers/CSS2DRenderer.js'
import type { SectionTimelineNode, SectionRelation, DateRange } from './types'
import { STYLE } from './types'
import { densityForDays } from './utils'
import { CardGroup } from './CardGroup'
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

  private rafId = 0
  private disposed = false
  private lastTime = 0
  private readonly canvas: HTMLCanvasElement
  private readonly css2dContainer: HTMLElement

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
    setupLighting(this.scene)

    this.composer = new WallPostProcessing(this.renderer, this.scene, this.camera)
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
    this.redStrings.build(relations, this.cardGroup, this.scene)
    this.fog.setDensityForDays(days)
  }

  /** Remove all cards and strings from the scene (keeps lights/fog). */
  clearScene(): void {
    this.cardGroup.clear(this.scene)
    this.redStrings.clear(this.scene)
  }

  startRenderLoop(): void {
    if (this.rafId) return
    this.lastTime = performance.now()
    const tick = () => {
      if (this.disposed) return
      const now = performance.now()
      const dt = (now - this.lastTime) / 1000
      this.lastTime = now
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
