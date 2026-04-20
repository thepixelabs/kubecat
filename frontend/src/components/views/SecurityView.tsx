import { useState, useEffect, useRef } from "react";
import { Sparkles, FileText, MoreVertical, X, Send } from "lucide-react";
import { motion, AnimatePresence } from "framer-motion";
import {
  ListResources,
  GetRBACAnalysis,
  GetResourceYAML,
} from "../../../wailsjs/go/main/App";
import { useAIStore } from "../../stores/aiStore";
import { AnalysisModal } from "../AnalysisModal";
import type { ResourceInfo } from "../../types/resources";

const REFRESH_BTN_CLASS =
  "px-3 py-1.5 text-sm bg-slate-200 dark:bg-slate-700 hover:bg-slate-300 dark:hover:bg-slate-600 text-slate-700 dark:text-slate-200 rounded-lg transition-colors disabled:opacity-50";

interface RBACSubject {
  kind: string;
  name: string;
  namespace?: string;
}

interface RBACPermission {
  verbs: string[];
  resources: string[];
  resourceNames?: string[];
  apiGroups: string[];
}

interface RBACBinding {
  name: string;
  namespace?: string;
  roleName: string;
  roleKind: string;
  subjects: RBACSubject[];
  permissions: RBACPermission[];
  isCluster: boolean;
}

interface DangerousAccessInfo {
  subject: RBACSubject;
  reason: string;
  binding: string;
  namespace?: string;
  permissions: string[];
}

interface RBACSummary {
  bindings: RBACBinding[];
  subjectSummary: Record<string, string[]>;
  dangerousAccess: DangerousAccessInfo[];
}

