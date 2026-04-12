import { useState, useEffect, useRef, useMemo } from "react";
import { ChevronDown } from "lucide-react";
import { ScanCluster } from "../../../wailsjs/go/main/App";

interface AnalyzerIssue {
  id: string;
  category: string;
  severity: string;
  title: string;
  message: string;
  resource: string;
  namespace: string;
  kind: string;
  fixes?: { description: string; yaml?: string; command?: string }[];
}

interface AnalyzerSummary {
  critical: number;
  warning: number;
  info: number;
  issuesByCategory: Record<string, AnalyzerIssue[]>;
  scannedAt: string;
}

export function AnalyzerView({ isConnected }: { isConnected: boolean }) {
  const [summary, setSummary] = useState<AnalyzerSummary | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [namespaceFilter, setNamespaceFilter] = useState("");
  const [textSearch, setTextSearch] = useState("");
  const [selectedIssueKey, setSelectedIssueKey] = useState<string | null>(null);
  const [selectedCategory, setSelectedCategory] = useState<string>("all");
  const [selectedIndex, setSelectedIndex] = useState<number>(-1);
  const listRef = useRef<HTMLDivElement>(null);
  const filterRef = useRef<HTMLInputElement>(null);
  const [showNamespaceDropdown, setShowNamespaceDropdown] = useState(false);
  const [showCategoryDropdown, setShowCategoryDropdown] = useState(false);
  const [showSortDropdown, setShowSortDropdown] = useState(false);

  // Sorting
  const [sortField, setSortField] = useState<
    "severity" | "category" | "resource" | "kind"
  >("severity");
  const [sortDirection, setSortDirection] = useState<"asc" | "desc">("desc");

  const runScan = async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await ScanCluster(namespaceFilter);
      setSummary(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Scan failed");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (isConnected) {
      runScan();
    } else {
      setSummary(null);
    }
  }, [isConnected]);

  // Severity/category helper functions
  const getSeverityColor = (severity: string) => {
    switch (severity) {
      case "Critical":
        return "text-red-400 bg-red-500/10 border-red-500/30";
      case "Warning":
        return "text-yellow-400 bg-yellow-500/10 border-yellow-500/30";
      default:
        return "text-blue-400 bg-blue-500/10 border-blue-500/30";
    }
  };

  const getSeverityIcon = (severity: string) => {
    switch (severity) {
      case "Critical":
        return "\u2716";
      case "Warning":
        return "\u26a0";
      default:
        return "\u2139";
    }
  };

  const allIssues = summary
    ? Object.values(summary.issuesByCategory).flat()
    : [];

  // Extract unique namespaces from issues for dropdown
  const availableNamespaces = useMemo(() => {
    const namespaces = new Set<string>();
    allIssues.forEach((issue) => {
      if (issue.namespace) {
        namespaces.add(issue.namespace);
      }
    });
    return Array.from(namespaces).sort();
  }, [allIssues]);

  // Filter by category first
  const categoryFilteredIssues =
    selectedCategory === "all"
      ? allIssues
      : allIssues.filter((i) => i.category === selectedCategory);

  // Apply text search filter
  const filteredIssues = textSearch.trim()
    ? categoryFilteredIssues.filter((issue) => {
        const searchLower = textSearch.toLowerCase();
        return (
          issue.title.toLowerCase().includes(searchLower) ||
          issue.message.toLowerCase().includes(searchLower) ||
          issue.resource.toLowerCase().includes(searchLower) ||
          issue.kind.toLowerCase().includes(searchLower) ||
          issue.category.toLowerCase().includes(searchLower)
        );
      })
    : categoryFilteredIssues;

  // Apply sorting
  const sortedIssues = [...filteredIssues].sort((a, b) => {
    let comparison = 0;
    switch (sortField) {
      case "severity": {
        const severityOrder: Record<string, number> = {
          Critical: 3,
          Warning: 2,
          Info: 1,
        };
        comparison =
          (severityOrder[a.severity] || 0) - (severityOrder[b.severity] || 0);
        break;
      }
      case "category":
        comparison = a.category.localeCompare(b.category);
        break;
      case "resource":
        comparison = a.resource.localeCompare(b.resource);
        break;
      case "kind":
        comparison = a.kind.localeCompare(b.kind);
        break;
    }
    return sortDirection === "asc" ? comparison : -comparison;
  });

  // Reset selection when issues change
  useEffect(() => {
    setSelectedIndex(-1);
  }, [summary, selectedCategory]);

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
        case "s":
          if (!loading && isConnected) {
            e.preventDefault();
            runScan();
          }
          break;
        case "a":
          e.preventDefault();
          setSelectedCategory("all");
          break;
        case "ArrowDown":
        case "j":
          e.preventDefault();
          setSelectedIndex((prev) =>
            prev < sortedIssues.length - 1 ? prev + 1 : prev
          );
          break;
        case "ArrowUp":
        case "k":
          e.preventDefault();
          setSelectedIndex((prev) => (prev > 0 ? prev - 1 : 0));
          break;
        case "Enter":
          e.preventDefault();
          if (selectedIndex >= 0 && selectedIndex < sortedIssues.length) {
            const issue = sortedIssues[selectedIndex];
            const key = `${issue.id}-${selectedIndex}`;
            setSelectedIssueKey(selectedIssueKey === key ? null : key);
          }
          break;
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [sortedIssues, selectedIndex, selectedIssueKey, loading, isConnected]);

  // Scroll selected item into view
  useEffect(() => {
    if (selectedIndex >= 0 && listRef.current) {
      const item = listRef.current.querySelector(
        `[data-index="${selectedIndex}"]`
      );
      item?.scrollIntoView({ block: "nearest", behavior: "smooth" });
    }
  }, [selectedIndex]);

  const availableCategories = [
    "Scheduling",
    "Storage",
    "Stuck",
    "Node",
    "CRD",
    "Config",
  ];

  const categories = summary
    ? [
        "all",
        ...availableCategories.filter(
          (cat) =>
            summary.issuesByCategory[cat] &&
            summary.issuesByCategory[cat].length > 0
        ),
      ]
    : ["all", ...availableCategories];

  return (
    <div className="h-full flex flex-col">
      <div className="flex items-center justify-between mb-4 flex-wrap gap-3">
        <div className="flex gap-3 items-center">
          <div className="relative">
            <button
              onClick={() => setShowNamespaceDropdown(!showNamespaceDropdown)}
              className="flex items-center gap-2 bg-stone-50 dark:bg-slate-800 border border-stone-200 dark:border-slate-700 rounded-lg px-3 py-1.5 text-sm hover:border-stone-300 dark:hover:border-slate-600 transition-colors shadow-sm dark:shadow-none min-w-[160px] justify-between"
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
              <div className="absolute left-0 z-50 mt-1 w-full bg-white dark:bg-slate-800 border border-stone-200 dark:border-slate-700 rounded-lg shadow-lg max-h-64 overflow-y-auto">
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
                {availableNamespaces.map((ns) => (
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
          <div className="relative">
            <button
              onClick={() => setShowCategoryDropdown(!showCategoryDropdown)}
              className="flex items-center gap-2 bg-stone-50 dark:bg-slate-800 border border-stone-200 dark:border-slate-700 rounded-lg px-3 py-1.5 text-sm hover:border-stone-300 dark:hover:border-slate-600 transition-colors shadow-sm dark:shadow-none min-w-[140px] justify-between"
            >
              <span className="capitalize">
                {selectedCategory === "all"
                  ? "All Categories"
                  : selectedCategory}
              </span>
              <ChevronDown
                size={14}
                className={`text-slate-400 dark:text-slate-500 transition-transform ${
                  showCategoryDropdown ? "rotate-180" : ""
                }`}
              />
            </button>
            {showCategoryDropdown && (
              <div className="absolute left-0 z-50 mt-1 w-full bg-white dark:bg-slate-800 border border-stone-200 dark:border-slate-700 rounded-lg shadow-lg max-h-64 overflow-y-auto">
                {categories.map((cat) => (
                  <button
                    key={cat}
                    onClick={() => {
                      setSelectedCategory(cat);
                      setShowCategoryDropdown(false);
                    }}
                    className={`w-full px-3 py-2 text-sm text-left capitalize transition-colors ${
                      selectedCategory === cat
                        ? "bg-accent-500/10 text-accent-600 dark:text-accent-400"
                        : "text-stone-600 dark:text-slate-300 hover:bg-stone-100 dark:hover:bg-slate-700"
                    }`}
                  >
                    {cat === "all" ? "All Categories" : cat}
                  </button>
                ))}
              </div>
            )}
          </div>
          <input
            ref={filterRef}
            type="text"
            placeholder="Search issues... (/)"
            value={textSearch}
            onChange={(e) => setTextSearch(e.target.value)}
            className="w-48 bg-stone-50 dark:bg-slate-800 border border-stone-200 dark:border-slate-700 rounded-lg px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-accent-500/50"
          />
          {/* Sort dropdown */}
          <div className="flex items-center gap-2">
            <span className="text-xs text-stone-500 dark:text-slate-500">
              Sort:
            </span>
            <div className="relative">
              <button
                onClick={() => setShowSortDropdown(!showSortDropdown)}
                className="px-2 py-1 text-xs bg-stone-50 dark:bg-slate-800 border border-stone-200 dark:border-slate-700 rounded hover:border-stone-300 dark:hover:border-slate-600 transition-colors shadow-sm dark:shadow-none flex items-center gap-1 min-w-[80px] justify-between"
              >
                <span className="capitalize">{sortField}</span>
                <ChevronDown
                  size={12}
                  className={`text-slate-400 dark:text-slate-500 transition-transform ${
                    showSortDropdown ? "rotate-180" : ""
                  }`}
                />
              </button>
              {showSortDropdown && (
                <div className="absolute right-0 z-50 mt-1 w-32 bg-white dark:bg-slate-800 border border-stone-200 dark:border-slate-700 rounded shadow-lg overflow-hidden">
                  {(["severity", "category", "resource", "kind"] as const).map(
                    (field) => (
                      <button
                        key={field}
                        onClick={() => {
                          setSortField(field);
                          setShowSortDropdown(false);
                        }}
                        className={`w-full px-3 py-1.5 text-xs text-left capitalize transition-colors ${
                          sortField === field
                            ? "bg-accent-500/10 text-accent-600 dark:text-accent-400"
                            : "text-stone-600 dark:text-slate-300 hover:bg-stone-100 dark:hover:bg-slate-700"
                        }`}
                      >
                        {field}
                      </button>
                    )
                  )}
                </div>
              )}
            </div>
            <button
              onClick={() =>
                setSortDirection((d) => (d === "asc" ? "desc" : "asc"))
              }
              className="px-2 py-1 text-xs bg-stone-50 dark:bg-slate-800 border border-stone-200 dark:border-slate-700 rounded hover:border-stone-300 dark:hover:border-slate-600 transition-colors"
              title={`Sort ${
                sortDirection === "asc" ? "ascending" : "descending"
              }`}
            >
              {sortDirection === "asc" ? "\u2191" : "\u2193"}
            </button>
          </div>
        </div>
        <button
          onClick={runScan}
          disabled={loading || !isConnected}
          className="px-4 py-2 text-sm bg-accent-500 hover:bg-accent-600 text-white rounded-lg transition-colors disabled:opacity-50 flex items-center gap-2"
        >
          {loading && (
            <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin" />
          )}
          Scan Cluster
        </button>
      </div>

      {/* Summary cards */}
      {summary && (
        <div className="grid grid-cols-3 gap-4 mb-4">
          <div className="bg-red-500/10 border border-red-500/30 rounded-lg p-4">
            <div className="text-3xl font-bold text-red-400">
              {summary.critical}
            </div>
            <div className="text-sm text-stone-600 dark:text-slate-400">
              Critical Issues
            </div>
          </div>
          <div className="bg-yellow-500/10 border border-yellow-500/30 rounded-lg p-4">
            <div className="text-3xl font-bold text-yellow-400">
              {summary.warning}
            </div>
            <div className="text-sm text-stone-600 dark:text-slate-400">
              Warnings
            </div>
          </div>
          <div className="bg-blue-500/10 border border-blue-500/30 rounded-lg p-4">
            <div className="text-3xl font-bold text-blue-400">
              {summary.info}
            </div>
            <div className="text-sm text-stone-600 dark:text-slate-400">
              Info
            </div>
          </div>
        </div>
      )}

      <div className="flex-1 window-glass rounded-xl overflow-hidden">
        {!isConnected ? (
          <p className="text-stone-400 dark:text-slate-400 text-center py-12">
            Connect to a cluster to scan for issues
          </p>
        ) : loading && !summary ? (
          <div className="flex flex-col items-center justify-center py-12 gap-3">
            <div className="w-8 h-8 border-2 border-accent-400 border-t-transparent rounded-full animate-spin" />
            <p className="text-stone-500 dark:text-slate-400">
              Scanning cluster for issues...
            </p>
          </div>
        ) : error ? (
          <p className="text-red-400 text-center py-12">{error}</p>
        ) : sortedIssues.length === 0 ? (
          <div className="text-center py-12">
            <p className="text-emerald-400 text-xl mb-2">
              {"\u2713"} No issues found
            </p>
            <p className="text-stone-500 dark:text-slate-500">
              Your cluster looks healthy!
            </p>
          </div>
        ) : (
          <div ref={listRef} className="overflow-auto h-full">
            <div className="p-4 space-y-3">
              {sortedIssues.map((issue, idx) => (
                <div
                  key={`${issue.id}-${idx}`}
                  data-index={idx}
                  onClick={() => {
                    const key = `${issue.id}-${idx}`;
                    setSelectedIssueKey(selectedIssueKey === key ? null : key);
                    setSelectedIndex(idx);
                  }}
                  className={`p-4 rounded-lg border cursor-pointer transition-colors ${getSeverityColor(
                    issue.severity
                  )} ${
                    selectedIndex === idx
                      ? "ring-2 ring-accent-500"
                      : selectedIssueKey === `${issue.id}-${idx}`
                      ? "ring-2 ring-accent-500/50"
                      : ""
                  }`}
                >
                  <div className="flex items-start gap-3">
                    <span className="text-xl">
                      {getSeverityIcon(issue.severity)}
                    </span>
                    <div className="flex-1">
                      <div className="flex items-center gap-2 mb-1">
                        <span className="font-medium">{issue.title}</span>
                        <span className="text-xs px-2 py-0.5 bg-stone-100 dark:bg-slate-700 rounded text-stone-600 dark:text-slate-300">
                          {issue.category}
                        </span>
                      </div>
                      <p className="text-sm text-stone-600 dark:text-slate-300 mb-2">
                        {issue.message}
                      </p>
                      <p className="text-xs text-stone-400 dark:text-slate-500 font-mono">
                        {issue.kind}/
                        {issue.namespace ? `${issue.namespace}/` : ""}
                        {issue.resource}
                      </p>
                      {selectedIssueKey === `${issue.id}-${idx}` &&
                        issue.fixes &&
                        issue.fixes.length > 0 && (
                          <div className="mt-4 pt-4 border-t border-stone-200 dark:border-slate-700">
                            <p className="text-sm font-medium mb-2 text-stone-700 dark:text-slate-300">
                              Suggested Fixes:
                            </p>
                            {issue.fixes.map((fix, i) => (
                              <div key={i} className="mb-3">
                                <p className="text-sm text-stone-600 dark:text-slate-400">
                                  {fix.description}
                                </p>
                                {fix.command && (
                                  <pre className="mt-1 p-2 bg-stone-100 dark:bg-slate-900 rounded text-xs font-mono text-emerald-600 dark:text-emerald-400 overflow-x-auto">
                                    {fix.command}
                                  </pre>
                                )}
                                {fix.yaml && (
                                  <pre className="mt-1 p-2 bg-stone-100 dark:bg-slate-900 rounded text-xs font-mono text-stone-600 dark:text-slate-300 overflow-x-auto">
                                    {fix.yaml}
                                  </pre>
                                )}
                              </div>
                            ))}
                          </div>
                        )}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      {summary && (
        <div className="mt-3 text-sm text-slate-500">
          {allIssues.length} issues found &bull; Last scan:{" "}
          {new Date(summary.scannedAt).toLocaleTimeString()}
        </div>
      )}
    </div>
  );
}
