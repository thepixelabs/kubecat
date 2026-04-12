import { useState, useEffect, useCallback } from "react";
import { motion, AnimatePresence } from "framer-motion";
import {
  GitCompare,
  History,
  Wifi,
  Loader2,
  AlertCircle,
  ChevronDown,
  Check,
  Layers,
} from "lucide-react";
import { SourceSelector } from "./SourceSelector";
import { DiffViewer } from "./DiffViewer";
import { DiffSummary } from "./DiffSummary";
import { ActionBar } from "./ActionBar";
import { ApplyConfirmModal } from "./ApplyConfirmModal";
import type {
  DiffSource,
  DiffResult,
  ApplyResult,
  SnapshotInfo,
  DiffMode,
  ClusterDiffPersistentState,
} from "./types";

// Resource kinds for selection
const resourceKinds = [
  { value: "deployments", label: "Deployments" },
  { value: "services", label: "Services" },
  { value: "configmaps", label: "ConfigMaps" },
  { value: "secrets", label: "Secrets" },
  { value: "statefulsets", label: "StatefulSets" },
  { value: "daemonsets", label: "DaemonSets" },
  { value: "ingresses", label: "Ingresses" },
];

interface ClusterDiffProps {
  contexts: string[];
  activeContext: string;
  namespaces: string[];
  isTimelineAvailable: boolean;
  initialState?: ClusterDiffPersistentState;
  onStateChange?: (state: ClusterDiffPersistentState) => void;
  // Wails bindings - using any for cross-module type compatibility
  onComputeDiff: (req: any) => Promise<any>;
  onGetSnapshots: (limit: number) => Promise<SnapshotInfo[]>;
  onListResources: (kind: string, namespace: string) => Promise<any[]>;
  onApplyResource: (
    context: string,
    kind: string,
    namespace: string,
    name: string,
    yaml: string,
    dryRun: boolean
  ) => Promise<any>;
  onGenerateReport: (result: any, format: string) => Promise<any>;
}

