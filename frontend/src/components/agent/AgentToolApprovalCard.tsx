/**
 * AgentToolApprovalCard — Per-tool approval card for AI agent actions.
 *
 * Three states:
 *  - emerald (read-only tool) → auto-approved, no interaction needed
 *  - amber (write tool)       → single confirm required
 *  - red (destructive tool)   → double confirm: first click shows "I understand the risk" step
 *
 * Expandable parameters section.
 * Collapsible results section once executed.
 */

import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import {
  CheckCircle2,
  AlertTriangle,
  AlertCircle,
  ChevronDown,
  ChevronUp,
  Check,
  X,
  Loader2,
  Eye,
} from "lucide-react";

// ── Types ────────────────────────────────────────────────────────────────────

export type ToolRisk = "read" | "write" | "destructive";

export type ApprovalState =
  | "pending"
  | "awaiting-confirm"
  | "awaiting-double-confirm"
  | "approved"
  | "rejected"
  | "running"
  | "done"
  | "error";

export interface AgentTool {
  id: string;
  name: string;
  description: string;
  risk: ToolRisk;
  parameters: Record<string, unknown>;
  result?: string;
  error?: string;
}

interface AgentToolApprovalCardProps {
  tool: AgentTool;
  state: ApprovalState;
  onApprove: (id: string) => void;
  onReject: (id: string) => void;
}

// ── Risk config ───────────────────────────────────────────────────────────────

const riskConfig: Record<
  ToolRisk,
  {
    icon: typeof CheckCircle2;
    label: string;
    border: string;
    bg: string;
    headerBg: string;
    text: string;
    badge: string;
  }
> = {
  read: {
    icon: Eye,
    label: "Read-only",
    border: "border-emerald-500/30 dark:border-emerald-400/20",
    bg: "bg-emerald-500/5 dark:bg-emerald-400/5",
    headerBg: "bg-emerald-500/10 dark:bg-emerald-400/10",
    text: "text-emerald-700 dark:text-emerald-300",
    badge: "bg-emerald-500/15 text-emerald-700 dark:bg-emerald-400/15 dark:text-emerald-300",
  },
  write: {
    icon: AlertTriangle,
    label: "Write",
    border: "border-amber-500/30 dark:border-amber-400/20",
    bg: "bg-amber-500/5 dark:bg-amber-400/5",
    headerBg: "bg-amber-500/10 dark:bg-amber-400/10",
    text: "text-amber-700 dark:text-amber-300",
    badge: "bg-amber-500/15 text-amber-700 dark:bg-amber-400/15 dark:text-amber-300",
  },
  destructive: {
    icon: AlertCircle,
    label: "Destructive",
    border: "border-red-500/30 dark:border-red-400/20",
    bg: "bg-red-500/5 dark:bg-red-400/5",
    headerBg: "bg-red-500/10 dark:bg-red-400/10",
    text: "text-red-700 dark:text-red-300",
    badge: "bg-red-500/15 text-red-700 dark:bg-red-400/15 dark:text-red-300",
  },
};

// ── Component ─────────────────────────────────────────────────────────────────