export function SecurityView({ isConnected }: { isConnected: boolean }) {
  const [mode, setMode] = useState<"resources" | "rbac" | "ai">("rbac");
  const [aiQuery, setAiQuery] = useState("");
  const [aiResponse, setAiResponse] = useState<string | null>(null);
  const [aiLoading, setAiLoading] = useState(false);
  const [aiError, setAiError] = useState<string | null>(null);
  const [roles, setRoles] = useState<ResourceInfo[]>([]);
  const [clusterRoles, setClusterRoles] = useState<ResourceInfo[]>([]);
  const [roleBindings, setRoleBindings] = useState<ResourceInfo[]>([]);
  const [clusterRoleBindings, setClusterRoleBindings] = useState<
    ResourceInfo[]
  >([]);
  const [networkPolicies, setNetworkPolicies] = useState<ResourceInfo[]>([]);
  const [serviceAccounts, setServiceAccounts] = useState<ResourceInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<
    | "roles"
    | "clusterroles"
    | "rolebindings"
    | "clusterrolebindings"
    | "networkpolicies"
    | "serviceaccounts"
  >("roles");
  const [namespaceFilter, setNamespaceFilter] = useState("");
  const [showSystem, setShowSystem] = useState(false);
  const [analyzingResource, setAnalyzingResource] =
    useState<ResourceInfo | null>(null);
  const [yamlResource, setYamlResource] = useState<ResourceInfo | null>(null);
  const [yamlContent, setYamlContent] = useState("");
  const [yamlLoading, setYamlLoading] = useState(false);
  const [activeMenuIndex, setActiveMenuIndex] = useState<number>(-1);

  const isSystemResource = (r: ResourceInfo) => {
    return (
      r.namespace === "kube-system" ||
      r.namespace === "kube-public" ||
      r.namespace === "kube-node-lease" ||
      r.name.startsWith("system:") ||
      r.name.startsWith("kube-")
    );
  };

  // RBAC Analysis state
  const [rbacSummary, setRbacSummary] = useState<RBACSummary | null>(null);
  const [rbacLoading, setRbacLoading] = useState(false);
  const [rbacError, setRbacError] = useState<string | null>(null);
  const [rbacView, setRbacView] = useState<
    "subjects" | "bindings" | "dangerous"
  >("subjects");
  const [selectedBinding, setSelectedBinding] = useState<RBACBinding | null>(
    null
  );
  const [searchFilter, setSearchFilter] = useState("");
  const [selectedIndex, setSelectedIndex] = useState<number>(-1);
  const tableRef = useRef<HTMLDivElement>(null);
  const filterRef = useRef<HTMLInputElement>(null);

  const fetchSecurityResources = async () => {
    setLoading(true);
    setError(null);
    try {
      const ns = namespaceFilter || "";
      const [r, cr, rb, crb, np, sa] = await Promise.allSettled([
        ListResources("roles", ns),
        ListResources("clusterroles", ""),
        ListResources("rolebindings", ns),
        ListResources("clusterrolebindings", ""),
        ListResources("networkpolicies", ns),
        ListResources("serviceaccounts", ns),
      ]);
      if (r.status === "fulfilled") setRoles(r.value || []);
      if (cr.status === "fulfilled") setClusterRoles(cr.value || []);
      if (rb.status === "fulfilled") setRoleBindings(rb.value || []);
      if (crb.status === "fulfilled") setClusterRoleBindings(crb.value || []);
      if (np.status === "fulfilled") setNetworkPolicies(np.value || []);
      if (sa.status === "fulfilled") setServiceAccounts(sa.value || []);
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Failed to fetch security resources"
      );
    } finally {
      setLoading(false);
    }
  };

  const fetchRBACAnalysis = async () => {
    setRbacLoading(true);
    setRbacError(null);
    try {
      const result = await GetRBACAnalysis();
      setRbacSummary(result);
    } catch (err) {
      setRbacError(
        err instanceof Error ? err.message : "Failed to analyze RBAC"
      );
    } finally {
      setRbacLoading(false);
    }
  };

  useEffect(() => {
    if (isConnected) {
      if (mode === "resources") {
        fetchSecurityResources();
      } else {
        fetchRBACAnalysis();
      }
    } else {
      setRoles([]);
      setClusterRoles([]);
      setRoleBindings([]);
      setClusterRoleBindings([]);
      setNetworkPolicies([]);
      setServiceAccounts([]);
      setRbacSummary(null);
    }
  }, [isConnected, namespaceFilter, mode]);

  // Get current list based on mode and tab
  const getCurrentList = () => {
    if (mode === "rbac") return [];
    switch (activeTab) {
      case "roles":
        return roles;
      case "clusterroles":
        return clusterRoles;
      case "rolebindings":
        return roleBindings;
      case "clusterrolebindings":
        return clusterRoleBindings;
      case "networkpolicies":
        return networkPolicies;
      case "serviceaccounts":
        return serviceAccounts;
      default:
        return [];
    }
  };
  const currentList = getCurrentList();

  // Reset selection when tab or data changes
  useEffect(() => {
    setSelectedIndex(-1);
  }, [activeTab, mode, currentList.length]);

  const handleAiQuery = async (queryOverride?: string) => {
    const q = queryOverride || aiQuery;
    if (!q.trim() || !isConnected) return;

    setAiLoading(true);
    setAiError(null);
    setAiResponse(null);

    try {
      // @ts-expect-error — dynamic Wails binding not typed
      const response = await window.go.main.App.QuerySecurityAI(q);
      setAiResponse(response);
    } catch (err) {
      setAiError(err instanceof Error ? err.message : String(err));
    } finally {
      setAiLoading(false);
    }
  };

  // Keyboard navigation
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

      switch (e.key) {
        case "/":
          if (mode === "ai") {
            return;
          }
          e.preventDefault();
          filterRef.current?.focus();
          break;
        case "r":
          if (!loading && !rbacLoading && isConnected) {
            e.preventDefault();
            if (mode === "resources") {
              fetchSecurityResources();
            } else {
              fetchRBACAnalysis();
            }
          }
          break;
        case "ArrowDown":
        case "j":
          if (mode !== "resources") return;
          e.preventDefault();
          setSelectedIndex((prev) =>
            prev < currentList.length - 1 ? prev + 1 : prev
          );
          break;
        case "ArrowUp":
        case "k":
          if (mode !== "resources") return;
          e.preventDefault();
          setSelectedIndex((prev) => (prev > 0 ? prev - 1 : 0));
          break;
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [currentList.length, mode, loading, rbacLoading, isConnected]);

  // Scroll selected row into view
  useEffect(() => {
    if (selectedIndex >= 0 && tableRef.current) {
      const row = tableRef.current.querySelector(
        `tr[data-index="${selectedIndex}"]`
      );
      row?.scrollIntoView({ block: "nearest", behavior: "smooth" });
    }
  }, [selectedIndex]);

  const tabs = [
    { id: "roles" as const, label: "Roles", count: roles.length },
    {
      id: "clusterroles" as const,
      label: "ClusterRoles",
      count: clusterRoles.length,
    },
    {
      id: "rolebindings" as const,
      label: "RoleBindings",
      count: roleBindings.length,
    },
    {
      id: "clusterrolebindings" as const,
      label: "ClusterRoleBindings",
      count: clusterRoleBindings.length,
    },
    {
      id: "networkpolicies" as const,
      label: "NetworkPolicies",
      count: networkPolicies.length,
    },
    {
      id: "serviceaccounts" as const,
      label: "ServiceAccounts",
      count: serviceAccounts.length,
    },
  ];

  const currentResources = (
    {
      roles,
      clusterroles: clusterRoles,
      rolebindings: roleBindings,
      clusterrolebindings: clusterRoleBindings,
      networkpolicies: networkPolicies,
      serviceaccounts: serviceAccounts,
    }[activeTab] || []
  ).filter((r) => {
    if (showSystem) return true;
    return !isSystemResource(r);
  });

  const isClusterScoped =
    activeTab === "clusterroles" || activeTab === "clusterrolebindings";

  // Filter subjects
  const filteredSubjects = rbacSummary
    ? Object.entries(rbacSummary.subjectSummary).filter(([key]) =>
        searchFilter
          ? key.toLowerCase().includes(searchFilter.toLowerCase())
          : true
      )
    : [];

  // Filter bindings
  const filteredBindings = rbacSummary
    ? rbacSummary.bindings.filter((b) =>
        searchFilter
          ? b.name.toLowerCase().includes(searchFilter.toLowerCase()) ||
            b.roleName.toLowerCase().includes(searchFilter.toLowerCase()) ||
            b.subjects.some((s) =>
              s.name.toLowerCase().includes(searchFilter.toLowerCase())
            )
          : true
      )
    : [];

  const getSubjectIcon = (kind: string) => {
    switch (kind) {
      case "User":
        return "\ud83d\udc64";
      case "Group":
        return "\ud83d\udc65";
      case "ServiceAccount":
        return "\ud83e\udd16";
      default:
        return "\u2753";
    }
  };

  const formatVerbs = (verbs: string[]) => {
    const dangerous = [
      "*",
      "delete",
      "deletecollection",
      "create",
      "patch",
      "update",
    ];
    return verbs.map((v) => (
      <span
        key={v}
        className={`inline-block px-1.5 py-0.5 mr-1 mb-1 rounded text-xs ${
          dangerous.includes(v)
            ? "bg-red-500/20 text-red-400"
            : "bg-slate-700 text-slate-300"
        }`}
      >
        {v}
      </span>
    ));
  };

  return (
    <div className="h-full flex flex-col">
      {/* Mode toggle */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex gap-2">
          <button
            onClick={() => setMode("rbac")}
            className={`px-4 py-2 text-sm rounded-lg border transition-colors ${
              mode === "rbac"
                ? "bg-accent-500/20 border-accent-500/50 text-accent-600 dark:text-accent-400"
                : "bg-white dark:bg-slate-800/50 border-stone-200 dark:border-slate-700/50 hover:bg-stone-50 dark:hover:bg-slate-700/50 text-stone-600 dark:text-slate-400"
            }`}
          >
            RBAC Analysis
          </button>
          <button
            onClick={() => setMode("resources")}
            className={`px-4 py-2 text-sm rounded-lg border transition-colors ${
              mode === "resources"
                ? "bg-accent-500/20 border-accent-500/50 text-accent-600 dark:text-accent-400"
                : "bg-white dark:bg-slate-800/50 border-stone-200 dark:border-slate-700/50 hover:bg-stone-50 dark:hover:bg-slate-700/50 text-stone-600 dark:text-slate-400"
            }`}
          >
            Resources
          </button>
          <button
            onClick={() => setMode("ai")}
            className={`px-4 py-2 text-sm rounded-lg border transition-colors flex items-center gap-2 ${
              mode === "ai"
                ? "bg-purple-500/20 border-purple-500/50 text-purple-600 dark:text-purple-400"
                : "bg-white dark:bg-slate-800/50 border-stone-200 dark:border-slate-700/50 hover:bg-stone-50 dark:hover:bg-slate-700/50 text-stone-600 dark:text-slate-400"
            }`}
          >
            <Sparkles size={14} />
            AI Insights
          </button>
        </div>

        {mode === "ai" && (
          <div className="flex items-center gap-2">
            <span className="text-xs text-stone-500 dark:text-slate-500">
              Powered by {useAIStore.getState().selectedModel || "AI"}
            </span>
          </div>
        )}
      </div>

      {mode === "ai" ? (
        <div className="flex-1 flex flex-col bg-white dark:bg-slate-800/50 rounded-xl border border-stone-200 dark:border-slate-700/50 overflow-hidden shadow-sm dark:shadow-none transition-colors">
          {!isConnected ? (
            <div className="flex flex-col items-center justify-center py-12">
              <p className="text-slate-400">
                Connect to a cluster to use AI Insights
              </p>
            </div>
          ) : (
            <div className="flex flex-col h-full">
              <div className="flex-1 overflow-auto p-6">
                {!aiResponse && !aiLoading && !aiError && (
                  <div className="flex flex-col items-center justify-center h-full text-center space-y-6">
                    <div className="w-16 h-16 bg-purple-500/10 rounded-full flex items-center justify-center">
                      <Sparkles className="w-8 h-8 text-purple-500" />
                    </div>
                    <div>
                      <h3 className="text-lg font-medium text-stone-800 dark:text-slate-200 mb-2">
                        Security &amp; Contextual Analysis
                      </h3>
                      <p className="text-stone-500 dark:text-slate-400 max-w-md mx-auto">
                        Ask any question about your cluster's security posture.
                        I analyze RBAC, Network Policies, and exposed services
                        to give you instant answers.
                      </p>
                    </div>

                    <div className="grid grid-cols-1 md:grid-cols-2 gap-3 max-w-2xl w-full">
                      {[
                        "Which ports are open to the world?",
                        "Who has cluster-admin access?",
                        "Are there any privileged pods?",
                        "What network policies are in default namespace?",
                      ].map((q) => (
                        <button
                          key={q}
                          onClick={() => {
                            setAiQuery(q);
                            handleAiQuery(q);
                          }}
                          className="text-left p-3 rounded-lg border border-stone-200 dark:border-slate-700 hover:bg-purple-50 dark:hover:bg-slate-800 hover:border-purple-300 dark:hover:border-purple-500/50 transition-colors text-sm text-stone-600 dark:text-slate-300"
                        >
                          {q}
                        </button>
                      ))}
                    </div>
                  </div>
                )}

                {aiLoading && (
                  <div className="flex flex-col items-center justify-center h-full gap-4">
                    <div className="w-8 h-8 border-2 border-purple-500 border-t-transparent rounded-full animate-spin" />
                    <p className="text-stone-500 dark:text-slate-400 animate-pulse">
                      Analyzing cluster security context...
                    </p>
                  </div>
                )}

                {aiError && (
                  <div className="p-4 bg-red-50 dark:bg-red-500/10 border border-red-200 dark:border-red-500/30 rounded-lg text-red-600 dark:text-red-400 text-sm">
                    Error: {aiError}
                  </div>
                )}

                {aiResponse && (
                  <div className="prose dark:prose-invert max-w-none">
                    <div className="flex items-center gap-2 mb-4 pb-4 border-b border-stone-200 dark:border-slate-700/50">
                      <div className="w-8 h-8 bg-purple-500/20 rounded-lg flex items-center justify-center text-purple-600 dark:text-purple-400 font-bold text-xs">
                        AI
                      </div>
                      <div className="font-medium text-stone-900 dark:text-slate-100">
                        Security Analysis
                      </div>
                    </div>
                    <div className="whitespace-pre-wrap text-sm text-stone-700 dark:text-slate-300 leading-relaxed">
                      {aiResponse}
                    </div>
                  </div>
                )}
              </div>

              <div className="p-4 border-t border-stone-200 dark:border-slate-700/50 bg-stone-50 dark:bg-slate-800/80">
                <div className="flex gap-2">
                  <textarea
                    value={aiQuery}
                    onChange={(e) => setAiQuery(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" && !e.shiftKey) {
                        e.preventDefault();
                        handleAiQuery();
                      }
                    }}
                    placeholder="Ask a security question..."
                    className="flex-1 bg-white dark:bg-slate-900 border border-stone-200 dark:border-slate-700 rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-purple-500/50 text-stone-900 dark:text-slate-100 resize-none h-[50px] min-h-[50px] max-h-[120px]"
                  />
                  <button
                    onClick={() => handleAiQuery()}
                    disabled={!aiQuery.trim() || aiLoading}
                    className="px-4 bg-purple-500 hover:bg-purple-600 text-white rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center p-3"
                  >
                    {aiLoading ? (
                      <div className="w-5 h-5 border-2 border-white border-t-transparent rounded-full animate-spin" />
                    ) : (
                      <Send size={20} />
                    )}
                  </button>
                </div>
              </div>
            </div>
          )}
        </div>
      ) : mode === "rbac" ? (
        <>
          {/* RBAC sub-tabs */}
          <div className="flex items-center justify-between mb-4 flex-wrap gap-3">
            <div className="flex gap-2">
              {(["subjects", "bindings", "dangerous"] as const).map((view) => (
                <button
                  key={view}
                  onClick={() => setRbacView(view)}
                  className={`px-3 py-1.5 text-sm rounded-lg border transition-colors ${
                    rbacView === view
                      ? "bg-accent-500/20 border-accent-500/50 text-accent-600 dark:text-accent-400"
                      : "bg-white dark:bg-slate-800/50 border-stone-200 dark:border-slate-700/50 hover:bg-stone-50 dark:hover:bg-slate-700/50 text-stone-600 dark:text-slate-400"
                  }`}
                >
                  {view === "subjects" && "Who Has Access"}
                  {view === "bindings" && "All Bindings"}
                  {view === "dangerous" && (
                    <>
                      Dangerous Access
                      {rbacSummary &&
                        rbacSummary.dangerousAccess.length > 0 && (
                          <span className="ml-1.5 px-1.5 py-0.5 text-xs bg-red-500/30 text-red-400 rounded">
                            {rbacSummary.dangerousAccess.length}
                          </span>
                        )}
                    </>
                  )}
                </button>
              ))}
            </div>
            <div className="flex items-center gap-3">
              <input
                ref={filterRef}
                type="text"
                placeholder="Search... (/)"
                value={searchFilter}
                onChange={(e) => setSearchFilter(e.target.value)}
                className="w-48 bg-stone-50 dark:bg-slate-800 border border-stone-200 dark:border-slate-700 rounded-lg px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-accent-500/50"
              />
              <button
                onClick={fetchRBACAnalysis}
                disabled={rbacLoading || !isConnected}
                className={REFRESH_BTN_CLASS}
              >
                Refresh
              </button>
            </div>
          </div>

          <div className="flex-1 bg-white dark:bg-slate-800/50 rounded-xl border border-stone-200 dark:border-slate-700/50 overflow-hidden shadow-sm dark:shadow-none transition-colors">
            {!isConnected ? (
              <p className="text-slate-400 text-center py-12">
                Connect to a cluster to analyze RBAC
              </p>
            ) : rbacLoading && !rbacSummary ? (
              <div className="flex flex-col items-center justify-center py-12 gap-3">
                <div className="w-8 h-8 border-2 border-accent-400 border-t-transparent rounded-full animate-spin" />
                <p className="text-stone-400 dark:text-slate-400">
                  Analyzing RBAC permissions...
                </p>
              </div>
            ) : rbacError ? (
              <p className="text-red-400 text-center py-12">{rbacError}</p>
            ) : rbacView === "subjects" ? (
              <div className="overflow-auto h-full p-4">
                <div className="space-y-2">
                  {filteredSubjects.map(([subject, namespaces]) => {
                    const [kind, ...rest] = subject.split(":");
                    const name = rest.join(":");
                    return (
                      <div
                        key={subject}
                        className="bg-stone-50 dark:bg-slate-900/50 rounded-lg p-4 border border-stone-200 dark:border-slate-700/50 shadow-sm dark:shadow-none transition-colors"
                      >
                        <div className="flex items-center gap-3 mb-2">
                          <span className="text-xl">
                            {getSubjectIcon(kind)}
                          </span>
                          <div>
                            <span className="font-mono text-accent-400">
                              {name}
                            </span>
                            <span className="ml-2 text-xs px-2 py-0.5 bg-stone-200 dark:bg-slate-700 rounded text-stone-600 dark:text-slate-300">
                              {kind}
                            </span>
                          </div>
                        </div>
                        <div className="text-sm text-stone-400 dark:text-slate-400">
                          <span className="text-stone-500 dark:text-slate-500">
                            Access to:{" "}
                          </span>
                          {namespaces.map((ns) => (
                            <span
                              key={ns}
                              className={`inline-block px-2 py-0.5 mr-1 mb-1 rounded text-xs ${
                                ns === "*"
                                  ? "bg-yellow-500/20 text-yellow-400"
                                  : "bg-stone-200 dark:bg-slate-700 text-stone-600 dark:text-slate-300"
                              }`}
                            >
                              {ns === "*" ? "cluster-wide" : ns}
                            </span>
                          ))}
                        </div>
                      </div>
                    );
                  })}
                  {filteredSubjects.length === 0 && (
                    <p className="text-stone-400 dark:text-slate-400 text-center py-8">
                      No subjects found
                    </p>
                  )}
                </div>
              </div>
            ) : rbacView === "bindings" ? (
              <div className="overflow-auto h-full p-4">
                <div className="space-y-2">
                  {filteredBindings.map((binding, idx) => (
                    <div
                      key={`${binding.name}-${idx}`}
                      className={`bg-stone-50 dark:bg-slate-900/50 rounded-lg border border-stone-200 dark:border-slate-700/50 overflow-hidden cursor-pointer transition-colors shadow-sm dark:shadow-none ${
                        selectedBinding?.name === binding.name
                          ? "ring-2 ring-accent-500"
                          : ""
                      }`}
                      onClick={() =>
                        setSelectedBinding(
                          selectedBinding?.name === binding.name
                            ? null
                            : binding
                        )
                      }
                    >
                      <div className="p-4">
                        <div className="flex items-center justify-between mb-2">
                          <div>
                            <span className="font-mono text-accent-400">
                              {binding.name}
                            </span>
                            <span
                              className={`ml-2 text-xs px-2 py-0.5 rounded ${
                                binding.isCluster
                                  ? "bg-yellow-500/20 text-yellow-400"
                                  : "bg-stone-200 dark:bg-slate-700 text-stone-600 dark:text-slate-300"
                              }`}
                            >
                              {binding.isCluster
                                ? "ClusterRoleBinding"
                                : "RoleBinding"}
                            </span>
                          </div>
                          {binding.namespace && (
                            <span className="text-sm text-slate-500">
                              ns: {binding.namespace}
                            </span>
                          )}
                        </div>
                        <div className="text-sm text-stone-400 dark:text-slate-400">
                          <span className="text-stone-500 dark:text-slate-500">
                            Role:{" "}
                          </span>
                          <span className="text-emerald-400">
                            {binding.roleKind}/{binding.roleName}
                          </span>
                        </div>
                        <div className="text-sm text-stone-400 dark:text-slate-400 mt-1">
                          <span className="text-stone-500 dark:text-slate-500">
                            Subjects:{" "}
                          </span>
                          {binding.subjects.map((s, i) => (
                            <span
                              key={i}
                              className="inline-flex items-center gap-1 mr-2"
                            >
                              {getSubjectIcon(s.kind)}
                              <span className="font-mono text-xs">
                                {s.namespace ? `${s.namespace}/` : ""}
                                {s.name}
                              </span>
                            </span>
                          ))}
                        </div>
                      </div>
                      {selectedBinding?.name === binding.name &&
                        binding.permissions.length > 0 && (
                          <div className="border-t border-stone-200 dark:border-slate-700 p-4 bg-stone-100 dark:bg-slate-950/50">
                            <p className="text-sm font-medium text-stone-700 dark:text-slate-300 mb-2">
                              Permissions:
                            </p>
                            <div className="space-y-2">
                              {binding.permissions.map((perm, i) => (
                                <div key={i} className="text-sm">
                                  <div className="mb-1">
                                    {formatVerbs(perm.verbs)}
                                  </div>
                                  <div className="text-stone-400 dark:text-slate-400">
                                    <span className="text-stone-500 dark:text-slate-500">
                                      on{" "}
                                    </span>
                                    {perm.resources.map((r) => (
                                      <span
                                        key={r}
                                        className="inline-block px-1.5 py-0.5 mr-1 bg-stone-200 dark:bg-slate-700 rounded text-xs text-stone-700 dark:text-slate-300"
                                      >
                                        {r}
                                      </span>
                                    ))}
                                    {perm.apiGroups.length > 0 &&
                                      perm.apiGroups[0] !== "" && (
                                        <span className="text-stone-500 dark:text-slate-500 text-xs ml-1">
                                          (apiGroups:{" "}
                                          {perm.apiGroups.join(", ")})
                                        </span>
                                      )}
                                  </div>
                                </div>
                              ))}
                            </div>
                          </div>
                        )}
                    </div>
                  ))}
                  {filteredBindings.length === 0 && (
                    <p className="text-slate-400 text-center py-8">
                      No bindings found
                    </p>
                  )}
                </div>
              </div>
            ) : (
              <div className="overflow-auto h-full p-4">
                {rbacSummary && rbacSummary.dangerousAccess.length === 0 ? (
                  <div className="text-center py-12">
                    <p className="text-emerald-400 text-xl mb-2">
                      {"\u2713"} No dangerous access detected
                    </p>
                    <p className="text-stone-500 dark:text-slate-500">
                      No subjects have overly permissive access
                    </p>
                  </div>
                ) : (
                  <div className="space-y-3">
                    {rbacSummary?.dangerousAccess.map((info, idx) => (
                      <div
                        key={idx}
                        className="bg-red-500/10 border border-red-500/30 rounded-lg p-4"
                      >
                        <div className="flex items-center gap-3 mb-2">
                          <span className="text-xl">
                            {getSubjectIcon(info.subject.kind)}
                          </span>
                          <div>
                            <span className="font-mono text-red-400">
                              {info.subject.namespace
                                ? `${info.subject.namespace}/`
                                : ""}
                              {info.subject.name}
                            </span>
                            <span className="ml-2 text-xs px-2 py-0.5 bg-slate-700 rounded">
                              {info.subject.kind}
                            </span>
                          </div>
                        </div>
                        <p className="text-sm text-red-300 mb-2">
                          {info.reason}
                        </p>
                        <div className="text-xs text-slate-400">
                          <span className="text-slate-500">Via binding: </span>
                          <span className="font-mono">{info.binding}</span>
                          {info.namespace && (
                            <span className="text-slate-500">
                              {" "}
                              in {info.namespace}
                            </span>
                          )}
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}
          </div>

          {rbacSummary && (
            <div className="mt-3 text-sm text-slate-500">
              {Object.keys(rbacSummary.subjectSummary).length} subjects &bull;{" "}
              {rbacSummary.bindings.length} bindings &bull;{" "}
              {rbacSummary.dangerousAccess.length} warnings
            </div>
          )}
        </>
      ) : (
        <>
          {/* Original resource view */}
          <div className="flex items-center justify-between mb-4 flex-wrap gap-3">
            <div className="flex gap-2 flex-wrap">
              {tabs.map((tab) => (
                <button
                  key={tab.id}
                  onClick={() => setActiveTab(tab.id)}
                  className={`px-3 py-1.5 text-sm rounded-lg border transition-colors ${
                    activeTab === tab.id
                      ? "bg-accent-500/20 border-accent-500/50 text-accent-600 dark:text-accent-400"
                      : "bg-white dark:bg-slate-800/50 border-stone-200 dark:border-slate-700/50 hover:bg-stone-50 dark:hover:bg-slate-700/50 text-stone-600 dark:text-slate-400"
                  }`}
                >
                  {tab.label}
                  {tab.count > 0 && (
                    <span className="ml-1.5 px-1.5 py-0.5 text-xs bg-stone-200 dark:bg-slate-700 rounded text-stone-600 dark:text-slate-300">
                      {tab.count}
                    </span>
                  )}
                </button>
              ))}
            </div>
            <div className="flex items-center gap-3">
              {!isClusterScoped && (
                <input
                  type="text"
                  placeholder="Filter by namespace..."
                  value={namespaceFilter}
                  onChange={(e) => setNamespaceFilter(e.target.value)}
                  className="w-48 bg-stone-50 dark:bg-slate-800 border border-stone-200 dark:border-slate-700 rounded-lg px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-accent-500/50"
                />
              )}
              <button
                onClick={fetchSecurityResources}
                disabled={loading || !isConnected}
                className={REFRESH_BTN_CLASS}
              >
                Refresh
              </button>
            </div>
          </div>

          <div className="flex-1 bg-white dark:bg-slate-800/50 rounded-xl border border-stone-200 dark:border-slate-700/50 overflow-hidden relative shadow-sm dark:shadow-none transition-colors">
            {!isConnected ? (
              <p className="text-slate-400 text-center py-12">
                Connect to a cluster to view security resources
              </p>
            ) : loading ? (
              <div className="flex items-center justify-center py-12">
                <div className="w-6 h-6 border-2 border-accent-400 border-t-transparent rounded-full animate-spin" />
              </div>
            ) : error ? (
              <p className="text-red-400 text-center py-12">{error}</p>
            ) : currentResources.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-12 gap-2">
                <p className="text-slate-400 text-center">
                  No {activeTab.replace(/([A-Z])/g, " $1").toLowerCase()} found
                </p>
                {!showSystem && (
                  <p className="text-xs text-slate-500">
                    System objects are hidden.{" "}
                    <button
                      onClick={() => setShowSystem(true)}
                      className="text-accent-400 hover:underline"
                    >
                      Show them
                    </button>
                  </p>
                )}
              </div>
            ) : (
              <div ref={tableRef} className="overflow-auto h-full pb-20">
                <table className="w-full text-sm">
                  <thead className="bg-stone-50 dark:bg-slate-900 sticky top-0 z-10 transition-colors">
                    <tr className="text-left text-stone-500 dark:text-slate-400">
                      <th className="px-4 py-3 font-medium">Name</th>
                      {!isClusterScoped && (
                        <th className="px-4 py-3 font-medium">Namespace</th>
                      )}
                      <th className="px-4 py-3 font-medium">Age</th>
                      <th className="px-4 py-3 font-medium w-10"></th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-stone-200 dark:divide-slate-700/50">
                    {currentResources.map((resource, idx) => (
                      <tr
                        key={`${resource.namespace}-${resource.name}-${idx}`}
                        data-index={idx}
                        className={`transition-colors group ${
                          selectedIndex === idx
                            ? "bg-accent-500/20 ring-1 ring-accent-500/50"
                            : "hover:bg-stone-50 dark:hover:bg-slate-700/30"
                        }`}
                        onMouseLeave={() => setActiveMenuIndex(-1)}
                      >
                        <td className="px-4 py-3 font-mono text-accent-600 dark:text-accent-400">
                          {resource.name}
                          {isSystemResource(resource) && (
                            <span className="ml-2 text-[10px] px-1.5 py-0.5 bg-stone-200 dark:bg-slate-700 text-stone-600 dark:text-slate-400 rounded-full border border-stone-300 dark:border-slate-600">
                              System
                            </span>
                          )}
                        </td>
                        {!isClusterScoped && (
                          <td className="px-4 py-3 text-stone-500 dark:text-slate-400">
                            {resource.namespace || "-"}
                          </td>
                        )}
                        <td className="px-4 py-3 text-stone-500 dark:text-slate-500">
                          {resource.age}
                        </td>
                        <td className="px-4 py-3 relative">
                          <button
                            onClick={(e) => {
                              e.stopPropagation();
                              setActiveMenuIndex(
                                activeMenuIndex === idx ? -1 : idx
                              );
                            }}
                            className="p-1 hover:bg-stone-200 dark:hover:bg-slate-600 rounded opacity-0 group-hover:opacity-100 transition-opacity"
                          >
                            <MoreVertical
                              size={14}
                              className="text-stone-400 dark:text-slate-400"
                            />
                          </button>
                          {activeMenuIndex === idx && (
                            <div className="absolute right-8 top-0 bg-white dark:bg-slate-800 border border-stone-200 dark:border-slate-700 rounded-lg shadow-xl z-50 w-48 overflow-hidden">
                              <button
                                onClick={async (e) => {
                                  e.stopPropagation();
                                  setActiveMenuIndex(-1);
                                  setYamlLoading(true);
                                  setYamlResource(resource);
                                  try {
                                    const yaml = await GetResourceYAML(
                                      resource.kind,
                                      resource.namespace,
                                      resource.name
                                    );
                                    setYamlContent(yaml);
                                  } catch (err) {
                                    console.error("Failed to fetch YAML", err);
                                  } finally {
                                    setYamlLoading(false);
                                  }
                                }}
                                className="w-full text-left px-4 py-2.5 text-sm text-stone-600 dark:text-slate-300 hover:bg-stone-100 dark:hover:bg-slate-700 flex items-center gap-2"
                              >
                                <FileText size={14} /> View YAML
                              </button>
                              <button
                                onClick={(e) => {
                                  e.stopPropagation();
                                  setActiveMenuIndex(-1);
                                  setAnalyzingResource(resource);
                                }}
                                className="w-full text-left px-4 py-2.5 text-sm text-purple-600 dark:text-purple-400 hover:bg-stone-100 dark:hover:bg-slate-700 flex items-center gap-2 border-t border-stone-200 dark:border-slate-700"
                              >
                                <Sparkles size={14} /> AI Security Audit
                              </button>
                            </div>
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>

          {/* Analysis Modal */}
          {analyzingResource && (
            <AnalysisModal
              isOpen={!!analyzingResource}
              onClose={() => setAnalyzingResource(null)}
              resource={analyzingResource}
            />
          )}

          {/* YAML Viewer Modal */}
          <AnimatePresence>
            {yamlResource && (
              <motion.div
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                className="fixed inset-0 z-[60] bg-black/60 backdrop-blur-sm flex items-center justify-center p-4"
                onClick={() => setYamlResource(null)}
              >
                <motion.div
                  initial={{ opacity: 0, scale: 0.95 }}
                  animate={{ opacity: 1, scale: 1 }}
                  exit={{ opacity: 0, scale: 0.95 }}
                  onClick={(e) => e.stopPropagation()}
                  className="bg-slate-900 border border-slate-700 rounded-xl shadow-2xl w-full max-w-4xl h-[80vh] flex flex-col overflow-hidden"
                >
                  <div className="flex items-center justify-between p-4 border-b border-slate-700 bg-slate-900/50">
                    <div>
                      <h3 className="text-lg font-semibold text-slate-100 flex items-center gap-2">
                        <FileText size={18} className="text-accent-400" />
                        YAML Viewer
                      </h3>
                      <p className="text-xs text-slate-400 font-mono">
                        {yamlResource.kind} / {yamlResource.namespace} /{" "}
                        {yamlResource.name}
                      </p>
                    </div>
                    <button
                      onClick={() => setYamlResource(null)}
                      className="p-1 hover:bg-slate-800 rounded-lg text-slate-400 hover:text-slate-200"
                    >
                      <X size={20} />
                    </button>
                  </div>
                  <div className="flex-1 overflow-auto p-4 bg-slate-950 font-mono text-xs text-slate-300">
                    {yamlLoading ? (
                      <div className="flex items-center justify-center h-full">
                        <div className="w-6 h-6 border-2 border-accent-400 border-t-transparent rounded-full animate-spin" />
                      </div>
                    ) : (
                      <pre>{yamlContent}</pre>
                    )}
                  </div>
                </motion.div>
              </motion.div>
            )}
          </AnimatePresence>

          <div className="mt-3 text-sm text-slate-500">
            {currentResources.length}{" "}
            {activeTab.replace(/([A-Z])/g, " $1").toLowerCase()}
          </div>
        </>
      )}
    </div>
  );
}
