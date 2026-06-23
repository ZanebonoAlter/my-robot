/**
 * AmbientEnv — procedural warm environment map for PBR reflections.
 *
 * Builds a tiny throwaway scene (dark wood floor + a warm glowing orb where the
 * desk lamp sits + a cool overhead wash) and bakes it via PMREMGenerator into an
 * env map assigned to `scene.environment`. Metallic surfaces (pin heads, brass
 * lamp) then reflect warm highlights instead of reading as flat plastic.
 *
 * Does NOT replace `scene.background` (kept as a solid color; Vignette edges
 * the frame). One-shot generation in the constructor; never per-frame.
 *
 * @see openspec/changes/detective-wall-ambiance/design.md §AmbientEnv
 */
import {
  Scene as ThreeScene, WebGLRenderer, PMREMGenerator,
  Mesh, PlaneGeometry, SphereGeometry, MeshBasicMaterial, Color,
  type Texture, type WebGLRenderTarget,
} from 'three'
import { STYLE } from './types'

/** Build the miniature scene PMREM will sample. */
function buildProbeScene(): ThreeScene {
  const s = new ThreeScene()

  // Dark wooden floor plane (the desk surface reflection base).
  const ground = new Mesh(
    new PlaneGeometry(120, 120),
    new MeshBasicMaterial({ color: new Color(STYLE.desk.color) }),
  )
  ground.rotation.x = -Math.PI / 2
  ground.position.y = -6
  s.add(ground)

  // Warm glowing orb — stands in for the desk lamp (right-front of scene).
  const lampOrb = new Mesh(
    new SphereGeometry(10, 20, 20),
    new MeshBasicMaterial({ color: new Color(STYLE.lamp.bulb) }),
  )
  lampOrb.position.set(24, 4, 32)
  s.add(lampOrb)

  // Warm ambient haze around the lamp.
  const haze = new Mesh(
    new SphereGeometry(20, 20, 20),
    new MeshBasicMaterial({ color: new Color(STYLE.lamp.spotColor), transparent: true, opacity: 0.25 }),
  )
  haze.position.copy(lampOrb.position)
  s.add(haze)

  // Cool overhead wash (subtle cold rim from above/behind).
  const sky = new Mesh(
    new PlaneGeometry(120, 120),
    new MeshBasicMaterial({ color: new Color(STYLE.lighting.hemiSky) }),
  )
  sky.position.set(0, 40, -10)
  sky.rotation.x = Math.PI / 2
  s.add(sky)

  return s
}

export class AmbientEnv {
  private readonly pmrem: PMREMGenerator
  private readonly envRT: WebGLRenderTarget
  private readonly probeScene: ThreeScene
  private disposed = false

  constructor(scene: ThreeScene, renderer: WebGLRenderer) {
    this.pmrem = new PMREMGenerator(renderer)
    this.probeScene = buildProbeScene()
    this.envRT = this.pmrem.fromScene(this.probeScene, 0.04)
    scene.environment = this.envRT.texture as Texture
  }

  dispose(): void {
    if (this.disposed) return
    this.disposed = true
    this.envRT.dispose()
    this.pmrem.dispose()
    this.probeScene.traverse((obj) => {
      const mesh = obj as Mesh
      if (mesh.geometry) mesh.geometry.dispose()
      const mat = mesh.material as MeshBasicMaterial | undefined
      if (mat) mat.dispose()
    })
  }
}