export function AgentToolApprovalCard({
  tool,
  state,
  onApprove,
  onReject,
}: AgentToolApprovalCardProps) {
  const [paramsExpanded, setParamsExpanded] = useState(false);
  const [resultExpanded, setResultExpanded] = useState(true);
  const [localState, setLocalState] = useState<ApprovalState>(state);

  const config = riskConfig[tool.risk];
  const Icon = config.icon;

  const handleApproveClick = () => {
    if (tool.risk === "read") {
      onApprove(tool.id);
    } else if (tool.risk === "write") {
      if (localState === "pending") {
        setLocalState("awaiting-confirm");
      } else if (localState === "awaiting-confirm") {
        onApprove(tool.id);
      }
    } else if (tool.risk === "destructive") {
      if (localState === "pending") {
        setLocalState("awaiting-confirm");
      } else if (localState === "awaiting-confirm") {
        setLocalState("awaiting-double-confirm");
      } else if (localState === "awaiting-double-confirm") {
        onApprove(tool.id);
      }
    }
  };

  const handleReject = () => {
    setLocalState("rejected");
    onReject(tool.id);
  };

  const isFinished =
    localState === "approved" ||
    localState === "done" ||
    localState === "rejected" ||
    localState === "error";

  const isRunning = localState === "running";

  return (
    <motion.div
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, scale: 0.97 }}
      transition={{ duration: 0.18 }}
      className={`
        rounded-xl border overflow-hidden
        ${config.border} ${config.bg}
        shadow-sm
      `}
      role="region"
      aria-label={`Tool: ${tool.name} (${config.label})`}
    >
      {/* Header */}
      <div className={`flex items-center gap-2.5 px-3 py-2.5 ${config.headerBg}`}>
        <Icon size={14} className={`${config.text} flex-shrink-0`} aria-hidden="true" />

        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="text-xs font-semibold text-stone-800 dark:text-slate-200 font-mono">
              {tool.name}
            </span>
            <span className={`text-[10px] px-1.5 py-0.5 rounded-full font-medium ${config.badge}`}>
              {config.label}
            </span>
            {isRunning && (
              <span className="flex items-center gap-1 text-[10px] text-stone-400 dark:text-slate-500">
                <Loader2 size={10} className="animate-spin" aria-hidden="true" />
                Running...
              </span>
            )}
            {(localState === "done" || localState === "approved") && (
              <span className="flex items-center gap-1 text-[10px] text-emerald-600 dark:text-emerald-400">
                <CheckCircle2 size={10} aria-hidden="true" />
                Done
              </span>
            )}
            {localState === "rejected" && (
              <span className="flex items-center gap-1 text-[10px] text-stone-400 dark:text-slate-500">
                <X size={10} aria-hidden="true" />
                Rejected
              </span>
            )}
            {localState === "error" && (
              <span className="flex items-center gap-1 text-[10px] text-red-500 dark:text-red-400">
                <AlertCircle size={10} aria-hidden="true" />
                Error
              </span>
            )}
          </div>
          <p className="text-[11px] text-stone-500 dark:text-slate-500 truncate mt-0.5">
            {tool.description}
          </p>
        </div>

        {/* Auto-approved badge for read tools */}
        {tool.risk === "read" && !isFinished && (
          <span className="text-[10px] text-emerald-600 dark:text-emerald-400 flex-shrink-0">
            auto
          </span>
        )}
      </div>

      {/* Parameters */}
      <div className="border-t border-current/10">
        <button
          onClick={() => setParamsExpanded(!paramsExpanded)}
          className="
            w-full flex items-center justify-between
            px-3 py-1.5 text-[11px]
            text-stone-500 dark:text-slate-500
            hover:text-stone-700 dark:hover:text-slate-300
            transition-colors
            focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent-500/50
          "
          aria-expanded={paramsExpanded}
          aria-controls={`params-${tool.id}`}
        >
          <span>Parameters ({Object.keys(tool.parameters).length})</span>
          {paramsExpanded ? (
            <ChevronUp size={12} aria-hidden="true" />
          ) : (
            <ChevronDown size={12} aria-hidden="true" />
          )}
        </button>

        <AnimatePresence>
          {paramsExpanded && (
            <motion.div
              id={`params-${tool.id}`}
              initial={{ height: 0, opacity: 0 }}
              animate={{ height: "auto", opacity: 1 }}
              exit={{ height: 0, opacity: 0 }}
              transition={{ duration: 0.15 }}
              className="overflow-hidden"
            >
              <pre className="px-3 pb-2 text-[10px] font-mono text-stone-600 dark:text-slate-400 overflow-x-auto whitespace-pre-wrap break-all leading-relaxed">
                {JSON.stringify(tool.parameters, null, 2)}
              </pre>
            </motion.div>
          )}
        </AnimatePresence>
      </div>

      {/* Result / Error */}
      {(tool.result || tool.error) && (
        <div className="border-t border-current/10">
          <button
            onClick={() => setResultExpanded(!resultExpanded)}
            className="
              w-full flex items-center justify-between
              px-3 py-1.5 text-[11px]
              text-stone-500 dark:text-slate-500
              hover:text-stone-700 dark:hover:text-slate-300
              transition-colors
              focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent-500/50
            "
            aria-expanded={resultExpanded}
            aria-controls={`result-${tool.id}`}
          >
            <span>{tool.error ? "Error" : "Result"}</span>
            {resultExpanded ? (
              <ChevronUp size={12} aria-hidden="true" />
            ) : (
              <ChevronDown size={12} aria-hidden="true" />
            )}
          </button>

          <AnimatePresence>
            {resultExpanded && (
              <motion.div
                id={`result-${tool.id}`}
                initial={{ height: 0, opacity: 0 }}
                animate={{ height: "auto", opacity: 1 }}
                exit={{ height: 0, opacity: 0 }}
                transition={{ duration: 0.15 }}
                className="overflow-hidden"
              >
                <pre
                  className={`
                    px-3 pb-2 text-[10px] font-mono overflow-x-auto whitespace-pre-wrap break-all leading-relaxed
                    ${tool.error ? "text-red-600 dark:text-red-400" : "text-stone-600 dark:text-slate-400"}
                  `}
                >
                  {tool.error ?? tool.result}
                </pre>
              </motion.div>
            )}
          </AnimatePresence>
        </div>
      )}

      {/* Approval actions — only shown for pending write/destructive, non-read */}
      {tool.risk !== "read" && !isFinished && !isRunning && (
        <div className="border-t border-current/10 px-3 py-2.5">
          <AnimatePresence mode="wait">
            {localState === "awaiting-double-confirm" ? (
              <motion.div
                key="double-confirm"
                initial={{ opacity: 0, y: 4 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0 }}
                className="space-y-2"
              >
                <p className="text-[11px] text-red-600 dark:text-red-400 font-medium">
                  This action is destructive and may be irreversible.
                </p>
                <div className="flex items-center gap-2">
                  <button
                    onClick={handleApproveClick}
                    className="
                      flex items-center gap-1.5 px-3 py-1.5 rounded-lg
                      text-[11px] font-semibold
                      bg-red-500 hover:bg-red-600 text-white
                      transition-colors
                      focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-red-500/50
                    "
                    aria-label="I understand the risk, proceed"
                  >
                    <Check size={11} aria-hidden="true" />
                    I understand the risk
                  </button>
                  <button
                    onClick={handleReject}
                    className="
                      px-3 py-1.5 rounded-lg
                      text-[11px] text-stone-500 dark:text-slate-400
                      hover:bg-stone-100 dark:hover:bg-slate-700/60
                      transition-colors
                      focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-slate-500/50
                    "
                    aria-label="Cancel"
                  >
                    Cancel
                  </button>
                </div>
              </motion.div>
            ) : localState === "awaiting-confirm" ? (
              <motion.div
                key="confirm"
                initial={{ opacity: 0, y: 4 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0 }}
                className="flex items-center gap-2"
              >
                <span className="text-[11px] text-stone-500 dark:text-slate-400 flex-1">
                  {tool.risk === "destructive"
                    ? "Confirm this action"
                    : "Allow this write?"}
                </span>
                <button
                  onClick={handleApproveClick}
                  className={`
                    flex items-center gap-1 px-2.5 py-1.5 rounded-lg
                    text-[11px] font-medium
                    transition-colors
                    focus-visible:outline-none focus-visible:ring-2
                    ${tool.risk === "destructive"
                      ? "bg-red-500/15 hover:bg-red-500/25 text-red-600 dark:text-red-400 focus-visible:ring-red-500/50"
                      : "bg-amber-500/15 hover:bg-amber-500/25 text-amber-700 dark:text-amber-300 focus-visible:ring-amber-500/50"
                    }
                  `}
                >
                  <Check size={11} aria-hidden="true" />
                  Confirm
                </button>
                <button
                  onClick={handleReject}
                  className="
                    flex items-center gap-1 px-2.5 py-1.5 rounded-lg
                    text-[11px] text-stone-500 dark:text-slate-400
                    hover:bg-stone-100 dark:hover:bg-slate-700/60
                    transition-colors
                    focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-slate-500/50
                  "
                >
                  <X size={11} aria-hidden="true" />
                  Reject
                </button>
              </motion.div>
            ) : (
              <motion.div
                key="initial"
                initial={{ opacity: 0, y: 4 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0 }}
                className="flex items-center gap-2"
              >
                <button
                  onClick={handleApproveClick}
                  className={`
                    flex items-center gap-1 px-2.5 py-1.5 rounded-lg
                    text-[11px] font-medium
                    transition-colors
                    focus-visible:outline-none focus-visible:ring-2
                    ${tool.risk === "destructive"
                      ? "bg-red-500/15 hover:bg-red-500/25 text-red-600 dark:text-red-400 focus-visible:ring-red-500/50"
                      : "bg-amber-500/15 hover:bg-amber-500/25 text-amber-700 dark:text-amber-300 focus-visible:ring-amber-500/50"
                    }
                  `}
                  aria-label={`Approve tool: ${tool.name}`}
                >
                  <Check size={11} aria-hidden="true" />
                  Allow
                </button>
                <button
                  onClick={handleReject}
                  className="
                    flex items-center gap-1 px-2.5 py-1.5 rounded-lg
                    text-[11px] text-stone-500 dark:text-slate-400
                    hover:bg-stone-100 dark:hover:bg-slate-700/60
                    transition-colors
                    focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-slate-500/50
                  "
                  aria-label={`Reject tool: ${tool.name}`}
                >
                  <X size={11} aria-hidden="true" />
                  Reject
                </button>
              </motion.div>
            )}
          </AnimatePresence>
        </div>
      )}
    </motion.div>
  );
}
