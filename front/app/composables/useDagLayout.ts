import { graphConnect, sugiyama } from 'd3-dag'

/** A positioned node returned by the composable */
export interface PositionedNode<T = Record<string, unknown>> {
  id: string
  x: number
  y: number
  /** Original input data */
  data: T
}

/** An edge path returned by the composable */
export interface EdgePath<T = Record<string, unknown>> {
  from: string
  to: string
  /** SVG path string (cubic bezier) */
  path: string
  /** Original input data */
  data: T
}

export interface DagLayoutResult<N = Record<string, unknown>, E = Record<string, unknown>> {
  nodes: PositionedNode<N>[]
  edges: EdgePath<E>[]
  width: number
  height: number
}

export type DagDirection = 'TB' | 'LR'

export interface UseDagLayoutOptions {
  /** Layout direction: top-bottom or left-right (default 'TB') */
  direction?: DagDirection
  /** [width, height] in abstract units (default [1, 1]) */
  nodeSize?: [number, number]
  /** [horizontalGap, verticalGap] in abstract units (default [1, 1]) */
  gap?: [number, number]
}

/**
 * Compute a Sugiyama-style DAG layout from nodes and edges.
 *
 * - nodeSize and gap use unit values ([1,1]); the calling component
 *   applies pixel scaling when rendering.
 * - Orphan nodes (no edges) are placed in a separate row below (TB) or
 *   to the right (LR) of the main layout.
 * - For LR direction, x and y are swapped after layout so that the
 *   "layers" go left-to-right.
 */
export function useDagLayout<
  N extends { id: number | string },
  E extends { from: number | string; to: number | string },
>(
  nodes: N[],
  edges: E[],
  options: UseDagLayoutOptions = {},
): DagLayoutResult<N, E> | null {
  if (nodes.length === 0) return null

  const direction: DagDirection = options.direction ?? 'TB'
  const nodeSize: [number, number] = options.nodeSize ?? [1, 1]
  const gap: [number, number] = options.gap ?? [1, 1]

  // Build a lookup from id -> original node data
  const nodeMap = new Map<string, N>()
  for (const n of nodes) {
    nodeMap.set(String(n.id), n)
  }

  // Separate nodes that participate in edges from orphans
  const connectedIds = new Set<string>()
  for (const e of edges) {
    connectedIds.add(String(e.from))
    connectedIds.add(String(e.to))
  }

  const orphanNodes = nodes.filter((n) => !connectedIds.has(String(n.id)))

  // Filter edges: only keep those whose both endpoints exist in the node set
  const validEdges = edges.filter(
    (e) => nodeMap.has(String(e.from)) && nodeMap.has(String(e.to)),
  )
  const edgePairs: [string, string][] = validEdges.map((e) => [String(e.from), String(e.to)])

  let layoutWidth = 0
  let layoutHeight = 0

  // Map from d3-dag node data (id string) -> { x, y }
  const positioned = new Map<string, { x: number; y: number }>()

  if (edgePairs.length > 0) {
    const builder = graphConnect()
    const graph = builder(edgePairs)

    const layout = sugiyama()
      .nodeSize(nodeSize)
      .gap(gap)

    const result = layout(graph)
    layoutWidth = result.width
    layoutHeight = result.height

    for (const node of graph.nodes()) {
      positioned.set(node.data, { x: node.x, y: node.y })
    }
  }

  // Place orphan nodes in a row below / to the right
  if (orphanNodes.length > 0) {
    const orphanStartX = 0
    // For TB: place below the graph; for LR: also below (before swap)
    const orphanStartY = layoutHeight > 0 ? layoutHeight + gap[1] + nodeSize[1] : 0

    for (let i = 0; i < orphanNodes.length; i++) {
      const id = String(orphanNodes[i].id)
      positioned.set(id, {
        x: orphanStartX + i * (nodeSize[0] + gap[0]),
        y: orphanStartY,
      })
    }

    // Expand bounding box to include orphans
    const maxOrphanX = orphanStartX + (orphanNodes.length - 1) * (nodeSize[0] + gap[0]) + nodeSize[0]
    const orphanRowY = orphanStartY + nodeSize[1]
    layoutWidth = Math.max(layoutWidth, maxOrphanX)
    layoutHeight = Math.max(layoutHeight, orphanRowY)
  }

  // Swap x/y for LR direction
  if (direction === 'LR') {
    for (const pos of positioned.values()) {
      const tmp = pos.x
      pos.x = pos.y
      pos.y = tmp
    }
    const tmp = layoutWidth
    layoutWidth = layoutHeight
    layoutHeight = tmp
  }

  // Build result nodes
  const resultNodes: PositionedNode<N>[] = nodes.map((n) => {
    const id = String(n.id)
    const pos = positioned.get(id) ?? { x: 0, y: 0 }
    return { id, x: pos.x, y: pos.y, data: n }
  })

  // Build edge paths
  const resultEdges: EdgePath<E>[] = edges.map((e) => {
    const fromId = String(e.from)
    const toId = String(e.to)
    const from = positioned.get(fromId) ?? { x: 0, y: 0 }
    const to = positioned.get(toId) ?? { x: 0, y: 0 }
    return {
      from: fromId,
      to: toId,
      path: buildEdgePath(from.x, from.y, to.x, to.y, nodeSize, direction),
      data: e,
    }
  })

  return {
    nodes: resultNodes,
    edges: resultEdges,
    width: layoutWidth,
    height: layoutHeight,
  }
}

/**
 * Build a cubic bezier SVG path from source center to target center.
 *
 * For TB: vertical-first (control points offset along Y).
 * For LR: horizontal-first (control points offset along X).
 */
function buildEdgePath(
  sx: number,
  sy: number,
  tx: number,
  ty: number,
  _nodeSize: [number, number],
  direction: DagDirection,
): string {
  if (direction === 'LR') {
    const midX = (sx + tx) / 2
    return `M${sx},${sy} C${midX},${sy} ${midX},${ty} ${tx},${ty}`
  }
  // TB (default)
  const midY = (sy + ty) / 2
  return `M${sx},${sy} C${sx},${midY} ${tx},${midY} ${tx},${ty}`
}
