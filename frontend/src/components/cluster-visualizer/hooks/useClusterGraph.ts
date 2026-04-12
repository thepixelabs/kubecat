import { useState, useEffect, useMemo, useRef, useCallback } from "react";
import {
  ListResources,
  GetClusterEdges,
} from "../../../../wailsjs/go/main/App";
import type {
  ClusterNode,
  ClusterEdge,
  ClusterGraphData,
  NodeType,
} from "../types";

interface ResourceInfo {
  kind: string;
  name: string;
  namespace: string;
  status: string;
  ownerKind?: string;
  ownerName?: string;
  age: string;
  labels?: Record<string, string>;
  restarts?: number;
  node?: string;
  readyContainers?: string;
  serviceType?: string;
  clusterIP?: string;
  ports?: string;
  // Node-specific
  cpuCapacity?: string;
  memCapacity?: string;
  cpuAllocatable?: string;
  memAllocatable?: string;
  nodeConditions?: string[];
}

interface BackendEdge {
  id: string;
  source: string;
  target: string;
  edgeType: string;
  label?: string;
}

interface UseClusterGraphOptions {
  namespace: string;
  isConnected: boolean;
}

export function useClusterGraph({
  namespace,
  isConnected,
}: UseClusterGraphOptions) {
  const [nodes, setNodes] = useState<ClusterNode[]>([]);
  const [edges, setEdges] = useState<ClusterEdge[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Request-generation counter so that results from a stale fetch (issued
  // before the user changed namespace) are discarded instead of overwriting
  // newer state. The Wails-generated bindings do not accept AbortSignal, so
  // we cannot truly cancel in-flight calls — but we can ignore their results.
  const fetchEpochRef = useRef(0);

  const fetchData = useCallback(async () => {
    if (!isConnected) {
      setNodes([]);
      setEdges([]);
      return;
    }

    const epoch = ++fetchEpochRef.current;
    setLoading(true);
    setError(null);

    try {
      // Fetch resources in parallel
      const [
        pods,
        services,
        ingresses,
        deployments,
        statefulsets,
        daemonsets,
        replicasets,
        jobs,
        cronjobs,
        k8sNodes,
        edgesData,
      ] = await Promise.all([
        ListResources("pods", namespace).catch(() => []),
        ListResources("services", namespace).catch(() => []),
        ListResources("ingresses", namespace).catch(() => []),
        ListResources("deployments", namespace).catch(() => []),
        ListResources("statefulsets", namespace).catch(() => []),
        ListResources("daemonsets", namespace).catch(() => []),
        ListResources("replicasets", namespace).catch(() => []),
        ListResources("jobs", namespace).catch(() => []),
        ListResources("cronjobs", namespace).catch(() => []),
        ListResources("nodes", "").catch(() => []), // Nodes are cluster-wide
        GetClusterEdges(namespace).catch(() => []),
      ]);

      // Transform resources to nodes
      const allNodes: ClusterNode[] = [];

      // Helper to detect operators
      const isOperator = (r: ResourceInfo): boolean => {
        const labels = r.labels || {};
        const name = r.name.toLowerCase();

        // Common operator patterns
        if (labels["app.kubernetes.io/component"] === "operator") return true;
        if (labels["app.kubernetes.io/name"]?.includes("operator")) return true;
        if (name.endsWith("-operator")) return true;
        if (name.includes("controller-manager")) return true;

        return false;
      };

      const findOwner = (
        r: ResourceInfo,
        candidates: ClusterNode[]
      ): string | undefined => {
        if (!r.ownerKind || !r.ownerName) return undefined;

        const ownerNode = candidates.find(
          (c) =>
            c.type === r.ownerKind &&
            c.name === r.ownerName &&
            c.namespace === r.namespace
        );
        return ownerNode?.id;
      };

      const transformResource = (
        r: ResourceInfo,
        type: NodeType,
        potentialParents: ClusterNode[] = []
      ): ClusterNode => {
        let finalType = type;

        // Check if Deployment is actually an Operator
        if (type === "Deployment" && isOperator(r)) {
          finalType = "Operator";
        }

        const ownerId = findOwner(r, potentialParents);

        return {
          id: `${finalType}/${r.namespace}/${r.name}`,
          type: finalType,
          name: r.name,
          namespace: r.namespace,
          status: r.status,
          labels: r.labels,
          restarts: r.restarts,
          node: r.node,
          readyContainers: r.readyContainers,
          serviceType: r.serviceType,
          clusterIP: r.clusterIP,
          ports: r.ports,
          age: r.age,
          // Node stats
          cpuCapacity: r.cpuCapacity,
          memCapacity: r.memCapacity,
          cpuAllocatable: r.cpuAllocatable,
          memAllocatable: r.memAllocatable,
          nodeConditions: r.nodeConditions,
          // Nesting
          parentId: ownerId,
          extent: ownerId ? "parent" : undefined,
          expandParent: !!ownerId,
        };
      };

      // 1. Create top-level Nodes (Compute Region)
      k8sNodes?.forEach((r: ResourceInfo) =>
        allNodes.push(transformResource(r, "Node", allNodes))
      );

      // 2. Create Namespaces (Network Region)
      // Collect namespaces from resources
      const distinctNamespaces = new Set<string>();
      if (namespace && namespace !== "") {
        distinctNamespaces.add(namespace);
      } else {
        [
          pods,
          services,
          deployments,
          statefulsets,
          daemonsets,
          jobs,
          cronjobs,
        ].forEach((list) => {
          list?.forEach((r: ResourceInfo) => {
            if (r.namespace) distinctNamespaces.add(r.namespace);
          });
        });
      }

      distinctNamespaces.forEach((ns) => {
        allNodes.push({
          id: `Namespace/${ns}`,
          type: "Namespace",
          name: ns,
          namespace: ns,
          status: "Active",
          parentId: undefined, // Top level
        });
      });

      // 3. Process Network Resources (Services, Ingress, etc.) -> Namespace
      services?.forEach((r: ResourceInfo) => {
        const node = transformResource(r, "Service", allNodes);
        if (r.namespace) node.parentId = `Namespace/${r.namespace}`;
        allNodes.push(node);
      });
      ingresses?.forEach((r: ResourceInfo) => {
        const node = transformResource(r, "Ingress", allNodes);
        if (r.namespace) node.parentId = `Namespace/${r.namespace}`;
        allNodes.push(node);
      });

      // 4. Process Workloads -> Placement Nodes -> Nodes
      // We don't add raw Deployments/StatefulSets to the graph anymore.
      // Instead, we create "Placement" nodes based on Pod distribution.

      // Helper map to track existence of Placement nodes
      const placementMap = new Map<string, ClusterNode>();

      // We still need to know about controllers to properly label placement nodes
      const controllerMap = new Map<string, ResourceInfo>();
      [
        ...(deployments || []),
        ...(statefulsets || []),
        ...(daemonsets || []),
      ].forEach((c) => {
        controllerMap.set(`${c.kind}/${c.namespace}/${c.name}`, c);
      });

      pods?.forEach((r: ResourceInfo) => {
        const node = transformResource(r, "Pod", allNodes);

        let parentId: string | undefined = undefined;

        // A. Determine Physical Parent (Node)
        const nodeName = r.node;
        // removed unused nodeClusterId
        // Check how we generated Node ID above: transformResource calls findOwner? No.
        // Let's check Node ID generation in transformResource: `Node/${r.namespace}/${r.name}`. Nodes usually come with empty namespace.
        // We'll trust the ID generation relative to the fetched node list.

        const physicalNode = k8sNodes?.find((n) => n.name === nodeName);
        const physicalNodeId = physicalNode
          ? `Node/${physicalNode.namespace}/${physicalNode.name}`
          : undefined;

        // B. Determine Logical Owner (Deployment/StatefulSet)
        // Pods are usually owned by ReplicaSets, which are owned by Deployments.
        // We need to traverse up.
        // transformResource already does one level check.
        let ownerId = node.parentId;
        let ownerKind = "Unknown";
        let ownerName = "Unknown";

        // Logic to ascend hierarchy (Pod -> RS -> Deployment)
        if (ownerId && ownerId.startsWith("ReplicaSet/")) {
          // Find RS info logic? We don't have RS nodes in graph necessarily if we skip them.
          // But we have the list `replicasets`.
          const rs = replicasets?.find(
            (rs: any) => `ReplicaSet/${rs.namespace}/${rs.name}` === ownerId
          );
          if (rs && rs.ownerKind && rs.ownerName) {
            ownerId = `${rs.ownerKind}/${rs.namespace}/${rs.ownerName}`;
            ownerKind = rs.ownerKind;
            ownerName = rs.ownerName;
          } else if (rs) {
            ownerKind = "ReplicaSet";
            ownerName = rs.name;
          }
        } else if (ownerId) {
          // Direct parent (StatefulSet, DaemonSet)
          const parts = ownerId.split("/");
          if (parts.length >= 3) {
            ownerKind = parts[0];
            ownerName = parts[parts.length - 1];
          }
        }

        // C. Create/Find Placement Node if we have a physical node
        if (physicalNodeId && ownerId) {
          const placementId = `placement/${physicalNodeId}/${ownerId}`; // e.g., placement/Node/def/node1/Deployment/def/nginx

          if (!placementMap.has(placementId)) {
            // Create synthetic placement node
            const placementNode: ClusterNode = {
              id: placementId,
              type: "Placement", // New Type
              name: `${ownerKind}: ${ownerName}`,
              namespace: r.namespace,
              status: "Active",
              parentId: physicalNodeId, // Visually INSIDE the Node
              extent: "parent",
              expandParent: true,
            };
            placementMap.set(placementId, placementNode);
            allNodes.push(placementNode);
          }
          parentId = placementId;
        } else if (physicalNodeId) {
          // Naked pod on node (no controller?)
          parentId = physicalNodeId;
        } else {
          // Unscheduled or unknown node? Put in Namespace
          parentId = `Namespace/${r.namespace}`;
        }

        node.parentId = parentId;
        node.extent = parentId ? "parent" : undefined;
        allNodes.push(node);
      });

      // Add placement nodes for DaemonSets that might not have pods yet?
      // No, only show what exists.

      // Also add CronJobs/Jobs to namespace as they might not be running
      jobs?.forEach((r: ResourceInfo) => {
        const node = transformResource(r, "Job", allNodes);
        node.parentId = `Namespace/${r.namespace}`;
        allNodes.push(node);
      });
      cronjobs?.forEach((r: ResourceInfo) => {
        const node = transformResource(r, "CronJob", allNodes);
        node.parentId = `Namespace/${r.namespace}`;
        allNodes.push(node);
      });

      // Discard if a newer fetch has been issued since we started.
      if (epoch !== fetchEpochRef.current) return;

      setNodes(allNodes);

      // Transform backend edges to our ClusterEdge type
      const transformedEdges: ClusterEdge[] = (edgesData || []).map(
        (e: BackendEdge) => ({
          id: e.id,
          source: e.source,
          target: e.target,
          edgeType: e.edgeType as ClusterEdge["edgeType"],
          label: e.label,
        })
      );

      // Generate Node -> Pod edges
      if (k8sNodes && pods) {
        pods.forEach((pod: ResourceInfo) => {
          if (pod.node) {
            // Find the node resource to ensure it exists (and get correct ID)
            const nodeResource = k8sNodes.find((n) => n.name === pod.node);
            if (nodeResource) {
              transformedEdges.push({
                id: `node-${nodeResource.name}-pod-${pod.name}`,
                source: `Node/${nodeResource.namespace}/${nodeResource.name}`,
                target: `Pod/${pod.namespace}/${pod.name}`,
                edgeType: "node-to-pod",
              });
            }
          }
        });
      }

      if (epoch !== fetchEpochRef.current) return;
      setEdges(transformedEdges);
    } catch (err) {
      if (epoch !== fetchEpochRef.current) return;
      console.error("Failed to fetch cluster graph:", err);
      setError(err instanceof Error ? err.message : "Failed to fetch data");
    } finally {
      if (epoch === fetchEpochRef.current) {
        setLoading(false);
      }
    }
  }, [isConnected, namespace]);

  // Debounce subsequent namespace changes by 300ms so that rapid switches
  // collapse to a single request instead of triggering an 11-call thundering
  // herd per change. The very first fetch (mount) runs immediately so initial
  // load is not delayed and existing tests that assert synchronous behavior
  // continue to pass. Stale in-flight fetches are guarded by the epoch counter.
  const hasFetchedRef = useRef(false);
  useEffect(() => {
    if (!hasFetchedRef.current) {
      hasFetchedRef.current = true;
      fetchData();
      return;
    }
    const timer = setTimeout(() => {
      fetchData();
    }, 300);
    return () => clearTimeout(timer);
  }, [namespace, isConnected, fetchData]);

  const graphData = useMemo<ClusterGraphData>(
    () => ({
      nodes,
      edges,
    }),
    [nodes, edges]
  );

  return {
    graphData,
    loading,
    error,
    refetch: fetchData,
  };
}
