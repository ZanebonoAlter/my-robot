# Tasks: topic-graph-fixes

## 1. Fix Time Filter

- [ ] 1.1 In `backend-go/internal/domain/daily_report/repository.go`, remove the `if days > 30 { days = 30 }` hard cap around lines 148-149 so that `ListReports` accepts any `days` value
- [ ] 1.2 Verify `BoardDailyReportTimeline.vue` time filter works correctly without frontend changes (the 7-day incremental loading should now go beyond 30 days)

## 2. BFS Highlight Utility

- [ ] 2.1 Create `front/app/features/topic-graph/utils/graphBfsHighlight.ts` with:
  - Exported constants `COMPONENT_THRESHOLD = 0.4` and `MAX_HOPS = 4`
  - Exported function `bfsHighlight(focusId, edges, totalNodes): Set<string>` that:
    - Builds an undirected adjacency list from `edges` (`source`/`target` fields)
    - BFS from `focusId` to collect full connected component
    - If `component.size < COMPONENT_THRESHOLD * totalNodes`, return full component
    - Otherwise, re-run BFS with `MAX_HOPS` hop limit and return that subset
    - Return value is `Set<string>` of highlighted node IDs

## 3. Apply BFS to TopicGraphPage

- [ ] 3.1 In `front/app/features/topic-graph/components/TopicGraphPage.vue`, update the `highlightedNodeIds` computed (lines 174-198) to call `bfsHighlight(selectedOrHoveredNodeId, edges, totalNodes)` instead of the current 1-hop neighbor logic
- [ ] 3.2 Verify that `isEdgeHighlighted` and `isNodeHighlighted` helpers still work correctly with the new `Set<string>` return type

## 4. Apply BFS to SectionLifecyclePanel

- [ ] 4.1 In `front/app/features/tags/components/SectionLifecyclePanel.vue`, import `bfsHighlight` from `graphBfsHighlight.ts` and replace the `isNodeHighlighted` / `isEdgeHighlighted` 1-hop logic (around lines 171-200) with BFS-based highlight
- [ ] 4.2 Cache the BFS result in a computed keyed on the hovered node ID so it recomputes only on hover change

## 5. Node Size / Font Size Controls

- [ ] 5.1 In `front/app/features/topic-graph/components/TopicGraphCanvas.client.vue`, add a collapsible settings panel (gear icon, top-right corner) with three range sliders:
  - **Global scale**: 0.5x-3.0x, default 1.0x — multiplies all node sizes and link widths
  - **Node size multiplier**: 0.5x-5.0x, default 1.0x — passed to `buildNodeSize()`
  - **Font size**: 8-32px, default 14px — applied to `SpriteText.fontSize`
- [ ] 5.2 Update `buildTopicGraphViewModel.ts` (or the canvas config) to accept and apply the node size multiplier and font size refs
- [ ] 5.3 Persist the three slider values in `localStorage` (key: `topic-graph-settings`), loading on mount and saving on change

## 6. Enable Node Dragging

- [ ] 6.1 In `front/app/features/topic-graph/components/TopicGraphCanvas.client.vue`, verify `enableNodeDrag` is not set to `false`; if it is, remove the override or set it to `true`
- [ ] 6.2 Test that nodes can be dragged to rearrange layout in the 3D force graph

## 7. Verify

- [ ] 7.1 Run `cd front && pnpm lint` — no new errors
- [ ] 7.2 Run `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` — passes
- [ ] 7.3 Manual test: topic graph hover highlight shows connected component (not just 1-hop), settings panel adjusts node/font size, node dragging works
- [ ] 7.4 Manual test: section lifecycle panel hover highlight shows connected component
- [ ] 7.5 Manual test: board daily report timeline loads reports beyond 30 days when clicking "Load More"