export function ClusterDiff({
  contexts,
  activeContext,
  namespaces = [],
  isTimelineAvailable,
  initialState,
  onStateChange,
  onComputeDiff,
  onGetSnapshots,
  onListResources,
  onApplyResource,
  onGenerateReport,
}: ClusterDiffProps) {
  // State
  const [mode, setMode] = useState<DiffMode>("cross-cluster");
  const [leftSource, setLeftSource] = useState<DiffSource>(
    initialState?.leftSource || {
      context: activeContext || contexts[0] || "",
      isLive: true,
    }
  );
  const [rightSource, setRightSource] = useState<DiffSource>(() => {
    if (initialState?.rightSource) {
      // If initialState has a rightSource and it's different from leftSource, use it
      const initialLeft =
        initialState.leftSource?.context || activeContext || contexts[0] || "";
      if (initialState.rightSource.context !== initialLeft) {
        return initialState.rightSource;
      }
    }
    // Otherwise, find a different context
    const leftCtx =
      initialState?.leftSource?.context || activeContext || contexts[0] || "";
    const differentContext =
      contexts.find((c) => c !== leftCtx) || contexts[1] || contexts[0] || "";
    return {
      context: differentContext,
      isLive: true,
    };
  });
  const [kind, setKind] = useState(initialState?.resourceKind || "deployments");
  const [namespace, setNamespace] = useState(
    initialState?.namespace || "default"
  );
  const [resourceName, setResourceName] = useState(
    initialState?.resourceName || ""
  );
  const [result, setResult] = useState<DiffResult | null>(
    initialState?.diffResult || null
  );
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [snapshots, setSnapshots] = useState<SnapshotInfo[]>([]);
  const [resources, setResources] = useState<any[]>([]);
  const [loadingResources, setLoadingResources] = useState(false);

  // Apply modal state
  const [showApplyModal, setShowApplyModal] = useState(false);
  const [isApplying, setIsApplying] = useState(false);
  const [applyResult, setApplyResult] = useState<ApplyResult | null>(null);
  const [isExporting, setIsExporting] = useState(false);

  // Dropdowns
  const [kindOpen, setKindOpen] = useState(false);
  const [resourceOpen, setResourceOpen] = useState(false);
  const [namespaceOpen, setNamespaceOpen] = useState(false);

  // Sync left source with active context
  useEffect(() => {
    if (activeContext && leftSource.context !== activeContext) {
      setLeftSource((prev) => ({ ...prev, context: activeContext }));
    }
  }, [activeContext]);

  // Prevent same cluster selection by resetting right source if it matches left
  useEffect(() => {
    if (leftSource.context && rightSource.context === leftSource.context) {
      const otherContext = contexts.find((c) => c !== leftSource.context);
      if (otherContext) {
        setRightSource((prev) => ({ ...prev, context: otherContext }));
      }
    }
  }, [leftSource.context, rightSource.context, contexts]);

  // Load snapshots when mode changes to historical
  useEffect(() => {
    if (mode === "historical" && isTimelineAvailable) {
      onGetSnapshots(50).then(setSnapshots).catch(console.error);
    }
  }, [mode, isTimelineAvailable, onGetSnapshots]);

  // Load resources when kind/namespace changes
  useEffect(() => {
    const activeContext = leftSource.context || rightSource.context;
    if (!activeContext || !kind) return;

    setLoadingResources(true);
    onListResources(kind, namespace)
      .then((res) => {
        setResources(res || []);
        setLoadingResources(false);
      })
      .catch((err) => {
        console.error("Failed to load resources:", err);
        setResources([]);
        setLoadingResources(false);
      });
  }, [kind, namespace, leftSource.context, onListResources]);

  // Persist state to parent
  useEffect(() => {
    if (onStateChange) {
      onStateChange({
        leftSource,
        rightSource,
        namespace,
        resourceName,
        resourceKind: kind,
        diffResult: result,
      });
    }
  }, [
    leftSource,
    rightSource,
    namespace,
    resourceName,
    kind,
    result,
     
  ]);

  // Debug initial state
  useEffect(() => {
    console.log("ClusterDiff initialized with:", {
      initialState,
      contexts,
      activeContext,
    });
  }, []);

  // Compare handler
  const handleCompare = useCallback(async () => {
    if (!resourceName) {
      setError("Please select a resource to compare");
      return;
    }

    setLoading(true);
    setError(null);
    setResult(null);

    try {
      const req = {
        kind,
        namespace,
        name: resourceName,
        left: leftSource,
        right: rightSource,
      };
      console.log("ClusterDiff: Comparing", req); // Debug log
      const res = await onComputeDiff(req);
      setResult(res);
    } catch (err) {
      console.error(err);
      setError("Failed to compute diff");
    } finally {
      setLoading(false);
    }
  }, [kind, namespace, resourceName, leftSource, rightSource, onComputeDiff]);

  // Apply handler
  const handleApply = useCallback(
    async (dryRun: boolean) => {
      if (!result) return;

      setIsApplying(true);
      setApplyResult(null);

      try {
        const applyRes = await onApplyResource(
          rightSource.context,
          kind,
          namespace,
          resourceName,
          result.leftYaml, // Apply left config to right cluster
          dryRun
        );
        setApplyResult(applyRes);
      } catch (err: any) {
        setApplyResult({
          success: false,
          dryRun,
          message: err.message || "Apply failed",
          changes: [],
          warnings: [],
        });
      } finally {
        setIsApplying(false);
      }
    },
    [
      result,
      rightSource.context,
      kind,
      namespace,
      resourceName,
      onApplyResource,
    ]
  );

  // Export handler
  const handleExport = useCallback(
    async (format: "markdown" | "json") => {
      if (!result) return;

      setIsExporting(true);
      try {
        const report = await onGenerateReport(result, format);
        // Create a download
        const blob = new Blob([report.content], { type: "text/plain" });
        const url = URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = report.filename;
        a.click();
        URL.revokeObjectURL(url);
      } catch (err) {
        console.error("Export failed:", err);
      } finally {
        setIsExporting(false);
      }
    },
    [result, onGenerateReport]
  );

  // Jump to diff
  const handleJumpTo = useCallback((path: string) => {
    // Find the element with the path and scroll to it
    const element = document.querySelector(`[data-path="${path}"]`);
    if (element) {
      element.scrollIntoView({ behavior: "smooth", block: "center" });
      element.classList.add("ring-2", "ring-teal-500");
      setTimeout(() => {
        element.classList.remove("ring-2", "ring-teal-500");
      }, 2000);
    }
  }, []);

  return (
    <div className="flex flex-col min-h-full bg-stone-50 dark:bg-slate-900">
      {/* Configuration section */}
      <div className="p-4 border-b border-stone-200 dark:border-slate-700 bg-stone-50/50 dark:bg-slate-800/30">
        <div className="flex items-center justify-between mb-4">
          {/* Mode toggle */}
          <div className="flex items-center gap-1 p-1 bg-stone-200 dark:bg-slate-900 rounded-lg">
            <button
              onClick={() => setMode("cross-cluster")}
              className={`flex items-center gap-2 px-3 py-1.5 rounded text-sm transition-colors ${
                mode === "cross-cluster"
                  ? "bg-white dark:bg-slate-800 text-accent-600 dark:text-accent-400 shadow-sm"
                  : "text-stone-500 dark:text-slate-400 hover:text-stone-700 dark:hover:text-slate-300"
              }`}
            >
              <Wifi className="w-4 h-4" />
              Cross-Cluster
            </button>
            <button
              onClick={() => setMode("historical")}
              disabled={!isTimelineAvailable}
              className={`flex items-center gap-2 px-3 py-1.5 rounded text-sm transition-colors ${
                mode === "historical"
                  ? "bg-white dark:bg-slate-800 text-purple-600 dark:text-purple-400 shadow-sm"
                  : "text-stone-500 dark:text-slate-400 hover:text-stone-700 dark:hover:text-slate-300"
              } ${!isTimelineAvailable ? "opacity-50 cursor-not-allowed" : ""}`}
              title={
                !isTimelineAvailable ? "Timeline not available" : undefined
              }
            >
              <History className="w-4 h-4" />
              Historical
            </button>
          </div>

          {/* Compare button */}
          <button
            onClick={handleCompare}
            disabled={loading || !leftSource.context || !rightSource.context}
            className="flex items-center gap-2 px-4 py-1.5 bg-accent-600 hover:bg-accent-500 
                     text-white text-sm rounded-lg transition-colors font-medium
                     disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {loading ? (
              <>
                <Loader2 className="w-4 h-4 animate-spin" />
                Comparing...
              </>
            ) : (
              <>
                <GitCompare className="w-4 h-4" />
                Compare
              </>
            )}
          </button>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
          <SourceSelector
            contexts={contexts}
            snapshots={snapshots}
            value={leftSource}
            onChange={setLeftSource}
            label="Source (Left)"
            isTimelineAvailable={isTimelineAvailable && mode === "historical"}
            readOnly={true}
          />

          {/* Resource picker */}
          <div className="flex flex-col gap-2 p-3 bg-white dark:bg-slate-800/50 rounded-lg border border-stone-200 dark:border-slate-700">
            <div className="flex items-center gap-2 text-sm font-medium text-stone-700 dark:text-slate-300">
              <Layers className="w-4 h-4 text-accent-500" />
              Resource
            </div>

            {/* Kind selector */}
            <div className="relative">
              <button
                onClick={() => setKindOpen(!kindOpen)}
                className="w-full flex items-center justify-between gap-2 px-3 py-2 bg-white dark:bg-slate-800 
                         border border-stone-200 dark:border-slate-700 rounded-lg hover:border-stone-300 dark:hover:border-slate-600 transition-colors shadow-sm dark:shadow-none"
              >
                <span className="text-sm text-stone-700 dark:text-slate-200">
                  {resourceKinds.find((k) => k.value === kind)?.label || kind}
                </span>
                <ChevronDown className="w-4 h-4 text-stone-400 dark:text-slate-500" />
              </button>
              <AnimatePresence>
                {kindOpen && (
                  <motion.div
                    initial={{ opacity: 0, y: -4 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -4 }}
                    className="absolute z-20 top-full left-0 right-0 mt-1 py-1 bg-white dark:bg-slate-800 
                             border border-stone-200 dark:border-slate-700 rounded-lg shadow-xl max-h-48 overflow-auto"
                  >
                    {resourceKinds.map((k) => (
                      <button
                        key={k.value}
                        onClick={() => {
                          setKind(k.value);
                          setResourceName("");
                          setKindOpen(false);
                        }}
                        className={`w-full flex items-center justify-between px-3 py-2 text-sm 
                                 hover:bg-stone-100 dark:hover:bg-slate-800 transition-colors ${
                                   kind === k.value
                                     ? "text-accent-600 dark:text-accent-400 bg-accent-50 dark:bg-accent-500/10"
                                     : "text-stone-700 dark:text-slate-300"
                                 }`}
                      >
                        {k.label}
                        {kind === k.value && <Check className="w-4 h-4" />}
                      </button>
                    ))}
                  </motion.div>
                )}
              </AnimatePresence>
            </div>

            {/* Namespace selector */}
            <div className="relative">
              <button
                onClick={() => setNamespaceOpen(!namespaceOpen)}
                className="w-full flex items-center justify-between gap-2 px-3 py-2 bg-white dark:bg-slate-800 
                         border border-stone-200 dark:border-slate-700 rounded-lg hover:border-stone-300 dark:hover:border-slate-600 transition-colors shadow-sm dark:shadow-none"
              >
                <span className="text-sm text-stone-700 dark:text-slate-200">
                  {namespace || "Select namespace..."}
                </span>
                <ChevronDown className="w-4 h-4 text-stone-400 dark:text-slate-500" />
              </button>
              <AnimatePresence>
                {namespaceOpen && (
                  <motion.div
                    initial={{ opacity: 0, y: -4 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -4 }}
                    className="absolute z-20 top-full left-0 right-0 mt-1 py-1 bg-white dark:bg-slate-800 
                             border border-stone-200 dark:border-slate-700 rounded-lg shadow-xl max-h-48 overflow-auto"
                  >
                    {(namespaces || []).map((ns) => (
                      <button
                        key={ns}
                        onClick={() => {
                          setNamespace(ns);
                          setResourceName("");
                          setNamespaceOpen(false);
                        }}
                        className={`w-full flex items-center justify-between px-3 py-2 text-sm 
                                 hover:bg-stone-100 dark:hover:bg-slate-800 transition-colors ${
                                   namespace === ns
                                     ? "text-accent-600 dark:text-accent-400 bg-accent-50 dark:bg-accent-500/10"
                                     : "text-stone-700 dark:text-slate-300"
                                 }`}
                      >
                        {ns}
                        {namespace === ns && <Check className="w-4 h-4" />}
                      </button>
                    ))}
                  </motion.div>
                )}
              </AnimatePresence>
            </div>

            {/* Resource name selector */}
            <div className="relative">
              <button
                onClick={() => setResourceOpen(!resourceOpen)}
                disabled={loadingResources}
                className="w-full flex items-center justify-between gap-2 px-3 py-2 bg-white dark:bg-slate-800 
                         border border-stone-200 dark:border-slate-700 rounded-lg hover:border-stone-300 dark:hover:border-slate-600 transition-colors
                         disabled:opacity-50 shadow-sm dark:shadow-none"
              >
                <span className="text-sm text-stone-700 dark:text-slate-200 truncate">
                  {loadingResources
                    ? "Loading..."
                    : resourceName || "Select resource..."}
                </span>
                {loadingResources ? (
                  <Loader2 className="w-4 h-4 text-stone-400 dark:text-slate-500 animate-spin" />
                ) : (
                  <ChevronDown className="w-4 h-4 text-stone-400 dark:text-slate-500" />
                )}
              </button>
              <AnimatePresence>
                {resourceOpen && (
                  <motion.div
                    initial={{ opacity: 0, y: -4 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -4 }}
                    className="absolute z-20 top-full left-0 right-0 mt-1 py-1 bg-white dark:bg-slate-800 
                             border border-stone-200 dark:border-slate-700 rounded-lg shadow-xl max-h-48 overflow-auto"
                  >
                    {resources.length === 0 ? (
                      <div className="px-3 py-2 text-sm text-stone-500 dark:text-slate-500 italic">
                        No resources found
                      </div>
                    ) : (
                      resources.map((r) => (
                        <button
                          key={`${r.namespace}/${r.name}`}
                          onClick={() => {
                            setResourceName(r.name);
                            setResourceOpen(false);
                          }}
                          className={`w-full flex items-center justify-between px-3 py-2 text-sm 
                                   hover:bg-stone-100 dark:hover:bg-slate-800 transition-colors ${
                                     resourceName === r.name
                                       ? "text-accent-600 dark:text-accent-400 bg-accent-50 dark:bg-accent-500/10"
                                       : "text-stone-700 dark:text-slate-300"
                                   }`}
                        >
                          <span className="truncate">{r.name}</span>
                          {resourceName === r.name && (
                            <Check className="w-4 h-4" />
                          )}
                        </button>
                      ))
                    )}
                  </motion.div>
                )}
              </AnimatePresence>
            </div>
          </div>

          {/* Right source */}
          <SourceSelector
            contexts={contexts.filter((c) => c !== leftSource.context)}
            snapshots={snapshots}
            value={rightSource}
            onChange={setRightSource}
            label="Target (Right)"
            isTimelineAvailable={isTimelineAvailable && mode === "historical"}
          />
        </div>
      </div>

      {/* Error display */}
      {error && (
        <motion.div
          initial={{ opacity: 0, y: -10 }}
          animate={{ opacity: 1, y: 0 }}
          className="mx-4 mt-4 p-3 bg-red-500/10 border border-red-500/30 rounded-lg 
                   flex items-center gap-2 text-red-600 dark:text-red-400"
        >
          <AlertCircle className="w-5 h-5 flex-shrink-0" />
          <span>{error}</span>
        </motion.div>
      )}

      {/* Results section */}
      <div className="flex flex-col">
        {result ? (
          <>
            {/* Diff summary */}
            <div className="p-4 border-b border-stone-200 dark:border-slate-700">
              <DiffSummary
                differences={result.differences || []}
                onJumpTo={handleJumpTo}
              />
            </div>

            {/* Diff viewer */}
            <div className="p-4">
              <DiffViewer
                leftYaml={result.leftYaml || ""}
                rightYaml={result.rightYaml || ""}
                leftLabel={`${leftSource.context}${
                  !leftSource.isLive ? " (snapshot)" : ""
                }`}
                rightLabel={`${rightSource.context}${
                  !rightSource.isLive ? " (snapshot)" : ""
                }`}
                differences={result.differences || []}
              />
            </div>

            {/* Action bar */}
            <ActionBar
              onExport={handleExport}
              onApply={() => setShowApplyModal(true)}
              hasResult={!!result}
              isExporting={isExporting}
            />
          </>
        ) : (
          <div className="flex-1 flex items-center justify-center py-20">
            <div className="text-center text-stone-400 dark:text-slate-500">
              <GitCompare className="w-12 h-12 mx-auto mb-3 opacity-30" />
              <div className="text-lg text-stone-500 dark:text-slate-400">
                Select resources to compare
              </div>
              <div className="text-sm mt-1">
                Choose clusters and a resource, then click Compare
              </div>
            </div>
          </div>
        )}
      </div>

      {/* Apply modal */}
      <ApplyConfirmModal
        isOpen={showApplyModal}
        onClose={() => {
          setShowApplyModal(false);
          setApplyResult(null);
        }}
        onConfirm={handleApply}
        targetContext={rightSource.context}
        resourceInfo={{ kind, namespace, name: resourceName }}
        differences={result?.differences || []}
        isApplying={isApplying}
        applyResult={applyResult}
      />
    </div>
  );
}
