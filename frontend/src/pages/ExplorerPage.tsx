/**
 * ExplorerPage — Kubernetes resource browser.
 *
 * Extracted from App.tsx ExplorerView (~550 lines).
 * Provides: resource kind tabs, namespace filter, search, sort, quick actions.
 *
 * Quick actions per resource:
 *  - All kinds:    Inspect YAML, Copy YAML, Ask AI
 *  - Pods:         View Logs
 *  - Secrets:      View Secret Data
 *
 * Column toggles (keyboard shortcuts):
 *  r=resources, i=images, o=owner, p=probes, v=volumes, s=security, !=issues-only
 */

import { useState, useEffect, useRef } from "react";
import { motion, AnimatePresence } from "framer-motion";
import type { LucideIcon } from "lucide-react";
import {
  Search,
  RefreshCw,
  ChevronDown,
  ChevronUp,
  X,
  Copy,
  Check,
  Eye,
  EyeOff,
  ScrollText,
  Bot,
  ArrowUpDown,
  AlertCircle,
} from "lucide-react";
import {
  ListResources,
  GetResourceYAML,
  GetSecretData,
  GetNodeMetrics,
} from "../../wailsjs/go/main/App";
import { useAIStore } from "../stores/aiStore";
import type { ResourceInfo, NodeMetricsInfo, SelectedPod } from "../types/resources";

// ── Types ────────────────────────────────────────────────────────────────────

interface ExplorerPageProps {
  isConnected: boolean;
  onSelectPod: (pod: SelectedPod) => void;
  namespaces: string[];
  selectedKind: string;
  setSelectedKind: (kind: string) => void;
  namespaceFilter: string;
  setNamespaceFilter: (ns: string) => void;
  searchInput: string;
  setSearchInput: (search: string) => void;
  contextMenuOpen?: boolean;
  activeContext?: string;
}

type SortField =
  | "name"
  | "namespace"
  | "status"
  | "age"
  | "replicas"
  | "restarts"
  | "node"
  | "type"
  | "clusterIP"
  | "ports"
  | "capacity";

// ── Constants ────────────────────────────────────────────────────────────────

const RESOURCE_KINDS = [
  "pods",
  "deployments",
  "statefulsets",
  "daemonsets",
  "replicasets",
  "services",
  "ingresses",
  "persistentvolumeclaims",
  "configmaps",
  "secrets",
  "nodes",
  "namespaces",
];

const STATUS_COLORS: Record<string, string> = {
  running: "text-emerald-400",
  active: "text-emerald-400",
  ready: "text-emerald-400",
  pending: "text-yellow-400",
  waiting: "text-yellow-400",
  failed: "text-red-400",
  error: "text-red-400",
};

function getStatusColor(status: string): string {
  return STATUS_COLORS[status?.toLowerCase()] ?? "text-slate-400";
}

function parseAge(age: string): number {
  if (!age) return 0;
  const match = age.match(/(\d+)([dhms])/);
  if (!match) return 0;
  const [, val, unit] = match;
  const n = parseInt(val);
  return unit === "d" ? n * 86400 : unit === "h" ? n * 3600 : unit === "m" ? n * 60 : n;
}

// ── Component ─────────────────────────────────────────────────────────────────

