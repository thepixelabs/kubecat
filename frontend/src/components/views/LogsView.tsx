import { useState, useEffect, useRef } from "react";
import { X, ChevronDown } from "lucide-react";
import {
  StartLogStream,
  StopLogStream,
  GetBufferedLogs,
  StartWorkloadLogStream,
  GetBufferedWorkloadLogs,
} from "../../../wailsjs/go/main/App";
import type { SelectedPod } from "../../types/resources";

interface LogLine {
  pod: string;
  container: string;
  line: string;
  colorIdx: number;
}

// Color palette for multi-pod log coloring
const POD_COLORS = [
  "text-cyan-400",
  "text-yellow-400",
  "text-pink-400",
  "text-emerald-400",
  "text-orange-400",
  "text-violet-400",
  "text-lime-400",
  "text-rose-400",
  "text-sky-400",
  "text-amber-400",
];

export function LogsView({
  isConnected,
  selectedPod,
  onClearPod,
}: {
  isConnected: boolean;
  selectedPod: SelectedPod | null;
  onClearPod: () => void;
}) {
  const [logs, setLogs] = useState<string[]>([]);
  const [workloadLogs, setWorkloadLogs] = useState<LogLine[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isStreaming, setIsStreaming] = useState(false);
  const logsEndRef = useRef<HTMLDivElement>(null);
  const logsContainerRef = useRef<HTMLDivElement>(null);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const [autoScroll, setAutoScroll] = useState(true);

  const isWorkload =
    selectedPod?.kind === "Deployment" ||
    selectedPod?.kind === "StatefulSet" ||
    selectedPod?.kind === "DaemonSet";

  useEffect(() => {
    if (selectedPod && isConnected) {
      startLogs();
    }
    return () => {
      stopLogs();
    };
  }, [selectedPod, isConnected]);

  // Handle ESC key to go back to previous page
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        stopLogs();
        onClearPod();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [onClearPod]);

  const startLogs = async () => {
    if (!selectedPod) return;
    setLoading(true);
    setError(null);
    setLogs([]);
    setWorkloadLogs([]);
    try {
      if (isWorkload) {
        // Stream logs from all pods in the workload
        await StartWorkloadLogStream(
          selectedPod.kind!,
          selectedPod.namespace,
          selectedPod.name,
          100
        );
        setIsStreaming(true);
        setLoading(false);
        // Poll for workload logs
        pollRef.current = setInterval(async () => {
          try {
            const buffered = await GetBufferedWorkloadLogs();
            if (buffered && buffered.length > 0) {
              setWorkloadLogs(buffered);
            }
          } catch (e) {
            console.error("Failed to get workload logs:", e);
          }
        }, 500);
      } else {
        // Stream logs from single pod
        await StartLogStream(selectedPod.namespace, selectedPod.name, "", 100);
        setIsStreaming(true);
        setLoading(false);
        // Poll for new logs
        pollRef.current = setInterval(async () => {
          try {
            const buffered = await GetBufferedLogs();
            if (buffered && buffered.length > 0) {
              setLogs(buffered);
            }
          } catch (e) {
            console.error("Failed to get logs:", e);
          }
        }, 500);
      }
    } catch (err: unknown) {
      const errMsg =
        typeof err === "string"
          ? err
          : (err as Error)?.message || JSON.stringify(err);
      // Ignore "context canceled" errors - these are expected when switching streams
      if (!errMsg.includes("context canceled")) {
        setError(`Failed to start log stream: ${errMsg}`);
      }
      setLoading(false);
    }
  };

  const stopLogs = () => {
    if (pollRef.current) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
    StopLogStream();
    setIsStreaming(false);
  };

  useEffect(() => {
    if (autoScroll) {
      logsEndRef.current?.scrollIntoView({ behavior: "smooth" });
    }
  }, [logs, workloadLogs, autoScroll]);

  const handleScroll = () => {
    const container = logsContainerRef.current;
    if (!container) return;
    // Check if user is near the bottom (within 50px)
    const isNearBottom =
      container.scrollHeight - container.scrollTop - container.clientHeight <
      50;
    setAutoScroll(isNearBottom);
  };

  const scrollToBottom = () => {
    setAutoScroll(true);
    logsEndRef.current?.scrollIntoView({ behavior: "smooth" });
  };

  const getKindLabel = () => {
    if (!selectedPod?.kind || selectedPod.kind === "Pod") return "pod";
    return selectedPod.kind.toLowerCase();
  };

  const hasLogs = isWorkload ? workloadLogs.length > 0 : logs.length > 0;

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        {selectedPod ? (
          <div className="flex items-center gap-3">
            <span className="text-slate-400">Logs from {getKindLabel()}:</span>
            <span className="font-mono text-accent-400">
              {selectedPod.namespace}/{selectedPod.name}
            </span>
            {isWorkload && (
              <span className="text-xs px-2 py-0.5 bg-slate-700 rounded text-slate-300">
                multi-pod
              </span>
            )}
            <button
              onClick={() => {
                stopLogs();
                onClearPod();
              }}
              className="p-1 hover:bg-slate-700 rounded transition-colors"
            >
              <X size={16} className="text-slate-400" />
            </button>
          </div>
        ) : (
          <span className="text-slate-400">
            Select a pod or workload from Explorer to view logs
          </span>
        )}
        {isStreaming && (
          <div className="flex items-center gap-2 text-emerald-400 text-sm">
            <div className="w-2 h-2 bg-emerald-400 rounded-full animate-pulse" />
            Streaming
          </div>
        )}
      </div>

      {/* Logs */}
      <div className="flex-1 relative">
        <div
          ref={logsContainerRef}
          onScroll={handleScroll}
          className="absolute inset-0 bg-white dark:bg-slate-900 rounded-xl border border-slate-200 dark:border-slate-700/50 p-4 font-mono text-sm overflow-auto text-slate-800 dark:text-slate-200"
        >
          {!isConnected ? (
            <p className="text-slate-500">Connect to a cluster first</p>
          ) : !selectedPod ? (
            <p className="text-slate-500">
              No resource selected. Go to Explorer and click "View Logs" on a
              pod, deployment, or statefulset.
            </p>
          ) : loading ? (
            <div className="flex items-center gap-2 text-slate-400">
              <div className="w-4 h-4 border-2 border-accent-400 border-t-transparent rounded-full animate-spin" />
              Loading logs...
            </div>
          ) : error ? (
            <p className="text-red-400">{error}</p>
          ) : !hasLogs ? (
            <p className="text-slate-500">No logs yet...</p>
          ) : isWorkload ? (
            <>
              {workloadLogs.map((logLine, idx) => (
                <div
                  key={idx}
                  className="log-line whitespace-pre-wrap break-all"
                >
                  <span
                    className={`font-semibold ${
                      POD_COLORS[logLine.colorIdx % POD_COLORS.length]
                    }`}
                  >
                    {logLine.pod}:
                  </span>{" "}
                  <span className="text-slate-300">{logLine.line}</span>
                </div>
              ))}
              <div ref={logsEndRef} />
            </>
          ) : (
            <>
              {logs.map((line, idx) => (
                <div
                  key={idx}
                  className="log-line text-slate-300 whitespace-pre-wrap break-all"
                >
                  {line}
                </div>
              ))}
              <div ref={logsEndRef} />
            </>
          )}
        </div>
        {/* Scroll to bottom button */}
        {!autoScroll && hasLogs && (
          <button
            onClick={scrollToBottom}
            className="absolute bottom-4 right-4 px-3 py-2 bg-accent-500/90 hover:bg-accent-500 text-white rounded-lg shadow-lg flex items-center gap-2 text-sm transition-colors"
          >
            <ChevronDown size={16} />
            Scroll to bottom
          </button>
        )}
      </div>
    </div>
  );
}
