import { useState, useCallback, useMemo, useEffect, useRef } from "react";
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  useNodesState,
  useEdgesState,
  type Node,
  type Edge,
  Panel,
} from "@xyflow/react";
import { useTheme } from "next-themes";
import "@xyflow/react/dist/style.css";
import ELK from "elkjs/lib/elk.bundled";
import { Loader2, RefreshCw } from "lucide-react";

import { PodNode } from "./nodes/PodNode";
import { ServiceNode } from "./nodes/ServiceNode";
import { IngressNode } from "./nodes/IngressNode";
import { ControllerNode } from "./nodes/ControllerNode";
import { ServerNode } from "./nodes/ServerNode";
import { NamespaceNode } from "./nodes/NamespaceNode";
import { AnimatedEdge } from "./edges/AnimatedEdge";
import { MetadataDrawer } from "./panels/MetadataDrawer";
import { AnalysisModal } from "../AnalysisModal";
import { TogglePanel } from "./panels/TogglePanel";
import { TerminalDrawer } from "../Terminal/TerminalDrawer";
import { useClusterGraph } from "./hooks/useClusterGraph";
import { usePathHighlight } from "./hooks/usePathHighlight";
import { StartTerminal, CloseTerminal } from "../../../wailsjs/go/main/App";
import type { ClusterNode, ToggleState } from "./types";
import { AnimatePresence, motion } from "framer-motion";
import { Check, ChevronDown } from "lucide-react";

// Use 'any' for node types to avoid React Flow's strict typing issues
const nodeTypes = {
  Pod: PodNode,
  Service: ServiceNode,
  Ingress: IngressNode,
  Deployment: ControllerNode,
  StatefulSet: ControllerNode,
  DaemonSet: ControllerNode,
  ReplicaSet: ControllerNode,
  Job: ControllerNode,
  CronJob: ControllerNode,
  Operator: ControllerNode,
  Node: ServerNode,
  Namespace: NamespaceNode,
  Placement: ControllerNode, // Reuse Controller visual for Placement
} as const;

const edgeTypes = {
  animated: AnimatedEdge,
} as const;

const elk = new ELK();

const elkOptions = {
  "elk.algorithm": "layered",
  "elk.layered.spacing.nodeNodeBetweenLayers": "80", // Vertical spacing between layers
  "elk.spacing.nodeNode": "60", // Horizontal spacing
  "elk.direction": "DOWN",
  "elk.aspectRatio": "UNDEFINED", // Let naturally flow (since containers are packed)
  "elk.edgeRouting": "ORTHOGONAL",
  "elk.hierarchyHandling": "INCLUDE_CHILDREN",
  "elk.padding": "[top=40,left=20,bottom=20,right=20]",
};

interface ClusterVisualizerProps {
  isConnected: boolean;
  namespaces: string[];
  onRefreshNamespaces?: () => void;
}

const getNodeColor = (node: Node) => {
  switch (node.type) {
    case "Pod":
      return "#64748b";
    case "Service":
      return "#06b6d4";
    case "Ingress":
      return "#a855f7";
    case "Deployment":
      return "#22c55e";
    case "StatefulSet":
      return "#f59e0b";
    case "DaemonSet":
      return "#f43f5e";
    case "Namespace":
      return "transparent"; // Container style handles it
    case "Node":
      return "#475569";
    case "Operator":
      return "#6366f1";
    case "Job":
    case "CronJob":
      return "#3b82f6";
    case "ReplicaSet":
      return "#94a3b8";
    default:
      return "#64748b";
  }
};

