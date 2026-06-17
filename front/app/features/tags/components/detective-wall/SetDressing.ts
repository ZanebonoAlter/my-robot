/**
 * SetDressing — the detective desk environment layer.
 *
 * Builds the foreground desk slab, a banker's desk lamp (the visual source of
 * the main spotlight), and dossier stacks. All
 * StandardMaterials get the shared directional-fog injection so the scene reads
 * "clear at today, fogged into the past".
 *
 * The lamp position is exposed so TopicWallScene can aim the main spotlight and
 * spawn dust at the right cone apex. Surgical: cards live on z≈0 and are never
 * touched here.
 *
 * @see openspec/changes/detective-wall-ambiance/design.md §SetDressing
 */
import {
  Group, Mesh, BoxGeometry, CylinderGeometry, SphereGeometry,
  MeshStandardMaterial, Color, CanvasTexture, RepeatWrapping, Vector3, Scene,
  TextureLoader, SRGBColorSpace, type Texture,
  type Material,
} from 'three'
import { STYLE } from './types'
import {
  injectDirectionalFog,
  type DirectionalFogUniforms,
} from './shaders/directionalFog'

export interface SetDressingOptions {
  latestDayX: number
  minX: number
  timelineWidth: number
}

/** Pure lamp placement (extracted for unit testing). */
export function lampPositionFor(
  latestDayX: number,
  offset: { x: number; z: number },
  deskY: number,
): Vector3 {
  return new Vector3(latestDayX + offset.x, deskY, offset.z)
}

function seededNoise(seed: number): () => number {
  let t = seed + 0x6d2b79f5
  return () => {
    t += 0x6d2b79f5
    let r = Math.imul(t ^ (t >>> 15), 1 | t)
    r ^= r + Math.imul(r ^ (r >>> 7), 61 | r)
    return ((r ^ (r >>> 14)) >>> 0) / 4294967296
  }
}

function makeDossierTexture(seed: number): CanvasTexture {
  const c = document.createElement('canvas')
  c.width = 256
  c.height = 192
  const ctx = c.getContext('2d')!
  const rnd = seededNoise(seed)

  const grad = ctx.createLinearGradient(0, 0, c.width, c.height)
  grad.addColorStop(0, STYLE.dossier.stackColor)
  grad.addColorStop(1, '#C9B080')
  ctx.fillStyle = grad
  ctx.fillRect(0, 0, c.width, c.height)

  // Faint file-tab lines.
  ctx.strokeStyle = 'rgba(80, 50, 20, 0.18)'
  ctx.lineWidth = 1
  for (let y = 22; y < c.height; y += 26) {
    ctx.beginPath()
    ctx.moveTo(0, y)
    ctx.lineTo(c.width, y)
    ctx.stroke()
  }
  // Wear.
  for (let i = 0; i < 500; i++) {
    ctx.fillStyle = `rgba(60, 40, 18, ${0.03 + rnd() * 0.08})`
    ctx.fillRect(rnd() * c.width, rnd() * c.height, 1 + rnd() * 2, 1)
  }
  const tex = new CanvasTexture(c)
  tex.needsUpdate = true
  return tex
}

function loadRepeatingTexture(path: string, repeatX: number, repeatY: number): Texture {
  const tex = new TextureLoader().load(path)
  tex.colorSpace = SRGBColorSpace
  tex.wrapS = tex.wrapT = RepeatWrapping
  tex.repeat.set(repeatX, repeatY)
  return tex
}

export class SetDressing {
  readonly group = new Group()
  readonly lampPosition: Vector3
  readonly fogUniforms: DirectionalFogUniforms
  private readonly textures: Texture[] = []

