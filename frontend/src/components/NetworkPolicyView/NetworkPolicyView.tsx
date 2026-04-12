import { useState, useCallback, useEffect } from "react";
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  useNodesState,
  useEdgesState,
  type Node,
  type Edge,
  MarkerType,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { ShieldAlert, ShieldCheck, RefreshCw, Loader2, Info } from "lucide-react";
import { AnalyzeNetworkPolicies } from "../../../wailsjs/go/main/App";

interface NetworkNode {
  id: string;
  name: string;
  namespace: string;
  kind: "Pod" | "Service" | "External";
  labels: Record<string, string>;
}

interface NetworkEdge {
  id: string;
  source: string;
  target: string;
  allowed: boolean;
  direction: string;
  policyName: string;
  ports: string;
}

interface NetworkGraph {
  nodes: NetworkNode[];
  edges: NetworkEdge[];
  hasPolicies: boolean;
  warning?: string;
}

function NodeShape({ data }: { data: { label: string; kind: string } }) {
  const kindColor =
    data.kind === "Pod"
      ? "bg-blue-500/20 border-blue-500/50 text-blue-300"
      : data.kind === "Service"
      ? "bg-green-500/20 border-green-500/50 text-green-300"
      : "bg-stone-500/20 border-stone-500/50 text-stone-300";
  return (
    <div
      className={`px-3 py-2 rounded-lg border text-xs font-mono max-w-[140px] truncate ${kindColor}`}
      title={data.label}
    >
      <div className="text-[10px] opacity-60 mb-0.5">{data.kind}</div>
      <div className="truncate">{data.label}</div>
    </div>
  );
}

const nodeTypes = { netnode: NodeShape };

export function NetworkPolicyView({
  activeCluster,
  namespaces,
}: {
  activeCluster: string;
  namespaces: string[];
}) {
  const [namespace, setNamespace] = useState(namespaces[0] ?? "default");
  const [graph, setGraph] = useState<NetworkGraph | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([]);

  const loadGraph = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const g = await AnalyzeNetworkPolicies(activeCluster, namespace);
      setGraph(g as NetworkGraph);
      buildFlow(g as NetworkGraph);
    } catch (e: any) {
      setError(e?.message ?? String(e));
    } finally {
      setLoading(false);
    }
  }, [activeCluster, namespace]);

  useEffect(() => {
    loadGraph();
  }, [loadGraph]);

  function buildFlow(g: NetworkGraph) {
    const cols = Math.ceil(Math.sqrt(g.nodes.length)) || 1;
    const flowNodes: Node[] = g.nodes.map((n, i) => ({
      id: n.id,
      type: "netnode",
      position: { x: (i % cols) * 180, y: Math.floor(i / cols) * 100 },
      data: { label: n.name, kind: n.kind },
    }));

    const flowEdges: Edge[] = g.edges
      .filter((e) => e.source !== "*" && e.target !== "*")
      .map((e) => ({
        id: e.id,
        source: e.source,
        target: e.target,
        animated: e.allowed,
        label: e.ports !== "all" ? e.ports : undefined,
        style: { stroke: e.allowed ? "#22c55e" : "#ef4444", strokeWidth: 1.5 },
        markerEnd: { type: MarkerType.ArrowClosed, color: e.allowed ? "#22c55e" : "#ef4444" },
        title: e.policyName,
      }));

    setNodes(flowNodes);
    setEdges(flowEdges);
  }

  return (
    <div className="flex flex-col h-full bg-stone-50 dark:bg-slate-900">
      {/* Header */}
      <div className="flex items-center gap-3 px-4 py-3 border-b border-stone-200 dark:border-slate-700/50">
        <ShieldCheck className="w-5 h-5 text-green-400" />
        <h2 className="text-sm font-semibold text-stone-800 dark:text-slate-100">
          Network Policy Visualizer
        </h2>
        <select
          value={namespace}
          onChange={(e) => setNamespace(e.target.value)}
          className="ml-auto text-xs bg-white dark:bg-slate-800 border border-stone-200 dark:border-slate-700 rounded px-2 py-1 text-stone-700 dark:text-slate-200"
        >
          {namespaces.map((ns) => (
            <option key={ns} value={ns}>
              {ns}
            </option>
          ))}
        </select>
        <button
          onClick={loadGraph}
          disabled={loading}
          className="p-1.5 rounded hover:bg-stone-100 dark:hover:bg-slate-700 text-stone-500 dark:text-slate-400 transition-colors"
        >
          {loading ? (
            <Loader2 className="w-4 h-4 animate-spin" />
          ) : (
            <RefreshCw className="w-4 h-4" />
          )}
        </button>
      </div>

      {/* Status strip */}
      {graph && (
        <div className="flex items-center gap-2 px-4 py-2 text-xs border-b border-stone-200 dark:border-slate-700/50">
          {graph.hasPolicies ? (
            <>
              <ShieldAlert className="w-3.5 h-3.5 text-yellow-400" />
              <span className="text-stone-600 dark:text-slate-300">
                {graph.edges.length} allowed connections from NetworkPolicies
              </span>
            </>
          ) : (
            <>
              <Info className="w-3.5 h-3.5 text-blue-400" />
              <span className="text-stone-500 dark:text-slate-400">
                No NetworkPolicies — all pods can communicate freely
              </span>
            </>
          )}
          {graph.warning && (
            <span className="ml-auto text-yellow-500">{graph.warning}</span>
          )}
        </div>
      )}

      {/* Graph */}
      <div className="flex-1 relative">
        {error ? (
          <div className="absolute inset-0 flex items-center justify-center text-red-400 text-sm">
            {error}
          </div>
        ) : loading && !graph ? (
          <div className="absolute inset-0 flex items-center justify-center">
            <Loader2 className="w-6 h-6 animate-spin text-stone-400" />
          </div>
        ) : (
          <ReactFlow
            nodes={nodes}
            edges={edges}
            onNodesChange={onNodesChange}
            onEdgesChange={onEdgesChange}
            nodeTypes={nodeTypes}
            fitView
            className="dark:bg-slate-900"
          >
            <Background />
            <Controls />
            <MiniMap
              nodeColor={(n: Node) =>
                (n.data as any)?.kind === "Pod"
                  ? "#3b82f6"
                  : (n.data as any)?.kind === "Service"
                  ? "#22c55e"
                  : "#6b7280"
              }
            />
          </ReactFlow>
        )}
      </div>

      {/* Legend */}
      <div className="flex items-center gap-4 px-4 py-2 border-t border-stone-200 dark:border-slate-700/50 text-xs text-stone-500 dark:text-slate-400">
        <span className="flex items-center gap-1">
          <span className="w-3 h-0.5 bg-green-500 inline-block" /> Allowed
        </span>
        <span className="flex items-center gap-1">
          <span className="w-3 h-0.5 bg-red-500 inline-block" /> Blocked
        </span>
        <span className="flex items-center gap-1">
          <span className="w-3 h-3 rounded-full bg-blue-500/30 border border-blue-500 inline-block" /> Pod
        </span>
        <span className="flex items-center gap-1">
          <span className="w-3 h-3 rounded-full bg-green-500/30 border border-green-500 inline-block" /> Service
        </span>
      </div>
    </div>
  );
}