export function ClusterVisualizer({
  isConnected,
  namespaces,
  onRefreshNamespaces,
}: ClusterVisualizerProps) {
  const { resolvedTheme } = useTheme();
  // Default to "" ("All Namespaces") so users whose `default` namespace is
  // empty still see a populated graph on first load. TogglePanel.tsx already
  // sends "" for the "All Namespaces" option, so this matches existing
  // convention.
  const [selectedNamespace, setSelectedNamespace] = useState<string>("");
  const [layoutError, setLayoutError] = useState<string | null>(null);
  const [toggles, setToggles] = useState<ToggleState>({
    showPods: true,
    showServices: true,
    showIngresses: true,
    showDeployments: true,
    showStatefulSets: true,
    showDaemonSets: true,
    showServiceToPod: true,
    showIngressToService: true,
    showControllerToPod: true,
    showNodes: true,
    showReplicaSets: false,
    showJobs: false,
    showCronJobs: false,
    showOperators: true,
    showNodeToPod: true,
    showOperatorToManaged: true,
  });
  const [selectedNode, setSelectedNode] = useState<ClusterNode | null>(null);
  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([]);
  const [analysisResource, setAnalysisResource] = useState<{
    kind: string;
    namespace: string;
    name: string;
  } | null>(null);

  // Terminal State
  const [showTerminal, setShowTerminal] = useState(false);
  const [terminalSessionId, setTerminalSessionId] = useState<string | null>(
    null
  );

  const { graphData, loading, error, refetch } = useClusterGraph({
    namespace: selectedNamespace,
    isConnected,
  });

  const { isNodeHighlighted, isEdgeHighlighted } = usePathHighlight(
    graphData.edges,
    selectedNode?.id || null
  );

  // Hold the latest highlight functions in a ref so the expensive ELK layout
  // effect can read them without including them in its dependency array. The
  // layout only needs to re-run on topology changes; selection changes are
  // applied separately by the highlight-patching effect below.
  const highlightFnsRef = useRef({ isNodeHighlighted, isEdgeHighlighted });
  useEffect(() => {
    highlightFnsRef.current = { isNodeHighlighted, isEdgeHighlighted };
  }, [isNodeHighlighted, isEdgeHighlighted]);

  const handleToggle = useCallback((key: keyof ToggleState) => {
    setToggles((prev) => ({ ...prev, [key]: !prev[key] }));
  }, []);

  const handleNodeClick = useCallback(
    (_: React.MouseEvent, node: Node) => {
      const clusterNode = graphData.nodes.find((n) => n.id === node.id);
      setSelectedNode(clusterNode || null);
    },
    [graphData.nodes]
  );

  const handlePaneClick = useCallback(() => {
    // Only clear selection if we clicked canvas, not if we clicked a node
    // But ReactFlow handles this, onPaneClick only fires on background click
    setSelectedNode(null);
    setShowTerminal(false);
  }, []);

  // Filter nodes based on toggles
  const filteredNodes = useMemo(() => {
    return graphData.nodes.filter((node) => {
      switch (node.type) {
        case "Pod":
          return toggles.showPods;
        case "Service":
          return toggles.showServices;
        case "Ingress":
          return toggles.showIngresses;
        case "Deployment":
          return toggles.showDeployments;
        case "StatefulSet":
          return toggles.showStatefulSets;
        case "DaemonSet":
          return toggles.showDaemonSets;
        case "Node":
          return toggles.showNodes;
        case "ReplicaSet":
          return toggles.showReplicaSets;
        case "Job":
          return toggles.showJobs;
        case "CronJob":
          return toggles.showCronJobs;
        case "Operator":
          return toggles.showOperators;
        default:
          return true;
      }
    });
  }, [graphData.nodes, toggles]);

  // Filter edges based on toggles
  const filteredEdges = useMemo(() => {
    const visibleNodeIds = new Set(filteredNodes.map((n) => n.id));
    return graphData.edges.filter((edge) => {
      // Must have both endpoints visible
      if (
        !visibleNodeIds.has(edge.source) ||
        !visibleNodeIds.has(edge.target)
      ) {
        return false;
      }
      switch (edge.edgeType) {
        case "service-to-pod":
          return toggles.showServiceToPod;
        case "ingress-to-service":
          return toggles.showIngressToService;
        case "controller-to-pod":
          return toggles.showControllerToPod;
        case "node-to-pod":
          return toggles.showNodeToPod;
        case "operator-to-managed":
          return toggles.showOperatorToManaged;
        default:
          return true;
      }
    });
  }, [graphData.edges, filteredNodes, toggles]);

  // Keyboard shortcut for Terminal
  useEffect(() => {
    const handleKeyDown = async (e: KeyboardEvent) => {
      // Ignore if input/textarea focused
      if (
        (e.target as HTMLElement).tagName === "INPUT" ||
        (e.target as HTMLElement).tagName === "TEXTAREA"
      ) {
        return;
      }

      if (e.key === "s") {
        if (selectedNode && selectedNode.type === "Pod") {
          // Toggle terminal
          if (showTerminal) {
            setShowTerminal(false);
            // Optional: Close session when closing drawer?
            // Better to keep it alive until user explicitly closes or navigates away?
            // For now, let's keep it simple: closing drawer closes session to save resources/avoid orphan shells
            if (terminalSessionId) {
              CloseTerminal(terminalSessionId);
              setTerminalSessionId(null);
            }
          } else {
            setShowTerminal(true);
            // Start session
            const sessionId = `term-${Date.now()}`;
            setTerminalSessionId(sessionId);
            try {
              // Default to /bin/sh for now, could be configurable
              // Start(id, command, args...)
              // For kubectl exec: kubectl exec -i -t -n <namespace> <pod> -- /bin/sh
              // Backend manager expects just the shell command?
              // Wait, the backend manager implementation I wrote executes `exec.Command(command, args...)`.
              // It does NOT automatically wrap kubectl exec.
              // I need to start `kubectl` directly from frontend args.

              // Command: kubectl
              // Args: exec, -i, -t, -n, namespace, pod, --, /bin/sh

              await StartTerminal(sessionId, "kubectl", [
                "exec",
                "-i",
                "-t",
                "-n",
                selectedNode.namespace,
                selectedNode.name,
                "--",
                "/bin/sh",
              ]);
            } catch (err) {
              console.error("Failed to start terminal:", err);
            }
          }
        }
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [selectedNode, showTerminal, terminalSessionId]);

  // Clean up terminal on unmount
  useEffect(() => {
    return () => {
      if (terminalSessionId) {
        CloseTerminal(terminalSessionId);
      }
    };
  }, [terminalSessionId]);

  // Apply ELK layout
  useEffect(() => {
    if (filteredNodes.length === 0) {
      setNodes([]);
      setEdges([]);
      return;
    }

    const getLayoutedElements = async () => {
      // 1. Build hierarchy for ELK
      const nodeMap = new Map<string, any>();
      const roots: any[] = [];

      // Create ELK node objects
      filteredNodes.forEach((node) => {
        const isNamespace = node.type === "Namespace";
        const isPlacement = node.type === "Placement";
        const isNode = node.type === "Node";

        // Global defaults
        let layoutOpts: any = {
          "elk.padding": "[top=40,left=20,bottom=20,right=20]",
        };

        if (isNamespace) {
          // Namespaces (Network Region)
          layoutOpts = {
            ...layoutOpts,
            "elk.algorithm": "layered",
            "elk.layered.spacing.nodeNodeBetweenLayers": "80",
            "elk.spacing.nodeNode": "60",
            "elk.direction": "DOWN",
            "elk.edgeRouting": "ORTHOGONAL",
          };
        } else if (isNode) {
          // Physical Nodes (Compute Region) -> Contain Placement Groups
          layoutOpts = {
            ...layoutOpts,
            "elk.algorithm": "rectpacking",
            "elk.rectpacking.width": "250",
            "elk.padding": "[top=40,left=20,bottom=20,right=20]",
            "elk.spacing.nodeNode": "20",
          };
        } else if (isPlacement) {
          // Placement Groups (Split Controllers) -> Contain Pods
          layoutOpts = {
            ...layoutOpts,
            "elk.algorithm": "rectpacking",
            "elk.rectpacking.width": "200",
            "elk.padding": "[top=40,left=10,bottom=10,right=10]",
            "elk.spacing.nodeNode": "10",
          };
        }

        nodeMap.set(node.id, {
          id: node.id,
          width: 150,
          height: 60,
          children: [],
          layoutOptions: layoutOpts,
        });
      });

      // 2. Build Tree
      filteredNodes.forEach((node) => {
        const elkNode = nodeMap.get(node.id);
        if (node.parentId && nodeMap.has(node.parentId)) {
          const parent = nodeMap.get(node.parentId);
          parent.children.push(elkNode);
        } else {
          roots.push(elkNode);
        }
      });

      // 3. Process Edges: Aggregation + Filtering
      const processedEdges: any[] = [];
      const addedEdgeIds = new Set<string>();

      filteredEdges.forEach((edge) => {
        // CRITICAL: Filter out Node-to-Pod edges to reduce crossing lines significantly
        if (edge.edgeType === "node-to-pod") return;

        let target = edge.target;
        const source = edge.source;
        let id = edge.id;

        // Aggregation: Service->Pod => Service->Placement
        if (edge.edgeType === "service-to-pod") {
          const targetNode = filteredNodes.find((n) => n.id === target);
          // Ensure parent exists
          if (targetNode?.parentId && nodeMap.has(targetNode.parentId)) {
            const parentNode = filteredNodes.find(
              (n) => n.id === targetNode.parentId
            );
            // If parent is Placement, aggregate to it
            if (parentNode && parentNode.type === "Placement") {
              target = targetNode.parentId;
              id = `${source}->${target}`;
            }
          }
        }

        if (!addedEdgeIds.has(id)) {
          addedEdgeIds.add(id);
          processedEdges.push({
            id,
            sources: [source],
            targets: [target],
          });
        }
      });

      const graph = {
        id: "root",
        layoutOptions: elkOptions,
        children: roots,
        edges: processedEdges,
      };

      try {
        const layoutedGraph = await elk.layout(graph);

        // 4. Flatten the graph back for React Flow
        const layoutedNodes: Node[] = [];

        const processNode = (elkNode: any, parentId?: string) => {
          const clusterNode = filteredNodes.find(
            (n) => n && n.id === elkNode.id
          );
          if (!clusterNode) return;

          const highlightState = highlightFnsRef.current.isNodeHighlighted(elkNode.id);

          layoutedNodes.push({
            id: elkNode.id,
            type: clusterNode.type,
            // If parentId exists, position is relative to parent
            position: { x: elkNode.x || 0, y: elkNode.y || 0 },
            parentId: parentId,
            extent: parentId ? "parent" : undefined,
            data: {
              label: clusterNode.name,
              resourceType: clusterNode.type,
              status: clusterNode.status,
              namespace: clusterNode.namespace,
              restarts: clusterNode.restarts,
              serviceType: clusterNode.serviceType,
              isHighlighted: highlightState === "selected",
              isUpstream: highlightState === "upstream",
              isDownstream: highlightState === "downstream",
              isDimmed: highlightState === "dimmed",
              // Node stats
              cpuCapacity: clusterNode.cpuCapacity,
              memCapacity: clusterNode.memCapacity,
              cpuAllocatable: clusterNode.cpuAllocatable,
              memAllocatable: clusterNode.memAllocatable,
              nodeConditions: clusterNode.nodeConditions,
            },
            style:
              elkNode.children && elkNode.children.length > 0
                ? {
                    width: elkNode.width,
                    height: elkNode.height,
                    backgroundColor: "rgba(255, 255, 255, 0.05)",
                    border: "1px dashed rgba(255,255,255,0.2)",
                    borderRadius: "8px",
                  }
                : undefined,
          });

          if (elkNode.children) {
            elkNode.children.forEach((child: any) =>
              processNode(child, elkNode.id)
            );
          }
        };

        (layoutedGraph.children || []).forEach((root) => processNode(root));

        // 5. Re-construct edge objects
        const layoutedEdges: Edge[] = (layoutedGraph.edges || []).map(
          (e: any) => {
            const source = e.sources[0];
            const target = e.targets[0];
            const originalId = e.id;
            const isAggregated = originalId.includes("->");

            let edgeType = "default";
            // Heuristic to recover type
            const sNode = layoutedNodes.find((n) => n.id === source);
            const tNode = layoutedNodes.find((n) => n.id === target);

            if (
              sNode?.type === "Service" &&
              (tNode?.type === "Placement" ||
                ["Deployment", "StatefulSet", "DaemonSet"].includes(
                  tNode?.type || ""
                ))
            ) {
              edgeType = "service-to-pod"; // Use service style for aggregated edges
            } else {
              const original = filteredEdges.find((fe) => fe.id === originalId);
              if (original) edgeType = original.edgeType;
            }

            const isEdgeHighlightedState =
              highlightFnsRef.current.isEdgeHighlighted(originalId) ||
              (isAggregated && highlightFnsRef.current.isEdgeHighlighted(source));

            return {
              id: originalId,
              source: source,
              target: target,
              type: "animated",
              data: {
                edgeType: edgeType,
                isHighlighted: isEdgeHighlightedState,
                isUpstream: false,
              },
              // Add style for visibility
              style: {
                strokeWidth: isEdgeHighlightedState ? 3 : 2, // Thicker base stroke
                stroke: isEdgeHighlightedState
                  ? undefined
                  : "rgba(100, 116, 139, 0.6)", // Slightly darker/more visible
                opacity: 1,
              },
              sections: e.sections,
            };
          }
        );

        setNodes(layoutedNodes);
        setEdges(layoutedEdges);
        setLayoutError(null);
      } catch (err) {
        // Surface ELK failures into the UI (see error branch below).
        // Swallowing into console.error alone left the canvas permanently
        // blank whenever the layout engine threw.
        console.error("Layout failed:", err);
        const message =
          err instanceof Error ? err.message : "Layout engine failed";
        setLayoutError(message);
      }
    };

    getLayoutedElements();
    // Highlight functions are intentionally excluded — they are read via
    // `highlightFnsRef` so that selection changes do not trigger the (very
    // expensive) ELK layout pass. Highlight visuals are patched onto the
    // already-laid-out nodes by the effect below.
  }, [filteredNodes, filteredEdges, graphData.edges, setNodes, setEdges]);

  // Highlight-patching effect: runs on selection change only. Walks the
  // already-laid-out nodes/edges (set by the layout effect above) and
  // rewrites the highlight-related fields in-place. This is O(N) over the
  // already-rendered graph instead of a full ELK relayout.
  useEffect(() => {
    // Cast through `any` because xyflow's useNodesState/useEdgesState setters
    // are typed as accepting only `Node[]`/`Edge[]` despite supporting
    // functional updaters at runtime. We need the functional form to read the
    // current laid-out state without including it in this effect's deps.
    (setNodes as any)((current: Node[]) =>
      current.map((n) => {
        const state = isNodeHighlighted(n.id);
        return {
          ...n,
          data: {
            ...n.data,
            isHighlighted: state === "selected",
            isUpstream: state === "upstream",
            isDownstream: state === "downstream",
            isDimmed: state === "dimmed",
          },
        };
      })
    );
    (setEdges as any)((current: Edge[]) =>
      current.map((e) => {
        const isAggregated = e.id.includes("->");
        const isHl =
          isEdgeHighlighted(e.id) || (isAggregated && isEdgeHighlighted(e.source));
        return {
          ...e,
          data: {
            ...(e.data || {}),
            isHighlighted: isHl,
            isUpstream: false,
          },
          style: {
            ...(e.style || {}),
            strokeWidth: isHl ? 3 : 2,
            stroke: isHl ? undefined : "rgba(100, 116, 139, 0.6)",
            opacity: 1,
          },
        };
      })
    );
  }, [isNodeHighlighted, isEdgeHighlighted, setNodes, setEdges]);

  if (!isConnected) {
    return (
      <div className="h-full flex items-center justify-center">
        <div className="text-center">
          <p className="text-slate-500 dark:text-slate-400 text-lg mb-2">
            No cluster connected
          </p>
          <p className="text-slate-400 dark:text-slate-600 text-sm">
            Please ensure you have a valid kubeconfig
          </p>
        </div>
      </div>
    );
  }

  if (loading && nodes.length === 0) {
    return (
      <div className="h-full flex items-center justify-center">
        <Loader2 className="w-8 h-8 animate-spin text-primary" />
        <span className="ml-2 text-muted-foreground">
          Loading cluster data...
        </span>
      </div>
    );
  }

  if (error || layoutError) {
    return (
      <div className="h-full flex items-center justify-center text-destructive">
        {error
          ? `Error loading cluster data: ${error}`
          : `Graph layout failed: ${layoutError}`}
      </div>
    );
  }

  return (
    <div className="w-full h-full relative group">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        nodeTypes={nodeTypes as any}
        edgeTypes={edgeTypes as any}
        onNodeClick={handleNodeClick}
        onPaneClick={handlePaneClick}
        fitView
        className="bg-white/50 dark:bg-slate-900/50 backdrop-blur-md border border-stone-200 dark:border-slate-700 transition-colors"
      >
        <Background
          gap={12}
          size={1}
          color={resolvedTheme === "dark" ? "#334155" : "#cbd5e1"}
        />
        <style>
          {`
            .react-flow__controls-button {
              background-color: ${
                resolvedTheme === "dark"
                  ? "rgba(30, 41, 59, 0.8) !important"
                  : "rgba(255, 255, 255, 0.8) !important"
              };
              border-bottom: 1px solid ${
                resolvedTheme === "dark"
                  ? "rgba(51, 65, 85, 1) !important"
                  : "rgba(231, 229, 228, 1) !important"
              };
              fill: ${
                resolvedTheme === "dark"
                  ? "#94a3b8 !important"
                  : "#57534e !important"
              };
            }
            .react-flow__controls-button:hover {
              background-color: ${
                resolvedTheme === "dark"
                  ? "rgba(51, 65, 85, 0.9) !important"
                  : "rgba(245, 245, 244, 0.9) !important"
              };
            }
            .react-flow__controls,
            .react-flow__minimap {
              box-shadow: 0 4px 6px -1px rgb(0 0 0 / 0.1), 0 2px 4px -2px rgb(0 0 0 / 0.1) !important;
              border: 1px solid ${
                resolvedTheme === "dark"
                  ? "rgba(51, 65, 85, 1) !important"
                  : "rgba(231, 229, 228, 1) !important"
              } !important;
              border-radius: 0.5rem !important;
              overflow: hidden !important;
              background-color: ${
                resolvedTheme === "dark"
                  ? "rgba(30, 41, 59, 0.8) !important"
                  : "rgba(255, 255, 255, 0.8) !important"
              };
            }
            .react-flow__attribution {
              display: none !important;
            }
          `}
        </style>
        <Controls showInteractive={false} />
        <MiniMap
          nodeColor={getNodeColor}
          className="!bg-transparent !border-0"
          maskColor={
            resolvedTheme === "dark"
              ? "rgba(30, 41, 59, 0.7)"
              : "rgba(255, 255, 255, 0.7)"
          }
        />
        <Panel position="top-right" className="flex gap-2">
          <button
            className="p-2 rounded-md border border-stone-200 dark:border-slate-700 bg-white/80 dark:bg-slate-800/90 backdrop-blur-md hover:bg-stone-100 dark:hover:bg-slate-700 transition-colors"
            onClick={() => {
              onRefreshNamespaces?.();
              refetch();
            }}
            title="Refresh"
          >
            <RefreshCw className="h-4 w-4 text-slate-600 dark:text-slate-400" />
          </button>
        </Panel>
      </ReactFlow>

      {/* Always-visible namespace HUD. Lives outside the draggable TogglePanel
          so it is discoverable even when users never open the panel. */}
      <NamespaceHud
        namespaces={namespaces}
        selectedNamespace={selectedNamespace}
        onNamespaceChange={setSelectedNamespace}
      />

      <TogglePanel toggles={toggles} onToggle={handleToggle} />

      {/* Metadata Drawer */}
      <MetadataDrawer
        node={selectedNode}
        onClose={handlePaneClick}
        onAnalyze={() => {
          if (selectedNode) {
            setAnalysisResource({
              kind: selectedNode.type,
              namespace: selectedNode.namespace,
              name: selectedNode.name,
            });
          }
        }}
      />

      <AnalysisModal
        isOpen={!!analysisResource}
        onClose={() => setAnalysisResource(null)}
        resource={analysisResource}
      />

      {/* Terminal Drawer */}
      <TerminalDrawer
        isOpen={showTerminal}
        onClose={() => {
          setShowTerminal(false);
          if (terminalSessionId) {
            CloseTerminal(terminalSessionId);
            setTerminalSessionId(null);
          }
        }}
        sessionId={terminalSessionId}
        nodeName={selectedNode?.name || ""}
        namespace={selectedNode?.namespace || ""}
      />
    </div>
  );
}

interface NamespaceHudProps {
  namespaces: string[];
  selectedNamespace: string;
  onNamespaceChange: (ns: string) => void;
}

// Compact always-visible namespace switcher anchored to the top-left of the
// graph. Deliberately NOT draggable — previously the only namespace control
// lived inside TogglePanel, which users can miss entirely, so they would see
// a blank canvas and have no idea the empty result came from a namespace
// filter. This sits above the draggable panel and always renders.
function NamespaceHud({
  namespaces,
  selectedNamespace,
  onNamespaceChange,
}: NamespaceHudProps) {
  const [isOpen, setIsOpen] = useState(false);

  return (
    <div className="absolute top-4 left-4 z-20 min-w-[200px] max-w-[240px]">
      <label className="text-[10px] text-stone-500 dark:text-slate-400 uppercase tracking-wider block mb-1 pl-1">
        Namespace
      </label>
      <div className="relative">
        <button
          onClick={() => setIsOpen((v) => !v)}
          className="w-full flex items-center justify-between gap-2 px-3 py-2 bg-white/80 dark:bg-slate-800/80 backdrop-blur-xl border border-stone-200 dark:border-slate-700 rounded-lg hover:border-stone-300 dark:hover:border-slate-600 transition-colors shadow-lg"
        >
          <span className="text-sm text-stone-700 dark:text-slate-200 truncate">
            {selectedNamespace || "All Namespaces"}
          </span>
          <ChevronDown className="w-4 h-4 text-stone-400 dark:text-slate-500 shrink-0" />
        </button>
        <AnimatePresence>
          {isOpen && (
            <motion.div
              initial={{ opacity: 0, y: -4 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -4 }}
              className="absolute z-30 top-full left-0 right-0 mt-1 py-1 bg-white dark:bg-slate-800 border border-stone-200 dark:border-slate-700 rounded-lg shadow-xl max-h-64 overflow-auto"
            >
              <button
                onClick={() => {
                  onNamespaceChange("");
                  setIsOpen(false);
                }}
                className={`w-full flex items-center justify-between px-3 py-2 text-sm hover:bg-stone-100 dark:hover:bg-slate-700 transition-colors ${
                  selectedNamespace === ""
                    ? "text-accent-600 dark:text-accent-400 bg-accent-50 dark:bg-accent-500/10"
                    : "text-stone-700 dark:text-slate-300"
                }`}
              >
                <span>All Namespaces</span>
                {selectedNamespace === "" && <Check className="w-4 h-4" />}
              </button>
              {namespaces.map((ns) => (
                <button
                  key={ns}
                  onClick={() => {
                    onNamespaceChange(ns);
                    setIsOpen(false);
                  }}
                  className={`w-full flex items-center justify-between px-3 py-2 text-sm hover:bg-stone-100 dark:hover:bg-slate-700 transition-colors ${
                    selectedNamespace === ns
                      ? "text-accent-600 dark:text-accent-400 bg-accent-50 dark:bg-accent-500/10"
                      : "text-stone-700 dark:text-slate-300"
                  }`}
                >
                  <span className="truncate">{ns}</span>
                  {selectedNamespace === ns && <Check className="w-4 h-4" />}
                </button>
              ))}
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </div>
  );
}

export default ClusterVisualizer;
