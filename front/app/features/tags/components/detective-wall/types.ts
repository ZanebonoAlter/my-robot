/**
 * Shared type definitions for the 3D detective topic wall.
 *
 * @see openspec/changes/detective-topic-wall/design.md
 * @see specs under openspec/changes/detective-topic-wall/specs/
 */
import type { Vector3, Group } from 'three'
import type { SectionTimelineNode, SectionRelation } from '~/api/dailyReports'

// Re-export data-layer types so detective-wall modules import from one place.
export type { SectionTimelineNode, SectionRelation }

// ----------------------------------------------------------------------------
// Layout
// ----------------------------------------------------------------------------

/** World-space position + rotation computed by the layout algorithm. */
export interface LayoutResult {
  position: Vector3
  /** Per-card fixed random rotation around Z (radians), ±3° per scene spec. */
  rotationZ: number
}

/** ISO date window (yyyy-mm-dd). Used to constrain BFS lifelines. */
export interface DateRange {
  start: string
  end: string
}

// ----------------------------------------------------------------------------
// Director camera
// ----------------------------------------------------------------------------

/** A single camera shot; transitions between shots are GSAP-driven. */
export interface CameraShot {
  position: Vector3
  target: Vector3
  fov: number
  /** Transition duration in seconds. */
  duration: number
  /** GSAP easing string, e.g. 'power2.inOut'. */
  ease: string
  /** Optional label for debugging. */
  name?: string
}

// ----------------------------------------------------------------------------
// Interaction
// ----------------------------------------------------------------------------

export type InteractionMode = 'idle' | 'focusing' | 'lifecycle'

export interface InteractionState {
  mode: InteractionMode
  focusedNodeId: number | null
  /** Node ids that belong to the current BFS lifeline. */
  lifelineNodeIds: Set<number>
  /** Normalized "minId-maxId" edge keys belonging to the lifeline. */
  lifelineEdgeKeys: Set<string>
  hoveredId: number | null
}

// ----------------------------------------------------------------------------
// Card / string contracts (implementation classes live in their own modules)
// ----------------------------------------------------------------------------

/**
 * A pinned topic card. Implementations are created by CardGroup; this interface
 * is what InteractionLayer and DirectorCamera depend on.
 */
export interface PinCard {
  readonly data: SectionTimelineNode
  readonly group: Group
  readonly position: Vector3
  /** Hover: lift along Z. */
  elevate(): void
  /** Cancel hover. */
  settle(): void
  /** Lifeline highlight: boost emissive. */
  highlight(): void
  /** Recede into background. */
  dim(): void
  /** Restore normal state. */
  reset(): void
}

/** A red string connecting two cards. */
export interface RedString {
  readonly fromId: number
  readonly toId: number
  readonly distance: number
  /** 'identity' (same persistent topic, solid) or 'similarity' (Hungarian, dashed). */
  readonly relationType: string
  /** Animate draw progress 0→1. */
  draw(progress: number): void
  highlight(): void
  dim(): void
  reset(): void
}

// ----------------------------------------------------------------------------
// Callbacks (Vue ↔ Three.js bridge)
// ----------------------------------------------------------------------------

export interface InteractionCallbacks {
  onCardHover(card: PinCard | null): void
  onCardClick(card: PinCard): void
  onStringClick(string: RedString): void
  onBackgroundClick(): void
  onLifelineReady(
    nodes: SectionTimelineNode[],
    edges: SectionRelation[],
    startNode: SectionTimelineNode,
  ): void
}

// ----------------------------------------------------------------------------
// Style constants (single source of truth — mirrors scene spec §Style Constants)
// ----------------------------------------------------------------------------

export const STYLE = {
  card: {
    paper: '#FFFBEB',
    paperWarm: '#F7E7C4',
    paperAged: '#E8D4A5',
    border: 'rgba(26, 26, 26, 0.12)',
    ink: '#1A1A1A',
    mutedInk: '#4B5563',
    shadow: '#130B06',
    tape: '#E7B96B',
    width: 2.28,
    height: 1.52,
    depth: 0.08,
    roughness: 0.85,
    metalness: 0.0,
  },
  pin: {
    color: '#DC2626',
    radius: 0.08,
    metalness: 0.7,
    roughness: 0.3,
  },
  string: {
    color: '#DC2626',
    darkColor: '#7F1D1D',
    baseOpacity: 0.46,
    highlightOpacity: 1.0,
    baseLinewidth: 2.6,
    highlightLinewidth: 5.2,
  },
  background: '#241f1b',
  cork: '#7A6754',
  fog: '#0a0f14',
  /** Day spacing → fog density mapping (scene spec §FogSystem). */
  fogDensityByDays: { 7: 0.08, 14: 0.05, 30: 0.03, 60: 0.02 } as Record<number, number>,
  /** Status → color (mirrors SectionLifecyclePanel statusColorMap). */
  statusColors: {
    emerging: '#16a34a',
    continuing: '#2563eb',
    split: '#ea580c',
    merge: '#9333ea',
    ending: '#9ca3af',
  } as Record<string, string>,
  layout: {
    colWidth: 3.0,
    rowHeight: 2.2,
    zJitter: 0.15,
    rotationZDeg: 3,
  },
  lighting: {
    ambient: 0.08,
    spotAngleDeg: 45,
    spotPenumbra: 0.42,
    hemiSky: '#3a2a1a',
    hemiGround: '#0a0f14',
    hemiIntensity: 0.28,
    followColor: '#fff0d0',
  },
  desk: {
    y: -1.6,
    zBack: -6,
    zFront: 24,
    color: '#A06F45',
  },
  wall: {
    backZ: -0.6,
    farZ: -4,
    farColor: '#1B2528',
  },
  lamp: {
    offset: { x: 2.8, z: 5.2 },
    brass: '#b08d3f',
    glass: '#1f3d2e',
    bulb: '#ffe9b0',
    bulbEmissive: 1.5,
    spotColor: '#ffd9a0',
    spotIntensity: 4.2,
  },
  dossier: {
    stackColor: '#e8d4a5',
  },
  directionalFog: {
    density: 1.2,
    range: 12,
    color: '#0a0f14',
  },
  dust: {
    count: 0,
    color: '#ffe9c8',
    size: 0.038,
  },
} as const

/** Supported day windows for the range control. */
export const SUPPORTED_DAYS = [7, 14, 30, 60] as const
export type SupportedDays = (typeof SUPPORTED_DAYS)[number]
