/**
 * WallCameraControls — OrbitControls wrapper for the detective wall.
 *
 * The wall is a 2.5D cork-board metaphor, so only pan (drag) + zoom (wheel)
 * are enabled; rotation is disabled to keep the axial "looking at a wall"
 * viewpoint stable.
 *
 * Coordination with DirectorCamera is the tricky part: DirectorCamera's GSAP
 * transitions write camera.position + lookAt directly, while OrbitControls
 * derives position from its own `target`. To keep them from fighting:
 *  - DirectorCamera.hooks.onTransitionStart  → disable orbit
 *  - DirectorCamera.hooks.onTargetUpdate      → sync controls.target (so the
 *    next orbit.update() won't snap the camera back)
 *  - DirectorCamera.hooks.onTransitionComplete → re-enable orbit
 *
 * Pan/zoom also pause hover raycasting (via the onStart/onEnd listener) so cards
 * don't flicker while dragging.
 *
 * @see specs/detective-wall-camera/spec.md
 */
import { MOUSE } from 'three'
import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls.js'
import type { PerspectiveCamera } from 'three'
import type { DirectorCamera } from './DirectorCamera'

export interface WallCameraControlsHooks {
  /** Called when an orbit pan/zoom begins; hover should pause. */
  onInteractStart?: () => void
  /** Called when an orbit pan/zoom ends; hover may resume. */
  onInteractEnd?: () => void
}

export class WallCameraControls {
  readonly controls: OrbitControls
  private readonly hooks: WallCameraControlsHooks

  constructor(
    camera: PerspectiveCamera,
    domElement: HTMLElement,
    directorCamera: DirectorCamera,
    hooks: WallCameraControlsHooks = {},
  ) {
    this.hooks = hooks
    this.controls = new OrbitControls(camera, domElement)

    // 2.5D: only pan + zoom, no rotation.
    this.controls.enableRotate = false
    this.controls.enablePan = true
    this.controls.enableZoom = true
    // Left-drag = pan (default is rotate, which we disabled anyway).
    this.controls.mouseButtons = {
      LEFT: MOUSE.PAN,
      MIDDLE: MOUSE.DOLLY,
      RIGHT: MOUSE.PAN,
    }
    // Keep the camera within a sensible zoom range of the wall.
    this.controls.minDistance = 3
    this.controls.maxDistance = 40

    // Pause hover while the user drags/zooms.
    this.controls.addEventListener('start', () => this.hooks.onInteractStart?.())
    this.controls.addEventListener('end', () => this.hooks.onInteractEnd?.())

    // Inject hooks into DirectorCamera so orbit stays in sync during transitions.
    directorCamera.hooks = {
      onTransitionStart: () => {
        this.controls.enabled = false
      },
      onTargetUpdate: (x, y, z) => {
        this.controls.target.set(x, y, z)
      },
      onTransitionComplete: () => {
        this.controls.enabled = true
      },
    }
  }

  /** Must be called every frame (OrbitControls requires update to apply pan). */
  update(): void {
    this.controls.update()
  }

  dispose(): void {
    this.controls.dispose()
  }
}
