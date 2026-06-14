import { describe, it, expect } from 'vitest'
import {
  bfsHighlight,
  COMPONENT_THRESHOLD,
  MAX_HOPS,
  SMALL_GRAPH_NODE_LIMIT,
  type GraphHighlightEdge,
} from './graphBfsHighlight'

describe('bfsHighlight', () => {
  describe('small graph (<=8 nodes)', () => {
    it('returns full connected component for small graph', () => {
      const edges: GraphHighlightEdge<string>[] = [
        { source: 'A', target: 'B' },
        { source: 'B', target: 'C' },
        { source: 'C', target: 'D' },
      ]
      const result = bfsHighlight('A', edges, 5)
      expect(result).toEqual(new Set(['A', 'B', 'C', 'D']))
    })

    it('returns only focus node for isolated node in small graph', () => {
      const edges: GraphHighlightEdge<string>[] = [
        { source: 'A', target: 'B' },
      ]
      const result = bfsHighlight('C', edges, 3)
      expect(result).toEqual(new Set(['C']))
    })
  })

  describe('sparse component', () => {
    it('returns full component when component is small relative to total', () => {
      // 100 nodes total, component has 25 nodes (25% < 40%)
      const edges: GraphHighlightEdge<string>[] = []
      for (let i = 0; i < 25; i++) {
        edges.push({ source: `node-${i}`, target: `node-${i + 1}` })
      }
      const result = bfsHighlight('node-0', edges, 100)
      expect(result.size).toBe(26) // 0..25
      expect(result.has('node-0')).toBe(true)
      expect(result.has('node-25')).toBe(true)
    })
  })

  describe('dense component', () => {
    it('limits nodes in dense star graph', () => {
      // 100 nodes, focus connects to all 99 others (star graph)
      const edges: GraphHighlightEdge<string>[] = []
      for (let i = 1; i < 100; i++) {
        edges.push({ source: 'center', target: `node-${i}` })
      }
      const result = bfsHighlight('center', edges, 100)
      // maxNodes = floor(100 * 0.4) = 40
      expect(result.size).toBeLessThanOrEqual(40)
      expect(result.has('center')).toBe(true)
    })

    it('limits by hop count for deep chain', () => {
      // 100 nodes in a chain: A-B-C-D-E-F-...
      const edges: GraphHighlightEdge<string>[] = []
      for (let i = 0; i < 99; i++) {
        edges.push({ source: `n${i}`, target: `n${i + 1}` })
      }
      const result = bfsHighlight('n0', edges, 100)
      // With MAX_HOPS=4, should include n0..n4 (5 nodes)
      // But also limited by maxNodes=40, so should be 5
      expect(result.size).toBeLessThanOrEqual(40)
      expect(result.has('n0')).toBe(true)
      expect(result.has('n4')).toBe(true)
      // n5 should NOT be included (distance = 5 > MAX_HOPS=4)
      expect(result.has('n5')).toBe(false)
    })
  })

  describe('isolated node', () => {
    it('returns only focus node when no edges exist', () => {
      const result = bfsHighlight('alone', [], 100)
      expect(result).toEqual(new Set(['alone']))
    })
  })

  describe('numeric IDs', () => {
    it('works with numeric node IDs', () => {
      const edges: GraphHighlightEdge<number>[] = [
        { source: 50, target: 60 },
        { source: 60, target: 70 },
      ]
      const result = bfsHighlight(50, edges, 5)
      expect(result).toEqual(new Set([50, 60, 70]))
    })
  })

  describe('cyclic graph', () => {
    it('handles cycles without infinite loop', () => {
      const edges: GraphHighlightEdge<string>[] = [
        { source: 'A', target: 'B' },
        { source: 'B', target: 'C' },
        { source: 'C', target: 'A' }, // cycle
      ]
      const result = bfsHighlight('A', edges, 5)
      expect(result).toEqual(new Set(['A', 'B', 'C']))
    })
  })

  describe('disconnected components', () => {
    it('only returns the component containing focus', () => {
      const edges: GraphHighlightEdge<string>[] = [
        { source: 'A', target: 'B' },
        { source: 'C', target: 'D' },
      ]
      const result = bfsHighlight('A', edges, 5)
      expect(result).toEqual(new Set(['A', 'B']))
    })
  })

  describe('constants', () => {
    it('exports expected constant values', () => {
      expect(COMPONENT_THRESHOLD).toBe(0.4)
      expect(MAX_HOPS).toBe(4)
      expect(SMALL_GRAPH_NODE_LIMIT).toBe(8)
    })
  })
})
