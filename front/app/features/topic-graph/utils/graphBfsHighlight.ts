/**
 * Shared bounded BFS highlight utility for topic graph and section lifecycle.
 *
 * Algorithm:
 * 1. BFS from focusId to collect the full connected component.
 * 2. If totalNodes <= SMALL_GRAPH_NODE_LIMIT, return full component.
 * 3. If component.size < totalNodes * COMPONENT_THRESHOLD, return full component.
 * 4. Otherwise, re-run BFS with maxHops=MAX_HOPS and maxNodes=floor(totalNodes * COMPONENT_THRESHOLD).
 */

export const COMPONENT_THRESHOLD = 0.4
export const MAX_HOPS = 4
export const SMALL_GRAPH_NODE_LIMIT = 8

export interface GraphHighlightEdge<T extends string | number> {
  source: T
  target: T
}

/** Return the full undirected component without density or hop limits. */
export function fullComponentHighlight<T extends string | number>(
  focusId: T,
  edges: GraphHighlightEdge<T>[],
): Set<T> {
  const adj = new Map<T, Set<T>>()
  const ensureNode = (id: T) => {
    if (!adj.has(id)) adj.set(id, new Set())
  }

  for (const edge of edges) {
    ensureNode(edge.source)
    ensureNode(edge.target)
    adj.get(edge.source)!.add(edge.target)
    adj.get(edge.target)!.add(edge.source)
  }
  ensureNode(focusId)

  return bfsCollect(focusId, adj, Infinity, Infinity)
}

/**
 * Returns the set of node IDs to highlight given a focus node.
 *
 * - Handles both string and number IDs via generics.
 * - Treats edges as undirected.
 * - Always includes the focus node even if it has no edges.
 */
export function bfsHighlight<T extends string | number>(
  focusId: T,
  edges: GraphHighlightEdge<T>[],
  totalNodes: number,
): Set<T> {
  // Build adjacency list (undirected)
  const adj = new Map<T, Set<T>>()
  const ensureNode = (id: T) => {
    if (!adj.has(id)) adj.set(id, new Set())
  }

  for (const edge of edges) {
    ensureNode(edge.source)
    ensureNode(edge.target)
    adj.get(edge.source)!.add(edge.target)
    adj.get(edge.target)!.add(edge.source)
  }

  // Ensure focus node exists
  ensureNode(focusId)

  // BFS to collect full connected component
  const fullComponent = bfsCollect(focusId, adj, Infinity, Infinity)

  // Small graph: return full component
  if (totalNodes <= SMALL_GRAPH_NODE_LIMIT) {
    return fullComponent
  }

  // Sparse component: return full component
  if (fullComponent.size < totalNodes * COMPONENT_THRESHOLD) {
    return fullComponent
  }

  // Dense component: bounded BFS with hop and node limits
  const maxNodes = Math.max(1, Math.floor(totalNodes * COMPONENT_THRESHOLD))
  return bfsCollect(focusId, adj, MAX_HOPS, maxNodes)
}

/**
 * BFS from startId, collecting nodes up to maxHops depth and maxNodes count.
 * Edges are treated as undirected.
 */
function bfsCollect<T extends string | number>(
  startId: T,
  adj: Map<T, Set<T>>,
  maxHops: number,
  maxNodes: number,
): Set<T> {
  const visited = new Set<T>()
  const queue: Array<{ id: T; depth: number }> = [{ id: startId, depth: 0 }]
  visited.add(startId)

  while (queue.length > 0) {
    const { id, depth } = queue.shift()!

    if (depth >= maxHops || visited.size >= maxNodes) {
      continue
    }

    const neighbors = adj.get(id)
    if (!neighbors) continue

    for (const neighbor of neighbors) {
      if (visited.has(neighbor)) continue
      if (visited.size >= maxNodes) break

      visited.add(neighbor)
      queue.push({ id: neighbor, depth: depth + 1 })
    }
  }

  return visited
}
