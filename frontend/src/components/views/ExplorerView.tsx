import { useState, useEffect, useRef } from "react";
import { motion, AnimatePresence } from "framer-motion";
import {
  Search,
  X,
  ChevronDown,
  ChevronUp,
  Eye,
  EyeOff,
  Check,
  AlertCircle,
  ArrowUpDown,
  Sparkles,
  FileText,
  Bot,
  Copy,
  ScrollText,
  Terminal,
} from "lucide-react";
import {
  ListResources,
  GetResourceYAML,
  GetSecretData,
  GetNodeMetrics,
  StartTerminal,
  CloseTerminal,
} from "../../../wailsjs/go/main/App";
import type { ResourceInfo, NodeMetricsInfo, SelectedPod } from "../../types/resources";
import { AnalysisModal } from "../AnalysisModal";
import { ErrorBoundary } from "../ErrorBoundary";
import { TerminalDrawer } from "../Terminal/TerminalDrawer";
import { useAIStore } from "../../stores/aiStore";

type SortField =
  | "name"
  | "namespace"
  | "status"
  | "age"
  | "replicas"
  | "ready"
  | "restarts"
  | "node"
  | "qos"
  | "type"
  | "clusterIP"
  | "ports"
  | "capacity"
  | "access"
  | "storageClass"
  | "resources"
  | "images"
  | "owner"
  | "probes"
  | "volumes"
  | "security"
  | "roles"
  | "cpuAlloc"
  | "memAlloc"
  | "conditions"
  | "version";

