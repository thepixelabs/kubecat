import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import {
  Server,
  Clock,
  ChevronDown,
  Check,
  History,
  Wifi,
  WifiOff,
} from "lucide-react";
import type { SourceSelectorProps } from "./types";

export function SourceSelector({
  contexts,
  snapshots,
  value,
  onChange,
  label,
  isTimelineAvailable,
  readOnly,
}: SourceSelectorProps) {
  const [contextOpen, setContextOpen] = useState(false);
  const [snapshotOpen, setSnapshotOpen] = useState(false);

  const handleContextChange = (context: string) => {
    onChange({
      ...value,
      context,
    });
    setContextOpen(false);
  };

  const handleModeChange = (isLive: boolean) => {
    onChange({
      ...value,
      isLive,
      snapshot: isLive ? undefined : value.snapshot,
    });
  };

  const handleSnapshotChange = (snapshot: string) => {
    onChange({
      ...value,
      snapshot,
      isLive: false,
    });
    setSnapshotOpen(false);
  };

  const formatTimestamp = (ts: string) => {
    try {
      const date = new Date(ts);
      return date.toLocaleString();
    } catch {
      return ts;
    }
  };

  return (
    <div className="flex flex-col gap-2 p-3 bg-white dark:bg-slate-800/50 rounded-lg border border-stone-200 dark:border-slate-700">
      {/* Label */}
      <div className="flex items-center gap-2 text-sm font-medium text-stone-700 dark:text-slate-300">
        {label}
      </div>

      {/* Context Selector */}
      <div className="relative">
        <button
          onClick={() => !readOnly && setContextOpen(!contextOpen)}
          disabled={readOnly}
          title={value.context || "Select cluster"}
          className={`w-full flex items-center justify-between gap-2 px-3 py-2 bg-stone-50 dark:bg-slate-900
                     border border-stone-200 dark:border-slate-700 rounded-md transition-colors ${
                       readOnly
                         ? "opacity-80 cursor-default"
                         : "hover:border-stone-300 dark:hover:border-slate-600"
                     }`}
        >
          <div className="flex items-center gap-2 min-w-0">
            <Server className="w-4 h-4 text-accent-500 flex-shrink-0" />
            <span
              className="truncate text-sm text-stone-700 dark:text-slate-200"
              title={value.context || "Select cluster"}
            >
              {value.context || "Select cluster..."}
            </span>
          </div>
          {!readOnly && (
            <ChevronDown
              className={`w-4 h-4 text-stone-400 dark:text-slate-500 transition-transform ${
                contextOpen ? "rotate-180" : ""
              }`}
            />
          )}
        </button>

        <AnimatePresence>
          {contextOpen && (
            <motion.div
              initial={{ opacity: 0, y: -4 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -4 }}
              className="absolute z-20 top-full left-0 right-0 mt-1 py-1 bg-white dark:bg-slate-900 
                         border border-stone-200 dark:border-slate-700 rounded-md shadow-xl max-h-48 overflow-auto"
            >
              {contexts.length === 0 ? (
                <div className="px-3 py-2 text-sm text-stone-500 dark:text-slate-500 italic">
                  No clusters available
                </div>
              ) : (
                contexts.map((ctx) => (
                  <button
                    key={ctx}
                    onClick={() => handleContextChange(ctx)}
                    title={ctx}
                    className={`w-full flex items-center justify-between px-3 py-2 text-sm
                               hover:bg-stone-100 dark:hover:bg-slate-800 transition-colors ${
                                 value.context === ctx
                                   ? "text-accent-600 dark:text-accent-400 bg-accent-50 dark:bg-accent-500/10"
                                   : "text-stone-700 dark:text-slate-300"
                               }`}
                  >
                    <span className="truncate" title={ctx}>{ctx}</span>
                    {value.context === ctx && <Check className="w-4 h-4 flex-shrink-0" />}
                  </button>
                ))
              )}
            </motion.div>
          )}
        </AnimatePresence>
      </div>

      {/* Mode Toggle: Live vs Historical */}
      <div className="flex items-center gap-1 p-1 bg-stone-50 dark:bg-slate-900 rounded-md">
        <button
          onClick={() => handleModeChange(true)}
          className={`flex-1 flex items-center justify-center gap-2 px-3 py-1.5 rounded text-sm 
                     transition-colors ${
                       value.isLive
                         ? "bg-white dark:bg-slate-800 text-accent-600 dark:text-accent-400 border border-stone-200 dark:border-slate-700 shadow-sm"
                         : "text-stone-500 dark:text-slate-400 hover:text-stone-700 dark:hover:text-slate-300"
                     }`}
        >
          <Wifi className="w-3.5 h-3.5" />
          Live
        </button>
        <button
          onClick={() => handleModeChange(false)}
          disabled={!isTimelineAvailable}
          className={`flex-1 flex items-center justify-center gap-2 px-3 py-1.5 rounded text-sm 
                     transition-colors ${
                       !value.isLive
                         ? "bg-white dark:bg-slate-800 text-purple-600 dark:text-purple-400 border border-stone-200 dark:border-slate-700 shadow-sm"
                         : "text-stone-500 dark:text-slate-400 hover:text-stone-700 dark:hover:text-slate-300"
                     } ${
            !isTimelineAvailable ? "opacity-50 cursor-not-allowed" : ""
          }`}
          title={
            !isTimelineAvailable ? "Timeline not available" : "Use snapshot"
          }
        >
          <History className="w-3.5 h-3.5" />
          Historical
        </button>
      </div>

      {/* Snapshot Selector (when historical mode) */}
      {!value.isLive && (
        <motion.div
          initial={{ opacity: 0, height: 0 }}
          animate={{ opacity: 1, height: "auto" }}
          exit={{ opacity: 0, height: 0 }}
          className="relative"
        >
          <button
            onClick={() => setSnapshotOpen(!snapshotOpen)}
            className="w-full flex items-center justify-between gap-2 px-3 py-2 bg-stone-50 dark:bg-slate-900 
                       border border-stone-200 dark:border-slate-700 rounded-md hover:border-stone-300 dark:hover:border-slate-600 transition-colors"
          >
            <div className="flex items-center gap-2 min-w-0">
              <Clock className="w-4 h-4 text-purple-600 dark:text-purple-400 flex-shrink-0" />
              <span className="truncate text-sm text-stone-700 dark:text-slate-200">
                {value.snapshot
                  ? formatTimestamp(value.snapshot)
                  : "Select snapshot..."}
              </span>
            </div>
            <ChevronDown
              className={`w-4 h-4 text-stone-400 dark:text-slate-500 transition-transform ${
                snapshotOpen ? "rotate-180" : ""
              }`}
            />
          </button>

          <AnimatePresence>
            {snapshotOpen && (
              <motion.div
                initial={{ opacity: 0, y: -4 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: -4 }}
                className="absolute z-20 top-full left-0 right-0 mt-1 py-1 bg-white dark:bg-slate-900 
                           border border-stone-200 dark:border-slate-700 rounded-md shadow-xl max-h-48 overflow-auto"
              >
                {snapshots.length === 0 ? (
                  <div className="px-3 py-2 text-sm text-stone-500 dark:text-slate-500 italic">
                    No snapshots available
                  </div>
                ) : (
                  snapshots.map((snap) => (
                    <button
                      key={snap.timestamp}
                      onClick={() => handleSnapshotChange(snap.timestamp)}
                      className={`w-full flex items-center justify-between px-3 py-2 text-sm 
                                 hover:bg-stone-100 dark:hover:bg-slate-800 transition-colors ${
                                   value.snapshot === snap.timestamp
                                     ? "text-purple-600 dark:text-purple-400 bg-purple-50 dark:bg-purple-500/10"
                                     : "text-stone-500 dark:text-slate-300"
                                 }`}
                    >
                      <span>{formatTimestamp(snap.timestamp)}</span>
                      {value.snapshot === snap.timestamp && (
                        <Check className="w-4 h-4" />
                      )}
                    </button>
                  ))
                )}
              </motion.div>
            )}
          </AnimatePresence>
        </motion.div>
      )}

      {/* Status indicator */}
      <div className="flex items-center gap-2 text-xs text-stone-500 dark:text-slate-500">
        {value.isLive ? (
          <>
            <Wifi className="w-3 h-3 text-emerald-500" />
            <span>Live cluster state</span>
          </>
        ) : value.snapshot ? (
          <>
            <Clock className="w-3 h-3 text-purple-600 dark:text-purple-400" />
            <span>Snapshot from {formatTimestamp(value.snapshot)}</span>
          </>
        ) : (
          <>
            <WifiOff className="w-3 h-3 text-stone-400 dark:text-slate-600" />
            <span>Select a snapshot</span>
          </>
        )}
      </div>
    </div>
  );
}
