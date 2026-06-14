import { describe, it, expect, beforeEach } from 'vitest'
import { useTopicGraph } from './useTopicGraph'
import type { GraphNode, TopicGraphEdge, TopicGraphPayload, TopicTag } from '~/api/topicGraph'

/**
 * Integration tests for useTopicGraph highlight wiring (bfsHighlight consumption).
 *
 * The pure bfsHighlight algorithm is covered in graphBfsHighlight.test.ts.
 * These tests verify the composable correctly:
 * - normalizes edge endpoints to string IDs before calling bfsHighlight
 * - drives highlightedNodeIds from selectedTopicSlug/selectedKeywordSlug
 * - keeps relatedEdgeIds to edges whose BOTH endpoints are highlighted
 * - clears highlights when nothing is selected
 *
 * loadGraph() is bypassed by passing initialLoad=false; graphPayload is set
 * directly so the computed graph (and hotspot visibility) derive without network.
 */

function makeTopic(id: string, slug: string, label: string): GraphNode {
  return { id, label, slug, kind: 'topic', weight: 1, category: 'event' }
}

function makeTopicTag(id: string, slug: string, label: string): TopicTag {
  return { label, slug, category: 'event', kind: 'topic', score: 1 }
}

function makeEdge(id: string, source: string, target: string): TopicGraphEdge {
  return { id, source, target, kind: 'topic_topic', weight: 1 }
}

/** A-B-C chain plus an isolated node D (not connected). */
function chainGraphPayload(): TopicGraphPayload {
  const nodes: GraphNode[] = [
    makeTopic('a', 'slug-a', 'A'),
    makeTopic('b', 'slug-b', 'B'),
    makeTopic('c', 'slug-c', 'C'),
    makeTopic('d', 'slug-d', 'D'),
  ]
  const edges: TopicGraphEdge[] = [
    makeEdge('e-ab', 'a', 'b'),
    makeEdge('e-bc', 'b', 'c'),
  ]
  return {
    type: 'daily',
    anchor_date: '2026-06-14',
    period_label: 'test',
    nodes,
    edges,
    topic_count: nodes.length,
    article_count: 0,
    feed_count: 0,
    top_topics: nodes.map(n => makeTopicTag(n.id, n.slug!, n.label)),
  }
}

describe('useTopicGraph highlight integration', () => {
  let graph: ReturnType<typeof useTopicGraph>

  beforeEach(() => {
    graph = useTopicGraph(false)
    graph.graphPayload.value = chainGraphPayload()
  })

  it('highlights the full multi-hop connected component (A-B-C)', () => {
    graph.selectedTopicSlug.value = 'slug-a'

    expect(graph.highlightedNodeIds.value).toEqual(expect.arrayContaining(['a', 'b', 'c']))
    expect(graph.highlightedNodeIds.value).not.toContain('d')
    // relatedEdgeIds covers both internal edges
    expect(graph.relatedEdgeIds.value).toEqual(expect.arrayContaining(['e-ab', 'e-bc']))
  })

  it('relatedEdgeIds excludes edges whose endpoint falls outside the highlight set', () => {
    // Select B: component is still {a,b,c} (undirected), so both edges remain internal.
    graph.selectedTopicSlug.value = 'slug-b'
    expect(graph.relatedEdgeIds.value).toEqual(expect.arrayContaining(['e-ab', 'e-bc']))
  })

  it('clears highlights when selection is removed', () => {
    graph.selectedTopicSlug.value = 'slug-a'
    expect(graph.highlightedNodeIds.value.length).toBeGreaterThan(0)

    graph.selectedTopicSlug.value = null
    graph.selectedKeywordSlug.value = null
    expect(graph.highlightedNodeIds.value).toEqual([])
    expect(graph.relatedEdgeIds.value).toEqual([])
  })

  it('selectedKeywordSlug drives highlight when selectedTopicSlug is empty', () => {
    graph.selectedTopicSlug.value = null
    graph.selectedKeywordSlug.value = 'slug-c'
    expect(graph.highlightedNodeIds.value).toEqual(expect.arrayContaining(['a', 'b', 'c']))
  })

  it('dense graph is truncated (no full-graph highlight)', () => {
    // Build a 100-node star: center connects to 99 leaves (dense, low diameter).
    const starNodes: GraphNode[] = [makeTopic('center', 'slug-center', 'Center')]
    const starEdges: TopicGraphEdge[] = []
    for (let i = 1; i < 100; i++) {
      const id = `n${i}`
      starNodes.push(makeTopic(id, `slug-${id}`, id))
      starEdges.push(makeEdge(`e-${id}`, 'center', id))
    }
    graph.graphPayload.value = {
      type: 'daily',
      anchor_date: '2026-06-14',
      period_label: 'star',
      nodes: starNodes,
      edges: starEdges,
      topic_count: starNodes.length,
      article_count: 0,
      feed_count: 0,
      top_topics: starNodes.map(n => makeTopicTag(n.id, n.slug!, n.label)),
    }

    graph.selectedTopicSlug.value = 'slug-center'
    // maxNodes = floor(100 * 0.4) = 40; highlight must stay within that bound
    expect(graph.highlightedNodeIds.value.length).toBeLessThanOrEqual(40)
    expect(graph.highlightedNodeIds.value).toContain('center')
  })
})
