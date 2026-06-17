/**
 * Three.js scene manager for the detective topic wall.
 *
 * Owns the renderer, camera, scene graph, CSS2D overlay renderer, render loop,
 * and lifecycle. Delegates card/string construction to CardGroup and RedString.
 *
 * @see specs/detective-wall-scene/spec.md
 */
import {
  Scene, PerspectiveCamera, WebGLRenderer, Color, PointLight, SpotLight, Mesh,
  BoxGeometry, MeshStandardMaterial, RepeatWrapping, Vector3,
  TextureLoader, SRGBColorSpace,
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
import { AmbientEnv } from './AmbientEnv'
import { SetDressing } from './SetDressing'
import { DustParticles } from './DustParticles'
import { injectDirectionalFog } from './shaders/directionalFog'

export interface WallCameraBounds {
  minX: number
  maxX: number
  minY: number
  maxY: number
}

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
  readonly spot: SpotLight
  readonly followLight: PointLight
  /** Red light above the selected card; intensity 0 unless focused. */
  readonly selectionLight: PointLight
  /** Procedural warm env map for PBR reflections (pin brass etc.). */
  readonly ambientEnv: AmbientEnv
  /** Desk/lamp/dossier environment layer (rebuilt per data load). */
  setDressing: SetDressing | null = null
  /** Dust motes in the lamp cone (rebuilt per data load). */
  dust: DustParticles | null = null

  private rafId = 0
  private disposed = false
  private lastTime = 0
  private readonly canvas: HTMLCanvasElement
  private readonly css2dContainer: HTMLElement
  /** Per-frame callbacks (e.g. orbit controls update). */
  private readonly frameCallbacks: Array<() => void> = []
  private wallMesh: Mesh | null = null
  private cameraBounds: WallCameraBounds | null = null
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
    this.spot = lights.spot
    this.followLight = lights.followLight
    this.selectionLight = lights.selectionLight
    this.ambientEnv = new AmbientEnv(this.scene, this.renderer)

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

    // Layout bounds drive desk/lamp/dust placement and the directional-fog origin.
    const xs = this.cardGroup.cards.map(c => c.position.x)
    const hasCards = xs.length > 0
    const minX = hasCards ? Math.min(...xs) : 0
    const latestDayX = hasCards ? Math.max(...xs) : 0
    const tw = hasCards ? latestDayX - minX : 0
    const ys = this.cardGroup.cards.map(c => c.position.y)
    const centerX = hasCards ? (minX + latestDayX) / 2 : 0
    const centerY = ys.length > 0 ? (Math.min(...ys) + Math.max(...ys)) / 2 : 0

    // Environment first: it owns the shared fog uniforms the cork wall injects below.
    this.buildEnvironment(latestDayX, minX, tw, centerX, centerY)

    this.rebuildWall()
    this.redStrings.build(relations, this.cardGroup, this.scene)
    this.fog.setDensityForDays(days)
  }

  /** (Re)build the desk/lamp/dust layer and aim the main spotlight from the lamp. */
  private buildEnvironment(
    latestDayX: number,
    minX: number,
    timelineWidth: number,
    wallCenterX: number,
    wallCenterY: number,
  ): void {
    this.setDressing = new SetDressing({ latestDayX, minX, timelineWidth })
    this.scene.add(this.setDressing.group)
    const lampConeOrigin = new Vector3(
      this.setDressing.lampPosition.x,
      this.setDressing.lampPosition.y + 0.82,
      this.setDressing.lampPosition.z - 0.42,
    )
    const lampTarget = new Vector3(wallCenterX, wallCenterY + 0.35, 0)
    this.dust = new DustParticles(lampConeOrigin, lampTarget)
    this.scene.add(this.dust.points)
    // Warm desk-lamp cone: start just outside the shade opening, toward the wall.
    this.spot.position.copy(lampConeOrigin)
    this.spot.target.position.copy(lampTarget)
    this.spot.target.updateMatrixWorld()
  }

  /** Remove cards, strings, and the environment layer (keeps lights/fog/env map). */
  clearScene(): void {
    this.disposeWall()
    this.cardGroup.clear(this.scene)
    this.redStrings.clear(this.scene)
    this.setDressing?.dispose(this.scene)
    this.setDressing = null
    this.dust?.dispose(this.scene)
    this.dust = null
  }

  getCameraBounds(): WallCameraBounds | null {
    return this.cameraBounds ? { ...this.cameraBounds } : null
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
    const width = Math.max(96, maxX - minX + 48)
    const wallTop = Math.max(maxY + 12, 16)
    const wallBottom = Math.min(minY - 12, STYLE.desk.y - 16)
    const height = Math.max(52, wallTop - wallBottom)
    const depth = 0.16
    const centerX = (minX + maxX) / 2
    const centerY = (wallTop + wallBottom) / 2

    const texture = new TextureLoader().load('/textures/detective-wall/wall_plaster_diff.jpg')
    texture.colorSpace = SRGBColorSpace
    texture.wrapS = RepeatWrapping
    texture.wrapT = RepeatWrapping
    texture.repeat.set(Math.max(2, width / 5), Math.max(2, height / 4))
    const frontMaterial = new MeshStandardMaterial({
      color: new Color(STYLE.cork),
      map: texture,
      roughness: 0.95,
      metalness: 0,
      emissive: new Color('#2f281f'),
      emissiveIntensity: 0.26,
    })
    if (this.setDressing) injectDirectionalFog(frontMaterial, this.setDressing.fogUniforms)
    const edgeMaterial = new MeshStandardMaterial({
      color: new Color('#6f5e50'),
      roughness: 0.96,
      metalness: 0,
      emissive: new Color('#3a3028'),
      emissiveIntensity: 0.34,
    })
    this.wallMesh = new Mesh(
      new BoxGeometry(width, height, depth),
      [edgeMaterial, edgeMaterial, edgeMaterial, edgeMaterial, frontMaterial, frontMaterial],
    )
    this.wallMesh.position.set(centerX, centerY, STYLE.wall.backZ - depth / 2)
    this.scene.add(this.wallMesh)
    this.cameraBounds = {
      minX: centerX - width / 2 + 12,
      maxX: centerX + width / 2 - 12,
      minY: centerY - height / 2 + 8,
      maxY: centerY + height / 2 - 8,
    }
  }

  private disposeWall(): void {
    if (!this.wallMesh) return
    this.scene.remove(this.wallMesh)
    this.wallMesh.geometry.dispose()
    const materials = Array.isArray(this.wallMesh.material)
      ? this.wallMesh.material as MeshStandardMaterial[]
      : [this.wallMesh.material as MeshStandardMaterial]
    const disposedMaps = new Set<{ dispose: () => void }>()
    const disposedMaterials = new Set<MeshStandardMaterial>()
    for (const mat of materials) {
      if (disposedMaterials.has(mat)) continue
      if (mat.map && !disposedMaps.has(mat.map)) {
        mat.map.dispose()
        disposedMaps.add(mat.map)
      }
      mat.dispose()
      disposedMaterials.add(mat)
    }
    this.wallMesh = null
    this.cameraBounds = null
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
      this.dust?.update(dt)
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
    this.ambientEnv.dispose()
    this.composer.dispose()
    this.renderer.dispose()
    this.css2d.domElement.remove()
  }
}

/** Resolve initial fog density helper (re-exported for callers). */
export function initialFogDensity(days: number): number {
  return densityForDays(days)
}
