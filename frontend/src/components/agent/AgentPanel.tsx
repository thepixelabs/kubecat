/**
 * AgentPanel — Real-time AI agent progress feed.
 *
 * Subscribes to Wails runtime events:
 *   ai:agent:thinking    — agent is processing, optional message
 *   ai:agent:tool-call   — agent is about to call a tool
 *   ai:agent:tool-result — tool returned a result
 *   ai:agent:complete    — agent finished, final answer provided
 *   ai:agent:error       — agent hit an error
 *
 * Session-scoped: only events matching the current sessionId are shown.
 * Status bar shows tool count, token usage, and iteration count.
 * Stop button fires a cancel callback.
 * Final answer rendered as sanitized Markdown.
 */

import { useEffect, useRef, useState, useCallback } from "react";
import { motion, AnimatePresence } from "framer-motion";
import {
  Bot,
  CheckCircle2,
  AlertCircle,
  Loader2,
  Square,
  ChevronDown,
  ChevronUp,
  Sparkles,
} from "lucide-react";
import Markdown from "react-markdown";
import rehypeSanitize, { defaultSchema } from "rehype-sanitize";
import { EventsOn } from "../../../wailsjs/runtime/runtime";
import {
  AgentToolApprovalCard,
  type AgentTool,
  type ApprovalState,
} from "./AgentToolApprovalCard";

// ── Markdown sanitization (mirrors AIQueryView) ───────────────────────────────

const markdownSchema = {
  ...defaultSchema,
  protocols: {
    href: ["http", "https", "mailto"],
    src: ["http", "https"],
  },
};

// ── Types ────────────────────────────────────────────────────────────────────

type AgentStatus = "idle" | "thinking" | "running" | "done" | "error";

interface AgentEvent {
  id: string;
  type:
    | "thinking"
    | "tool-call"
    | "tool-result"
    | "complete"
    | "error";
  timestamp: Date;
  message?: string;
  tool?: AgentTool;
  toolApprovalState?: ApprovalState;
  tokens?: number;
}

interface AgentPanelProps {
  /** Session ID — only events with this sessionId are processed */
  sessionId: string;
  /** Called when the user presses Stop */
  onStop?: () => void;
  /** Called when user approves a tool */
  onApproveTool?: (toolId: string) => void;
  /** Called when user rejects a tool */
  onRejectTool?: (toolId: string) => void;
}

// ── Helpers ──────────────────────────────────────────────────────────────────

function genId() {
  return Math.random().toString(36).slice(2);
}

// ── Component ─────────────────────────────────────────────────────────────────