export function ExplorerView({
  isConnected,
  onSelectPod,
  namespaces,
  selectedKind,
  setSelectedKind,
  namespaceFilter,
  setNamespaceFilter,
  searchInput,
  setSearchInput,
  contextMenuOpen,
  activeContext,
}: {
  isConnected: boolean;
  onSelectPod: (pod: SelectedPod) => void;
  namespaces: string[];
  selectedKind: string;
  setSelectedKind: (kind: string) => void;
  namespaceFilter: string;
  setNamespaceFilter: (ns: string) => void;
  searchInput: string;
  setSearchInput: (search: string) => void;
  contextMenuOpen: boolean;
  activeContext: string;
}) {
  const [resources, setResources] = useState<ResourceInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const fetchIdRef = useRef(0);
  const [describeModal, setDescribeModal] = useState<{
    kind: string;
    namespace: string;
    name: string;
  } | null>(null);
  const [describeYaml, setDescribeYaml] = useState<string>("");
  const [describeLoading, setDescribeLoading] = useState(false);
  const [copySuccess, setCopySuccess] = useState(false);
  const [secretDataModal, setSecretDataModal] = useState<{
    namespace: string;
    name: string;
  } | null>(null);
  const [secretData, setSecretData] = useState<Record<string, string>>({});
  const [secretDataLoading, setSecretDataLoading] = useState(false);
  const [revealedSecretKeys, setRevealedSecretKeys] = useState<Set<string>>(new Set());
  const [selectedIndex, setSelectedIndex] = useState<number>(-1);
  const [showTerminal, setShowTerminal] = useState(false);
  const [terminalSessionId, setTerminalSessionId] = useState<string | null>(null);
  const [resourceContextMenu, setResourceContextMenu] = useState<{
    x: number;
    y: number;
    resource: ResourceInfo;
  } | null>(null);

  useEffect(() => {
    const handleClick = () => setResourceContextMenu(null);
    window.addEventListener("click", handleClick);
    return () => window.removeEventListener("click", handleClick);
  }, []);

  const [showKindDropdown, setShowKindDropdown] = useState(false);
  const [showNamespaceDropdown, setShowNamespaceDropdown] = useState(false);
  const [showSortDropdown, setShowSortDropdown] = useState(false);
  const [dropdownIndex, setDropdownIndex] = useState<number>(-1);
  const tableRef = useRef<HTMLDivElement>(null);
  const namespaceDropdownRef = useRef<HTMLDivElement>(null);

  const [analysisResource, setAnalysisResource] = useState<{
    kind: string;
    namespace: string;
    name: string;
  } | null>(null);

  // Toggleable info columns
  const [showResources, setShowResources] = useState(false);
  const [showImages, setShowImages] = useState(false);
  const [showOwner, setShowOwner] = useState(false);
  const [showProbes, setShowProbes] = useState(false);
  const [showVolumes, setShowVolumes] = useState(false);
  const [showSecurity, setShowSecurity] = useState(false);
  const [showSelectors, setShowSelectors] = useState(false);
  const [showDataKeys, setShowDataKeys] = useState(false);
  const [showTLS, setShowTLS] = useState(false);
  const [showBackends, setShowBackends] = useState(false);
  const [showUtilization, setShowUtilization] = useState(false);
  const [showTaints, setShowTaints] = useState(false);
  const [showNodeInfo, setShowNodeInfo] = useState(false);
  const [showIssuesOnly, setShowIssuesOnly] = useState(false);
  const [nodeMetrics, setNodeMetrics] = useState<Record<string, NodeMetricsInfo>>({});

  const [sortField, setSortField] = useState<SortField>("name");
  const [sortDirection, setSortDirection] = useState<"asc" | "desc">("asc");

  const handleSort = (field: SortField) => {
    if (sortField === field) {
      setSortDirection((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortField(field);
      setSortDirection("asc");
    }
  };

  const SortIndicator = ({ field }: { field: string }) => {
    if (sortField !== field) {
      return <ArrowUpDown size={14} className="text-slate-600" />;
    }
    return sortDirection === "asc" ? (
      <ChevronUp size={14} className="text-accent-400" />
    ) : (
      <ChevronDown size={14} className="text-accent-400" />
    );
  };

  const resourceKinds = [
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

  const togglesForKind: Record<string, string[]> = {
    pods: ["resources", "images", "owner", "probes", "volumes", "security"],
    deployments: ["images", "owner"],
    statefulsets: ["images", "owner"],
    daemonsets: ["images", "owner"],
    replicasets: ["images", "owner"],
    services: ["selectors", "owner"],
    ingresses: ["tls", "backends", "owner"],
    persistentvolumeclaims: ["owner"],
    configmaps: ["dataKeys", "owner"],
    secrets: ["dataKeys", "owner"],
    nodes: ["utilization", "taints", "nodeInfo"],
    namespaces: [],
  };

  const currentToggles = togglesForKind[selectedKind] || [];

  useEffect(() => {
    if (isConnected) {
      if (debounceRef.current) clearTimeout(debounceRef.current);
      debounceRef.current = setTimeout(() => {
        fetchResources();
      }, 300);
    } else {
      setResources([]);
    }
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [isConnected, selectedKind, namespaceFilter]);

  useEffect(() => {
    if (isConnected && selectedKind === "nodes" && showUtilization) {
      fetchNodeMetrics();
    }
  }, [isConnected, selectedKind, showUtilization]);

  const fetchNodeMetrics = async () => {
    try {
      const metrics = await GetNodeMetrics();
      const metricsMap: Record<string, NodeMetricsInfo> = {};
      for (const m of metrics || []) {
        metricsMap[m.nodeName] = m;
      }
      setNodeMetrics(metricsMap);
    } catch (err) {
      console.error("Failed to fetch node metrics:", err);
    }
  };

  const fetchResources = async () => {
    const currentFetchId = ++fetchIdRef.current;
    setLoading(true);
    setError(null);
    try {
      const ns =
        selectedKind === "namespaces" || selectedKind === "nodes"
          ? ""
          : namespaceFilter;
      const result = await ListResources(selectedKind, ns);
      if (currentFetchId !== fetchIdRef.current) return;
      setResources(result || []);
    } catch (err) {
      if (currentFetchId !== fetchIdRef.current) return;
      setError(err instanceof Error ? err.message : "Failed to fetch resources");
      setResources([]);
    } finally {
      if (currentFetchId === fetchIdRef.current) {
        setLoading(false);
      }
    }
  };

  const handleDescribe = async (resource: ResourceInfo) => {
    setDescribeModal({
      kind: selectedKind,
      namespace: resource.namespace,
      name: resource.name,
    });
    setDescribeLoading(true);
    setDescribeYaml("");
    try {
      const yaml = await GetResourceYAML(selectedKind, resource.namespace, resource.name);
      setDescribeYaml(yaml);
    } catch (err) {
      setDescribeYaml(
        `Error: ${err instanceof Error ? err.message : "Failed to get resource"}`
      );
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
      setSecretData({
        error: err instanceof Error ? err.message : "Failed to get secret data",
      });
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
    } catch (err) {
      console.error("Failed to copy YAML:", err);
    }
  };

  const getStatusColor = (status: string) => {
    switch (status?.toLowerCase()) {
      case "running":
      case "active":
      case "ready":
        return "text-emerald-400";
      case "pending":
      case "waiting":
        return "text-yellow-400";
      case "failed":
      case "error":
        return "text-red-400";
      default:
        return "text-slate-400";
    }
  };

  const healthyStatuses = ["running", "active", "ready", "completed", "succeeded"];
  const filteredResources = resources.filter((r) => {
    const matchesSearch =
      !searchInput || r.name.toLowerCase().includes(searchInput.toLowerCase());
    const hasIssue = !healthyStatuses.includes(r.status?.toLowerCase() || "");
    const matchesIssues = !showIssuesOnly || hasIssue;
    return matchesSearch && matchesIssues;
  });

  const parseAge = (age: string): number => {
    if (!age) return 0;
    const match = age.match(/(\d+)([dhms])/);
    if (!match) return 0;
    const value = parseInt(match[1]);
    const unit = match[2];
    switch (unit) {
      case "d": return value * 86400;
      case "h": return value * 3600;
      case "m": return value * 60;
      case "s": return value;
      default: return 0;
    }
  };

  const parseReplicas = (rep: string | undefined): number => {
    if (!rep) return 0;
    const parts = rep.split("/");
    return parseInt(parts[0]) || 0;
  };

  const sortedResources = [...filteredResources].sort((a, b) => {
    let comparison = 0;
    switch (sortField) {
      case "name": comparison = a.name.localeCompare(b.name); break;
      case "namespace": comparison = (a.namespace || "").localeCompare(b.namespace || ""); break;
      case "status": comparison = (a.status || "").localeCompare(b.status || ""); break;
      case "age": comparison = parseAge(a.age) - parseAge(b.age); break;
      case "replicas": comparison = parseReplicas(a.replicas) - parseReplicas(b.replicas); break;
      case "ready": comparison = parseReplicas(a.readyContainers) - parseReplicas(b.readyContainers); break;
      case "restarts": comparison = (a.restarts || 0) - (b.restarts || 0); break;
      case "node": comparison = (a.node || "").localeCompare(b.node || ""); break;
      case "qos": comparison = (a.qosClass || "").localeCompare(b.qosClass || ""); break;
      case "type": comparison = (a.serviceType || "").localeCompare(b.serviceType || ""); break;
      case "clusterIP": comparison = (a.clusterIP || "").localeCompare(b.clusterIP || ""); break;
      case "ports": comparison = (a.ports || "").localeCompare(b.ports || ""); break;
      case "capacity": comparison = (a.capacity || "").localeCompare(b.capacity || ""); break;
      case "access": comparison = (a.accessModes || "").localeCompare(b.accessModes || ""); break;
      case "storageClass": comparison = (a.storageClass || "").localeCompare(b.storageClass || ""); break;
      case "resources": comparison = (a.cpuRequest || "").localeCompare(b.cpuRequest || ""); break;
      case "images": comparison = (a.images?.join(",") || "").localeCompare(b.images?.join(",") || ""); break;
      case "owner":
        comparison = `${a.ownerKind || ""}/${a.ownerName || ""}`.localeCompare(
          `${b.ownerKind || ""}/${b.ownerName || ""}`
        );
        break;
      case "probes": {
        const aProbes = (a.hasLiveness ? 1 : 0) + (a.hasReadiness ? 1 : 0);
        const bProbes = (b.hasLiveness ? 1 : 0) + (b.hasReadiness ? 1 : 0);
        comparison = aProbes - bProbes;
        break;
      }
      case "volumes": comparison = (a.volumes?.length || 0) - (b.volumes?.length || 0); break;
      case "security": comparison = (a.securityIssues?.length || 0) - (b.securityIssues?.length || 0); break;
    }
    return sortDirection === "asc" ? comparison : -comparison;
  });

  useEffect(() => {
    setSelectedIndex(-1);
  }, [sortedResources.length, selectedKind, namespaceFilter]);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement;
      if (
        target.tagName === "INPUT" ||
        target.tagName === "TEXTAREA" ||
        target.isContentEditable
      ) {
        return;
      }

      if (e.key === "Escape") {
        if (showKindDropdown) { setShowKindDropdown(false); return; }
        if (showNamespaceDropdown) { setShowNamespaceDropdown(false); setDropdownIndex(-1); return; }
        if (describeModal) { setDescribeModal(null); return; }
      }

      if (describeModal || contextMenuOpen) return;

      if (showNamespaceDropdown) {
        const nsOptions = ["", ...namespaces];
        switch (e.key) {
          case "ArrowDown":
          case "j":
            e.preventDefault();
            setDropdownIndex((prev) => prev < nsOptions.length - 1 ? prev + 1 : prev);
            break;
          case "ArrowUp":
          case "k":
            e.preventDefault();
            setDropdownIndex((prev) => (prev > 0 ? prev - 1 : 0));
            break;
          case "Enter":
            e.preventDefault();
            if (dropdownIndex >= 0 && dropdownIndex < nsOptions.length) {
              setNamespaceFilter(nsOptions[dropdownIndex]);
            }
            setShowNamespaceDropdown(false);
            setDropdownIndex(-1);
            break;
        }
        return;
      }

      if (showKindDropdown) return;

      const PAGE_SIZE = 10;

      switch (e.key) {
        case "ArrowDown":
        case "j":
          e.preventDefault();
          if (e.shiftKey) {
            setSelectedIndex((prev) => Math.min(prev + PAGE_SIZE, sortedResources.length - 1));
          } else {
            setSelectedIndex((prev) => prev < sortedResources.length - 1 ? prev + 1 : prev);
          }
          break;
        case "ArrowUp":
        case "k":
          e.preventDefault();
          if (e.shiftKey) {
            setSelectedIndex((prev) => Math.max(prev - PAGE_SIZE, 0));
          } else {
            setSelectedIndex((prev) => (prev > 0 ? prev - 1 : 0));
          }
          break;
        case "PageDown":
          e.preventDefault();
          setSelectedIndex((prev) => Math.min(prev + PAGE_SIZE, sortedResources.length - 1));
          break;
        case "PageUp":
          e.preventDefault();
          setSelectedIndex((prev) => Math.max(prev - PAGE_SIZE, 0));
          break;
        case "ArrowLeft": {
          e.preventDefault();
          const currentKindIdx = resourceKinds.indexOf(selectedKind);
          const prevKindIdx = currentKindIdx > 0 ? currentKindIdx - 1 : resourceKinds.length - 1;
          setSelectedKind(resourceKinds[prevKindIdx]);
          break;
        }
        case "ArrowRight": {
          e.preventDefault();
          const kindIdx = resourceKinds.indexOf(selectedKind);
          const nextKindIdx = kindIdx < resourceKinds.length - 1 ? kindIdx + 1 : 0;
          setSelectedKind(resourceKinds[nextKindIdx]);
          break;
        }
        case "l":
          e.preventDefault();
          if (selectedIndex >= 0 && selectedIndex < sortedResources.length) {
            const resource = sortedResources[selectedIndex];
            if (["pods", "deployments", "statefulsets", "daemonsets"].includes(selectedKind)) {
              onSelectPod({ name: resource.name, namespace: resource.namespace, kind: resource.kind });
            }
          }
          break;
        case "d":
          e.preventDefault();
          if (selectedIndex >= 0 && selectedIndex < sortedResources.length) {
            handleDescribe(sortedResources[selectedIndex]);
          }
          break;
        case "n":
          e.preventDefault();
          if (selectedKind !== "namespaces") {
            const newState = !showNamespaceDropdown;
            setShowNamespaceDropdown(newState);
            if (newState) {
              const nsOptions = ["", ...namespaces];
              const currentIdx = nsOptions.indexOf(namespaceFilter);
              setDropdownIndex(currentIdx >= 0 ? currentIdx : 0);
            } else {
              setDropdownIndex(-1);
            }
          }
          break;
        case "Enter":
          e.preventDefault();
          if (selectedIndex >= 0 && selectedIndex < sortedResources.length) {
            handleDescribe(sortedResources[selectedIndex]);
          }
          break;
        case "r":
          e.preventDefault();
          setShowResources((prev) => !prev);
          break;
        case "i":
          e.preventDefault();
          setShowImages((prev) => !prev);
          break;
        case "o":
          e.preventDefault();
          setShowOwner((prev) => !prev);
          break;
        case "p":
          e.preventDefault();
          setShowProbes((prev) => !prev);
          break;
        case "v":
          e.preventDefault();
          setShowVolumes((prev) => !prev);
          break;
        case "t":
          e.preventDefault();
          if (selectedKind === "pods" && selectedIndex >= 0 && sortedResources[selectedIndex]) {
            const resource = sortedResources[selectedIndex];
            if (resource.status !== "Running") {
              console.warn(`Cannot open terminal: Pod is ${resource.status}`);
              break;
            }
            const newSessionId = `explore-${resource.namespace}-${resource.name}-${Date.now()}`;
            setTerminalSessionId(newSessionId);
            setShowTerminal(true);
            StartTerminal(newSessionId, "kubectl", [
              "exec", "-i", "-t", "-n", resource.namespace, resource.name, "--", "/bin/sh",
            ]).catch((err: Error) => {
              console.error("Failed to start terminal:", err);
              setShowTerminal(false);
              setTerminalSessionId(null);
            });
          }
          break;
        case "s":
          e.preventDefault();
          setShowSecurity((prev) => !prev);
          break;
        case "e":
          e.preventDefault();
          setShowSelectors((prev) => !prev);
          break;
        case "x":
          e.preventDefault();
          setShowDataKeys((prev) => !prev);
          break;
        case "g":
          e.preventDefault();
          setShowTLS((prev) => !prev);
          break;
        case "b":
          e.preventDefault();
          setShowBackends((prev) => !prev);
          break;
        case "c":
          e.preventDefault();
          if (selectedIndex >= 0 && selectedIndex < sortedResources.length) {
            handleCopyYaml(sortedResources[selectedIndex]);
          }
          break;
        case "h":
          e.preventDefault();
          if (selectedKind === "secrets" && selectedIndex >= 0 && selectedIndex < sortedResources.length) {
            handleViewSecretData(sortedResources[selectedIndex]);
          }
          break;
      }

      if (e.shiftKey) {
        switch (e.key) {
          case "N": e.preventDefault(); handleSort("name"); break;
          case "M": e.preventDefault(); handleSort("namespace"); break;
          case "S": e.preventDefault(); handleSort("status"); break;
          case "A": e.preventDefault(); handleSort("age"); break;
          case "R":
            e.preventDefault();
            if (selectedKind === "pods") { handleSort("ready"); } else { handleSort("replicas"); }
            break;
          case "X": e.preventDefault(); handleSort("restarts"); break;
          case "O": e.preventDefault(); handleSort("node"); break;
          case "Q": e.preventDefault(); handleSort("qos"); break;
          case "T": e.preventDefault(); handleSort("type"); break;
          case "C": e.preventDefault(); handleSort("clusterIP"); break;
          case "P": e.preventDefault(); handleSort("ports"); break;
          case "K": e.preventDefault(); handleSort("capacity"); break;
          case "E": e.preventDefault(); handleSort("access"); break;
          case "G": e.preventDefault(); handleSort("storageClass"); break;
        }
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [
    sortedResources,
    selectedIndex,
    selectedKind,
    showKindDropdown,
    showNamespaceDropdown,
    describeModal,
    dropdownIndex,
    namespaces,
    namespaceFilter,
    sortField,
  ]);

  useEffect(() => {
    if (selectedIndex >= 0 && tableRef.current) {
      const row = tableRef.current.querySelector(`tr[data-index="${selectedIndex}"]`);
      row?.scrollIntoView({ block: "nearest", behavior: "smooth" });
    }
  }, [selectedIndex]);

  useEffect(() => {
    if (dropdownIndex >= 0 && namespaceDropdownRef.current) {
      const container = namespaceDropdownRef.current;
      const item = container.children[dropdownIndex] as HTMLElement;
      if (item) {
        const containerRect = container.getBoundingClientRect();
        const itemRect = item.getBoundingClientRect();
        if (itemRect.bottom > containerRect.bottom) {
          container.scrollTop += itemRect.bottom - containerRect.bottom;
        } else if (itemRect.top < containerRect.top) {
          container.scrollTop -= containerRect.top - itemRect.top;
        }
      }
    }
  }, [dropdownIndex]);

  return (
    <div className="h-full flex flex-col">
      {/* Resource type selector and Namespace filter */}
      <div className="flex gap-4 mb-4 items-center">
        {/* Resource Kind Dropdown */}
        <div className="relative">
          <button
            onClick={() => setShowKindDropdown(!showKindDropdown)}
            className="flex items-center gap-2 px-3 py-1.5 bg-white dark:bg-slate-800 border border-stone-200 dark:border-slate-700 rounded-lg text-sm hover:border-stone-300 dark:hover:border-slate-600 transition-colors min-w-[160px] text-stone-700 dark:text-slate-200 shadow-sm dark:shadow-none"
          >
            <span className="capitalize flex-1 text-left">{selectedKind}</span>
            <kbd className="text-xs bg-stone-100 dark:bg-slate-700/50 px-1 py-0.5 rounded text-stone-500 dark:text-slate-400">
              &#8592;&#8594;
            </kbd>
            <ChevronDown
              size={14}
              className={`text-slate-400 dark:text-slate-500 transition-transform ${showKindDropdown ? "rotate-180" : ""}`}
            />
          </button>
          {showKindDropdown && (
            <div className="absolute z-50 mt-1 w-full bg-white dark:bg-slate-800 border border-stone-200 dark:border-slate-700 rounded-lg shadow-xl max-h-64 overflow-y-auto">
              {resourceKinds.map((kind) => (
                <button
                  key={kind}
                  onClick={() => { setSelectedKind(kind); setShowKindDropdown(false); }}
                  className={`w-full px-3 py-2 text-sm text-left capitalize transition-colors ${
                    selectedKind === kind
                      ? "bg-accent-500/10 text-accent-600 dark:text-accent-400"
                      : "text-stone-600 dark:text-slate-300 hover:bg-stone-100 dark:hover:bg-slate-700"
                  }`}
                >
                  {kind}
                </button>
              ))}
            </div>
          )}
        </div>

        {/* Namespace Dropdown */}
        {selectedKind !== "namespaces" && selectedKind !== "nodes" && (
          <div className="relative">
            <button
              onClick={() => {
                const newState = !showNamespaceDropdown;
                setShowNamespaceDropdown(newState);
                if (newState) {
                  const nsOptions = ["", ...namespaces];
                  const currentIdx = nsOptions.indexOf(namespaceFilter);
                  setDropdownIndex(currentIdx >= 0 ? currentIdx : 0);
                }
              }}
              className="flex items-center gap-2 px-3 py-1.5 bg-white/80 dark:bg-slate-800 border border-stone-200 dark:border-slate-700/50 rounded-md text-sm text-stone-700 dark:text-slate-200 hover:bg-white dark:hover:bg-slate-700 transition-colors min-w-[180px] shadow-sm"
            >
              <span className="flex-1 text-left">{namespaceFilter || "All namespaces"}</span>
              <kbd className="text-xs bg-stone-100 dark:bg-slate-600 px-1 py-0.5 rounded text-stone-500 dark:text-slate-400">
                n
              </kbd>
              <ChevronDown
                size={14}
                className={`text-slate-400 dark:text-slate-500 transition-transform ${showNamespaceDropdown ? "rotate-180" : ""}`}
              />
            </button>
            {showNamespaceDropdown && (
              <div
                ref={namespaceDropdownRef}
                className="absolute z-50 mt-1 w-full bg-white dark:bg-slate-800 border border-stone-200 dark:border-slate-700 rounded-md shadow-lg max-h-64 overflow-y-auto"
              >
                <button
                  onClick={() => { setNamespaceFilter(""); setShowNamespaceDropdown(false); setDropdownIndex(-1); }}
                  className={`w-full px-3 py-2 text-sm text-left transition-colors ${
                    dropdownIndex === 0
                      ? "bg-accent-500 text-white"
                      : !namespaceFilter
                      ? "bg-accent-500/20 text-accent-600 dark:text-accent-400"
                      : "text-stone-600 dark:text-slate-200 hover:bg-stone-100 dark:hover:bg-slate-700"
                  }`}
                >
                  All namespaces
                </button>
                {namespaces.map((ns, idx) => (
                  <button
                    key={ns}
                    onClick={() => { setNamespaceFilter(ns); setShowNamespaceDropdown(false); setDropdownIndex(-1); }}
                    className={`w-full px-3 py-2 text-sm text-left transition-colors ${
                      dropdownIndex === idx + 1
                        ? "bg-accent-500 text-white"
                        : namespaceFilter === ns
                        ? "bg-accent-500/20 text-accent-600 dark:text-accent-400"
                        : "text-stone-600 dark:text-slate-200 hover:bg-stone-100 dark:hover:bg-slate-700"
                    }`}
                  >
                    {ns}
                  </button>
                ))}
              </div>
            )}
          </div>
        )}

        <div className="relative flex-1 max-w-xs">
          <Search size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 dark:text-slate-500" />
          <input
            type="text"
            placeholder="Search by name..."
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            className="w-full bg-white dark:bg-slate-800 border border-stone-200 dark:border-slate-700 rounded-lg pl-9 pr-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-accent-500/50 text-stone-900 dark:text-slate-100 placeholder:text-stone-400 dark:placeholder:text-slate-500 shadow-sm dark:shadow-none transition-colors duration-200"
          />
        </div>

        <button
          onClick={() => setShowIssuesOnly((p) => !p)}
          className={`p-1.5 rounded-lg transition-colors ${
            showIssuesOnly
              ? "bg-amber-500/20 text-amber-500 border border-amber-500/50"
              : "bg-white dark:bg-slate-800 text-stone-400 dark:text-slate-500 border border-stone-200 dark:border-slate-700 hover:border-stone-300 dark:hover:border-slate-600"
          }`}
          title={showIssuesOnly ? "Showing issues only" : "Show resources with issues"}
        >
          <AlertCircle size={16} />
        </button>

        {copySuccess && (
          <span className="text-emerald-500 dark:text-emerald-400 text-sm flex items-center gap-1">
            <Check size={14} /> Copied!
          </span>
        )}

        <div className="flex items-center gap-2">
          <span className="text-xs text-slate-500">Sort:</span>
          <div className="relative">
            <button
              onClick={() => setShowSortDropdown(!showSortDropdown)}
              className="px-2 py-1 text-xs bg-white dark:bg-slate-800 border border-stone-200 dark:border-slate-700 rounded hover:border-stone-300 dark:hover:border-slate-600 transition-colors text-stone-700 dark:text-slate-200 shadow-sm dark:shadow-none flex items-center gap-1"
            >
              <span className="capitalize">{sortField}</span>
              <ChevronDown
                size={12}
                className={`text-slate-400 dark:text-slate-500 transition-transform ${showSortDropdown ? "rotate-180" : ""}`}
              />
            </button>
            {showSortDropdown && (
              <div className="absolute right-0 z-50 mt-1 w-32 bg-white dark:bg-slate-800 border border-stone-200 dark:border-slate-700 rounded shadow-lg overflow-hidden">
                {(["name", "namespace", "status", "age"] as const).map((field) => (
                  <button
                    key={field}
                    onClick={() => { setSortField(field); setShowSortDropdown(false); }}
                    className={`w-full px-3 py-1.5 text-xs text-left capitalize transition-colors ${
                      sortField === field
                        ? "bg-accent-500/10 text-accent-600 dark:text-accent-400"
                        : "text-stone-600 dark:text-slate-300 hover:bg-stone-100 dark:hover:bg-slate-700"
                    }`}
                  >
                    {field}
                  </button>
                ))}
              </div>
            )}
          </div>
          <button
            onClick={() => setSortDirection((d) => (d === "asc" ? "desc" : "asc"))}
            className="px-2 py-1 text-xs bg-white dark:bg-slate-800 border border-stone-200 dark:border-slate-700 rounded hover:border-stone-300 dark:hover:border-slate-600 transition-colors text-stone-600 dark:text-slate-300 shadow-sm dark:shadow-none"
            title={`Sort ${sortDirection === "asc" ? "ascending" : "descending"}`}
          >
            {sortDirection === "asc" ? "\u2191" : "\u2193"}
          </button>
        </div>
      </div>

      {/* Toggle buttons for extra columns */}
      {currentToggles.length > 0 && (
        <div className="flex gap-2 mb-2 flex-wrap">
          {currentToggles.includes("resources") && (
            <button
              onClick={() => setShowResources((p) => !p)}
              className={`px-2 py-1 text-xs rounded transition-colors flex items-center gap-1 shadow-sm dark:shadow-none ${
                showResources
                  ? "bg-accent-500/20 text-accent-600 dark:text-accent-400 border border-accent-500/50"
                  : "bg-white dark:bg-slate-800 text-stone-600 dark:text-slate-400 border border-stone-200 dark:border-slate-700 hover:border-stone-300 dark:hover:border-slate-600"
              }`}
              title="Toggle CPU/Memory (r)"
            >
              <kbd className="text-[10px] bg-stone-100 dark:bg-slate-700/50 px-1 rounded text-stone-500 dark:text-slate-400">r</kbd>
              Resources
            </button>
          )}
          {currentToggles.includes("images") && (
            <button
              onClick={() => setShowImages((p) => !p)}
              className={`px-2 py-1 text-xs rounded transition-colors flex items-center gap-1 shadow-sm dark:shadow-none ${
                showImages
                  ? "bg-accent-500/20 text-accent-600 dark:text-accent-400 border border-accent-500/50"
                  : "bg-white dark:bg-slate-800 text-stone-600 dark:text-slate-400 border border-stone-200 dark:border-slate-700 hover:border-stone-300 dark:hover:border-slate-600"
              }`}
              title="Toggle Images (i)"
            >
              <kbd className="text-[10px] bg-stone-100 dark:bg-slate-700/50 px-1 rounded text-stone-500 dark:text-slate-400">i</kbd>
              Images
            </button>
          )}
          {currentToggles.includes("owner") && (
            <button
              onClick={() => setShowOwner((p) => !p)}
              className={`px-2 py-1 text-xs rounded transition-colors flex items-center gap-1 shadow-sm dark:shadow-none ${
                showOwner
                  ? "bg-accent-500/20 text-accent-600 dark:text-accent-400 border border-accent-500/50"
                  : "bg-white dark:bg-slate-800 text-stone-600 dark:text-slate-400 border border-stone-200 dark:border-slate-700 hover:border-stone-300 dark:hover:border-slate-600"
              }`}
              title="Toggle Owner (o)"
            >
              <kbd className="text-[10px] bg-stone-100 dark:bg-slate-700/50 px-1 rounded text-stone-500 dark:text-slate-400">o</kbd>
              Owner
            </button>
          )}
          {currentToggles.includes("probes") && (
            <button
              onClick={() => setShowProbes((p) => !p)}
              className={`px-2 py-1 text-xs rounded transition-colors flex items-center gap-1 shadow-sm dark:shadow-none ${
                showProbes
                  ? "bg-accent-500/20 text-accent-600 dark:text-accent-400 border border-accent-500/50"
                  : "bg-white dark:bg-slate-800 text-stone-600 dark:text-slate-400 border border-stone-200 dark:border-slate-700 hover:border-stone-300 dark:hover:border-slate-600"
              }`}
              title="Toggle Probes (p)"
            >
              <kbd className="text-[10px] bg-stone-100 dark:bg-slate-700/50 px-1 rounded text-stone-500 dark:text-slate-400">p</kbd>
              Probes
            </button>
          )}
          {currentToggles.includes("volumes") && (
            <button
              onClick={() => setShowVolumes((p) => !p)}
              className={`px-2 py-1 text-xs rounded transition-colors flex items-center gap-1 shadow-sm dark:shadow-none ${
                showVolumes
                  ? "bg-accent-500/20 text-accent-600 dark:text-accent-400 border border-accent-500/50"
                  : "bg-white dark:bg-slate-800 text-stone-600 dark:text-slate-400 border border-stone-200 dark:border-slate-700 hover:border-stone-300 dark:hover:border-slate-600"
              }`}
              title="Toggle Volumes (v)"
            >
              <kbd className="text-[10px] bg-stone-100 dark:bg-slate-700/50 px-1 rounded text-stone-500 dark:text-slate-400">v</kbd>
              Volumes
            </button>
          )}
          {currentToggles.includes("security") && (
            <button
              onClick={() => setShowSecurity((p) => !p)}
              className={`px-2 py-1 text-xs rounded transition-colors flex items-center gap-1 ${
                showSecurity
                  ? "bg-red-500/30 text-red-400 border border-red-500/50"
                  : "bg-white dark:bg-slate-800 text-slate-500 dark:text-slate-400 border border-slate-200 dark:border-slate-700 hover:border-slate-300 dark:hover:border-slate-600"
              }`}
              title="Toggle Security Issues (s)"
            >
              <kbd className="text-[10px] bg-stone-100 dark:bg-slate-700/50 px-1 rounded text-stone-500 dark:text-slate-400">s</kbd>
              Security
            </button>
          )}
          {currentToggles.includes("selectors") && (
            <button
              onClick={() => setShowSelectors((p) => !p)}
              className={`px-2 py-1 text-xs rounded transition-colors flex items-center gap-1 ${
                showSelectors
                  ? "bg-purple-500/30 text-purple-400 border border-purple-500/50"
                  : "bg-white dark:bg-slate-800 text-slate-500 dark:text-slate-400 border border-slate-200 dark:border-slate-700 hover:border-slate-300 dark:hover:border-slate-600"
              }`}
              title="Toggle Selectors (e)"
            >
              <kbd className="text-[10px] bg-stone-100 dark:bg-slate-700/50 px-1 rounded text-stone-500 dark:text-slate-400">e</kbd>
              Selectors
            </button>
          )}
          {currentToggles.includes("dataKeys") && (
            <button
              onClick={() => setShowDataKeys((p) => !p)}
              className={`px-2 py-1 text-xs rounded transition-colors flex items-center gap-1 ${
                showDataKeys
                  ? "bg-cyan-500/30 text-cyan-400 border border-cyan-500/50"
                  : "bg-white dark:bg-slate-800 text-slate-500 dark:text-slate-400 border border-slate-200 dark:border-slate-700 hover:border-slate-300 dark:hover:border-slate-600"
              }`}
              title="Toggle Data Keys (x)"
            >
              <kbd className="text-[10px] bg-stone-100 dark:bg-slate-700/50 px-1 rounded text-stone-500 dark:text-slate-400">x</kbd>
              Data Keys
            </button>
          )}
          {currentToggles.includes("tls") && (
            <button
              onClick={() => setShowTLS((p) => !p)}
              className={`px-2 py-1 text-xs rounded transition-colors flex items-center gap-1 ${
                showTLS
                  ? "bg-emerald-500/30 text-emerald-400 border border-emerald-500/50"
                  : "bg-white dark:bg-slate-800 text-slate-500 dark:text-slate-400 border border-slate-200 dark:border-slate-700 hover:border-slate-300 dark:hover:border-slate-600"
              }`}
              title="Toggle TLS Info (g)"
            >
              <kbd className="text-[10px] bg-stone-100 dark:bg-slate-700/50 px-1 rounded text-stone-500 dark:text-slate-400">g</kbd>
              TLS
            </button>
          )}
          {currentToggles.includes("backends") && (
            <button
              onClick={() => setShowBackends((p) => !p)}
              className={`px-2 py-1 text-xs rounded transition-colors flex items-center gap-1 ${
                showBackends
                  ? "bg-orange-500/30 text-orange-400 border border-orange-500/50"
                  : "bg-white dark:bg-slate-800 text-slate-500 dark:text-slate-400 border border-slate-200 dark:border-slate-700 hover:border-slate-300 dark:hover:border-slate-600"
              }`}
              title="Toggle Backends (b)"
            >
              <kbd className="text-[10px] bg-stone-100 dark:bg-slate-700/50 px-1 rounded text-stone-500 dark:text-slate-400">b</kbd>
              Backends
            </button>
          )}
          {currentToggles.includes("utilization") && (
            <button
              onClick={() => setShowUtilization((p) => !p)}
              className={`px-2 py-1 text-xs rounded transition-colors flex items-center gap-1 ${
                showUtilization
                  ? "bg-emerald-500/30 text-emerald-400 border border-emerald-500/50"
                  : "bg-white dark:bg-slate-800 text-slate-500 dark:text-slate-400 border border-slate-200 dark:border-slate-700 hover:border-slate-300 dark:hover:border-slate-600"
              }`}
              title="Toggle Utilization (u)"
            >
              <kbd className="text-[10px] bg-stone-100 dark:bg-slate-700/50 px-1 rounded text-stone-500 dark:text-slate-400">u</kbd>
              Utilization
            </button>
          )}
          {currentToggles.includes("taints") && (
            <button
              onClick={() => setShowTaints((p) => !p)}
              className={`px-2 py-1 text-xs rounded transition-colors flex items-center gap-1 ${
                showTaints
                  ? "bg-yellow-500/30 text-yellow-400 border border-yellow-500/50"
                  : "bg-white dark:bg-slate-800 text-slate-500 dark:text-slate-400 border border-slate-200 dark:border-slate-700 hover:border-slate-300 dark:hover:border-slate-600"
              }`}
              title="Toggle Taints (a)"
            >
              <kbd className="text-[10px] bg-stone-100 dark:bg-slate-700/50 px-1 rounded text-stone-500 dark:text-slate-400">a</kbd>
              Taints
            </button>
          )}
          {currentToggles.includes("nodeInfo") && (
            <button
              onClick={() => setShowNodeInfo((p) => !p)}
              className={`px-2 py-1 text-xs rounded transition-colors flex items-center gap-1 ${
                showNodeInfo
                  ? "bg-purple-500/30 text-purple-400 border border-purple-500/50"
                  : "bg-white dark:bg-slate-800 text-slate-500 dark:text-slate-400 border border-slate-200 dark:border-slate-700 hover:border-slate-300 dark:hover:border-slate-600"
              }`}
              title="Toggle Node Info (f)"
            >
              <kbd className="text-[10px] bg-stone-100 dark:bg-slate-700/50 px-1 rounded text-stone-500 dark:text-slate-400">f</kbd>
              Node Info
            </button>
          )}
        </div>
      )}

      {/* Resources table */}
      <div className="flex-1 window-glass rounded-xl overflow-hidden shadow-sm dark:shadow-none transition-colors duration-200">
        {!isConnected ? (
          <p className="text-stone-500 dark:text-slate-400 text-center py-12">
            Connect to a cluster to explore resources
          </p>
        ) : loading ? (
          <div className="flex items-center justify-center py-12">
            <div className="w-6 h-6 border-2 border-accent-500 dark:border-accent-400 border-t-transparent rounded-full animate-spin" />
          </div>
        ) : error ? (
          <p className="text-red-500 dark:text-red-400 text-center py-12">{error}</p>
        ) : sortedResources.length === 0 ? (
          <p className="text-stone-500 dark:text-slate-400 text-center py-12">
            No {selectedKind} found
          </p>
        ) : (
          <div ref={tableRef} className="overflow-auto h-full">
            <table className="w-full text-sm">
              <thead className="bg-stone-50/80 dark:bg-slate-900/80 backdrop-blur-sm sticky top-0 z-10 border-b border-stone-200 dark:border-slate-700">
                <tr className="text-left text-stone-500 dark:text-slate-400">
                  <th className="px-4 py-3 font-medium cursor-pointer hover:bg-stone-100 dark:hover:bg-slate-800 transition-colors select-none" onClick={() => handleSort("name")}>
                    <span className="flex items-center gap-1">Name <SortIndicator field="name" /></span>
                  </th>
                  <th className="px-4 py-3 font-medium cursor-pointer hover:bg-stone-100 dark:hover:bg-slate-800 transition-colors select-none" onClick={() => handleSort("namespace")}>
                    <span className="flex items-center gap-1">Namespace <SortIndicator field="namespace" /></span>
                  </th>
                  <th className="px-4 py-3 font-medium cursor-pointer hover:bg-stone-100 dark:hover:bg-slate-800 transition-colors select-none" onClick={() => handleSort("status")}>
                    <span className="flex items-center gap-1">Status <SortIndicator field="status" /></span>
                  </th>
                  {(selectedKind === "deployments" || selectedKind === "statefulsets" || selectedKind === "daemonsets") && (
                    <th className="px-4 py-3 font-medium cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors select-none" onClick={() => handleSort("replicas")}>
                      <span className="flex items-center gap-1">Replicas <SortIndicator field="replicas" /></span>
                    </th>
                  )}
                  {selectedKind === "pods" && (
                    <>
                      <th className="px-4 py-3 font-medium cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors select-none" onClick={() => handleSort("ready")}>
                        <span className="flex items-center gap-1">Ready <SortIndicator field="ready" /></span>
                      </th>
                      <th className="px-4 py-3 font-medium cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors select-none" onClick={() => handleSort("restarts")}>
                        <span className="flex items-center gap-1">Restarts <SortIndicator field="restarts" /></span>
                      </th>
                      <th className="px-4 py-3 font-medium cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors select-none" onClick={() => handleSort("node")}>
                        <span className="flex items-center gap-1">Node <SortIndicator field="node" /></span>
                      </th>
                      <th className="px-4 py-3 font-medium cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors select-none" onClick={() => handleSort("qos")}>
                        <span className="flex items-center gap-1">QoS <SortIndicator field="qos" /></span>
                      </th>
                    </>
                  )}
                  {selectedKind === "services" && (
                    <>
                      <th className="px-4 py-3 font-medium cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors select-none" onClick={() => handleSort("type")}>
                        <span className="flex items-center gap-1">Type <SortIndicator field="type" /></span>
                      </th>
                      <th className="px-4 py-3 font-medium cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors select-none" onClick={() => handleSort("clusterIP")}>
                        <span className="flex items-center gap-1">Cluster IP <SortIndicator field="clusterIP" /></span>
                      </th>
                      <th className="px-4 py-3 font-medium cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors select-none" onClick={() => handleSort("ports")}>
                        <span className="flex items-center gap-1">Ports <SortIndicator field="ports" /></span>
                      </th>
                    </>
                  )}
                  {selectedKind === "persistentvolumeclaims" && (
                    <>
                      <th className="px-4 py-3 font-medium cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors select-none" onClick={() => handleSort("capacity")}>
                        <span className="flex items-center gap-1">Capacity <SortIndicator field="capacity" /></span>
                      </th>
                      <th className="px-4 py-3 font-medium cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors select-none" onClick={() => handleSort("access")}>
                        <span className="flex items-center gap-1">Access <SortIndicator field="access" /></span>
                      </th>
                      <th className="px-4 py-3 font-medium cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors select-none" onClick={() => handleSort("storageClass")}>
                        <span className="flex items-center gap-1">Storage Class <SortIndicator field="storageClass" /></span>
                      </th>
                    </>
                  )}
                  {selectedKind === "ingresses" && (
                    <>
                      <th className="px-4 py-3 font-medium">Class</th>
                      <th className="px-4 py-3 font-medium">Hosts</th>
                      <th className="px-4 py-3 font-medium">Paths</th>
                    </>
                  )}
                  {selectedKind === "nodes" && (
                    <>
                      <th className="px-4 py-3 font-medium cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors select-none" onClick={() => handleSort("roles")}>
                        <span className="flex items-center gap-1">Roles <SortIndicator field="roles" /></span>
                      </th>
                      <th className="px-4 py-3 font-medium cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors select-none" onClick={() => handleSort("cpuAlloc")}>
                        <span className="flex items-center gap-1">CPU <SortIndicator field="cpuAlloc" /></span>
                      </th>
                      <th className="px-4 py-3 font-medium cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors select-none" onClick={() => handleSort("memAlloc")}>
                        <span className="flex items-center gap-1">Memory <SortIndicator field="memAlloc" /></span>
                      </th>
                      <th className="px-4 py-3 font-medium cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors select-none" onClick={() => handleSort("conditions")}>
                        <span className="flex items-center gap-1">Conditions <SortIndicator field="conditions" /></span>
                      </th>
                      <th className="px-4 py-3 font-medium cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors select-none" onClick={() => handleSort("version")}>
                        <span className="flex items-center gap-1">Version <SortIndicator field="version" /></span>
                      </th>
                    </>
                  )}
                  {showResources && currentToggles.includes("resources") && (
                    <th className="px-4 py-3 font-medium text-yellow-400 cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors select-none" onClick={() => handleSort("resources")}>
                      <span className="flex items-center gap-1">Resources <SortIndicator field="resources" /></span>
                    </th>
                  )}
                  {showImages && currentToggles.includes("images") && (
                    <th className="px-4 py-3 font-medium text-cyan-400 cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors select-none" onClick={() => handleSort("images")}>
                      <span className="flex items-center gap-1">Images <SortIndicator field="images" /></span>
                    </th>
                  )}
                  {showOwner && currentToggles.includes("owner") && (
                    <th className="px-4 py-3 font-medium text-orange-400 cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors select-none" onClick={() => handleSort("owner")}>
                      <span className="flex items-center gap-1">Owner <SortIndicator field="owner" /></span>
                    </th>
                  )}
                  {showProbes && currentToggles.includes("probes") && (
                    <th className="px-4 py-3 font-medium text-emerald-400 cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors select-none" onClick={() => handleSort("probes")}>
                      <span className="flex items-center gap-1">Probes <SortIndicator field="probes" /></span>
                    </th>
                  )}
                  {showVolumes && currentToggles.includes("volumes") && (
                    <th className="px-4 py-3 font-medium text-blue-400 cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors select-none" onClick={() => handleSort("volumes")}>
                      <span className="flex items-center gap-1">Volumes <SortIndicator field="volumes" /></span>
                    </th>
                  )}
                  {showSecurity && currentToggles.includes("security") && (
                    <th className="px-4 py-3 font-medium text-red-400 cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors select-none" onClick={() => handleSort("security")}>
                      <span className="flex items-center gap-1">Security <SortIndicator field="security" /></span>
                    </th>
                  )}
                  {showSelectors && currentToggles.includes("selectors") && (
                    <th className="px-4 py-3 font-medium text-purple-400">Selectors</th>
                  )}
                  {showDataKeys && currentToggles.includes("dataKeys") && (
                    <th className="px-4 py-3 font-medium text-cyan-400">Data Keys</th>
                  )}
                  {showTLS && currentToggles.includes("tls") && (
                    <th className="px-4 py-3 font-medium text-emerald-400">TLS</th>
                  )}
                  {showBackends && currentToggles.includes("backends") && (
                    <th className="px-4 py-3 font-medium text-orange-400">Backends</th>
                  )}
                  {showUtilization && currentToggles.includes("utilization") && (
                    <th className="px-4 py-3 font-medium text-emerald-400">Pod Resources</th>
                  )}
                  {showTaints && currentToggles.includes("taints") && (
                    <th className="px-4 py-3 font-medium text-yellow-400">Taints</th>
                  )}
                  {showNodeInfo && currentToggles.includes("nodeInfo") && (
                    <th className="px-4 py-3 font-medium text-purple-400">Node Info</th>
                  )}
                  <th className="px-4 py-3 font-medium cursor-pointer hover:bg-slate-800 transition-colors select-none" onClick={() => handleSort("age")}>
                    <span className="flex items-center gap-1">Age <SortIndicator field="age" /></span>
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-stone-200 dark:divide-slate-700/50">
                {sortedResources.map((resource, idx) => (
                  <tr
                    key={`${resource.namespace}-${resource.name}-${idx}`}
                    data-index={idx}
                    onClick={() => setSelectedIndex(idx)}
                    onContextMenu={(e) => {
                      e.preventDefault();
                      setResourceContextMenu({ x: e.clientX, y: e.clientY, resource });
                    }}
                    className={`cursor-pointer transition-colors group ${
                      selectedIndex === idx
                        ? "bg-accent-500/10 ring-1 ring-accent-500/50"
                        : "hover:bg-stone-50/50 dark:hover:bg-slate-700/30 bg-transparent"
                    }`}
                  >
                    <td className="px-4 py-3 font-mono text-stone-900 dark:text-slate-200 font-medium">{resource.name}</td>
                    <td className="px-4 py-3 text-stone-500 dark:text-slate-400">{resource.namespace || "-"}</td>
                    <td className={`px-4 py-3 ${getStatusColor(resource.status)}`}>{resource.status || "-"}</td>
                    {(selectedKind === "deployments" || selectedKind === "statefulsets" || selectedKind === "replicasets" || selectedKind === "daemonsets") && (
                      <td className="px-4 py-3">
                        {resource.replicas ? (
                          <span className={
                            resource.replicas.includes("/") && resource.replicas.split("/")[0] === resource.replicas.split("/")[1]
                              ? "text-emerald-500 dark:text-emerald-400"
                              : parseInt(resource.replicas.split("/")[0]) > 0
                              ? "text-yellow-500 dark:text-yellow-400"
                              : "text-red-500 dark:text-red-400"
                          }>
                            {resource.replicas}
                          </span>
                        ) : "-"}
                      </td>
                    )}
                    {selectedKind === "pods" && (
                      <>
                        <td className="px-4 py-3">
                          {resource.readyContainers ? (
                            <span className={
                              resource.readyContainers.includes("/") && resource.readyContainers.split("/")[0] === resource.readyContainers.split("/")[1]
                                ? "text-emerald-500 dark:text-emerald-400"
                                : "text-yellow-500 dark:text-yellow-400"
                            }>
                              {resource.readyContainers}
                            </span>
                          ) : "-"}
                        </td>
                        <td className="px-4 py-3">
                          <span className={
                            resource.restarts && resource.restarts > 0
                              ? resource.restarts > 5 ? "text-red-500 dark:text-red-400" : "text-yellow-500 dark:text-yellow-400"
                              : "text-slate-500 dark:text-slate-400"
                          }>
                            {resource.restarts ?? 0}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-xs truncate max-w-[150px]" title={resource.node ? `Click to view node: ${resource.node}` : undefined}>
                          {resource.node ? (
                            <button
                              onClick={(e) => { e.stopPropagation(); setSelectedKind("nodes"); setSearchInput(resource.node || ""); }}
                              className="text-cyan-600 dark:text-cyan-400 hover:text-cyan-500 dark:hover:text-cyan-300 hover:underline transition-colors"
                            >
                              {resource.node}
                            </button>
                          ) : (
                            <span className="text-slate-500">-</span>
                          )}
                        </td>
                        <td className={`px-4 py-3 text-xs ${
                          resource.qosClass === "Guaranteed" ? "text-emerald-500 dark:text-emerald-400"
                          : resource.qosClass === "Burstable" ? "text-yellow-500 dark:text-yellow-400"
                          : "text-orange-500 dark:text-orange-400"
                        }`}>
                          {resource.qosClass || "-"}
                        </td>
                      </>
                    )}
                    {selectedKind === "services" && (
                      <>
                        <td className="px-4 py-3">
                          <span className={`px-1.5 py-0.5 rounded text-xs ${
                            resource.serviceType === "LoadBalancer" ? "bg-purple-500/20 text-purple-600 dark:text-purple-400"
                            : resource.serviceType === "NodePort" ? "bg-orange-500/20 text-orange-600 dark:text-orange-400"
                            : "bg-slate-200 dark:bg-slate-700 text-slate-600 dark:text-slate-400"
                          }`}>
                            {resource.serviceType || "ClusterIP"}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-slate-500 dark:text-slate-400 font-mono text-xs">{resource.clusterIP || "-"}</td>
                        <td className="px-4 py-3 text-slate-500 dark:text-slate-400 text-xs">{resource.ports || "-"}</td>
                      </>
                    )}
                    {selectedKind === "persistentvolumeclaims" && (
                      <>
                        <td className="px-4 py-3 text-cyan-600 dark:text-cyan-400">{resource.capacity || "-"}</td>
                        <td className="px-4 py-3 text-slate-500 dark:text-slate-400 text-xs">{resource.accessModes || "-"}</td>
                        <td className="px-4 py-3 text-slate-500 dark:text-slate-400 text-xs">{resource.storageClass || "-"}</td>
                      </>
                    )}
                    {selectedKind === "ingresses" && (
                      <>
                        <td className="px-4 py-3 text-slate-500 dark:text-slate-400 text-xs">{resource.ingressClass || "-"}</td>
                        <td className="px-4 py-3 text-cyan-600 dark:text-cyan-400 text-xs max-w-[200px] truncate" title={resource.hosts}>{resource.hosts || "-"}</td>
                        <td className="px-4 py-3 text-slate-500 dark:text-slate-400 text-xs max-w-[150px] truncate" title={resource.paths}>{resource.paths || "-"}</td>
                      </>
                    )}
                    {selectedKind === "nodes" && (
                      <>
                        <td className="px-4 py-3 text-xs">
                          {resource.roles ? (
                            <div className="flex gap-1 flex-wrap">
                              {resource.roles.split(",").map((role, i) => (
                                <span key={i} className={`px-1.5 py-0.5 rounded ${
                                  role === "control-plane" || role === "master"
                                    ? "bg-purple-500/20 text-purple-600 dark:text-purple-400"
                                    : "bg-slate-200 dark:bg-slate-700 text-slate-600 dark:text-slate-300"
                                }`}>
                                  {role}
                                </span>
                              ))}
                            </div>
                          ) : (
                            <span className="text-slate-500">worker</span>
                          )}
                        </td>
                        <td className="px-4 py-3 text-xs">
                          <div className="flex flex-col gap-0.5">
                            <span className="text-cyan-600 dark:text-cyan-400 font-mono">{resource.cpuAllocatable || "-"}</span>
                            {showUtilization && nodeMetrics[resource.name] && (
                              <div className="flex items-center gap-1">
                                <div className="w-16 h-1.5 bg-slate-200 dark:bg-slate-700 rounded-full overflow-hidden">
                                  <div
                                    className={`h-full ${
                                      nodeMetrics[resource.name].cpuRequestPct > 80 ? "bg-red-500"
                                      : nodeMetrics[resource.name].cpuRequestPct > 60 ? "bg-yellow-500"
                                      : "bg-emerald-500"
                                    }`}
                                    style={{ width: `${Math.min(nodeMetrics[resource.name].cpuRequestPct, 100)}%` }}
                                  />
                                </div>
                                <span className="text-slate-500 text-[10px]">{nodeMetrics[resource.name].cpuRequestPct}%</span>
                              </div>
                            )}
                          </div>
                        </td>
                        <td className="px-4 py-3 text-xs">
                          <div className="flex flex-col gap-0.5">
                            <span className="text-purple-600 dark:text-purple-400 font-mono">{resource.memAllocatable || "-"}</span>
                            {showUtilization && nodeMetrics[resource.name] && (
                              <div className="flex items-center gap-1">
                                <div className="w-16 h-1.5 bg-slate-200 dark:bg-slate-700 rounded-full overflow-hidden">
                                  <div
                                    className={`h-full ${
                                      nodeMetrics[resource.name].memRequestPct > 80 ? "bg-red-500"
                                      : nodeMetrics[resource.name].memRequestPct > 60 ? "bg-yellow-500"
                                      : "bg-emerald-500"
                                    }`}
                                    style={{ width: `${Math.min(nodeMetrics[resource.name].memRequestPct, 100)}%` }}
                                  />
                                </div>
                                <span className="text-slate-500 text-[10px]">{nodeMetrics[resource.name].memRequestPct}%</span>
                              </div>
                            )}
                          </div>
                        </td>
                        <td className="px-4 py-3 text-xs">
                          {resource.nodeConditions && resource.nodeConditions.length > 0 ? (
                            <div className="flex gap-1 flex-wrap">
                              {resource.nodeConditions.map((cond, i) => (
                                <span key={i} className={`px-1.5 py-0.5 rounded text-[10px] ${
                                  cond === "Ready" ? "bg-emerald-500/20 text-emerald-600 dark:text-emerald-400"
                                  : cond === "NotReady" ? "bg-red-500/20 text-red-600 dark:text-red-400"
                                  : "bg-yellow-500/20 text-yellow-600 dark:text-yellow-400"
                                }`}>
                                  {cond}
                                </span>
                              ))}
                              {resource.unschedulable && (
                                <span className="px-1.5 py-0.5 rounded text-[10px] bg-orange-500/20 text-orange-600 dark:text-orange-400">
                                  Cordoned
                                </span>
                              )}
                            </div>
                          ) : (
                            <span className="text-slate-500">-</span>
                          )}
                        </td>
                        <td className="px-4 py-3 text-slate-500 dark:text-slate-400 text-xs">{resource.kubeletVersion || "-"}</td>
                      </>
                    )}
                    {/* Toggleable columns */}
                    {showResources && currentToggles.includes("resources") && (
                      <td className="px-4 py-3 text-xs">
                        {resource.cpuRequest || resource.memRequest ? (
                          <div className="space-y-0.5">
                            <div className="flex gap-2">
                              <span className={resource.cpuRequest || resource.memRequest ? "text-emerald-400" : "text-red-400"}>
                                Req: {resource.cpuRequest || resource.memRequest ? "\u2713" : "\u2717"}
                              </span>
                              <span className={resource.cpuLimit || resource.memLimit ? "text-emerald-400" : "text-orange-400"}>
                                Lim: {resource.cpuLimit || resource.memLimit ? "\u2713" : "\u2717"}
                              </span>
                            </div>
                            {resource.cpuRequest && (
                              <div className="text-yellow-400 font-mono">CPU: {resource.cpuRequest}{resource.cpuLimit && `/${resource.cpuLimit}`}</div>
                            )}
                            {resource.memRequest && (
                              <div className="text-purple-400 font-mono">Mem: {resource.memRequest}{resource.memLimit && `/${resource.memLimit}`}</div>
                            )}
                          </div>
                        ) : (
                          <span className="text-red-400">No resources</span>
                        )}
                      </td>
                    )}
                    {showImages && currentToggles.includes("images") && (
                      <td className="px-4 py-3 text-cyan-400 text-xs max-w-[200px]">
                        {resource.images && resource.images.length > 0 ? (
                          <div className="flex flex-col gap-0.5">
                            {resource.images.map((img: string, i: number) => (
                              <span key={i} className="truncate" title={img}>{img.split("/").pop()?.split("@")[0]}</span>
                            ))}
                          </div>
                        ) : (
                          <span className="text-slate-600">-</span>
                        )}
                      </td>
                    )}
                    {showOwner && currentToggles.includes("owner") && (
                      <td className="px-4 py-3 text-orange-400 text-xs">
                        {resource.ownerKind && resource.ownerName ? (
                          <span title={`${resource.ownerKind}/${resource.ownerName}`}>
                            {resource.ownerKind}: {resource.ownerName.substring(0, 20)}{resource.ownerName.length > 20 && "..."}
                          </span>
                        ) : (
                          <span className="text-slate-600">-</span>
                        )}
                      </td>
                    )}
                    {showProbes && currentToggles.includes("probes") && (
                      <td className="px-4 py-3">
                        <div className="flex gap-1">
                          <span className={`px-1 py-0.5 rounded text-[10px] ${resource.hasLiveness ? "bg-emerald-500/20 text-emerald-400" : "bg-slate-700 text-slate-500"}`} title="Liveness Probe">L</span>
                          <span className={`px-1 py-0.5 rounded text-[10px] ${resource.hasReadiness ? "bg-emerald-500/20 text-emerald-400" : "bg-slate-700 text-slate-500"}`} title="Readiness Probe">R</span>
                        </div>
                      </td>
                    )}
                    {showVolumes && currentToggles.includes("volumes") && (
                      <td className="px-4 py-3 text-blue-400 text-xs">
                        {resource.volumes && resource.volumes.length > 0 ? (
                          <div className="flex flex-col gap-0.5">
                            {resource.volumes.map((vol: string, i: number) => (
                              <span key={i} className="truncate" title={vol}>{vol}</span>
                            ))}
                          </div>
                        ) : (
                          <span className="text-slate-600">-</span>
                        )}
                      </td>
                    )}
                    {showSecurity && currentToggles.includes("security") && (
                      <td className="px-4 py-3">
                        {resource.securityIssues && resource.securityIssues.length > 0 ? (
                          <div className="flex gap-1 flex-wrap">
                            {resource.securityIssues.map((issue, i) => (
                              <span key={i} className="px-1 py-0.5 rounded text-[10px] bg-red-500/20 text-red-400" title={issue}>
                                {issue.substring(0, 10)}
                              </span>
                            ))}
                          </div>
                        ) : (
                          <span className="text-emerald-400 text-xs">\u2713 OK</span>
                        )}
                      </td>
                    )}
                    {showSelectors && currentToggles.includes("selectors") && (
                      <td className="px-4 py-3 text-purple-400 text-xs max-w-[200px]">
                        {resource.selectors ? (
                          <span className="truncate" title={resource.selectors}>{resource.selectors}</span>
                        ) : (
                          <span className="text-slate-600">-</span>
                        )}
                      </td>
                    )}
                    {showDataKeys && currentToggles.includes("dataKeys") && (
                      <td className="px-4 py-3 text-cyan-400 text-xs max-w-[200px]">
                        {resource.dataKeys && resource.dataKeys.length > 0 ? (
                          <div className="flex flex-col gap-0.5">
                            {resource.dataKeys.slice(0, 5).map((key: string, i: number) => (
                              <span key={i} className="truncate" title={key}>{key}</span>
                            ))}
                            {resource.dataKeys.length > 5 && (
                              <span className="text-slate-500">+{resource.dataKeys.length - 5} more</span>
                            )}
                          </div>
                        ) : (
                          <span className="text-slate-600">-</span>
                        )}
                      </td>
                    )}
                    {showTLS && currentToggles.includes("tls") && (
                      <td className="px-4 py-3 text-emerald-400 text-xs">
                        {resource.tlsHosts ? (
                          <span title={resource.tlsHosts}>\uD83D\uDD12 {resource.tlsHosts}</span>
                        ) : (
                          <span className="text-orange-400">No TLS</span>
                        )}
                      </td>
                    )}
                    {showBackends && currentToggles.includes("backends") && (
                      <td className="px-4 py-3 text-orange-400 text-xs max-w-[200px]">
                        {resource.backends ? (
                          <span className="truncate" title={resource.backends}>{resource.backends}</span>
                        ) : (
                          <span className="text-slate-600">-</span>
                        )}
                      </td>
                    )}
                    {showUtilization && currentToggles.includes("utilization") && (
                      <td className="px-4 py-3 text-xs">
                        {nodeMetrics[resource.name] ? (
                          <div className="space-y-1">
                            <div className="flex items-center gap-2">
                              <span className="text-slate-500 w-8">Pods:</span>
                              <span className="text-cyan-400">{nodeMetrics[resource.name].podCount}</span>
                            </div>
                            <div className="flex items-center gap-2">
                              <span className="text-slate-500 w-8">CPU:</span>
                              <span className="text-yellow-400 font-mono">{nodeMetrics[resource.name].cpuRequests}</span>
                              <span className="text-slate-600">/</span>
                              <span className="text-orange-400 font-mono">{nodeMetrics[resource.name].cpuLimits}</span>
                            </div>
                            <div className="flex items-center gap-2">
                              <span className="text-slate-500 w-8">Mem:</span>
                              <span className="text-yellow-400 font-mono">{nodeMetrics[resource.name].memRequests}</span>
                              <span className="text-slate-600">/</span>
                              <span className="text-orange-400 font-mono">{nodeMetrics[resource.name].memLimits}</span>
                            </div>
                          </div>
                        ) : (
                          <span className="text-slate-500">Loading...</span>
                        )}
                      </td>
                    )}
                    {showTaints && currentToggles.includes("taints") && (
                      <td className="px-4 py-3 text-xs max-w-[250px]">
                        {resource.taints && resource.taints.length > 0 ? (
                          <div className="flex flex-col gap-0.5">
                            {resource.taints.map((taint: string, i: number) => (
                              <span key={i} className="px-1.5 py-0.5 rounded bg-yellow-500/20 text-yellow-400 text-[10px] truncate" title={taint}>{taint}</span>
                            ))}
                          </div>
                        ) : (
                          <span className="text-slate-500">No taints</span>
                        )}
                      </td>
                    )}
                    {showNodeInfo && currentToggles.includes("nodeInfo") && (
                      <td className="px-4 py-3 text-xs">
                        <div className="space-y-0.5">
                          {resource.osImage && <div className="text-slate-400 truncate max-w-[200px]" title={resource.osImage}>{resource.osImage}</div>}
                          {resource.containerRuntime && <div className="text-purple-400 text-[10px]">{resource.containerRuntime}</div>}
                          {resource.architecture && <div className="text-cyan-400 text-[10px]">{resource.architecture}</div>}
                          {resource.internalIP && <div className="text-slate-500 font-mono text-[10px]">{resource.internalIP}</div>}
                        </div>
                      </td>
                    )}
                    <td className="px-4 py-3 text-slate-500">{resource.age}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Resource Context Menu */}
      <AnimatePresence>
        {resourceContextMenu && (
          <motion.div
            initial={{ opacity: 0, scale: 0.95 }}
            animate={{ opacity: 1, scale: 1 }}
            exit={{ opacity: 0, scale: 0.95 }}
            transition={{ duration: 0.1 }}
            className="fixed z-50 bg-white dark:bg-slate-800 border border-stone-200 dark:border-slate-700 rounded-lg shadow-xl py-1 min-w-[200px]"
            style={{
              top: Math.min(resourceContextMenu.y, window.innerHeight - 250),
              left: Math.min(resourceContextMenu.x, window.innerWidth - 220),
            }}
          >
            <div className="px-3 py-2 border-b border-stone-200 dark:border-slate-700 mb-1">
              <div className="font-medium text-sm text-stone-900 dark:text-slate-200">{resourceContextMenu.resource.name}</div>
              <div className="text-xs text-stone-500 dark:text-slate-400">{resourceContextMenu.resource.kind}</div>
            </div>

            <button
              onClick={() => {
                const addToContext = useAIStore.getState().addToContext;
                const contextQueue = useAIStore.getState().contextQueue;
                const isInQueue = contextQueue.some(
                  (item) =>
                    item.type === (resourceContextMenu.resource.kind || selectedKind) &&
                    item.name === resourceContextMenu.resource.name &&
                    item.namespace === resourceContextMenu.resource.namespace
                );
                if (!isInQueue) {
                  addToContext({
                    id: `${activeContext}_${resourceContextMenu.resource.kind || selectedKind}_${resourceContextMenu.resource.namespace || "default"}_${resourceContextMenu.resource.name}_${Date.now()}`,
                    type: (resourceContextMenu.resource.kind || selectedKind) as any,
                    namespace: resourceContextMenu.resource.namespace,
                    name: resourceContextMenu.resource.name,
                    cluster: activeContext,
                    addedAt: new Date(),
                  });
                }
                setResourceContextMenu(null);
              }}
              className="w-full text-left px-3 py-2 text-sm text-stone-700 dark:text-slate-300 hover:bg-stone-100 dark:hover:bg-slate-700 flex items-center gap-2"
            >
              <Bot size={14} className="text-purple-500" />
              Add to AI Context
            </button>

            {selectedKind === "pods" && resourceContextMenu.resource.status === "Running" && (
              <button
                onClick={() => {
                  const resource = resourceContextMenu.resource;
                  const newSessionId = `context-${resource.namespace}-${resource.name}-${Date.now()}`;
                  setTerminalSessionId(newSessionId);
                  setShowTerminal(true);
                  setResourceContextMenu(null);
                  StartTerminal(newSessionId, "kubectl", [
                    "exec", "-i", "-t", "-n", resource.namespace, resource.name, "--", "/bin/sh",
                  ]).catch((err: Error) => {
                    console.error("Failed to start terminal:", err);
                    setShowTerminal(false);
                    setTerminalSessionId(null);
                  });
                }}
                className="w-full text-left px-3 py-2 text-sm text-stone-700 dark:text-slate-300 hover:bg-stone-100 dark:hover:bg-slate-700 flex items-center gap-2"
              >
                <Terminal size={14} className="text-emerald-500" />
                Shell
              </button>
            )}

            <button
              onClick={() => {
                setAnalysisResource({
                  kind: resourceContextMenu.resource.kind || selectedKind,
                  namespace: resourceContextMenu.resource.namespace,
                  name: resourceContextMenu.resource.name,
                });
                setResourceContextMenu(null);
              }}
              className="w-full text-left px-3 py-2 text-sm text-stone-700 dark:text-slate-300 hover:bg-stone-100 dark:hover:bg-slate-700 flex items-center gap-2"
            >
              <Sparkles size={14} className="text-cyan-500" />
              Analyze with AI
            </button>

            <button
              onClick={() => { handleDescribe(resourceContextMenu.resource); setResourceContextMenu(null); }}
              className="w-full text-left px-3 py-2 text-sm text-stone-700 dark:text-slate-300 hover:bg-stone-100 dark:hover:bg-slate-700 flex items-center gap-2"
            >
              <FileText size={14} />
              Describe
              <span className="ml-auto text-xs text-stone-400 dark:text-slate-500">d</span>
            </button>

            <button
              onClick={() => { handleCopyYaml(resourceContextMenu.resource); setResourceContextMenu(null); }}
              className="w-full text-left px-3 py-2 text-sm text-stone-700 dark:text-slate-300 hover:bg-stone-100 dark:hover:bg-slate-700 flex items-center gap-2"
            >
              <Copy size={14} />
              Copy YAML
              <span className="ml-auto text-xs text-stone-400 dark:text-slate-500">c</span>
            </button>

            {(selectedKind === "pods" || selectedKind === "deployments" || selectedKind === "statefulsets" || selectedKind === "daemonsets") && (
              <button
                onClick={() => {
                  onSelectPod({
                    name: resourceContextMenu.resource.name,
                    namespace: resourceContextMenu.resource.namespace,
                    kind: resourceContextMenu.resource.kind,
                  });
                  setResourceContextMenu(null);
                }}
                className="w-full text-left px-3 py-2 text-sm text-stone-700 dark:text-slate-300 hover:bg-stone-100 dark:hover:bg-slate-700 flex items-center gap-2"
              >
                <ScrollText size={14} />
                Logs
                <span className="ml-auto text-xs text-stone-400 dark:text-slate-500">l</span>
              </button>
            )}

            {selectedKind === "secrets" && (
              <button
                onClick={() => { handleViewSecretData(resourceContextMenu.resource); setResourceContextMenu(null); }}
                className="w-full text-left px-3 py-2 text-sm text-stone-700 dark:text-slate-300 hover:bg-stone-100 dark:hover:bg-slate-700 flex items-center gap-2"
              >
                <Eye size={14} />
                View Data
                <span className="ml-auto text-xs text-stone-400 dark:text-slate-500">h</span>
              </button>
            )}
          </motion.div>
        )}
      </AnimatePresence>

      {/* Describe Modal */}
      <AnimatePresence>
        {describeModal && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-8"
            onClick={() => setDescribeModal(null)}
          >
            <motion.div
              initial={{ scale: 0.95 }}
              animate={{ scale: 1 }}
              exit={{ scale: 0.95 }}
              onClick={(e) => e.stopPropagation()}
              className="bg-white dark:bg-slate-900 border border-stone-200 dark:border-slate-700 rounded-xl shadow-2xl w-full max-w-4xl max-h-[80vh] flex flex-col transition-colors duration-200"
            >
              <div className="flex items-center justify-between p-4 border-b border-slate-700">
                <div>
                  <h2 className="text-lg font-semibold">{describeModal.name}</h2>
                  <p className="text-sm text-slate-400">
                    {describeModal.kind} {describeModal.namespace && `in ${describeModal.namespace}`}
                  </p>
                </div>
                <div className="flex gap-2">
                  <button
                    onClick={() => { navigator.clipboard.writeText(describeYaml); setCopySuccess(true); setTimeout(() => setCopySuccess(false), 2000); }}
                    className="px-3 py-1.5 text-sm bg-slate-700 hover:bg-slate-600 rounded-lg transition-colors"
                  >
                    {copySuccess ? "Copied!" : "Copy"}
                  </button>
                  <button onClick={() => setDescribeModal(null)} className="p-2 hover:bg-slate-800 rounded-lg transition-colors">
                    <X size={18} />
                  </button>
                </div>
              </div>
              <div className="flex-1 overflow-auto p-4">
                {describeLoading ? (
                  <div className="flex items-center justify-center py-12">
                    <div className="w-6 h-6 border-2 border-accent-400 border-t-transparent rounded-full animate-spin" />
                  </div>
                ) : (
                  <pre className="text-sm font-mono text-slate-300 whitespace-pre-wrap">{describeYaml}</pre>
                )}
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>

      {/* Secret Data Modal */}
      <AnimatePresence>
        {secretDataModal && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-8"
            onClick={() => setSecretDataModal(null)}
          >
            <motion.div
              initial={{ scale: 0.95 }}
              animate={{ scale: 1 }}
              exit={{ scale: 0.95 }}
              className="bg-slate-800 rounded-xl border border-slate-700 w-full max-w-2xl max-h-[80vh] flex flex-col"
              onClick={(e) => e.stopPropagation()}
            >
              <div className="flex items-center justify-between p-4 border-b border-slate-700">
                <div>
                  <h2 className="text-lg font-semibold flex items-center gap-2">
                    <span className="text-yellow-400">\uD83D\uDD13</span>
                    {secretDataModal.name}
                  </h2>
                  <p className="text-sm text-slate-400">Secret Data (Decoded) &bull; {secretDataModal.namespace}</p>
                </div>
                <button onClick={() => setSecretDataModal(null)} className="p-2 hover:bg-slate-700 rounded-lg transition-colors">
                  <X size={18} />
                </button>
              </div>
              <div className="flex-1 overflow-auto p-4">
                {secretDataLoading ? (
                  <div className="flex items-center justify-center py-12">
                    <div className="w-6 h-6 border-2 border-yellow-400 border-t-transparent rounded-full animate-spin" />
                  </div>
                ) : Object.keys(secretData).length === 0 ? (
                  <p className="text-slate-400 text-center py-8">No data in this secret</p>
                ) : (
                  <div className="space-y-4">
                    {Object.entries(secretData).map(([key, value]) => {
                      const isRevealed = revealedSecretKeys.has(key);
                      const toggleReveal = () =>
                        setRevealedSecretKeys((prev) => {
                          const next = new Set(prev);
                          if (next.has(key)) { next.delete(key); } else { next.add(key); }
                          return next;
                        });
                      return (
                        <div key={key} className="bg-slate-50 dark:bg-slate-900 rounded-lg p-3 border border-slate-200 dark:border-slate-700/50">
                          <div className="flex items-center justify-between mb-2">
                            <span className="text-sm font-medium text-cyan-400">{key}</span>
                            <div className="flex items-center gap-1">
                              <button onClick={toggleReveal} title={isRevealed ? "Hide value" : "Reveal value"} className="p-1 text-slate-400 hover:text-slate-200 transition-colors">
                                {isRevealed ? <EyeOff size={14} /> : <Eye size={14} />}
                              </button>
                              <button onClick={() => { navigator.clipboard.writeText(value); }} className="px-2 py-1 text-xs bg-slate-700 hover:bg-slate-600 rounded transition-colors">
                                Copy
                              </button>
                            </div>
                          </div>
                          <pre className="text-sm font-mono text-slate-300 whitespace-pre-wrap break-all bg-slate-950 p-2 rounded">
                            {isRevealed ? value : "\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022"}
                          </pre>
                        </div>
                      );
                    })}
                  </div>
                )}
              </div>
              <div className="p-3 border-t border-stone-200 dark:border-slate-700 bg-stone-50 dark:bg-slate-900/50">
                <p className="text-xs text-orange-400 flex items-center gap-1">
                  <span>\u26A0\uFE0F</span>
                  Secret data is sensitive. Be careful when copying or sharing.
                </p>
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>

      <AnalysisModal
        isOpen={!!analysisResource}
        onClose={() => setAnalysisResource(null)}
        resource={analysisResource}
      />

      <ErrorBoundary componentName="Terminal">
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
          nodeName={selectedIndex >= 0 && sortedResources[selectedIndex] ? sortedResources[selectedIndex].name : ""}
          namespace={selectedIndex >= 0 && sortedResources[selectedIndex] ? sortedResources[selectedIndex].namespace || "" : ""}
        />
      </ErrorBoundary>
    </div>
  );
}