  constructor(opts: SetDressingOptions) {
    const { latestDayX, minX, timelineWidth } = opts
    const { desk, lamp, directionalFog } = STYLE

    this.fogUniforms = {
      uFogOriginX: { value: latestDayX },
      uDirFogDensity: { value: directionalFog.density },
      uDirFogRange: { value: directionalFog.range },
      uDirFogColor: { value: new Color(directionalFog.color) },
    }

    const deskWidth = Math.max(timelineWidth + 36, 72)
    const centerX = minX + timelineWidth / 2
    const deskDepth = desk.zFront - desk.zBack
    const deskThickness = 0.3

    // --- Desk slab (oversized box, not a 2D plane) ---
    const deskTex = loadRepeatingTexture('/textures/detective-wall/desk_oak_diff.jpg', 8, 4)
    this.textures.push(deskTex)
    const deskMat = new MeshStandardMaterial({
      color: new Color(desk.color),
      map: deskTex,
      roughness: 0.92,
      metalness: 0.05,
    })
    injectDirectionalFog(deskMat, this.fogUniforms)
    const deskMesh = new Mesh(new BoxGeometry(deskWidth, deskThickness, deskDepth), deskMat)
    deskMesh.position.set(centerX, desk.y - deskThickness / 2, (desk.zBack + desk.zFront) / 2)
    this.group.add(deskMesh)

    // --- Desk lamp (banker's style: base + stem + green shade + bulb + cap) ---
    this.lampPosition = lampPositionFor(latestDayX, lamp.offset, desk.y)
    this.buildLamp()

    // --- Dossier stacks (left past + right far) ---
    this.buildDossierStack(minX - 1.5, desk.y, 3.0, 4, 11)
    this.buildDossierStack(latestDayX + 4, desk.y, 2.4, 3, 23)
  }

  private buildLamp(): void {
    const { desk, lamp } = STYLE
    const lampGroup = new Group()
    lampGroup.position.set(this.lampPosition.x, desk.y, this.lampPosition.z)

    const brass = new MeshStandardMaterial({
      color: new Color(lamp.brass), roughness: 0.35, metalness: 0.85,
    })
    injectDirectionalFog(brass, this.fogUniforms)
    const glass = new MeshStandardMaterial({
      color: new Color(lamp.glass),
      roughness: 0.36,
      metalness: 0.18,
      emissive: new Color(lamp.glass),
      emissiveIntensity: 0.08,
    })
    injectDirectionalFog(glass, this.fogUniforms)
    const bulb = new MeshStandardMaterial({
      color: new Color(lamp.bulb),
      emissive: new Color(lamp.bulb), emissiveIntensity: lamp.bulbEmissive,
    })

    // Base disc.
    const base = new Mesh(new CylinderGeometry(0.42, 0.5, 0.12, 24), brass)
    base.position.y = 0.06
    lampGroup.add(base)

    // Stem.
    const stem = new Mesh(new CylinderGeometry(0.05, 0.06, 1.0, 12), brass)
    stem.position.y = 0.62
    lampGroup.add(stem)

    // Shade (truncated cone, open-ended, wide at bottom).
    const shade = new Mesh(
      new CylinderGeometry(0.18, 0.4, 0.46, 22, 1, true),
      glass,
    )
    shade.position.y = 1.2
    lampGroup.add(shade)

    // Shade top cap.
    const cap = new Mesh(new CylinderGeometry(0.18, 0.18, 0.04, 22), brass)
    cap.position.y = 1.43
    lampGroup.add(cap)

    // Bulb (emissive — blooms via postprocessing).
    const bulbMesh = new Mesh(new SphereGeometry(0.13, 16, 16), bulb)
    bulbMesh.position.y = 1.05
    lampGroup.add(bulbMesh)

    this.group.add(lampGroup)
  }

  private buildDossierStack(
    x: number, deskY: number, z: number, count: number, seed: number,
  ): void {
    const { dossier } = STYLE
    const tex = makeDossierTexture(seed)
    this.textures.push(tex)
    const stack = new Group()
    const rnd = seededNoise(seed)
    for (let i = 0; i < count; i++) {
      const mat = new MeshStandardMaterial({
        color: new Color(dossier.stackColor),
        map: tex,
        roughness: 0.9,
        metalness: 0,
      })
      injectDirectionalFog(mat, this.fogUniforms)
      const box = new Mesh(new BoxGeometry(1.8, 0.16, 1.3), mat)
      box.position.set((rnd() - 0.5) * 0.12, deskY + 0.08 + i * 0.17, z + (rnd() - 0.5) * 0.1)
      box.rotation.z = (rnd() - 0.5) * 0.05
      box.rotation.y = (rnd() - 0.5) * 0.1
      stack.add(box)
    }
    this.group.add(stack)
  }

  /** Update the directional-fog origin (today column) after a reflow. */
  setFogOrigin(latestDayX: number): void {
    this.fogUniforms.uFogOriginX.value = latestDayX
  }

  dispose(scene: Scene): void {
    scene.remove(this.group)
    this.group.traverse((obj) => {
      const mesh = obj as Mesh
      const geo = mesh.geometry
      if (geo) geo.dispose()
      const mat = mesh.material as Material | Material[] | undefined
      if (Array.isArray(mat)) mat.forEach((m) => m.dispose())
      else if (mat) mat.dispose()
    })
    for (const t of this.textures) t.dispose()
    this.textures.length = 0
  }
}
