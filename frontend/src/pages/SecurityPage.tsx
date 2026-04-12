/**
 * SecurityPage — RBAC and security scanner view.
 *
 * Extracted from App.tsx SecurityView.
 * Modes: RBAC Analysis, Resource Browser (roles/bindings/etc.)
 * Tabs: roles, clusterroles, rolebindings, clusterrolebindings, networkpolicies, serviceaccounts
 */

import { useState, useEffect, useRef } from "react";
import { motion, AnimatePresence } from "framer-motion";
import type { LucideIcon } from "lucide-react";
import {
  Shield,
  RefreshCw,
  Search,
  X,
  AlertCircle,
  AlertTriangle,
  Check,
  Eye,
  Copy,
  Users,
  Lock,
  Network,
} from "lucide-react";
import {
  ListResources,
  GetRBACAnalysis,
  GetResourceYAML,
} from "../../wailsjs/go/main/App";
import type { ResourceInfo } from "../types/resources";

// ── Types ────────────────────────────────────────────────────────────────────

interface SecurityPageProps {
  isConnected: boolean;
}

type SecurityMode = "rbac" | "resources";

type ResourceTab =
  | "roles"
  | "clusterroles"
  | "rolebindings"
  | "clusterrolebindings"
  | "networkpolicies"
  | "serviceaccounts";

interface RBACSubject {
  kind: string;
  name: string;
  namespace?: string;
  roles: string[];
  clusterRoles: string[];
  isAdmin: boolean;
  dangerousPermissions: string[];
}

interface RBACBinding {
  name: string;
  namespace?: string;
  kind: string;
  roleName: string;
  roleKind: string;
  subjects: { kind: string; name: string; namespace?: string }[];
  isDangerous: boolean;
  dangerousRules: string[];
}

interface RBACSummary {
  subjects: RBACSubject[];
  bindings: RBACBinding[];
  dangerousBindings: RBACBinding[];
  clusterAdmins: string[];
}

// ── Component ─────────────────────────────────────────────────────────────────