export function ExplorerPage({
  isConnected,
  onSelectPod,
  namespaces,
  selectedKind,
  setSelectedKind,
  namespaceFilter,
  setNamespaceFilter,
  searchInput,
  setSearchInput,
  activeContext = "",
}: ExplorerPageProps) {
  const [resources, setResources] = useState<ResourceInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const fetchIdRef = useRef(0);

  // Modal state
  const [describeModal, setDescribeModal] = useState<{
    kind: string; namespace: string; name: string;
  } | null>(null);
  const [describeYaml, setDescribeYaml] = useState("");
  const [describeLoading, setDescribeLoading] = useState(false);
  const [copySuccess, setCopySuccess] = useState(false);
  const [secretDataModal, setSecretDataModal] = useState<{
    namespace: string; name: string;
  } | null>(null);
  const [secretData, setSecretData] = useState<Record<string, string>>({});
  const [secretDataLoading, setSecretDataLoading] = useState(false);
  const [revealedSecretKeys, setRevealedSecretKeys] = useState<Set<string>>(new Set());
  const [_nodeMetrics, setNodeMetrics] = useState<Record<string, NodeMetricsInfo>>({});

  // Column toggles
  const [showResources, setShowResources] = useState(false);
  const [showImages, setShowImages] = useState(false);
  const [showOwner, setShowOwner] = useState(false);
  const [showProbes, setShowProbes] = useState(false);
  const [showVolumes, setShowVolumes] = useState(false);
  const [showSecurity, setShowSecurity] = useState(false);
  const [showIssuesOnly, setShowIssuesOnly] = useState(false);
  const [showUtilization, _setShowUtilization] = useState(false);

  // UI state
  const [showKindDropdown, setShowKindDropdown] = useState(false);
  const [showNamespaceDropdown, setShowNamespaceDropdown] = useState(false);
  const [selectedIndex, setSelectedIndex] = useState(-1);
  const [sortField, setSortField] = useState<SortField>("name");
  const [sortDirection, setSortDirection] = useState<"asc" | "desc">("asc");
  const [resourceContextMenu, setResourceContextMenu] = useState<{
    x: number; y: number; resource: ResourceInfo;
  } | null>(null);

  const addToContext = useAIStore((state) => state.addToContext);

  // Close context menu on click outside
  useEffect(() => {
    const close = () => setResourceContextMenu(null);
    window.addEventListener("click", close);
    return () => window.removeEventListener("click", close);
  }, []);

  // Fetch resources
  useEffect(() => {
    if (!isConnected) { setResources([]); return; }
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(fetchResources, 300);
    return () => { if (debounceRef.current) clearTimeout(debounceRef.current); };
  }, [isConnected, selectedKind, namespaceFilter]);

  useEffect(() => {
    if (isConnected && selectedKind === "nodes" && showUtilization) fetchNodeMetrics();
  }, [isConnected, selectedKind, showUtilization]);

  const fetchResources = async () => {
    const id = ++fetchIdRef.current;
    setLoading(true);
    setError(null);
    try {
      const ns = ["namespaces", "nodes"].includes(selectedKind) ? "" : namespaceFilter;
      const result = await ListResources(selectedKind, ns);
      if (id !== fetchIdRef.current) return;
      setResources(result || []);
    } catch (err) {
      if (id !== fetchIdRef.current) return;
      setError(err instanceof Error ? err.message : "Failed to fetch resources");
      setResources([]);
    } finally {
      if (id === fetchIdRef.current) setLoading(false);
    }
  };

  const fetchNodeMetrics = async () => {
    try {
      const metrics = await GetNodeMetrics();
      const map: Record<string, NodeMetricsInfo> = {};
      for (const m of metrics || []) map[m.nodeName] = m;
      setNodeMetrics(map);
    } catch { /* non-blocking */ }
  };

  const handleDescribe = async (resource: ResourceInfo) => {
    setDescribeModal({ kind: selectedKind, namespace: resource.namespace, name: resource.name });
    setDescribeLoading(true);
    setDescribeYaml("");
    try {
      const yaml = await GetResourceYAML(selectedKind, resource.namespace, resource.name);
      setDescribeYaml(yaml);
    } catch (err) {
      setDescribeYaml(`Error: ${err instanceof Error ? err.message : "Failed to get resource"}`);
    } finally {
      setDescribeLoading(false);
    }
  };

  const handleViewSecretData = async (resource: ResourceInfo) => {
    setSecretDataModal({ namespace: resource.namespace, name: resource.name });
    setSecretDataLoading(true);
    setSecretData({});
    setRevealedSecretKeys(new Set());
    try {
      const data = await GetSecretData(resource.namespace, resource.name);
      setSecretData(data || {});
    } catch (err) {
      setSecretData({ error: err instanceof Error ? err.message : "Failed to get secret data" });
    } finally {
      setSecretDataLoading(false);
    }
  };

  const handleCopyYaml = async (resource: ResourceInfo) => {
    try {
      const yaml = await GetResourceYAML(selectedKind, resource.namespace, resource.name);
      await navigator.clipboard.writeText(yaml);
      setCopySuccess(true);
      setTimeout(() => setCopySuccess(false), 2000);
    } catch { /* silent */ }
  };

  const handleSort = (field: SortField) => {
    if (sortField === field) {
      setSortDirection((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortField(field);
      setSortDirection("asc");
    }
  };

  // Filter
  const HEALTHY = ["running", "active", "ready", "completed", "succeeded"];
  const filtered = resources.filter((r) => {
    const matchSearch = !searchInput || r.name.toLowerCase().includes(searchInput.toLowerCase());
    const hasIssue = !HEALTHY.includes(r.status?.toLowerCase() ?? "");
    return matchSearch && (!showIssuesOnly || hasIssue);
  });

  // Sort
  const sorted = [...filtered].sort((a, b) => {
    let cmp = 0;
    if (sortField === "name") cmp = a.name.localeCompare(b.name);
    else if (sortField === "namespace") cmp = (a.namespace ?? "").localeCompare(b.namespace ?? "");
    else if (sortField === "status") cmp = (a.status ?? "").localeCompare(b.status ?? "");
    else if (sortField === "age") cmp = parseAge(a.age) - parseAge(b.age);
    else if (sortField === "restarts") cmp = (a.restarts ?? 0) - (b.restarts ?? 0);
    return sortDirection === "asc" ? cmp : -cmp;
  });

  // Keyboard navigation
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement;
      if (target.tagName === "INPUT" || target.tagName === "TEXTAREA" || target.isContentEditable) return;

      if (showKindDropdown || showNamespaceDropdown) return;

      switch (e.key) {
        case "r": setShowResources((v) => !v); break;
        case "i": setShowImages((v) => !v); break;
        case "o": setShowOwner((v) => !v); break;
        case "p": setShowProbes((v) => !v); break;
        case "v": setShowVolumes((v) => !v); break;
        case "s": setShowSecurity((v) => !v); break;
        case "!": setShowIssuesOnly((v) => !v); break;
        case "ArrowDown":
          e.preventDefault();
          setSelectedIndex((i) => Math.min(i + 1, sorted.length - 1));
          break;
        case "ArrowUp":
          e.preventDefault();
          setSelectedIndex((i) => Math.max(i - 1, 0));
          break;
        case "Enter":
          if (selectedIndex >= 0 && sorted[selectedIndex]) {
            handleDescribe(sorted[selectedIndex]);
          }
          break;
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [showKindDropdown, showNamespaceDropdown, sorted, selectedIndex]);


  return (
    <div className="flex flex-col h-full gap-3">
      {/* Toolbar */}
      <div className="flex items-center gap-2 flex-wrap">
        {/* Kind selector */}
        <div className="relative">
          <button
            onClick={() => setShowKindDropdown(!showKindDropdown)}
            className="
              flex items-center gap-1.5 h-8 px-3 rounded-lg
              bg-slate-800/40 border border-slate-700/50
              text-sm text-slate-200 font-mono
              hover:bg-slate-700/50 transition-colors
              focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500/50
            "
            aria-haspopup="listbox"
            aria-expanded={showKindDropdown}
          >
            {selectedKind}
            <ChevronDown size={13} aria-hidden="true" />
          </button>
          <AnimatePresence>
            {showKindDropdown && (
              <motion.div
                initial={{ opacity: 0, y: -4 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: -4 }}
                transition={{ duration: 0.1 }}
                className="
                  absolute top-full mt-1 left-0 z-30
                  w-52 max-h-72 overflow-y-auto
                  bg-slate-900/95 backdrop-blur-xl
                  border border-slate-700/50 rounded-xl
                  shadow-2xl shadow-black/40 py-1
                "
                role="listbox"
                aria-label="Resource kind"
              >
                {RESOURCE_KINDS.map((kind) => (
                  <button
                    key={kind}
                    role="option"
                    aria-selected={selectedKind === kind}
                    onClick={() => { setSelectedKind(kind); setShowKindDropdown(false); setSelectedIndex(-1); }}
                    className={`
                      w-full text-left px-3 py-1.5 text-xs font-mono transition-colors
                      ${selectedKind === kind
                        ? "bg-accent-500/15 text-accent-400"
                        : "text-slate-300 hover:bg-slate-700/60"
                      }
                    `}
                  >
                    {kind}
                  </button>
                ))}
              </motion.div>
            )}
          </AnimatePresence>
        </div>

        {/* Namespace filter */}
        {!["namespaces", "nodes"].includes(selectedKind) && (
          <div className="relative">
            <button
              onClick={() => setShowNamespaceDropdown(!showNamespaceDropdown)}
              className="
                flex items-center gap-1.5 h-8 px-3 rounded-lg
                bg-slate-800/40 border border-slate-700/50
                text-xs text-slate-300
                hover:bg-slate-700/50 transition-colors
                focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500/50
              "
              aria-haspopup="listbox"
              aria-expanded={showNamespaceDropdown}
            >
              {namespaceFilter || "All namespaces"}
              <ChevronDown size={12} aria-hidden="true" />
            </button>
            <AnimatePresence>
              {showNamespaceDropdown && (
                <motion.div
                  initial={{ opacity: 0, y: -4 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -4 }}
                  transition={{ duration: 0.1 }}
                  className="
                    absolute top-full mt-1 left-0 z-30
                    w-52 max-h-60 overflow-y-auto
                    bg-slate-900/95 backdrop-blur-xl
                    border border-slate-700/50 rounded-xl
                    shadow-2xl shadow-black/40 py-1
                  "
                  role="listbox"
                  aria-label="Namespace filter"
                >
                  <button
                    role="option"
                    aria-selected={namespaceFilter === ""}
                    onClick={() => { setNamespaceFilter(""); setShowNamespaceDropdown(false); }}
                    className={`w-full text-left px-3 py-1.5 text-xs transition-colors ${namespaceFilter === "" ? "bg-accent-500/15 text-accent-400" : "text-slate-300 hover:bg-slate-700/60"}`}
                  >
                    All namespaces
                  </button>
                  {namespaces.map((ns) => (
                    <button
                      key={ns}
                      role="option"
                      aria-selected={namespaceFilter === ns}
                      onClick={() => { setNamespaceFilter(ns); setShowNamespaceDropdown(false); }}
                      className={`w-full text-left px-3 py-1.5 text-xs font-mono transition-colors ${namespaceFilter === ns ? "bg-accent-500/15 text-accent-400" : "text-slate-300 hover:bg-slate-700/60"}`}
                    >
                      {ns}
                    </button>
                  ))}
                </motion.div>
              )}
            </AnimatePresence>
          </div>
        )}

        {/* Search */}
        <div className="relative flex-1 min-w-[180px] max-w-xs">
          <Search
            size={13}
            className="absolute left-2.5 top-1/2 -translate-y-1/2 text-slate-500 pointer-events-none"
            aria-hidden="true"
          />
          <input
            type="search"
            placeholder="Search resources... (/)"
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            className="
              w-full h-8 pl-8 pr-3 rounded-lg
              bg-slate-800/40 border border-slate-700/50
              text-xs text-slate-200 placeholder-slate-500
              focus:outline-none focus:ring-2 focus:ring-accent-500/50
              transition-colors
            "
            aria-label="Search resources"
          />
          {searchInput && (
            <button
              onClick={() => setSearchInput("")}
              className="absolute right-2 top-1/2 -translate-y-1/2 text-slate-500 hover:text-slate-300"
              aria-label="Clear search"
            >
              <X size={12} aria-hidden="true" />
            </button>
          )}
        </div>

        {/* Column toggles */}
        <div className="flex items-center gap-1 ml-auto">
          {showIssuesOnly && (
            <span className="flex items-center gap-1 px-2 py-1 rounded-md bg-red-500/15 text-red-400 text-[10px] font-medium border border-red-500/20">
              <AlertCircle size={10} aria-hidden="true" />
              Issues only
              <button onClick={() => setShowIssuesOnly(false)} className="ml-0.5 hover:text-red-300" aria-label="Clear issues filter">
                <X size={9} aria-hidden="true" />
              </button>
            </span>
          )}

          <button
            onClick={fetchResources}
            disabled={loading}
            className="
              flex items-center gap-1 h-8 px-2.5 rounded-lg
              bg-slate-800/40 border border-slate-700/50
              text-xs text-slate-400 hover:text-slate-200
              hover:bg-slate-700/50 disabled:opacity-40
              transition-colors
              focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500/50
            "
            aria-label="Refresh resources"
          >
            <RefreshCw size={13} className={loading ? "animate-spin" : ""} aria-hidden="true" />
          </button>
        </div>
      </div>

      {/* Column toggle pills */}
      {["pods", "deployments", "statefulsets", "daemonsets"].includes(selectedKind) && (
        <div className="flex items-center gap-1.5 flex-wrap text-[10px]">
          <span className="text-slate-600 mr-1">Columns:</span>
          {[
            { key: "r", label: "Resources", active: showResources, toggle: () => setShowResources(!showResources) },
            { key: "i", label: "Images", active: showImages, toggle: () => setShowImages(!showImages) },
            { key: "o", label: "Owner", active: showOwner, toggle: () => setShowOwner(!showOwner) },
            ...(selectedKind === "pods" ? [
              { key: "p", label: "Probes", active: showProbes, toggle: () => setShowProbes(!showProbes) },
              { key: "v", label: "Volumes", active: showVolumes, toggle: () => setShowVolumes(!showVolumes) },
              { key: "s", label: "Security", active: showSecurity, toggle: () => setShowSecurity(!showSecurity) },
            ] : []),
            { key: "!", label: "Issues only", active: showIssuesOnly, toggle: () => setShowIssuesOnly(!showIssuesOnly) },
          ].map((t) => (
            <button
              key={t.key}
              onClick={t.toggle}
              className={`
                flex items-center gap-1 px-2 py-0.5 rounded-full border transition-colors
                focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent-500/50
                ${t.active
                  ? "bg-accent-500/15 border-accent-500/30 text-accent-400"
                  : "bg-slate-800/40 border-slate-700/40 text-slate-500 hover:text-slate-300"
                }
              `}
              aria-pressed={t.active}
            >
              <kbd className="font-mono">{t.key}</kbd>
              {t.label}
            </button>
          ))}
        </div>
      )}

      {/* Not connected state */}
      {!isConnected && (
        <div className="flex-1 flex items-center justify-center">
          <div className="text-center space-y-2">
            <p className="text-slate-400 text-sm">Not connected to a cluster</p>
            <p className="text-slate-600 text-xs">Select a cluster from the top bar to get started</p>
          </div>
        </div>
      )}

      {/* Error state */}
      {isConnected && error && (
        <div className="flex items-center gap-2 px-3 py-2.5 rounded-lg bg-red-500/10 border border-red-500/20 text-red-400 text-xs">
          <AlertCircle size={14} aria-hidden="true" />
          {error}
        </div>
      )}

      {/* Resource table */}
      {isConnected && !error && (
        <div className="flex-1 overflow-hidden rounded-xl border border-slate-700/40 bg-slate-800/20 min-h-0">
          {loading && resources.length === 0 ? (
            <div className="flex items-center justify-center h-full">
              <div className="flex items-center gap-2 text-slate-500 text-sm">
                <RefreshCw size={15} className="animate-spin" aria-hidden="true" />
                Loading {selectedKind}...
              </div>
            </div>
          ) : sorted.length === 0 ? (
            <div className="flex items-center justify-center h-full">
              <p className="text-slate-500 text-sm">
                {searchInput ? `No ${selectedKind} matching "${searchInput}"` : `No ${selectedKind} found`}
              </p>
            </div>
          ) : (
            <div className="overflow-auto h-full">
              <table className="w-full text-xs" role="grid" aria-label={`${selectedKind} resources`}>
                <thead className="sticky top-0 z-10 bg-slate-900/90 backdrop-blur-sm">
                  <tr className="border-b border-slate-700/40">
                    <SortableHeader field="name" current={sortField} dir={sortDirection} onSort={() => handleSort("name")}>
                      Name
                    </SortableHeader>
                    {!["namespaces", "nodes", "persistentvolumes"].includes(selectedKind) && (
                      <SortableHeader field="namespace" current={sortField} dir={sortDirection} onSort={() => handleSort("namespace")}>
                        Namespace
                      </SortableHeader>
                    )}
                    <SortableHeader field="status" current={sortField} dir={sortDirection} onSort={() => handleSort("status")}>
                      Status
                    </SortableHeader>
                    <SortableHeader field="age" current={sortField} dir={sortDirection} onSort={() => handleSort("age")}>
                      Age
                    </SortableHeader>
                    {selectedKind === "pods" && (
                      <SortableHeader field="restarts" current={sortField} dir={sortDirection} onSort={() => handleSort("restarts")}>
                        Restarts
                      </SortableHeader>
                    )}
                    {showResources && <th className="px-3 py-2 text-left text-slate-500 font-medium">CPU/Mem</th>}
                    {showImages && <th className="px-3 py-2 text-left text-slate-500 font-medium">Images</th>}
                    {showOwner && <th className="px-3 py-2 text-left text-slate-500 font-medium">Owner</th>}
                    {showSecurity && <th className="px-3 py-2 text-left text-slate-500 font-medium">Issues</th>}
                    <th className="px-3 py-2 w-20" />
                  </tr>
                </thead>
                <tbody>
                  {sorted.map((resource, idx) => {
                    const isSelected = idx === selectedIndex;
                    return (
                      <tr
                        key={`${resource.namespace}-${resource.name}`}
                        className={`
                          border-b border-slate-700/20 transition-colors cursor-pointer
                          ${isSelected
                            ? "bg-accent-500/10"
                            : "hover:bg-slate-700/20"
                          }
                        `}
                        onClick={() => setSelectedIndex(idx)}
                        onContextMenu={(e) => {
                          e.preventDefault();
                          setResourceContextMenu({ x: e.clientX, y: e.clientY, resource });
                        }}
                        role="row"
                        aria-selected={isSelected}
                      >
                        <td className="px-3 py-2 font-mono text-slate-200 max-w-[200px]">
                          <span className="truncate block">{resource.name}</span>
                        </td>
                        {!["namespaces", "nodes", "persistentvolumes"].includes(selectedKind) && (
                          <td className="px-3 py-2 text-slate-500 font-mono">{resource.namespace || "-"}</td>
                        )}
                        <td className={`px-3 py-2 font-medium ${getStatusColor(resource.status)}`}>
                          {resource.status}
                        </td>
                        <td className="px-3 py-2 text-slate-500">{resource.age}</td>
                        {selectedKind === "pods" && (
                          <td className={`px-3 py-2 ${(resource.restarts ?? 0) > 0 ? "text-amber-400" : "text-slate-500"}`}>
                            {resource.restarts ?? 0}
                          </td>
                        )}
                        {showResources && (
                          <td className="px-3 py-2 text-slate-500 text-[10px] font-mono">
                            <div>CPU: {resource.cpuRequest ?? "-"}/{resource.cpuLimit ?? "-"}</div>
                            <div>Mem: {resource.memRequest ?? "-"}/{resource.memLimit ?? "-"}</div>
                          </td>
                        )}
                        {showImages && (
                          <td className="px-3 py-2 text-slate-500 text-[10px] max-w-[200px]">
                            {resource.images?.map((img) => (
                              <div key={img} className="truncate font-mono">{img.split("/").pop()}</div>
                            ))}
                          </td>
                        )}
                        {showOwner && (
                          <td className="px-3 py-2 text-slate-500 text-[10px] font-mono">
                            {resource.ownerKind ? `${resource.ownerKind}/${resource.ownerName}` : "-"}
                          </td>
                        )}
                        {showSecurity && (
                          <td className="px-3 py-2">
                            {resource.securityIssues?.length ? (
                              <span className="text-amber-400 text-[10px]">
                                {resource.securityIssues.length} issue{resource.securityIssues.length !== 1 ? "s" : ""}
                              </span>
                            ) : (
                              <span className="text-slate-600 text-[10px]">-</span>
                            )}
                          </td>
                        )}
                        <td className="px-2 py-2">
                          <div className="flex items-center gap-1 justify-end">
                            {selectedKind === "pods" && (
                              <ActionButton
                                icon={ScrollText}
                                label={`View logs for ${resource.name}`}
                                onClick={(e) => {
                                  e.stopPropagation();
                                  onSelectPod({ name: resource.name, namespace: resource.namespace, kind: "Pod" });
                                }}
                              />
                            )}
                            {selectedKind === "secrets" && (
                              <ActionButton
                                icon={Eye}
                                label={`View secret data for ${resource.name}`}
                                onClick={(e) => {
                                  e.stopPropagation();
                                  handleViewSecretData(resource);
                                }}
                              />
                            )}
                            <ActionButton
                              icon={Search}
                              label={`Inspect ${resource.name}`}
                              onClick={(e) => {
                                e.stopPropagation();
                                handleDescribe(resource);
                              }}
                            />
                            <ActionButton
                              icon={copySuccess ? Check : Copy}
                              label={`Copy YAML for ${resource.name}`}
                              onClick={(e) => {
                                e.stopPropagation();
                                handleCopyYaml(resource);
                              }}
                            />
                            <ActionButton
                              icon={Bot}
                              label={`Ask AI about ${resource.name}`}
                              onClick={(e) => {
                                e.stopPropagation();
                                addToContext({
                                  id: `${activeContext}_${selectedKind}_${resource.namespace ?? "default"}_${resource.name}_${Date.now()}`,
                                  type: selectedKind.replace(/s$/, "") as any,
                                  name: resource.name,
                                  namespace: resource.namespace,
                                  cluster: activeContext,
                                  addedAt: new Date(),
                                });
                              }}
                            />
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}

      {/* Resource count */}
      {isConnected && !error && sorted.length > 0 && (
        <p className="text-[11px] text-slate-500 text-right">
          {sorted.length} of {resources.length} {selectedKind}
          {searchInput ? ` matching "${searchInput}"` : ""}
        </p>
      )}

      {/* Describe modal */}
      <AnimatePresence>
        {describeModal && (
          <>
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              className="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm"
              onClick={() => setDescribeModal(null)}
              aria-hidden="true"
            />
            <motion.div
              initial={{ opacity: 0, scale: 0.97 }}
              animate={{ opacity: 1, scale: 1 }}
              exit={{ opacity: 0, scale: 0.97 }}
              transition={{ duration: 0.18 }}
              className="fixed inset-4 md:inset-8 lg:inset-16 z-50 flex flex-col"
              role="dialog"
              aria-modal="true"
              aria-label={`YAML: ${describeModal.name}`}
            >
              <div className="flex-1 flex flex-col bg-slate-900/98 backdrop-blur-xl border border-slate-700/50 rounded-2xl shadow-2xl overflow-hidden">
                <div className="flex items-center gap-3 px-4 py-3 border-b border-slate-700/40 flex-shrink-0">
                  <span className="text-sm font-mono font-semibold text-slate-200">{describeModal.name}</span>
                  <span className="text-xs text-slate-500">{describeModal.kind}/{describeModal.namespace}</span>
                  <div className="ml-auto flex items-center gap-2">
                    <button
                      onClick={() => navigator.clipboard.writeText(describeYaml)}
                      className="flex items-center gap-1 px-2 py-1 rounded-lg text-xs text-slate-400 hover:text-slate-200 hover:bg-slate-700/60 transition-colors"
                      aria-label="Copy YAML"
                    >
                      <Copy size={12} aria-hidden="true" />
                      Copy
                    </button>
                    <button
                      onClick={() => setDescribeModal(null)}
                      className="p-1.5 rounded-lg text-slate-500 hover:text-slate-300 hover:bg-slate-700/60 transition-colors"
                      aria-label="Close"
                    >
                      <X size={15} aria-hidden="true" />
                    </button>
                  </div>
                </div>
                <div className="flex-1 overflow-auto p-4">
                  {describeLoading ? (
                    <div className="flex items-center justify-center h-full">
                      <RefreshCw size={20} className="animate-spin text-slate-500" aria-hidden="true" />
                    </div>
                  ) : (
                    <pre className="text-xs font-mono text-slate-300 leading-relaxed whitespace-pre-wrap break-all">
                      {describeYaml}
                    </pre>
                  )}
                </div>
              </div>
            </motion.div>
          </>
        )}
      </AnimatePresence>

      {/* Secret data modal */}
      <AnimatePresence>
        {secretDataModal && (
          <>
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              className="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm"
              onClick={() => setSecretDataModal(null)}
              aria-hidden="true"
            />
            <motion.div
              initial={{ opacity: 0, scale: 0.97 }}
              animate={{ opacity: 1, scale: 1 }}
              exit={{ opacity: 0, scale: 0.97 }}
              transition={{ duration: 0.18 }}
              className="fixed inset-4 md:inset-[20%] z-50 flex flex-col"
              role="dialog"
              aria-modal="true"
              aria-label={`Secret: ${secretDataModal.name}`}
            >
              <div className="flex flex-col bg-slate-900/98 backdrop-blur-xl border border-amber-500/30 rounded-2xl shadow-2xl overflow-hidden">
                <div className="flex items-center gap-3 px-4 py-3 border-b border-slate-700/40">
                  <span className="text-sm font-semibold text-amber-400">Secret Data</span>
                  <span className="text-xs font-mono text-slate-400">{secretDataModal.name}</span>
                  <button
                    onClick={() => setSecretDataModal(null)}
                    className="ml-auto p-1.5 rounded-lg text-slate-500 hover:text-slate-300 hover:bg-slate-700/60 transition-colors"
                    aria-label="Close"
                  >
                    <X size={15} aria-hidden="true" />
                  </button>
                </div>
                <div className="overflow-auto max-h-96 p-4">
                  {secretDataLoading ? (
                    <div className="flex items-center justify-center py-8">
                      <RefreshCw size={18} className="animate-spin text-slate-500" aria-hidden="true" />
                    </div>
                  ) : (
                    <div className="space-y-2">
                      {Object.entries(secretData).map(([key, value]) => {
                        const revealed = revealedSecretKeys.has(key);
                        return (
                          <div key={key} className="flex items-center gap-2 p-2 rounded-lg bg-slate-800/40 border border-slate-700/30">
                            <span className="text-xs font-mono text-slate-400 w-32 flex-shrink-0 truncate">{key}</span>
                            <span className={`flex-1 text-xs font-mono truncate ${revealed ? "text-slate-200" : "text-transparent bg-slate-700 rounded select-none"}`}>
                              {revealed ? value : "••••••••••••"}
                            </span>
                            <button
                              onClick={() => setRevealedSecretKeys((prev) => {
                                const next = new Set(prev);
                                if (revealed) { next.delete(key); } else { next.add(key); }
                                return next;
                              })}
                              className="flex-shrink-0 p-1 rounded text-slate-500 hover:text-slate-300 transition-colors"
                              aria-label={revealed ? `Hide ${key}` : `Reveal ${key}`}
                            >
                              {revealed ? <EyeOff size={12} aria-hidden="true" /> : <Eye size={12} aria-hidden="true" />}
                            </button>
                          </div>
                        );
                      })}
                    </div>
                  )}
                </div>
              </div>
            </motion.div>
          </>
        )}
      </AnimatePresence>

      {/* Context menu */}
      <AnimatePresence>
        {resourceContextMenu && (
          <motion.div
            initial={{ opacity: 0, scale: 0.96 }}
            animate={{ opacity: 1, scale: 1 }}
            exit={{ opacity: 0, scale: 0.96 }}
            transition={{ duration: 0.1 }}
            style={{ left: resourceContextMenu.x, top: resourceContextMenu.y }}
            className="fixed z-50 w-48 bg-slate-900/95 backdrop-blur-xl border border-slate-700/50 rounded-xl shadow-2xl py-1 overflow-hidden"
            role="menu"
            onClick={(e) => e.stopPropagation()}
          >
            {[
              { icon: Search, label: "Inspect YAML", action: () => handleDescribe(resourceContextMenu.resource) },
              { icon: Copy, label: "Copy YAML", action: () => handleCopyYaml(resourceContextMenu.resource) },
              { icon: Bot, label: "Ask AI", action: () => addToContext({ id: `${activeContext}_${selectedKind}_${resourceContextMenu.resource.namespace ?? "default"}_${resourceContextMenu.resource.name}_${Date.now()}`, type: selectedKind.replace(/s$/, "") as any, name: resourceContextMenu.resource.name, namespace: resourceContextMenu.resource.namespace, cluster: activeContext, addedAt: new Date() }) },
              ...(selectedKind === "pods" ? [{ icon: ScrollText, label: "View Logs", action: () => onSelectPod({ name: resourceContextMenu.resource.name, namespace: resourceContextMenu.resource.namespace, kind: "Pod" }) }] : []),
            ].map((item) => {
              const Icon = item.icon;
              return (
                <button
                  key={item.label}
                  onClick={() => { item.action(); setResourceContextMenu(null); }}
                  className="w-full flex items-center gap-2 px-3 py-1.5 text-xs text-slate-300 hover:bg-slate-700/60 transition-colors"
                  role="menuitem"
                >
                  <Icon size={12} aria-hidden="true" />
                  {item.label}
                </button>
              );
            })}
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

// ── Sub-components ────────────────────────────────────────────────────────────

function SortableHeader({
  field,
  current,
  dir,
  onSort,
  children,
}: {
  field: string;
  current: string;
  dir: "asc" | "desc";
  onSort: () => void;
  children: React.ReactNode;
}) {
  const isActive = field === current;
  return (
    <th className="px-3 py-2 text-left">
      <button
        onClick={onSort}
        className={`
          flex items-center gap-1 text-xs font-medium transition-colors
          focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent-500/50 rounded
          ${isActive ? "text-accent-400" : "text-slate-500 hover:text-slate-300"}
        `}
        aria-sort={isActive ? (dir === "asc" ? "ascending" : "descending") : "none"}
      >
        {children}
        {isActive ? (
          dir === "asc" ? <ChevronUp size={12} aria-hidden="true" /> : <ChevronDown size={12} aria-hidden="true" />
        ) : (
          <ArrowUpDown size={11} className="opacity-40" aria-hidden="true" />
        )}
      </button>
    </th>
  );
}

function ActionButton({
  icon: Icon,
  label,
  onClick,
}: {
  icon: LucideIcon;
  label: string;
  onClick: (e: React.MouseEvent) => void;
}) {
  return (
    <button
      onClick={onClick}
      title={label}
      aria-label={label}
      className="
        p-1 rounded-md
        text-slate-600 hover:text-slate-300
        hover:bg-slate-700/60
        transition-colors
        focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent-500/50
      "
    >
      <Icon size={12} aria-hidden="true" />
    </button>
  );
}