export function AgentPanel({
  sessionId,
  onStop,
  onApproveTool,
  onRejectTool,
}: AgentPanelProps) {
  const [status, setStatus] = useState<AgentStatus>("idle");
  const [events, setEvents] = useState<AgentEvent[]>([]);
  const [finalAnswer, setFinalAnswer] = useState<string | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [totalTokens, setTotalTokens] = useState(0);
  const [toolCount, setToolCount] = useState(0);
  const [iterations, setIterations] = useState(0);
  const [answerExpanded, setAnswerExpanded] = useState(true);
  const feedRef = useRef<HTMLDivElement>(null);
  const [autoScroll, setAutoScroll] = useState(true);

  // Auto-scroll to bottom when new events arrive
  useEffect(() => {
    if (!autoScroll || !feedRef.current) return;
    feedRef.current.scrollTop = feedRef.current.scrollHeight;
  }, [events, autoScroll]);

  const handleScroll = useCallback(() => {
    if (!feedRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = feedRef.current;
    setAutoScroll(scrollHeight - scrollTop - clientHeight < 60);
  }, []);

  // Subscribe to Wails runtime events for this session
  useEffect(() => {
    if (!sessionId) return;

    const addEvent = (event: Omit<AgentEvent, "id" | "timestamp">) => {
      setEvents((prev) => [
        ...prev,
        { ...event, id: genId(), timestamp: new Date() },
      ]);
    };

    const handleThinking = (data: { sessionId: string; message?: string }) => {
      if (data.sessionId !== sessionId) return;
      setStatus("thinking");
      setIterations((n) => n + 1);
      addEvent({ type: "thinking", message: data.message ?? "Thinking..." });
    };

    const handleToolCall = (data: {
      sessionId: string;
      tool: AgentTool;
    }) => {
      if (data.sessionId !== sessionId) return;
      setStatus("running");
      setToolCount((n) => n + 1);
      addEvent({
        type: "tool-call",
        tool: { ...data.tool },
        toolApprovalState:
          data.tool.risk === "read" ? "approved" : "pending",
      });
    };

    const handleToolResult = (data: {
      sessionId: string;
      toolId: string;
      result: string;
      tokens?: number;
    }) => {
      if (data.sessionId !== sessionId) return;
      if (data.tokens) setTotalTokens((t) => t + data.tokens!);
      setEvents((prev) =>
        prev.map((ev) =>
          ev.tool?.id === data.toolId
            ? {
                ...ev,
                type: "tool-result" as const,
                tool: ev.tool ? { ...ev.tool, result: data.result } : ev.tool,
                toolApprovalState: "done",
              }
            : ev
        )
      );
    };

    const handleComplete = (data: {
      sessionId: string;
      answer: string;
      totalTokens?: number;
    }) => {
      if (data.sessionId !== sessionId) return;
      setStatus("done");
      setFinalAnswer(data.answer);
      if (data.totalTokens) setTotalTokens(data.totalTokens);
      addEvent({ type: "complete" });
    };

    const handleError = (data: {
      sessionId: string;
      message: string;
    }) => {
      if (data.sessionId !== sessionId) return;
      setStatus("error");
      setErrorMessage(data.message);
      addEvent({ type: "error", message: data.message });
    };

    const unsubThinking = EventsOn("ai:agent:thinking", handleThinking);
    const unsubToolCall = EventsOn("ai:agent:tool-call", handleToolCall);
    const unsubToolResult = EventsOn("ai:agent:tool-result", handleToolResult);
    const unsubComplete = EventsOn("ai:agent:complete", handleComplete);
    const unsubError = EventsOn("ai:agent:error", handleError);

    return () => {
      unsubThinking?.();
      unsubToolCall?.();
      unsubToolResult?.();
      unsubComplete?.();
      unsubError?.();
    };
  }, [sessionId]);

  const isActive = status === "thinking" || status === "running";

  return (
    <div
      className="
        flex flex-col h-full
        bg-slate-900/40 dark:bg-slate-950/40
        border border-slate-700/40
        rounded-xl overflow-hidden
        backdrop-blur-sm
      "
      role="region"
      aria-label="AI Agent progress"
      aria-live="polite"
    >
      {/* Status bar */}
      <div className="flex items-center gap-3 px-3 py-2.5 border-b border-slate-700/40 bg-slate-800/30 flex-shrink-0">
        <div className="flex items-center gap-1.5">
          {isActive ? (
            <Loader2
              size={13}
              className="text-accent-400 animate-spin"
              aria-hidden="true"
            />
          ) : status === "done" ? (
            <CheckCircle2 size={13} className="text-emerald-400" aria-hidden="true" />
          ) : status === "error" ? (
            <AlertCircle size={13} className="text-red-400" aria-hidden="true" />
          ) : (
            <Bot size={13} className="text-slate-500" aria-hidden="true" />
          )}
          <span className="text-xs font-medium text-slate-300">
            {status === "thinking"
              ? "Thinking..."
              : status === "running"
              ? "Running tools"
              : status === "done"
              ? "Complete"
              : status === "error"
              ? "Error"
              : "Agent"}
          </span>
        </div>

        {/* Stats */}
        <div className="flex items-center gap-3 ml-auto text-[10px] text-slate-500 font-mono">
          {toolCount > 0 && (
            <span aria-label={`${toolCount} tool calls`}>
              {toolCount} tool{toolCount !== 1 ? "s" : ""}
            </span>
          )}
          {iterations > 0 && (
            <span aria-label={`${iterations} iterations`}>
              {iterations} iter
            </span>
          )}
          {totalTokens > 0 && (
            <span aria-label={`${totalTokens.toLocaleString()} tokens used`}>
              {totalTokens.toLocaleString()} tok
            </span>
          )}
        </div>

        {/* Stop button */}
        {isActive && onStop && (
          <button
            onClick={onStop}
            className="
              flex items-center gap-1 px-2 py-1 rounded-md
              text-[11px] font-medium
              text-red-400 hover:text-red-300
              bg-red-400/10 hover:bg-red-400/20
              border border-red-400/20 hover:border-red-400/30
              transition-colors duration-150
              focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-red-400/50
            "
            aria-label="Stop agent"
          >
            <Square size={10} aria-hidden="true" />
            Stop
          </button>
        )}
      </div>

      {/* Event feed */}
      <div
        ref={feedRef}
        onScroll={handleScroll}
        className="flex-1 overflow-y-auto p-3 space-y-2 min-h-0"
      >
        <AnimatePresence initial={false}>
          {events.map((event) => (
            <AgentEventRow
              key={event.id}
              event={event}
              onApprove={onApproveTool}
              onReject={onRejectTool}
            />
          ))}
        </AnimatePresence>

        {events.length === 0 && status === "idle" && (
          <div className="flex flex-col items-center justify-center h-full py-8 text-center">
            <Bot
              size={28}
              className="text-slate-600 mb-2"
              aria-hidden="true"
            />
            <p className="text-xs text-slate-500">Agent is ready</p>
          </div>
        )}
      </div>

      {/* Final answer */}
      <AnimatePresence>
        {finalAnswer && (
          <motion.div
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: "auto", opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            className="border-t border-slate-700/40 overflow-hidden flex-shrink-0"
          >
            {/* Answer header */}
            <button
              onClick={() => setAnswerExpanded(!answerExpanded)}
              className="
                w-full flex items-center gap-2 px-3 py-2
                bg-emerald-500/10 hover:bg-emerald-500/15
                transition-colors
                focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-emerald-500/50
              "
              aria-expanded={answerExpanded}
              aria-controls="agent-final-answer"
            >
              <Sparkles
                size={12}
                className="text-emerald-400 flex-shrink-0"
                aria-hidden="true"
              />
              <span className="text-xs font-semibold text-emerald-300 flex-1 text-left">
                Final Answer
              </span>
              {answerExpanded ? (
                <ChevronDown size={12} className="text-slate-500" aria-hidden="true" />
              ) : (
                <ChevronUp size={12} className="text-slate-500" aria-hidden="true" />
              )}
            </button>

            <AnimatePresence>
              {answerExpanded && (
                <motion.div
                  id="agent-final-answer"
                  initial={{ height: 0 }}
                  animate={{ height: "auto" }}
                  exit={{ height: 0 }}
                  className="overflow-hidden"
                >
                  <div className="px-3 py-3 max-h-60 overflow-y-auto">
                    <div className="prose prose-sm prose-invert max-w-none text-slate-300 text-xs leading-relaxed">
                      <Markdown
                        rehypePlugins={[[rehypeSanitize, markdownSchema]]}
                      >
                        {finalAnswer}
                      </Markdown>
                    </div>
                  </div>
                </motion.div>
              )}
            </AnimatePresence>
          </motion.div>
        )}
      </AnimatePresence>

      {/* Error message */}
      <AnimatePresence>
        {errorMessage && (
          <motion.div
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: "auto", opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            className="border-t border-red-500/20 bg-red-500/10 px-3 py-2.5 flex-shrink-0"
          >
            <div className="flex items-start gap-2">
              <AlertCircle
                size={13}
                className="text-red-400 flex-shrink-0 mt-0.5"
                aria-hidden="true"
              />
              <p className="text-xs text-red-300 leading-relaxed">{errorMessage}</p>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

// ── AgentEventRow ─────────────────────────────────────────────────────────────

interface AgentEventRowProps {
  event: AgentEvent;
  onApprove?: (id: string) => void;
  onReject?: (id: string) => void;
}

function AgentEventRow({ event, onApprove, onReject }: AgentEventRowProps) {
  if (event.type === "tool-call" || event.type === "tool-result") {
    if (!event.tool) return null;
    return (
      <motion.div
        initial={{ opacity: 0, y: 6 }}
        animate={{ opacity: 1, y: 0 }}
        exit={{ opacity: 0 }}
        transition={{ duration: 0.16 }}
      >
        <AgentToolApprovalCard
          tool={event.tool}
          state={event.toolApprovalState ?? "pending"}
          onApprove={onApprove ?? (() => {})}
          onReject={onReject ?? (() => {})}
        />
      </motion.div>
    );
  }

  if (event.type === "thinking") {
    return (
      <motion.div
        initial={{ opacity: 0, x: -4 }}
        animate={{ opacity: 1, x: 0 }}
        exit={{ opacity: 0 }}
        transition={{ duration: 0.14 }}
        className="flex items-center gap-2 text-[11px] text-slate-500 py-0.5"
      >
        <Loader2 size={11} className="animate-spin text-accent-500/70 flex-shrink-0" aria-hidden="true" />
        <span>{event.message}</span>
      </motion.div>
    );
  }

  if (event.type === "complete") {
    return (
      <motion.div
        initial={{ opacity: 0, x: -4 }}
        animate={{ opacity: 1, x: 0 }}
        exit={{ opacity: 0 }}
        transition={{ duration: 0.14 }}
        className="flex items-center gap-2 text-[11px] text-emerald-500/80 py-0.5"
      >
        <CheckCircle2 size={11} className="flex-shrink-0" aria-hidden="true" />
        <span>Agent completed</span>
      </motion.div>
    );
  }

  if (event.type === "error") {
    return (
      <motion.div
        initial={{ opacity: 0, x: -4 }}
        animate={{ opacity: 1, x: 0 }}
        exit={{ opacity: 0 }}
        transition={{ duration: 0.14 }}
        className="flex items-start gap-2 text-[11px] text-red-400 py-0.5"
      >
        <AlertCircle size={11} className="flex-shrink-0 mt-0.5" aria-hidden="true" />
        <span>{event.message}</span>
      </motion.div>
    );
  }

  return null;
}
