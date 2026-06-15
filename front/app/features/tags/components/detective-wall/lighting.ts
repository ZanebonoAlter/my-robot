/**
 * Scene lighting setup for the detective wall (dim "darkroom" atmosphere).
 *
 * @see specs/detective-wall-scene/spec.md §Style Constants — 光照
 */
import { AmbientLight, SpotLight, PointLight, Scene, Color } from 'three'
import { STYLE } from './types'

const RAD = Math.PI / 180

/**
 * Adds the four-scene-spec lights to `scene`. The follow-light and selection
 * light are returned so callers can update them per-frame.
 */
export function setupLighting(scene: Scene): {
  followLight: PointLight
  selectionLight: PointLight
} {
  // 1. Ambient — dim base fill.
  const ambient = new AmbientLight(0xffffff, STYLE.lighting.ambient)
  scene.add(ambient)

  // 2. SpotLight — overhead flashlight, angled.
  const spot = new SpotLight(
    0xffffff,
    1.2,
    0, // no distance limit
    STYLE.lighting.spotAngleDeg * RAD,
    STYLE.lighting.spotPenumbra,
    1,
  )
  spot.position.set(0, 12, 6)
  scene.add(spot)
  scene.add(spot.target)

  // 3. PointLight — follows camera (explorer lamp).
  const followLight = new PointLight(0xfff4e6, 0.6, 20, 2)
  followLight.position.set(0, 0, 8)
  scene.add(followLight)

  // 4. PointLight — red, sits above the selected card; off by default.
  const selectionLight = new PointLight(new Color(STYLE.pin.color), 0, 8, 2)
  selectionLight.position.set(0, 2, 1)
  scene.add(selectionLight)

  return { followLight, selectionLight }
}
