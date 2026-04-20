import { motion, AnimatePresence } from "framer-motion";
import {
  X,
  AlertTriangle,
  Check,
  Loader2,
  Play,
  Rocket,
  ArrowRight,
} from "lucide-react";
import type { ApplyConfirmModalProps } from "./types";

export function ApplyConfirmModal({
  isOpen,
  onClose,
  onConfirm,
  targetContext,
  resourceInfo,
  differences,
  isApplying,
  applyResult,
}: ApplyConfirmModalProps) {
  const isProduction =
    targetContext.toLowerCase().includes("prod") ||
    targetContext.toLowerCase().includes("production");

  const criticalCount = differences.filter(
    (d) => d.severity === "critical"
  ).length;
  const warningCount = differences.filter(
    (d) => d.severity === "warning"
  ).length;

  return (
    <AnimatePresence>
      {isOpen && (
        <>
          {/* Backdrop */}
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={onClose}
            className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50"
          />

          {/* Modal */}
          <motion.div
            initial={{ opacity: 0, scale: 0.95, y: 20 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.95, y: 20 }}
            className="fixed inset-0 z-50 flex items-center justify-center p-4"
          >
            <div className="w-full max-w-lg bg-white dark:bg-zinc-900 border border-zinc-200 dark:border-zinc-700 rounded-xl shadow-2xl overflow-hidden">
              {/* Header */}
              <div className="flex items-center justify-between px-4 py-3 bg-zinc-50 dark:bg-zinc-800 border-b border-zinc-200 dark:border-zinc-700">
                <div className="flex items-center gap-2">
                  <Rocket className="w-5 h-5 text-teal-600 dark:text-teal-400" />
                  <h2 className="text-lg font-medium text-zinc-900 dark:text-zinc-100">
                    Apply Changes
                  </h2>
                </div>
                <button
                  onClick={onClose}
                  disabled={isApplying}
                  className="p-1 text-zinc-500 hover:text-zinc-700 dark:hover:text-zinc-300 rounded transition-colors 
                           disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  <X className="w-5 h-5" />
                </button>
              </div>

              {/* Content */}
              <div className="p-4 space-y-4">
                {/* Target cluster warning */}
                {isProduction && (
                  <motion.div
                    initial={{ opacity: 0, x: -10 }}
                    animate={{ opacity: 1, x: 0 }}
                    className="flex items-start gap-3 p-3 bg-red-500/10 border border-red-500/30 
                             rounded-lg text-red-600 dark:text-red-400"
                  >
                    <AlertTriangle className="w-5 h-5 flex-shrink-0 mt-0.5" />
                    <div>
                      <div className="font-medium">Production Cluster</div>
                      <div className="text-sm text-red-600/80 dark:text-red-400/80">
                        You are about to modify a production cluster. Please
                        proceed with caution.
                      </div>
                    </div>
                  </motion.div>
                )}

                {/* Resource info */}
                <div className="p-3 bg-zinc-50 dark:bg-zinc-800/50 rounded-lg border border-zinc-200 dark:border-zinc-700">
                  <div className="text-sm text-zinc-500 mb-1">
                    Target Resource
                  </div>
                  <div className="font-mono text-sm text-zinc-800 dark:text-zinc-200">
                    {resourceInfo.kind}/{resourceInfo.namespace}/
                    {resourceInfo.name}
                  </div>
                  <div className="text-xs text-zinc-500 mt-1">
                    in cluster:{" "}
                    <span
                      className="text-teal-600 dark:text-teal-400 break-all"
                      title={targetContext}
                    >
                      {targetContext}
                    </span>
                  </div>
                </div>

                {/* Changes summary */}
                <div className="p-3 bg-zinc-50 dark:bg-zinc-800/50 rounded-lg border border-zinc-200 dark:border-zinc-700">
                  <div className="text-sm text-zinc-500 mb-2">
                    Changes to Apply ({differences.length})
                  </div>
                  <div className="space-y-1.5 max-h-40 overflow-auto">
                    {differences.slice(0, 10).map((diff, idx) => {
                      // Hard-truncating the values being applied is a
                      // data-integrity footgun — the user literally cannot
                      // see what they're about to push. Show the full value
                      // inline on a second line (monospace, wrapping) and
                      // keep the inline preview for the common short case.
                      const leftDisplay = diff.leftValue || "(empty)";
                      const rightDisplay = diff.rightValue || "(empty)";
                      const longValue =
                        leftDisplay.length > 20 || rightDisplay.length > 20;
                      return (
                        <div
                          key={idx}
                          className="flex flex-col gap-0.5 text-xs py-0.5"
                        >
                          <div className="flex items-center gap-2">
                            <span
                              className={`w-1.5 h-1.5 rounded-full flex-shrink-0 ${
                                diff.severity === "critical"
                                  ? "bg-red-500"
                                  : diff.severity === "warning"
                                  ? "bg-amber-500"
                                  : "bg-blue-500"
                              }`}
                            />
                            <span
                              className="font-mono text-zinc-600 dark:text-zinc-400 flex-1 truncate"
                              title={diff.path}
                            >
                              {diff.path}
                            </span>
                            {!longValue && (
                              <>
                                <span
                                  className="text-zinc-700 dark:text-zinc-600 truncate max-w-[120px]"
                                  title={leftDisplay}
                                >
                                  {leftDisplay}
                                </span>
                                <ArrowRight className="w-3 h-3 text-zinc-400 dark:text-zinc-600 flex-shrink-0" />
                                <span
                                  className="text-zinc-900 dark:text-zinc-300 truncate max-w-[120px]"
                                  title={rightDisplay}
                                >
                                  {rightDisplay}
                                </span>
                              </>
                            )}
                          </div>
                          {longValue && (
                            <div className="pl-3.5 grid grid-cols-[1fr_auto_1fr] items-start gap-2 font-mono text-[11px]">
                              <code className="text-zinc-700 dark:text-zinc-400 bg-zinc-100 dark:bg-zinc-800/60 rounded px-1.5 py-0.5 break-all whitespace-pre-wrap">
                                {leftDisplay}
                              </code>
                              <ArrowRight className="w-3 h-3 text-zinc-400 dark:text-zinc-600 flex-shrink-0 mt-1" />
                              <code className="text-zinc-900 dark:text-zinc-200 bg-zinc-100 dark:bg-zinc-800/60 rounded px-1.5 py-0.5 break-all whitespace-pre-wrap">
                                {rightDisplay}
                              </code>
                            </div>
                          )}
                        </div>
                      );
                    })}
                    {differences.length > 10 && (
                      <div className="text-xs text-zinc-500 pt-1">
                        +{differences.length - 10} more changes
                      </div>
                    )}
                  </div>
                </div>

                {/* Severity summary */}
                {(criticalCount > 0 || warningCount > 0) && (
                  <div className="flex items-center gap-3 text-xs">
                    {criticalCount > 0 && (
                      <span className="flex items-center gap-1 text-red-600 dark:text-red-400">
                        <AlertTriangle className="w-3.5 h-3.5" />
                        {criticalCount} critical
                      </span>
                    )}
                    {warningCount > 0 && (
                      <span className="flex items-center gap-1 text-amber-600 dark:text-amber-400">
                        <AlertTriangle className="w-3.5 h-3.5" />
                        {warningCount} warnings
                      </span>
                    )}
                  </div>
                )}

                {/* Apply result */}
                {applyResult && (
                  <motion.div
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    className={`p-3 rounded-lg border ${
                      applyResult.success
                        ? "bg-green-500/10 border-green-500/30 text-green-600 dark:text-green-400"
                        : "bg-red-500/10 border-red-500/30 text-red-600 dark:text-red-400"
                    }`}
                  >
                    <div className="flex items-center gap-2 font-medium">
                      {applyResult.success ? (
                        <Check className="w-4 h-4" />
                      ) : (
                        <X className="w-4 h-4" />
                      )}
                      {applyResult.dryRun ? "Dry Run " : ""}
                      {applyResult.success ? "Success" : "Failed"}
                    </div>
                    <div className="text-sm mt-1 opacity-80">
                      {applyResult.message}
                    </div>
                    {applyResult.warnings.length > 0 && (
                      <div className="mt-2 text-xs text-amber-500 dark:text-amber-400">
                        {applyResult.warnings.map((w, i) => (
                          <div key={i}>⚠ {w}</div>
                        ))}
                      </div>
                    )}
                  </motion.div>
                )}
              </div>

              {/* Actions */}
              <div className="flex items-center justify-between px-4 py-3 bg-zinc-50 dark:bg-zinc-800/50 border-t border-zinc-200 dark:border-zinc-700">
                {applyResult?.success ? (
                  <button
                    onClick={onClose}
                    className="w-full flex items-center justify-center gap-2 px-4 py-2 bg-stone-200 hover:bg-stone-300 dark:bg-slate-700 dark:hover:bg-slate-600 text-stone-900 dark:text-slate-200 rounded-lg transition-colors font-medium"
                  >
                    Close
                  </button>
                ) : (
                  <>
                    <button
                      onClick={onClose}
                      disabled={isApplying}
                      className="px-4 py-2 text-sm text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200 
                               transition-colors disabled:opacity-50"
                    >
                      Cancel
                    </button>
                    <div className="flex items-center gap-2">
                      <button
                        onClick={() => onConfirm(true)}
                        disabled={isApplying}
                        className="flex items-center gap-2 px-4 py-2 text-sm bg-zinc-200 hover:bg-zinc-300 dark:bg-zinc-700 
                                 dark:hover:bg-zinc-600 text-zinc-900 dark:text-zinc-200 rounded-md transition-colors
                                 disabled:opacity-50 disabled:cursor-not-allowed"
                      >
                        {isApplying ? (
                          <Loader2 className="w-4 h-4 animate-spin" />
                        ) : (
                          <Play className="w-4 h-4" />
                        )}
                        Dry Run
                      </button>
                      <button
                        onClick={() => onConfirm(false)}
                        disabled={isApplying}
                        className={`flex items-center gap-2 px-4 py-2 text-sm rounded-md 
                                 transition-colors disabled:opacity-50 disabled:cursor-not-allowed
                                 ${
                                   isProduction
                                     ? "bg-red-600 hover:bg-red-500 text-white"
                                     : "bg-teal-600 hover:bg-teal-500 text-white"
                                 }`}
                      >
                        {isApplying ? (
                          <Loader2 className="w-4 h-4 animate-spin" />
                        ) : (
                          <Rocket className="w-4 h-4" />
                        )}
                        Apply Changes
                      </button>
                    </div>
                  </>
                )}
              </div>
            </div>
          </motion.div>
        </>
      )}
    </AnimatePresence>
  );
}
