import { describe, it, expect } from 'vitest';
import { renderHook } from '@testing-library/react';
import { usePathHighlight } from './usePathHighlight';
import type { ClusterEdge } from '../types';

describe('usePathHighlight', () => {
  // Sample graph: Ingress -> Service -> Pod1, Pod2
  const sampleEdges: ClusterEdge[] = [
    {
      id: 'edge-1',
      source: 'Ingress/default/my-ingress',
      target: 'Service/default/my-service',
      edgeType: 'ingress-to-service',
    },
    {
      id: 'edge-2',
      source: 'Service/default/my-service',
      target: 'Pod/default/pod-1',
      edgeType: 'service-to-pod',
    },
    {
      id: 'edge-3',
      source: 'Service/default/my-service',
      target: 'Pod/default/pod-2',
      edgeType: 'service-to-pod',
    },
    {
      id: 'edge-4',
      source: 'Deployment/default/my-deployment',
      target: 'Pod/default/pod-1',
      edgeType: 'controller-to-pod',
    },
  ];

  describe('when no node is selected', () => {
    it('should return null for isNodeHighlighted', () => {
      const { result } = renderHook(() => usePathHighlight(sampleEdges, null));

      expect(result.current.isNodeHighlighted('Pod/default/pod-1')).toBe(null);
      expect(result.current.isNodeHighlighted('Service/default/my-service')).toBe(null);
    });

    it('should return empty sets for upstream/downstream', () => {
      const { result } = renderHook(() => usePathHighlight(sampleEdges, null));

      expect(result.current.upstreamNodeIds.size).toBe(0);
      expect(result.current.downstreamNodeIds.size).toBe(0);
      expect(result.current.highlightedEdgeIds.size).toBe(0);
    });

    it('should return false for isEdgeHighlighted', () => {
      const { result } = renderHook(() => usePathHighlight(sampleEdges, null));

      expect(result.current.isEdgeHighlighted('edge-1')).toBe(false);
      expect(result.current.isEdgeHighlighted('edge-2')).toBe(false);
    });
  });

  describe('when a node is selected', () => {
    it('should return "selected" for the selected node', () => {
      const { result } = renderHook(() =>
        usePathHighlight(sampleEdges, 'Service/default/my-service')
      );

      expect(result.current.isNodeHighlighted('Service/default/my-service')).toBe('selected');
    });

    it('should correctly identify upstream nodes', () => {
      const { result } = renderHook(() =>
        usePathHighlight(sampleEdges, 'Service/default/my-service')
      );

      // Ingress points TO Service, so Ingress is upstream
      expect(result.current.isNodeHighlighted('Ingress/default/my-ingress')).toBe('upstream');
      expect(result.current.upstreamNodeIds.has('Ingress/default/my-ingress')).toBe(true);
    });

    it('should correctly identify downstream nodes', () => {
      const { result } = renderHook(() =>
        usePathHighlight(sampleEdges, 'Service/default/my-service')
      );

      // Pods are targets of Service, so Pods are downstream
      expect(result.current.isNodeHighlighted('Pod/default/pod-1')).toBe('downstream');
      expect(result.current.isNodeHighlighted('Pod/default/pod-2')).toBe('downstream');
      expect(result.current.downstreamNodeIds.has('Pod/default/pod-1')).toBe(true);
      expect(result.current.downstreamNodeIds.has('Pod/default/pod-2')).toBe(true);
    });

    it('should mark unrelated nodes as dimmed', () => {
      const { result } = renderHook(() =>
        usePathHighlight(sampleEdges, 'Service/default/my-service')
      );

      // Deployment is only connected to Pod, not through Service's path
      expect(result.current.isNodeHighlighted('Deployment/default/my-deployment')).toBe('dimmed');
    });

    it('should highlight relevant edges', () => {
      const { result } = renderHook(() =>
        usePathHighlight(sampleEdges, 'Service/default/my-service')
      );

      // Edges connected to Service should be highlighted
      expect(result.current.isEdgeHighlighted('edge-1')).toBe(true); // Ingress -> Service
      expect(result.current.isEdgeHighlighted('edge-2')).toBe(true); // Service -> Pod1
      expect(result.current.isEdgeHighlighted('edge-3')).toBe(true); // Service -> Pod2

      // Deployment -> Pod edge is not on Service's path
      expect(result.current.isEdgeHighlighted('edge-4')).toBe(false);
    });
  });

  describe('when selecting a leaf node (Pod)', () => {
    it('should find all upstream connections', () => {
      const { result } = renderHook(() =>
        usePathHighlight(sampleEdges, 'Pod/default/pod-1')
      );

      expect(result.current.isNodeHighlighted('Pod/default/pod-1')).toBe('selected');
      expect(result.current.isNodeHighlighted('Service/default/my-service')).toBe('upstream');
      expect(result.current.isNodeHighlighted('Ingress/default/my-ingress')).toBe('upstream');
      expect(result.current.isNodeHighlighted('Deployment/default/my-deployment')).toBe('upstream');
    });

    it('should have no downstream nodes for a leaf', () => {
      const { result } = renderHook(() =>
        usePathHighlight(sampleEdges, 'Pod/default/pod-1')
      );

      expect(result.current.downstreamNodeIds.size).toBe(0);
    });
  });

  describe('when selecting a root node (Ingress)', () => {
    it('should find all downstream connections', () => {
      const { result } = renderHook(() =>
        usePathHighlight(sampleEdges, 'Ingress/default/my-ingress')
      );

      expect(result.current.isNodeHighlighted('Ingress/default/my-ingress')).toBe('selected');
      expect(result.current.isNodeHighlighted('Service/default/my-service')).toBe('downstream');
      expect(result.current.isNodeHighlighted('Pod/default/pod-1')).toBe('downstream');
      expect(result.current.isNodeHighlighted('Pod/default/pod-2')).toBe('downstream');
    });

    it('should have no upstream nodes for a root', () => {
      const { result } = renderHook(() =>
        usePathHighlight(sampleEdges, 'Ingress/default/my-ingress')
      );

      expect(result.current.upstreamNodeIds.size).toBe(0);
    });
  });

  describe('with empty edges', () => {
    it('should handle empty edge list', () => {
      const { result } = renderHook(() =>
        usePathHighlight([], 'Pod/default/pod-1')
      );

      expect(result.current.isNodeHighlighted('Pod/default/pod-1')).toBe('selected');
      expect(result.current.upstreamNodeIds.size).toBe(0);
      expect(result.current.downstreamNodeIds.size).toBe(0);
    });
  });

  describe('memoization', () => {
    it('should return stable references when inputs do not change', () => {
      const { result, rerender } = renderHook(
        ({ edges, selectedId }) => usePathHighlight(edges, selectedId),
        {
          initialProps: {
            edges: sampleEdges,
            selectedId: 'Service/default/my-service',
          },
        }
      );

      const initialIsNodeHighlighted = result.current.isNodeHighlighted;
      const initialIsEdgeHighlighted = result.current.isEdgeHighlighted;

      rerender({ edges: sampleEdges, selectedId: 'Service/default/my-service' });

      // Functions should be the same reference if inputs haven't changed
      expect(result.current.isNodeHighlighted).toBe(initialIsNodeHighlighted);
      expect(result.current.isEdgeHighlighted).toBe(initialIsEdgeHighlighted);
    });
  });
});

