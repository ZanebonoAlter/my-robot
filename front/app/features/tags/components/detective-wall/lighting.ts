/**
 * Scene lighting setup for the detective desk ("banker's lamp" atmosphere).
 *
 * Replaces the original flat white ambient with a warm-sky/cold-ground
 * HemisphereLight, and retargets the main spotlight as a warm cone emanating
 * from the desk lamp (its world position/target are wired per-data by
 * TopicWallScene — the defaults here are just sane initial values).
 *
 * @see openspec/changes/detective-wall-ambiance/specs/detective-wall-scene/spec.md §Lighting
 */
import { HemisphereLight, SpotLight, PointLight, Scene, Color } from 'three'
import { STYLE } from './types'

/** Spotlight half-angle in radians (~28°): a focused desk-lamp cone. */
const SPOT_HALF_ANGLE = 0.5

/**
 * Adds the scene lights. `spot` is returned so TopicWallScene can reposition it
 * to the desk lamp and aim it at today's column on each data load.
 */
export function setupLighting(scene: Scene): {
  spot: SpotLight
  followLight: PointLight
  selectionLight: PointLight
} {
  // 1. Hemisphere — warm sky over cold ground; air + form, not flat fill.
  const hemi = new HemisphereLight(
    new Color(STYLE.lighting.hemiSky),
    new Color(STYLE.lighting.hemiGround),
    STYLE.lighting.hemiIntensity,
  )
  hemi.position.set(0, 12, 0)
  scene.add(hemi)

  // 2. Main SpotLight — warm desk-lamp cone. Repositioned per-data by the scene.
  const spot = new SpotLight(
    new Color(STYLE.lamp.spotColor),
    STYLE.lamp.spotIntensity,
    0, // no distance limit
    SPOT_HALF_ANGLE,
    STYLE.lighting.spotPenumbra,
    1.5, // gentle physical decay
  )
  spot.position.set(0, 4, 5)
  spot.target.position.set(0, 0, 0)
  scene.add(spot)
  scene.add(spot.target)

  // 3. PointLight — follows the camera (explorer lamp), warm.
  const followLight = new PointLight(new Color(STYLE.lighting.followColor), 0.6, 20, 2)
  followLight.position.set(0, 0, 8)
  scene.add(followLight)

  // 4. PointLight — red, above the selected card; off by default.
  const selectionLight = new PointLight(new Color(STYLE.pin.color), 0, 8, 2)
  selectionLight.position.set(0, 2, 1)
  scene.add(selectionLight)

  return { spot, followLight, selectionLight }
}
