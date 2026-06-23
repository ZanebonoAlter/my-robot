/**
 * Fog system — exponential fog that hides out-of-window cards.
 *
 * Density maps to the day window (7d → dense/focused, 60d → thin/wide).
 * Animations are GSAP-driven per design.md §Animation Library Split.
 *
 * @see specs/detective-wall-scene/spec.md §FogSystem
 */
import { Scene, FogExp2, Color } from 'three'
import gsap from 'gsap'
import { densityForDays } from './utils'

export class FogSystem {
  readonly fog: FogExp2
  private currentDensity: number
  private enabled = true

  constructor(scene: Scene, color: string) {
    this.currentDensity = densityForDays(7)
    this.fog = new FogExp2(new Color(color), this.currentDensity)
    scene.fog = this.fog
  }

  /** Snap density to the value for `days` (no animation). */
  setDensityForDays(days: number): void {
    this.currentDensity = densityForDays(days)
    if (this.enabled) this.fog.density = this.currentDensity
  }

  /** Animate density over `durationSec`. Returns the tween (GSAP). */
  animateToDensity(density: number, durationSec: number): gsap.core.Tween {
    return gsap.to(this, {
      currentDensity: density,
      duration: durationSec,
      ease: 'power2.inOut',
      onUpdate: () => {
        if (this.enabled) this.fog.density = this.currentDensity
      },
    })
  }

  /** Disable fog entirely (lifecycle mode). */
  disable(): void {
    this.enabled = false
    this.fog.density = 0
  }

  /** Re-enable fog and restore density for `days`. */
  enable(days: number): void {
    this.enabled = true
    this.setDensityForDays(days)
  }
}
