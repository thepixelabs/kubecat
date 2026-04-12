import { useState, useEffect, useRef } from "react";
import { ArrowUpDown, ChevronUp, ChevronDown } from "lucide-react";
import { ListResources } from "../../../wailsjs/go/main/App";
import type { ResourceInfo } from "../../types/resources";

const REFRESH_BTN_CLASS =
  "px-3 py-1.5 text-sm bg-slate-200 dark:bg-slate-700 hover:bg-slate-300 dark:hover:bg-slate-600 text-slate-700 dark:text-slate-200 rounded-lg transition-colors disabled:opacity-50";

export function GitOpsView({ isConnected }: { isConnected: boolean }) {
  const [applications, setApplications] = useState<ResourceInfo[]>([]);
  const [helmReleases, setHelmReleases] = useState<ResourceInfo[]>([]);
  const [kustomizations, setKustomizations] = useState<ResourceInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<
    "argo" | "flux-helm" | "flux-kustomize"
  >("flux-helm");
  const [namespaceFilter, setNamespaceFilter] = useState("");
  const [namespaces, setNamespaces] = useState<string[]>([]);
  const [selectedIndex, setSelectedIndex] = useState<number>(-1);
  const tableRef = useRef<HTMLDivElement>(null);
  const filterRef = useRef<HTMLInputElement>(null);
  const [showNamespaceDropdown, setShowNamespaceDropdown] = useState(false);

  // Sorting
  const [sortField, setSortField] = useState<
    "name" | "namespace" | "status" | "age"
  >("name");
  const [sortDirection, setSortDirection] = useState<"asc" | "desc">("asc");

  const handleSort = (field: "name" | "namespace" | "status" | "age") => {
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

  const fetchGitOpsResources = async () => {
    setLoading(true);
    setError(null);
    try {
      const ns = namespaceFilter;
      const [argoApps, helm, kust, nsList] = await Promise.allSettled([
        ListResources("applications", ns),
        ListResources("helmreleases", ns),
        ListResources("kustomizations", ns),
        ListResources("namespaces", ""),
      ]);

      if (argoApps.status === "fulfilled")
        setApplications(argoApps.value || []);
      if (helm.status === "fulfilled") {
        setHelmReleases(helm.value || []);
      }
      if (kust.status === "fulfilled") setKustomizations(kust.value || []);
      if (nsList.status === "fulfilled") {
        setNamespaces((nsList.value || []).map((n: ResourceInfo) => n.name));
      }
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to fetch GitOps resources"
      );
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (isConnected) {
      fetchGitOpsResources();
    } else {
      setApplications([]);
      setHelmReleases([]);
      setKustomizations([]);
      setNamespaces([]);
    }
  }, [isConnected, namespaceFilter]);

  // Get current list based on active tab
  const currentList =
    activeTab === "argo"
      ? applications
      : activeTab === "flux-helm"
      ? helmReleases
      : kustomizations;

  // Reset selection when tab or data changes
  useEffect(() => {
    setSelectedIndex(-1);
  }, [activeTab, currentList.length]);

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
          e.preventDefault();
          filterRef.current?.focus();
          break;
        case "r":
          if (!loading && isConnected) {
            e.preventDefault();
            fetchGitOpsResources();
          }
          break;
        case "ArrowDown":
        case "j":
          e.preventDefault();
          setSelectedIndex((prev) =>
            prev < currentList.length - 1 ? prev + 1 : prev
          );
          break;
        case "ArrowUp":
        case "k":
          e.preventDefault();
          setSelectedIndex((prev) => (prev > 0 ? prev - 1 : 0));
          break;
      }

      // Shift+letter sorting shortcuts
      if (e.shiftKey) {
        switch (e.key) {
          case "N":
            e.preventDefault();
            handleSort("name");
            break;
          case "M":
            e.preventDefault();
            handleSort("namespace");
            break;
          case "S":
            e.preventDefault();
            handleSort("status");
            break;
          case "A":
            e.preventDefault();
            handleSort("age");
            break;
        }
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [currentList.length, loading, isConnected, sortField]);

  // Scroll selected row into view
  useEffect(() => {
    if (selectedIndex >= 0 && tableRef.current) {
      const row = tableRef.current.querySelector(
        `tr[data-index="${selectedIndex}"]`
      );
      row?.scrollIntoView({ block: "nearest", behavior: "smooth" });
    }
  }, [selectedIndex]);

  const getStatusColor = (status: string) => {
    const s = status?.toLowerCase() || "";
    if (
      s.includes("healthy") ||
      s.includes("ready") ||
      s.includes("synced") ||
      s.includes("released")
    )
      return "text-emerald-400";
    if (
      s.includes("progressing") ||
      s.includes("pending") ||
      s.includes("reconciling")
    )
      return "text-yellow-400";
    if (s.includes("degraded") || s.includes("failed") || s.includes("error"))
      return "text-red-400";
    return "text-stone-400 dark:text-slate-400";
  };

  const tabs = [
    {
      id: "argo" as const,
      label: "ArgoCD Applications",
      count: applications.length,
    },
    {
      id: "flux-helm" as const,
      label: "Helm Releases",
      count: helmReleases.length,
    },
    {
      id: "flux-kustomize" as const,
      label: "Kustomizations",
      count: kustomizations.length,
    },
  ];

  const currentResources =
    activeTab === "argo"
      ? applications
      : activeTab === "flux-helm"
      ? helmReleases
      : kustomizations;

  const hasAnyResources =
    applications.length > 0 ||
    helmReleases.length > 0 ||
    kustomizations.length > 0;

  return (
    <div className="h-full flex flex-col">
      <div className="flex items-center justify-between mb-4">
        <div className="flex gap-2">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`px-4 py-2 text-sm rounded-lg border transition-colors ${
                activeTab === tab.id
                  ? "bg-accent-500/20 border-accent-500/50 text-accent-400"
                  : "bg-white dark:bg-slate-800/50 border-stone-200 dark:border-slate-700/50 hover:bg-stone-50 dark:hover:bg-slate-700/50 text-stone-600 dark:text-slate-400"
              }`}
            >
              {tab.label}
              {tab.count > 0 && (
                <span className="ml-2 px-1.5 py-0.5 text-xs bg-stone-200 dark:bg-slate-700 rounded text-stone-600 dark:text-slate-300">
                  {tab.count}
                </span>
              )}
            </button>
          ))}
        </div>
        <div className="flex items-center gap-2">
          <div className="relative">
            <button
              onClick={() => setShowNamespaceDropdown(!showNamespaceDropdown)}
              className="flex items-center gap-2 bg-white dark:bg-slate-800 border border-stone-200 dark:border-slate-700 rounded-lg px-3 py-1.5 text-sm hover:border-stone-300 dark:hover:border-slate-600 transition-colors text-stone-800 dark:text-slate-100 shadow-sm dark:shadow-none min-w-[160px] justify-between"
            >
              <span>{namespaceFilter || "All Namespaces"}</span>
              <ChevronDown
                size={14}
                className={`text-slate-400 dark:text-slate-500 transition-transform ${
                  showNamespaceDropdown ? "rotate-180" : ""
                }`}
              />
            </button>
            {showNamespaceDropdown && (
              <div className="absolute right-0 z-50 mt-1 w-full bg-white dark:bg-slate-800 border border-stone-200 dark:border-slate-700 rounded-lg shadow-lg max-h-64 overflow-y-auto">
                <button
                  onClick={() => {
                    setNamespaceFilter("");
                    setShowNamespaceDropdown(false);
                  }}
                  className={`w-full px-3 py-2 text-sm text-left transition-colors ${
                    namespaceFilter === ""
                      ? "bg-accent-500/10 text-accent-600 dark:text-accent-400"
                      : "text-stone-600 dark:text-slate-300 hover:bg-stone-100 dark:hover:bg-slate-700"
                  }`}
                >
                  All Namespaces
                </button>
                {namespaces.map((ns) => (
                  <button
                    key={ns}
                    onClick={() => {
                      setNamespaceFilter(ns);
                      setShowNamespaceDropdown(false);
                    }}
                    className={`w-full px-3 py-2 text-sm text-left transition-colors ${
                      namespaceFilter === ns
                        ? "bg-accent-500/10 text-accent-600 dark:text-accent-400"
                        : "text-stone-600 dark:text-slate-300 hover:bg-stone-100 dark:hover:bg-slate-700"
                    }`}
                  >
                    {ns}
                  </button>
                ))}
              </div>
            )}
          </div>
          <button
            onClick={fetchGitOpsResources}
            disabled={loading || !isConnected}
            className={REFRESH_BTN_CLASS}
          >
            Refresh
          </button>
        </div>
      </div>

      <div className="flex-1 window-glass rounded-xl overflow-hidden">
        {!isConnected ? (
          <p className="text-stone-400 dark:text-slate-400 text-center py-12">
            Connect to a cluster to view GitOps resources
          </p>
        ) : loading ? (
          <div className="flex items-center justify-center py-12">
            <div className="w-6 h-6 border-2 border-accent-400 border-t-transparent rounded-full animate-spin" />
          </div>
        ) : error ? (
          <p className="text-red-400 text-center py-12">{error}</p>
        ) : !hasAnyResources ? (
          <div className="text-center py-12">
            <p className="text-stone-400 dark:text-slate-400 mb-2">
              No GitOps resources found
            </p>
            <p className="text-sm text-stone-500 dark:text-slate-500">
              Install ArgoCD or Flux to manage your applications with GitOps
            </p>
          </div>
        ) : currentResources.length === 0 ? (
          <p className="text-stone-400 dark:text-slate-400 text-center py-12">
            No{" "}
            {activeTab === "argo"
              ? "ArgoCD applications"
              : activeTab === "flux-helm"
              ? "Helm releases"
              : "Kustomizations"}{" "}
            found
          </p>
        ) : (
          <div ref={tableRef} className="overflow-auto h-full">
            <table className="w-full text-sm">
              <thead className="bg-slate-900 sticky top-0 z-10">
                <tr className="text-left text-slate-400">
                  <th
                    className="px-4 py-3 font-medium cursor-pointer hover:bg-slate-800 transition-colors select-none"
                    onClick={() => handleSort("name")}
                  >
                    <span className="flex items-center gap-1">
                      Name <SortIndicator field="name" />
                    </span>
                  </th>
                  <th
                    className="px-4 py-3 font-medium cursor-pointer hover:bg-stone-100 dark:hover:bg-slate-800 transition-colors select-none"
                    onClick={() => handleSort("namespace")}
                  >
                    <span className="flex items-center gap-1">
                      Namespace <SortIndicator field="namespace" />
                    </span>
                  </th>
                  <th
                    className="px-4 py-3 font-medium cursor-pointer hover:bg-stone-100 dark:hover:bg-slate-800 transition-colors select-none"
                    onClick={() => handleSort("status")}
                  >
                    <span className="flex items-center gap-1">
                      Status <SortIndicator field="status" />
                    </span>
                  </th>
                  <th
                    className="px-4 py-3 font-medium cursor-pointer hover:bg-stone-100 dark:hover:bg-slate-800 transition-colors select-none"
                    onClick={() => handleSort("age")}
                  >
                    <span className="flex items-center gap-1">
                      Age <SortIndicator field="age" />
                    </span>
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-stone-200 dark:divide-slate-700/50">
                {[...currentResources]
                  .sort((a, b) => {
                    let comparison = 0;
                    switch (sortField) {
                      case "name":
                        comparison = a.name.localeCompare(b.name);
                        break;
                      case "namespace":
                        comparison = (a.namespace || "").localeCompare(
                          b.namespace || ""
                        );
                        break;
                      case "status":
                        comparison = (a.status || "").localeCompare(
                          b.status || ""
                        );
                        break;
                      case "age":
                        comparison = a.age.localeCompare(b.age);
                        break;
                    }
                    return sortDirection === "asc" ? comparison : -comparison;
                  })
                  .map((resource, idx) => (
                    <tr
                      key={`${resource.namespace}-${resource.name}-${idx}`}
                      data-index={idx}
                      className={`border-t border-slate-700/50 transition-colors ${
                        selectedIndex === idx
                          ? "bg-accent-500/20 ring-1 ring-accent-500/50"
                          : "hover:bg-slate-700/30"
                      }`}
                    >
                      <td className="px-4 py-3 font-mono text-accent-600 dark:text-accent-400 font-medium">
                        {resource.name}
                      </td>
                      <td className="px-4 py-3 text-stone-500 dark:text-slate-400">
                        {resource.namespace || "-"}
                      </td>
                      <td
                        className={`px-4 py-3 ${getStatusColor(
                          resource.status
                        )}`}
                      >
                        {resource.status || "-"}
                      </td>
                      <td className="px-4 py-3 text-stone-500 dark:text-slate-500">
                        {resource.age}
                      </td>
                    </tr>
                  ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
