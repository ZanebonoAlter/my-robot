/**
 * Post-processing pipeline: Render → Bloom → Vignette → Noise (grain).
 *
 * Uses pmndrs/postprocessing exclusively. Effects are wrapped in EffectPass
 * (the correct API — not addPass(effect)). Film grain uses NoiseEffect (pure
 * pmndrs, interface-compatible) rather than three/examples FilmPass, which has
 * a different Pass contract and crashes EffectComposer.addPass.
 *
 * @see specs/detective-wall-scene/spec.md §Post Processing
 */
import { WebGLRenderer, Scene, PerspectiveCamera } from 'three'
import { EffectComposer, RenderPass, EffectPass, BloomEffect, VignetteEffect, NoiseEffect } from 'postprocessing'

export class WallPostProcessing {
  readonly composer: EffectComposer

  constructor(renderer: WebGLRenderer, scene: Scene, camera: PerspectiveCamera) {
    this.composer = new EffectComposer(renderer)
    this.composer.addPass(new RenderPass(scene, camera))

    const bloom = new BloomEffect({
      intensity: 0.6,
      luminanceThreshold: 0.8, // only red strings glow
      luminanceSmoothing: 0.3,
    })
    this.composer.addPass(new EffectPass(camera, bloom))

    const vignette = new VignetteEffect({ darkness: 0.5 })
    this.composer.addPass(new EffectPass(camera, vignette))

    // Film grain — subtle. NoiseEffect (pure pmndrs, premultiply).
    const noise = new NoiseEffect({ premultiply: true })
    this.composer.addPass(new EffectPass(camera, noise))
  }

  render(deltaTime: number): void {
    this.composer.render(deltaTime)
  }

  setSize(width: number, height: number): void {
    this.composer.setSize(width, height)
  }

  dispose(): void {
    this.composer.dispose()
  }
}
