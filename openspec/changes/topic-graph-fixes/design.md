# Design: topic-graph-fixes

## Context

The topic graph and related components have several usability bugs that impair daily use:

1. **Time filter cap**: `ListReports` hard-caps `days` at 30 (`repository.go:143`). The frontend's "Load More" button keeps incrementing days by 7 but silently hits this ceiling, returning stale results.
2. **1-hop highlight**: Both `TopicGraphPage.vue:174-198` and `SectionLifecyclePanel.vue:171-200` only highlight direct neighbors. Users expect the entire connected sub-graph of the selected/hovered node to light up.
3. **No size/zoom controls**: The 3D force graph has no UI for adjusting node size, font size, or global scale. OrbitControls exist but are not exposed as configurable, and node dragging is disabled.

These are independent bugs but share a common theme: the visualization layer lacks the polish needed for graphs with 50+ nodes.

## Goals

- Fix the 30-day time filter cap so frontend pagination works correctly.
- Replace 1-hop highlight with BFS connected-component highlight, with a threshold to avoid lighting up the entire graph.
- Add UI controls for node size, font size, and global zoom in the topic graph.
- Apply the same BFS highlight to `SectionLifecyclePanel`.
- Enable node dragging in the 3D force graph.

## Non-Goals

- Configurable threshold/hop-limit via UI (hard-coded constants for now, tunable later).
- Persisting user display preferences across sessions.
- Rewriting the graph rendering engine or switching libraries.
- Changing the backend BFS in the lifecycle API (already returns correct connected sub-graph).

## Decisions

### D1: Remove the 30-day hard cap entirely

The cap was a safety guard but causes silent data loss. The frontend already increments by 7, so a user requesting 90 days just wants all available data. PostgreSQL handles the date range filter efficiently with the existing index on `(semantic_board_id, period_date)`.

**Alternative considered**: Increase cap to 90 and add frontend feedback. Rejected because the cap serves no purpose with indexed queries.

### D2: BFS connected-component highlight with threshold

Algorithm:

```
function bfsHighlight(focusNode, edges):
  // Build adjacency list from edges (undirected)
  adj = buildAdjacencyList(edges)

  // BFS from focusNode
  component = BFS(focusNode, adj)

  totalNodes = count displayed nodes
  if len(component) < 0.4 * totalNodes:
    return component  // highlight entire component
  else:
    return BFS(focusNode, adj, maxHops=4)  // fall back to 4-hop
```

Constants:
- `COMPONENT_THRESHOLD = 0.4` (40% of total nodes)
- `MAX_HOPS = 4`

These are exported as named constants from a shared util so they can be tuned without touching component logic.

**Why this approach**: In dense graphs (most nodes connected), the connected component IS the whole graph, so highlighting it is meaningless. The 40% threshold detects this case and falls back to a fixed-hop limit. In sparse graphs (most topic graphs), the connected component is small and meaningful.

**Alternative considered**: Pure hop-based highlight (always 3-4 hops). Rejected because it arbitrarily cuts off sparse connected components while still being too broad in dense clusters.

### D3: Shared BFS highlight utility

Both `TopicGraphPage.vue` and `SectionLifecyclePanel.vue` need the same BFS logic. Extract to `front/app/features/topic-graph/utils/graphBfsHighlight.ts`:

```typescript
export const COMPONENT_THRESHOLD = 0.4
export const MAX_HOPS = 4

export function bfsHighlight(
  focusId: string | number,
  edges: Array<{ source: string | number; target: string | number }>,
  totalNodes: number
): Set<string | number>
```

`SectionLifecyclePanel` imports from the same utility. The adjacency list is built per-call (edges are typically < 200, cost is negligible).

### D4: UI controls placement

Add a collapsible settings panel (gear icon) in the top-right corner of `TopicGraphCanvas.client.vue`:

- **Global scale**: range slider, 0.5x-3.0x, default 1.0x — multiplies all node sizes and link widths
- **Node size**: range slider, 0.5x-5.0x, default 1.0x — multiplier passed to `buildNodeSize()`
- **Font size**: range slider, 8-32px, default 14px — applied to `SpriteText.fontSize`

The panel is collapsed by default. Values are reactive refs passed to the graph config. No persistence.

### D5: Enable node dragging

`3d-force-graph` supports `enableNodeDrag(true)` (default is actually true, but it may be overridden). Verify the current config and ensure `enableNodeDrag` is not explicitly set to `false`. No UI toggle needed — dragging is a standard graph interaction.

### D6: SectionLifecyclePanel highlight reuse

The lifecycle panel's `isNodeHighlighted` / `isEdgeHighlighted` functions currently do 1-hop lookups. Replace with the shared `bfsHighlight` utility:

- On hover, compute the full highlight set via `bfsHighlight(hoveredId, relations, totalNodes)`
- Cache the result in a `computed` keyed on `hoveredId`
- `isNodeHighlighted` and `isEdgeHighlighted` check membership in the cached set

The panel already renders the full connected sub-graph from the lifecycle API, so the BFS operates on a small, pre-filtered graph (typically < 30 nodes). The 40% threshold may rarely trigger here, but consistency with the topic graph behavior is the goal.

## Risks & Trade-offs

| Risk | Mitigation |
|------|-----------|
| BFS on very large graphs (> 500 nodes) could lag on hover | Unlikely in practice; topic graphs are typically 50-200 nodes. Monitor and add debounce if needed. |
| Removing the 30-day cap could return large result sets | Indexed queries on `period_date` are fast. Realistic worst case: 365 days * ~5 boards = ~1800 rows, well within PostgreSQL's comfort zone. |
| Threshold/hop constants may need tuning per dataset | Exported as named constants, easy to change. Can add UI controls later. |
| Settings panel adds UI complexity | Collapsed by default, minimal footprint. |

## Affected Files

- `backend-go/internal/domain/daily_report/repository.go` — remove `if days > 30` cap (2 locations)
- `front/app/features/topic-graph/utils/graphBfsHighlight.ts` — **new**, shared BFS utility
- `front/app/features/topic-graph/components/TopicGraphPage.vue` — replace `highlightedNodeIds` with BFS
- `front/app/features/topic-graph/components/TopicGraphCanvas.client.vue` — add settings panel, enable drag
- `front/app/features/topic-graph/utils/buildTopicGraphViewModel.ts` — accept size multipliers
- `front/app/features/tags/components/SectionLifecyclePanel.vue` — replace highlight with BFS
- `front/app/features/tags/components/BoardDailyReportTimeline.vue` — add cap-reached feedback (if we keep a cap; per D1 this becomes moot)
