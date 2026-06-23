/**
 * Directional fog — injects an X-axis fog term into MeshStandardMaterial so the
 * scene reads "clear at today (right), fogged toward the past (left)".
 *
 * Patches `onBeforeCompile`: vertex emits world `x`; fragment, after the stock
 * `fog_fragment` include, blends an extra fog color whose weight grows with the
 * distance into the past. Today/future (x >= uFogOriginX) is unaffected.
 *
 * The uniforms object is shared across every injected material (one update to
 * `uFogOriginX` reflows the whole environment layer). Not applied to cards —
 * their highlight/dim states must stay clean (see design.md §Directional Fog).
 *
 * @see openspec/changes/detective-wall-ambiance/design.md §Directional Fog
 */
import type { MeshStandardMaterial, Color } from 'three'

/** Shared uniforms injected into every directional-fog material. */
export interface DirectionalFogUniforms {
  uFogOriginX: { value: number }
  uDirFogDensity: { value: number }
  uDirFogRange: { value: number }
  uDirFogColor: { value: Color }
}

/**
 * Pure factor mirroring the GLSL formula — extracted for unit testing without a
 * WebGL context. Returns the extra-fog blend weight in [0, 1).
 *
 *   dx = originX - worldX        (left/past > 0, today/future <= 0)
 *   past = clamp(dx / range, 0, 1)
 *   factor = 1 - exp(-density * past)
 *
 * worldX at/after origin → 0 (no extra fog on today/future).
 */
export function directionalFogFactor(
  worldX: number,
  originX: number,
  density: number,
  range: number,
): number {
  const dx = originX - worldX
  if (dx <= 0) return 0
  const past = Math.min(dx / range, 1)
  return 1 - Math.exp(-density * past)
}

/**
 * Patch `material.onBeforeCompile` to apply the directional fog term. Binds the
 * shared `fogUniforms` so a single `uFogOriginX.value =` update reflows all
 * injected materials.
 */
export function injectDirectionalFog(
  material: MeshStandardMaterial,
  fogUniforms: DirectionalFogUniforms,
): void {
  material.onBeforeCompile = (shader) => {
    // Bind shared uniform objects (same reference → updates propagate).
    shader.uniforms.uFogOriginX = fogUniforms.uFogOriginX
    shader.uniforms.uDirFogDensity = fogUniforms.uDirFogDensity
    shader.uniforms.uDirFogRange = fogUniforms.uDirFogRange
    shader.uniforms.uDirFogColor = fogUniforms.uDirFogColor

    // Vertex: emit world-space x.
    shader.vertexShader = shader.vertexShader
      .replace('#include <common>\n', '#include <common>\nvarying float vWorldX;\n')
      .replace(
        '#include <project_vertex>',
        '#include <project_vertex>\n  vWorldX = (modelMatrix * vec4(transformed, 1.0)).x;',
      )

    // Fragment: declare varyings/uniforms, then blend after stock fog.
    shader.fragmentShader = shader.fragmentShader
      .replace(
        '#include <common>\n',
        '#include <common>\n' +
        'varying float vWorldX;\n' +
        'uniform float uFogOriginX;\n' +
        'uniform float uDirFogDensity;\n' +
        'uniform float uDirFogRange;\n' +
        'uniform vec3 uDirFogColor;\n',
      )
      .replace(
        '#include <fog_fragment>',
        '#include <fog_fragment>\n' +
        '  {\n' +
        '    float dx = uFogOriginX - vWorldX;\n' +
        '    float past = clamp(-dx / uDirFogRange, 0.0, 1.0);\n' +
        '    float dirFactor = 1.0 - exp(-uDirFogDensity * past);\n' +
        '    gl_FragColor.rgb = mix(gl_FragColor.rgb, uDirFogColor, dirFactor);\n' +
        '  }',
      )
  }
}
