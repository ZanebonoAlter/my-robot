/**
 * DirectorCamera — GSAP-driven camera cinematography.
 *
 * All camera movement goes through transitionTo(); direct camera.position writes
 * are forbidden (except snapTo for init). Each transition kills the prior
 * timeline. fov clamped to [35,65], camera.y floored at 2.
 *
 * @see specs/detective-wall-camera/spec.md
 */
import { PerspectiveCamera, Vector3 } from 'three'
import gsap from 'gsap'
import type { CameraShot, PinCard } from './types'
import { STYLE } from './types'
import { timelineWidth } from './utils'

const FOV_MIN = 35
const FOV_MAX = 65
const Y_MIN = 2

function clampFov(fov: number): number {
  return Math.min(FOV_MAX, Math.max(FOV_MIN, fov))
}

export class DirectorCamera {
  currentShot: CameraShot
  private activeTimeline: gsap.core.Timeline | null = null

  constructor(private readonly camera: PerspectiveCamera) {
    // Default: a neutral overview shot; callers snapTo a real preset at init.
    this.currentShot = {
      position: new Vector3(0, 6, 8),
      target: new Vector3(0, 0, 0),
      fov: 50,
      duration: 0,
      ease: 'power2.inOut',
      name: 'initial',
    }
  }

  // --- Presets (return new CameraShot objects) ---

  /** Focus on the latest day column (today), slightly elevated, slight tilt. */
  todayFocus(latestDayXValue: number): CameraShot {
    return {
      position: new Vector3(latestDayXValue, 5.5, 8),
      target: new Vector3(latestDayXValue, 0, 0),
      fov: 50,
      duration: 0.6,
      ease: 'power2.inOut',
      name: 'todayFocus',
    }
  }

  /** Bird's-eye overview centered on the full timeline. */
  overview(dayCount: number): CameraShot {
    const w = timelineWidth(dayCount)
    const cx = w / 2
    return {
      position: new Vector3(cx, 12, 16),
      target: new Vector3(cx, 0, 0),
      fov: 55,
      duration: 0.8,
      ease: 'power2.inOut',
      name: 'overview',
    }
  }

  /** Close on a specific card, slight offset angle. */
  topicFocus(card: PinCard): CameraShot {
    const p = card.position
    return {
      position: new Vector3(p.x, p.y + 1, 5),
      target: new Vector3(p.x, p.y, 0),
      fov: 45,
      duration: 0.5,
      ease: 'power2.out',
      name: 'topicFocus',
    }
  }

  /** Pull back to see a full lifecycle line. */
  lifecycleFull(dayCount: number): CameraShot {
    const w = timelineWidth(dayCount)
    const cx = w / 2
    return {
      position: new Vector3(cx, 8, 14),
      target: new Vector3(cx, 0, 0),
      fov: 60,
      duration: 0.8,
      ease: 'power2.inOut',
      name: 'lifecycleFull',
    }
  }

  /** Animate to `shot` via GSAP. Kills any active transition first. */
  transitionTo(shot: CameraShot): gsap.core.Timeline {
    if (this.activeTimeline) this.activeTimeline.kill()

    const tl = gsap.timeline()
    this.currentShot = shot

    const fovTarget = clampFov(shot.fov)
    const posY = Math.max(Y_MIN, shot.position.y)

    // Proxy object so onUpdate can drive camera.lookAt toward interpolated target.
    const lookProxy = { x: this.camera.position.x, y: this.camera.position.y, z: 0 }

    tl.to(this.camera.position, {
      x: shot.position.x,
      y: posY,
      z: shot.position.z,
      duration: shot.duration,
      ease: shot.ease,
      onUpdate: () => {
        // Interpolate lookAt target in parallel by tweening the proxy.
      },
    }, 0)
    tl.to(lookProxy, {
      x: shot.target.x,
      y: shot.target.y,
      z: shot.target.z,
      duration: shot.duration,
      ease: shot.ease,
      onUpdate: () => {
        this.camera.lookAt(lookProxy.x, lookProxy.y, lookProxy.z)
      },
    }, 0)
    tl.to(this.camera, {
      fov: fovTarget,
      duration: shot.duration,
      ease: shot.ease,
      onUpdate: () => this.camera.updateProjectionMatrix(),
    }, 0)

    this.activeTimeline = tl
    return tl
  }

  /** Instant jump (no animation), for initialization. */
  snapTo(shot: CameraShot): void {
    if (this.activeTimeline) {
      this.activeTimeline.kill()
      this.activeTimeline = null
    }
    this.currentShot = shot
    this.camera.position.set(
      shot.position.x,
      Math.max(Y_MIN, shot.position.y),
      shot.position.z,
    )
    this.camera.lookAt(shot.target.x, shot.target.y, shot.target.z)
    this.camera.fov = clampFov(shot.fov)
    this.camera.updateProjectionMatrix()
  }
}

/** Shared layout constant for external callers. */
export { STYLE }