export function SecurityPage({ isConnected }: SecurityPageProps) {
  const [mode, setMode] = useState<SecurityMode>("rbac");
  const [activeTab, setActiveTab] = useState<ResourceTab>("roles");
  const [namespaceFilter, _setNamespaceFilter] = useState("");
  const [searchFilter, setSearchFilter] = useState("");
  const [showSystem, setShowSystem] = useState(false);
  const filterRef = useRef<HTMLInputElement>(null);
  const tableRef = useRef<HTMLDivElement>(null);

  // Resource lists
  const [roles, setRoles] = useState<ResourceInfo[]>([]);
  const [clusterRoles, setClusterRoles] = useState<ResourceInfo[]>([]);
  const [roleBindings, setRoleBindings] = useState<ResourceInfo[]>([]);
  const [clusterRoleBindings, setClusterRoleBindings] = useState<ResourceInfo[]>([]);
  const [networkPolicies, setNetworkPolicies] = useState<ResourceInfo[]>([]);
  const [serviceAccounts, setServiceAccounts] = useState<ResourceInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // RBAC analysis
  const [rbacSummary, setRbacSummary] = useState<RBACSummary | null>(null);
  const [rbacLoading, setRbacLoading] = useState(false);
  const [rbacError, setRbacError] = useState<string | null>(null);
  const [rbacView, setRbacView] = useState<"subjects" | "bindings" | "dangerous">("subjects");

  // YAML modal
  const [yamlResource, setYamlResource] = useState<ResourceInfo | null>(null);
  const [yamlContent, setYamlContent] = useState("");
  const [yamlLoading, setYamlLoading] = useState(false);
  const [selectedIndex, setSelectedIndex] = useState(-1);

  const isSystemResource = (r: ResourceInfo) =>
    ["kube-system", "kube-public", "kube-node-lease"].includes(r.namespace ?? "") ||
    r.name.startsWith("system:") ||
    r.name.startsWith("kube-");

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
      setError(err instanceof Error ? err.message : "Failed to fetch security resources");
    } finally {
      setLoading(false);
    }
  };

  const fetchRBACAnalysis = async () => {
    setRbacLoading(true);
    setRbacError(null);
    try {
      const result = await GetRBACAnalysis();
      setRbacSummary(result as unknown as RBACSummary);
    } catch (err) {
      setRbacError(err instanceof Error ? err.message : "Failed to analyze RBAC");
    } finally {
      setRbacLoading(false);
    }
  };

  useEffect(() => {
    if (!isConnected) {
      setRoles([]); setClusterRoles([]); setRoleBindings([]);
      setClusterRoleBindings([]); setNetworkPolicies([]); setServiceAccounts([]);
      setRbacSummary(null);
      return;
    }
    if (mode === "resources") fetchSecurityResources();
    else fetchRBACAnalysis();
  }, [isConnected, namespaceFilter, mode]);

  const getCurrentList = (): ResourceInfo[] => {
    if (mode !== "resources") return [];
    const map: Record<ResourceTab, ResourceInfo[]> = {
      roles, clusterroles: clusterRoles, rolebindings: roleBindings,
      clusterrolebindings: clusterRoleBindings, networkpolicies: networkPolicies,
      serviceaccounts: serviceAccounts,
    };
    return map[activeTab] || [];
  };

  const currentList = getCurrentList();
  const filteredList = currentList.filter((r) => {
    if (!showSystem && isSystemResource(r)) return false;
    if (searchFilter && !r.name.toLowerCase().includes(searchFilter.toLowerCase())) return false;
    return true;
  });

  useEffect(() => { setSelectedIndex(-1); }, [activeTab, mode]);

  // Keyboard shortcuts
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement;
      if (t.tagName === "INPUT" || t.tagName === "TEXTAREA" || t.isContentEditable) return;
      switch (e.key) {
        case "/":
          e.preventDefault();
          filterRef.current?.focus();
          break;
        case "r":
          if (!loading && !rbacLoading && isConnected) {
            e.preventDefault();
            if (mode === "resources") { fetchSecurityResources(); } else { fetchRBACAnalysis(); }
          }
          break;
        case "ArrowDown": case "j":
          if (mode !== "resources") return;
          e.preventDefault();
          setSelectedIndex((p) => Math.min(p + 1, filteredList.length - 1));
          break;
        case "ArrowUp": case "k":
          if (mode !== "resources") return;
          e.preventDefault();
          setSelectedIndex((p) => Math.max(p - 1, 0));
          break;
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [filteredList.length, mode, loading, rbacLoading, isConnected]);

  const openYaml = async (resource: ResourceInfo, kind: string) => {
    setYamlResource(resource);
    setYamlLoading(true);
    setYamlContent("");
    try {
      const yaml = await GetResourceYAML(kind, resource.namespace ?? "", resource.name);
      setYamlContent(yaml);
    } catch (err) {
      setYamlContent(`Error: ${err instanceof Error ? err.message : "Failed"}`);
    } finally {
      setYamlLoading(false);
    }
  };

  const TABS: { id: ResourceTab; label: string; count: number; icon: typeof Shield }[] = [
    { id: "roles", label: "Roles", count: roles.length, icon: Lock },
    { id: "clusterroles", label: "ClusterRoles", count: clusterRoles.length, icon: Lock },
    { id: "rolebindings", label: "RoleBindings", count: roleBindings.length, icon: Users },
    { id: "clusterrolebindings", label: "ClusterRoleBindings", count: clusterRoleBindings.length, icon: Users },
    { id: "networkpolicies", label: "NetworkPolicies", count: networkPolicies.length, icon: Network },
    { id: "serviceaccounts", label: "ServiceAccounts", count: serviceAccounts.length, icon: Shield },
  ];

  return (
    <div className="flex flex-col h-full gap-4">
      {/* Mode switcher */}
      <div className="flex items-center gap-3">
        <div className="flex gap-1 p-1 bg-slate-800/40 rounded-xl border border-slate-700/40">
          {(["rbac", "resources"] as SecurityMode[]).map((m) => (
            <button
              key={m}
              onClick={() => setMode(m)}
              className={`
                px-3 py-1.5 rounded-lg text-xs font-medium transition-colors
                focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500/50
                ${mode === m
                  ? "bg-accent-500/15 text-accent-400 border border-accent-500/25"
                  : "text-slate-400 hover:text-slate-200 hover:bg-slate-700/40"
                }
              `}
            >
              {m === "rbac" ? "RBAC Analysis" : "Resource Browser"}
            </button>
          ))}
        </div>

        {/* Refresh */}
        <button
          onClick={() => mode === "resources" ? fetchSecurityResources() : fetchRBACAnalysis()}
          disabled={loading || rbacLoading || !isConnected}
          className="
            flex items-center gap-1.5 h-8 px-3 rounded-lg
            bg-slate-800/40 border border-slate-700/50
            text-xs text-slate-400 hover:text-slate-200
            hover:bg-slate-700/50 disabled:opacity-40 transition-colors
            focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500/50
          "
          aria-label="Refresh"
        >
          <RefreshCw size={13} className={(loading || rbacLoading) ? "animate-spin" : ""} aria-hidden="true" />
          Refresh
        </button>
      </div>

      {/* Not connected */}
      {!isConnected && (
        <div className="flex-1 flex items-center justify-center">
          <div className="text-center space-y-2">
            <Shield size={32} className="text-slate-600 mx-auto" aria-hidden="true" />
            <p className="text-slate-400 text-sm">Not connected to a cluster</p>
          </div>
        </div>
      )}

      {/* RBAC Analysis Mode */}
      {isConnected && mode === "rbac" && (
        <div className="flex-1 overflow-hidden flex flex-col gap-3 min-h-0">
          {rbacLoading && !rbacSummary && (
            <div className="flex-1 flex items-center justify-center">
              <div className="flex items-center gap-2 text-slate-500">
                <RefreshCw size={16} className="animate-spin" aria-hidden="true" />
                Analyzing RBAC...
              </div>
            </div>
          )}

          {rbacError && (
            <div className="flex items-center gap-2 px-3 py-2.5 rounded-lg bg-red-500/10 border border-red-500/20 text-red-400 text-xs">
              <AlertCircle size={14} aria-hidden="true" />
              {rbacError}
            </div>
          )}

          {rbacSummary && (
            <>
              {/* Summary stats */}
              <div className="grid grid-cols-3 gap-3">
                <StatCard
                  label="Cluster Admins"
                  value={rbacSummary.clusterAdmins?.length ?? 0}
                  color={rbacSummary.clusterAdmins?.length > 2 ? "amber" : "slate"}
                  icon={Shield}
                />
                <StatCard
                  label="Dangerous Bindings"
                  value={rbacSummary.dangerousBindings?.length ?? 0}
                  color={rbacSummary.dangerousBindings?.length > 0 ? "red" : "emerald"}
                  icon={AlertTriangle}
                />
                <StatCard
                  label="Total Bindings"
                  value={rbacSummary.bindings?.length ?? 0}
                  color="slate"
                  icon={Users}
                />
              </div>

              {/* RBAC sub-tabs */}
              <div className="flex gap-1">
                {(["subjects", "bindings", "dangerous"] as const).map((v) => (
                  <button
                    key={v}
                    onClick={() => setRbacView(v)}
                    className={`
                      px-3 py-1.5 rounded-lg text-xs font-medium transition-colors capitalize
                      focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500/50
                      ${rbacView === v
                        ? "bg-accent-500/15 text-accent-400 border border-accent-500/25"
                        : "text-slate-500 hover:text-slate-300 bg-slate-800/30 border border-slate-700/30"
                      }
                    `}
                  >
                    {v}
                    {v === "dangerous" && (rbacSummary.dangerousBindings?.length ?? 0) > 0 && (
                      <span className="ml-1.5 px-1.5 py-0.5 bg-red-500/20 text-red-400 text-[9px] rounded-full">
                        {rbacSummary.dangerousBindings.length}
                      </span>
                    )}
                  </button>
                ))}
              </div>

              {/* RBAC content */}
              <div ref={tableRef} className="flex-1 overflow-auto rounded-xl border border-slate-700/40 bg-slate-800/20 min-h-0">
                {rbacView === "subjects" && (
                  <table className="w-full text-xs" aria-label="RBAC subjects">
                    <thead className="sticky top-0 bg-slate-900/90 backdrop-blur-sm">
                      <tr className="border-b border-slate-700/40">
                        <th className="px-3 py-2 text-left text-slate-500 font-medium">Subject</th>
                        <th className="px-3 py-2 text-left text-slate-500 font-medium">Kind</th>
                        <th className="px-3 py-2 text-left text-slate-500 font-medium">Roles</th>
                        <th className="px-3 py-2 text-left text-slate-500 font-medium">Risk</th>
                      </tr>
                    </thead>
                    <tbody>
                      {(rbacSummary.subjects ?? []).map((subj) => (
                        <tr key={subj.name} className="border-b border-slate-700/20 hover:bg-slate-700/20 transition-colors">
                          <td className="px-3 py-2 font-mono text-slate-200">{subj.name}</td>
                          <td className="px-3 py-2 text-slate-400">{subj.kind}</td>
                          <td className="px-3 py-2">
                            <div className="flex flex-wrap gap-1">
                              {[...(subj.roles ?? []), ...(subj.clusterRoles ?? [])].slice(0, 3).map((r) => (
                                <span key={r} className="px-1.5 py-0.5 bg-slate-700/50 text-slate-400 rounded text-[10px] font-mono">{r}</span>
                              ))}
                              {((subj.roles?.length ?? 0) + (subj.clusterRoles?.length ?? 0)) > 3 && (
                                <span className="px-1.5 py-0.5 text-slate-600 text-[10px]">+{(subj.roles?.length ?? 0) + (subj.clusterRoles?.length ?? 0) - 3} more</span>
                              )}
                            </div>
                          </td>
                          <td className="px-3 py-2">
                            {subj.isAdmin ? (
                              <span className="flex items-center gap-1 text-red-400 text-[10px]">
                                <AlertTriangle size={10} aria-hidden="true" />
                                cluster-admin
                              </span>
                            ) : (subj.dangerousPermissions?.length ?? 0) > 0 ? (
                              <span className="flex items-center gap-1 text-amber-400 text-[10px]">
                                <AlertCircle size={10} aria-hidden="true" />
                                {subj.dangerousPermissions.length} risk{subj.dangerousPermissions.length !== 1 ? "s" : ""}
                              </span>
                            ) : (
                              <span className="text-emerald-500 text-[10px]">ok</span>
                            )}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}

                {rbacView === "bindings" && (
                  <table className="w-full text-xs" aria-label="RBAC bindings">
                    <thead className="sticky top-0 bg-slate-900/90 backdrop-blur-sm">
                      <tr className="border-b border-slate-700/40">
                        <th className="px-3 py-2 text-left text-slate-500 font-medium">Binding</th>
                        <th className="px-3 py-2 text-left text-slate-500 font-medium">Role</th>
                        <th className="px-3 py-2 text-left text-slate-500 font-medium">Subjects</th>
                        <th className="px-3 py-2 text-left text-slate-500 font-medium">Namespace</th>
                      </tr>
                    </thead>
                    <tbody>
                      {(rbacSummary.bindings ?? []).map((b) => (
                        <tr key={`${b.namespace}-${b.name}`} className={`border-b border-slate-700/20 hover:bg-slate-700/20 transition-colors ${b.isDangerous ? "bg-red-500/5" : ""}`}>
                          <td className="px-3 py-2 font-mono text-slate-200">
                            <div className="flex items-center gap-1">
                              {b.isDangerous && <AlertTriangle size={11} className="text-red-400 flex-shrink-0" aria-hidden="true" />}
                              {b.name}
                            </div>
                          </td>
                          <td className="px-3 py-2 text-slate-400 font-mono text-[10px]">{b.roleKind}/{b.roleName}</td>
                          <td className="px-3 py-2 text-slate-500 text-[10px]">
                            {(b.subjects ?? []).slice(0, 2).map((s) => `${s.kind}:${s.name}`).join(", ")}
                            {(b.subjects?.length ?? 0) > 2 && ` +${b.subjects.length - 2} more`}
                          </td>
                          <td className="px-3 py-2 text-slate-500 font-mono">{b.namespace || "cluster"}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}

                {rbacView === "dangerous" && (
                  <div className="p-4 space-y-3">
                    {(rbacSummary.dangerousBindings ?? []).length === 0 ? (
                      <div className="flex items-center justify-center py-12 gap-2 text-emerald-500">
                        <Check size={16} aria-hidden="true" />
                        <span className="text-sm">No dangerous bindings found</span>
                      </div>
                    ) : (
                      (rbacSummary.dangerousBindings ?? []).map((b) => (
                        <div key={`${b.namespace}-${b.name}`} className="rounded-xl border border-red-500/25 bg-red-500/5 p-3">
                          <div className="flex items-start gap-2">
                            <AlertTriangle size={14} className="text-red-400 flex-shrink-0 mt-0.5" aria-hidden="true" />
                            <div className="flex-1">
                              <p className="text-xs font-semibold text-red-300 font-mono">{b.name}</p>
                              <p className="text-[11px] text-slate-400 mt-0.5">
                                {b.roleKind}/{b.roleName} • {b.namespace || "cluster-scoped"}
                              </p>
                              {(b.dangerousRules ?? []).map((rule) => (
                                <p key={rule} className="text-[10px] text-red-400/70 mt-1 font-mono">{rule}</p>
                              ))}
                            </div>
                          </div>
                        </div>
                      ))
                    )}
                  </div>
                )}
              </div>
            </>
          )}
        </div>
      )}

      {/* Resource Browser Mode */}
      {isConnected && mode === "resources" && (
        <div className="flex-1 overflow-hidden flex flex-col gap-3 min-h-0">
          {/* Toolbar */}
          <div className="flex items-center gap-2 flex-wrap">
            {/* Search */}
            <div className="relative">
              <Search size={12} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-slate-500 pointer-events-none" aria-hidden="true" />
              <input
                ref={filterRef}
                type="search"
                placeholder="Filter... (/)"
                value={searchFilter}
                onChange={(e) => setSearchFilter(e.target.value)}
                className="
                  h-8 pl-7 pr-3 rounded-lg w-48
                  bg-slate-800/40 border border-slate-700/50
                  text-xs text-slate-200 placeholder-slate-500
                  focus:outline-none focus:ring-2 focus:ring-accent-500/50
                "
                aria-label="Filter resources"
              />
            </div>

            {/* System toggle */}
            <button
              onClick={() => setShowSystem(!showSystem)}
              className={`
                flex items-center gap-1.5 h-8 px-3 rounded-lg text-xs transition-colors
                focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500/50
                ${showSystem
                  ? "bg-accent-500/15 border border-accent-500/25 text-accent-400"
                  : "bg-slate-800/40 border border-slate-700/50 text-slate-400 hover:text-slate-200 hover:bg-slate-700/50"
                }
              `}
              aria-pressed={showSystem}
            >
              {showSystem ? <Eye size={12} aria-hidden="true" /> : <Eye size={12} aria-hidden="true" />}
              System
            </button>
          </div>

          {/* Tabs */}
          <div className="flex gap-1 flex-wrap" role="tablist" aria-label="Security resource types">
            {TABS.map((tab) => {
              const Icon = tab.icon;
              const isActive = activeTab === tab.id;
              return (
                <button
                  key={tab.id}
                  role="tab"
                  aria-selected={isActive}
                  onClick={() => setActiveTab(tab.id)}
                  className={`
                    flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors
                    focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500/50
                    ${isActive
                      ? "bg-accent-500/15 text-accent-400 border border-accent-500/25"
                      : "text-slate-500 hover:text-slate-300 bg-slate-800/30 border border-slate-700/30"
                    }
                  `}
                >
                  <Icon size={11} aria-hidden="true" />
                  {tab.label}
                  <span className={`text-[10px] px-1 rounded ${isActive ? "text-accent-500/70" : "text-slate-600"}`}>
                    {tab.count}
                  </span>
                </button>
              );
            })}
          </div>

          {/* Error */}
          {error && (
            <div className="flex items-center gap-2 px-3 py-2.5 rounded-lg bg-red-500/10 border border-red-500/20 text-red-400 text-xs">
              <AlertCircle size={14} aria-hidden="true" />
              {error}
            </div>
          )}

          {/* Table */}
          <div ref={tableRef} className="flex-1 overflow-auto rounded-xl border border-slate-700/40 bg-slate-800/20 min-h-0">
            {loading ? (
              <div className="flex items-center justify-center h-full">
                <div className="flex items-center gap-2 text-slate-500 text-sm">
                  <RefreshCw size={15} className="animate-spin" aria-hidden="true" />
                  Loading...
                </div>
              </div>
            ) : filteredList.length === 0 ? (
              <div className="flex items-center justify-center h-full">
                <p className="text-slate-500 text-sm">No {activeTab} found</p>
              </div>
            ) : (
              <table className="w-full text-xs" role="grid" aria-label={activeTab}>
                <thead className="sticky top-0 bg-slate-900/90 backdrop-blur-sm">
                  <tr className="border-b border-slate-700/40">
                    <th className="px-3 py-2 text-left text-slate-500 font-medium">Name</th>
                    {!["clusterroles", "clusterrolebindings"].includes(activeTab) && (
                      <th className="px-3 py-2 text-left text-slate-500 font-medium">Namespace</th>
                    )}
                    <th className="px-3 py-2 text-left text-slate-500 font-medium">Age</th>
                    <th className="px-3 py-2 w-16" />
                  </tr>
                </thead>
                <tbody>
                  {filteredList.map((resource, idx) => {
                    const isSelected = idx === selectedIndex;
                    return (
                      <tr
                        key={`${resource.namespace}-${resource.name}`}
                        data-index={idx}
                        className={`
                          border-b border-slate-700/20 transition-colors cursor-pointer
                          ${isSelected ? "bg-accent-500/10" : "hover:bg-slate-700/20"}
                        `}
                        onClick={() => setSelectedIndex(idx)}
                        role="row"
                        aria-selected={isSelected}
                      >
                        <td className="px-3 py-2 font-mono text-slate-200">{resource.name}</td>
                        {!["clusterroles", "clusterrolebindings"].includes(activeTab) && (
                          <td className="px-3 py-2 text-slate-500 font-mono">{resource.namespace || "-"}</td>
                        )}
                        <td className="px-3 py-2 text-slate-500">{resource.age}</td>
                        <td className="px-2 py-2">
                          <button
                            onClick={(e) => { e.stopPropagation(); openYaml(resource, activeTab); }}
                            className="p-1 rounded text-slate-600 hover:text-slate-300 hover:bg-slate-700/60 transition-colors"
                            aria-label={`View YAML for ${resource.name}`}
                          >
                            <Eye size={12} aria-hidden="true" />
                          </button>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            )}
          </div>

          <p className="text-[11px] text-slate-500 text-right">
            {filteredList.length} of {currentList.length} {activeTab}
          </p>
        </div>
      )}

      {/* YAML modal */}
      <AnimatePresence>
        {yamlResource && (
          <>
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              className="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm"
              onClick={() => setYamlResource(null)}
              aria-hidden="true"
            />
            <motion.div
              initial={{ opacity: 0, scale: 0.97 }}
              animate={{ opacity: 1, scale: 1 }}
              exit={{ opacity: 0, scale: 0.97 }}
              transition={{ duration: 0.18 }}
              className="fixed inset-4 md:inset-12 z-50 flex flex-col"
              role="dialog"
              aria-modal="true"
              aria-label={`YAML: ${yamlResource.name}`}
            >
              <div className="flex-1 flex flex-col bg-slate-900/98 backdrop-blur-xl border border-slate-700/50 rounded-2xl shadow-2xl overflow-hidden">
                <div className="flex items-center gap-3 px-4 py-3 border-b border-slate-700/40 flex-shrink-0">
                  <span className="text-sm font-mono font-semibold text-slate-200">{yamlResource.name}</span>
                  <div className="ml-auto flex items-center gap-2">
                    <button
                      onClick={() => navigator.clipboard.writeText(yamlContent)}
                      className="flex items-center gap-1 px-2 py-1 rounded-lg text-xs text-slate-400 hover:text-slate-200 hover:bg-slate-700/60 transition-colors"
                      aria-label="Copy YAML"
                    >
                      <Copy size={12} aria-hidden="true" />
                      Copy
                    </button>
                    <button
                      onClick={() => setYamlResource(null)}
                      className="p-1.5 rounded-lg text-slate-500 hover:text-slate-300 hover:bg-slate-700/60 transition-colors"
                      aria-label="Close"
                    >
                      <X size={15} aria-hidden="true" />
                    </button>
                  </div>
                </div>
                <div className="flex-1 overflow-auto p-4">
                  {yamlLoading ? (
                    <div className="flex items-center justify-center h-full">
                      <RefreshCw size={18} className="animate-spin text-slate-500" aria-hidden="true" />
                    </div>
                  ) : (
                    <pre className="text-xs font-mono text-slate-300 leading-relaxed whitespace-pre-wrap break-all">
                      {yamlContent}
                    </pre>
                  )}
                </div>
              </div>
            </motion.div>
          </>
        )}
      </AnimatePresence>
    </div>
  );
}

// ── StatCard ──────────────────────────────────────────────────────────────────

const STAT_COLORS = {
  red: "border-red-500/25 bg-red-500/5 text-red-400",
  amber: "border-amber-500/25 bg-amber-500/5 text-amber-400",
  emerald: "border-emerald-500/25 bg-emerald-500/5 text-emerald-400",
  slate: "border-slate-700/40 bg-slate-800/30 text-slate-300",
};

function StatCard({
  label,
  value,
  color,
  icon: Icon,
}: {
  label: string;
  value: number;
  color: keyof typeof STAT_COLORS;
  icon: LucideIcon;
}) {
  const cls = STAT_COLORS[color];
  return (
    <div className={`rounded-xl border p-3 ${cls}`}>
      <div className="flex items-center gap-2 mb-1">
        <Icon size={13} className="flex-shrink-0" aria-hidden="true" />
        <span className="text-[10px] font-medium uppercase tracking-wider opacity-70">{label}</span>
      </div>
      <p className="text-2xl font-bold font-mono">{value}</p>
    </div>
  );
}

