import { useMemo, useCallback } from "react";
import type { ClusterEdge } from "../types";

interface PathHighlightResult {
  upstreamNodeIds: Set<string>;
  downstreamNodeIds: Set<string>;
  highlightedEdgeIds: Set<string>;
}

export function usePathHighlight(
  edges: ClusterEdge[],
  selectedNodeId: string | null
) {
  // Build adjacency maps for quick traversal
  const { incomingEdges, outgoingEdges } = useMemo(() => {
    const incoming = new Map<string, ClusterEdge[]>();
    const outgoing = new Map<string, ClusterEdge[]>();

    edges.forEach((edge) => {
      // Outgoing from source
      if (!outgoing.has(edge.source)) {
        outgoing.set(edge.source, []);
      }
      outgoing.get(edge.source)!.push(edge);

      // Incoming to target
      if (!incoming.has(edge.target)) {
        incoming.set(edge.target, []);
      }
      incoming.get(edge.target)!.push(edge);
    });

    return { incomingEdges: incoming, outgoingEdges: outgoing };
  }, [edges]);

  // Compute highlighted paths
  const highlightResult = useMemo<PathHighlightResult>(() => {
    if (!selectedNodeId) {
      return {
        upstreamNodeIds: new Set(),
        downstreamNodeIds: new Set(),
        highlightedEdgeIds: new Set(),
      };
    }

    const upstreamNodeIds = new Set<string>();
    const downstreamNodeIds = new Set<string>();
    const highlightedEdgeIds = new Set<string>();

    // BFS for upstream (who points TO this node)
    const upstreamQueue = [selectedNodeId];
    const visitedUpstream = new Set<string>();

    while (upstreamQueue.length > 0) {
      const current = upstreamQueue.shift()!;
      if (visitedUpstream.has(current)) continue;
      visitedUpstream.add(current);

      const incoming = incomingEdges.get(current) || [];
      incoming.forEach((edge) => {
        highlightedEdgeIds.add(edge.id);
        if (!visitedUpstream.has(edge.source)) {
          upstreamNodeIds.add(edge.source);
          upstreamQueue.push(edge.source);
        }
      });
    }

    // BFS for downstream (who this node points TO)
    const downstreamQueue = [selectedNodeId];
    const visitedDownstream = new Set<string>();

    while (downstreamQueue.length > 0) {
      const current = downstreamQueue.shift()!;
      if (visitedDownstream.has(current)) continue;
      visitedDownstream.add(current);

      const outgoing = outgoingEdges.get(current) || [];
      outgoing.forEach((edge) => {
        highlightedEdgeIds.add(edge.id);
        if (!visitedDownstream.has(edge.target)) {
          downstreamNodeIds.add(edge.target);
          downstreamQueue.push(edge.target);
        }
      });
    }

    return { upstreamNodeIds, downstreamNodeIds, highlightedEdgeIds };
  }, [selectedNodeId, incomingEdges, outgoingEdges]);

  const isNodeHighlighted = useCallback(
    (
      nodeId: string
    ): "selected" | "upstream" | "downstream" | "dimmed" | null => {
      if (!selectedNodeId) return null;
      if (nodeId === selectedNodeId) return "selected";
      if (highlightResult.upstreamNodeIds.has(nodeId)) return "upstream";
      if (highlightResult.downstreamNodeIds.has(nodeId)) return "downstream";
      return "dimmed";
    },
    [selectedNodeId, highlightResult]
  );

  const isEdgeHighlighted = useCallback(
    (edgeId: string): boolean => {
      return highlightResult.highlightedEdgeIds.has(edgeId);
    },
    [highlightResult]
  );

  return {
    isNodeHighlighted,
    isEdgeHighlighted,
    upstreamNodeIds: highlightResult.upstreamNodeIds,
    downstreamNodeIds: highlightResult.downstreamNodeIds,
    highlightedEdgeIds: highlightResult.highlightedEdgeIds,
  };
}
