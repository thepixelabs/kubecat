/**
 * EpicsPopup — Agent task board viewer
 *
 * Shows the .tasks/ epics directory: plan status, phases, and execution logs.
 *
 * Scroll contract for the execution log:
 *   - Auto-scroll to the latest entry ONLY when the user is already near
 *     the bottom (within SCROLL_THRESHOLD px).
 *   - When the user manually scrolls up, auto-scroll is suspended.
 *   - Auto-scroll resumes automatically once the user scrolls back down
 *     to within SCROLL_THRESHOLD px of the bottom.
 *   - A "Jump to bottom" button appears whenever auto-scroll is suspended.
 */

import { useState, useEffect, useRef, useCallback } from "react";
import { motion, AnimatePresence } from "framer-motion";
import {
  X,
  ChevronDown,
  CheckCircle2,
  AlertCircle,
  Loader2,
  RefreshCw,
  ArrowDown,
  GitBranch,
  Circle,
} from "lucide-react";
import { ExecuteCommand } from "../../wailsjs/go/main/App";

// ── constants ────────────────────────────────────────────────────────────────

/** Distance from the bottom (px) within which auto-scroll stays active */
const SCROLL_THRESHOLD = 80;

/** Polling interval (ms) for live-refreshing the selected epic's log */
const POLL_INTERVAL_MS = 3000;

// ── types ────────────────────────────────────────────────────────────────────

interface Phase {
  id: number;
  title: string;
  persona: string;
  status: "TODO" | "IN_PROGRESS" | "DONE" | "BLOCKED";
}

interface Epic {
  name: string;
  status: string;
  phases: Phase[];
  rawPlan: string;
}

// ── helpers ──────────────────────────────────────────────────────────────────

function parsePhaseStatus(raw: string): Phase["status"] {
  const s = raw.trim().toUpperCase();
  if (s === "DONE") return "DONE";
  if (s === "IN_PROGRESS") return "IN_PROGRESS";
  if (s === "BLOCKED") return "BLOCKED";
  return "TODO";
}

/** Parses minimal YAML frontmatter from plan.md content */
function parsePlanFrontmatter(content: string): Omit<Epic, "name" | "rawPlan"> {
  const fmMatch = content.match(/^---\n([\s\S]*?)\n---/);
  if (!fmMatch) return { status: "UNKNOWN", phases: [] };

  const fm = fmMatch[1];

  // status: field
  const statusMatch = fm.match(/^status:\s*(\S+)/m);
  const epicStatus = statusMatch ? statusMatch[1] : "UNKNOWN";

  // phases: block — parse each phase entry
  const phases: Phase[] = [];
  const phaseBlocks = fm.split(/\n {2}- id:/g).slice(1);
  for (const block of phaseBlocks) {
    const idMatch = block.match(/^(\d+)/);
    const titleMatch = block.match(/title:\s*"([^"]+)"/);
    const personaMatch = block.match(/persona:\s*(\S+)/);
    const statusMatch2 = block.match(/status:\s*(\S+)/);
    if (idMatch) {
      phases.push({
        id: parseInt(idMatch[1], 10),
        title: titleMatch ? titleMatch[1] : `Phase ${idMatch[1]}`,
        persona: personaMatch ? personaMatch[1] : "",
        status: statusMatch2 ? parsePhaseStatus(statusMatch2[1]) : "TODO",
      });
    }
  }

  return { status: epicStatus, phases };
}

async function fetchEpicList(): Promise<string[]> {
  try {
    const out = await ExecuteCommand(
      "ls -1 .tasks/ 2>/dev/null | grep -v '^\\.' | sort"
    );
    return out
      .split("\n")
      .map((l) => l.trim())
      .filter(Boolean);
  } catch {
    return [];
  }
}

async function fetchPlanContent(epicName: string): Promise<string> {
  try {
    return await ExecuteCommand(`cat .tasks/${epicName}/plan.md 2>/dev/null`);
  } catch {
    return "";
  }
}

async function fetchExecutionLog(epicName: string): Promise<string> {
  try {
    return await ExecuteCommand(
      `cat .tasks/${epicName}/execution-log.md 2>/dev/null`
    );
  } catch {
    return "";
  }
}

// ── sub-components ───────────────────────────────────────────────────────────

function PhaseStatusIcon({ status }: { status: Phase["status"] }) {
  switch (status) {
    case "DONE":
      return <CheckCircle2 size={13} className="text-emerald-500 dark:text-emerald-400 flex-shrink-0" />;
    case "IN_PROGRESS":
      return <Loader2 size={13} className="text-amber-500 dark:text-amber-400 animate-spin flex-shrink-0" />;
    case "BLOCKED":
      return <AlertCircle size={13} className="text-red-500 dark:text-red-400 flex-shrink-0" />;
    default:
      return <Circle size={13} className="text-stone-400 dark:text-slate-600 flex-shrink-0" />;
  }
}

function EpicStatusBadge({ status }: { status: string }) {
  const s = status.toUpperCase();
  let cls = "bg-stone-100 dark:bg-slate-700/60 text-stone-500 dark:text-slate-400 border-stone-200/60 dark:border-slate-600/40";
  if (s === "DONE")
    cls = "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20";
  else if (s === "IN_PROGRESS" || s === "TODO")
    cls = "bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20";
  else if (s === "BLOCKED")
    cls = "bg-red-500/10 text-red-600 dark:text-red-400 border-red-500/20";

  return (
    <span
      className={`inline-flex items-center px-1.5 py-0.5 rounded text-[9px] font-semibold font-mono border ${cls}`}
    >
      {s}
    </span>
  );
}

// ── ExecutionLogPanel ─────────────────────────────────────────────────────────

interface ExecutionLogPanelProps {
  epicName: string | null;
}

function ExecutionLogPanel({ epicName }: ExecutionLogPanelProps) {
  const [logContent, setLogContent] = useState<string>("");
  const [loading, setLoading] = useState(false);

  // Scroll state refs — we use refs (not state) to avoid re-renders on every
  // scroll event, which would itself trigger the scroll-to-bottom effect.
  const containerRef = useRef<HTMLDivElement>(null);
  const bottomRef = useRef<HTMLDivElement>(null);
  const isNearBottomRef = useRef(true);
  const isProgrammaticScrollRef = useRef(false);
  const [showJumpButton, setShowJumpButton] = useState(false);

  // ── log fetching ──────────────────────────────────────────────────────────

  const loadLog = useCallback(async () => {
    if (!epicName) return;
    const content = await fetchExecutionLog(epicName);
    setLogContent(content);
    setLoading(false);
  }, [epicName]);

  useEffect(() => {
    if (!epicName) {
      setLogContent("");
      return;
    }
    setLoading(true);
    loadLog();

    // Poll for live updates
    const timer = setInterval(loadLog, POLL_INTERVAL_MS);
    return () => clearInterval(timer);
  }, [epicName, loadLog]);

  // ── smart scroll ──────────────────────────────────────────────────────────

  /**
   * When new log content arrives, scroll to bottom ONLY if the user is
   * already near the bottom (i.e. hasn't scrolled up to read).
   * We flag programmatic scrolls so the onScroll handler ignores them —
   * otherwise the smooth-scroll animation fires scroll events that
   * reset isNearBottomRef to true, overriding the user's intent.
   */
  useEffect(() => {
    if (isNearBottomRef.current && bottomRef.current) {
      isProgrammaticScrollRef.current = true;
      bottomRef.current.scrollIntoView({ behavior: "smooth" });
      // Clear the flag after the smooth-scroll animation settles
      setTimeout(() => {
        isProgrammaticScrollRef.current = false;
      }, 400);
    }
  }, [logContent]);

  /**
   * Track whether the user is near the bottom after every scroll event.
   * Ignore events generated by programmatic scrollIntoView — those would
   * falsely reset isNearBottomRef to true during the animation.
   */
  const handleScroll = useCallback(() => {
    if (isProgrammaticScrollRef.current) return;
    const el = containerRef.current;
    if (!el) return;
    const distanceFromBottom =
      el.scrollHeight - el.scrollTop - el.clientHeight;
    const nearBottom = distanceFromBottom < SCROLL_THRESHOLD;
    isNearBottomRef.current = nearBottom;
    // Only update React state (which causes a re-render) when the button
    // visibility needs to change — not on every scroll tick.
    setShowJumpButton(!nearBottom);
  }, []);

  const jumpToBottom = useCallback(() => {
    isNearBottomRef.current = true;
    setShowJumpButton(false);
    isProgrammaticScrollRef.current = true;
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
    setTimeout(() => {
      isProgrammaticScrollRef.current = false;
    }, 400);
  }, []);

  // ── render ────────────────────────────────────────────────────────────────

  if (!epicName) {
    return (
      <div className="flex-1 flex items-center justify-center text-stone-400 dark:text-slate-500 text-sm">
        Select an epic to view its execution log
      </div>
    );
  }

  return (
    <div className="flex-1 flex flex-col min-h-0 relative">
      <div className="px-4 py-2.5 border-b border-stone-200/60 dark:border-slate-700/40 flex items-center justify-between flex-shrink-0">
        <span className="text-xs font-semibold text-stone-600 dark:text-slate-400 uppercase tracking-wide">
          Execution Log
        </span>
        {loading && (
          <Loader2 size={12} className="text-stone-400 dark:text-slate-500 animate-spin" />
        )}
      </div>

      {/* Scrollable log body */}
      <div
        ref={containerRef}
        onScroll={handleScroll}
        className="flex-1 overflow-y-auto p-4 font-mono text-xs text-stone-700 dark:text-slate-300 whitespace-pre-wrap break-words scrollbar-thin scrollbar-thumb-stone-300 dark:scrollbar-thumb-slate-600 scrollbar-track-transparent"
      >
        {logContent ? (
          logContent
        ) : (
          <span className="text-stone-400 dark:text-slate-600 italic">
            No entries yet.
          </span>
        )}
        {/* Invisible sentinel at the very bottom — scrolled into view on update */}
        <div ref={bottomRef} className="h-px" />
      </div>

      {/* Jump-to-bottom button — appears when user has scrolled away */}
      <AnimatePresence>
        {showJumpButton && (
          <motion.button
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 8 }}
            transition={{ duration: 0.15 }}
            onClick={jumpToBottom}
            className="
              absolute bottom-4 right-4
              flex items-center gap-1.5
              px-3 py-1.5 rounded-full
              bg-accent-500 hover:bg-accent-600
              text-white text-xs font-medium
              shadow-lg shadow-accent-500/30
              transition-colors
            "
          >
            <ArrowDown size={12} />
            Jump to bottom
          </motion.button>
        )}
      </AnimatePresence>
    </div>
  );
}

// ── EpicsPopup (main export) ──────────────────────────────────────────────────

interface EpicsPopupProps {
  isOpen: boolean;
  onClose: () => void;
}

export function EpicsPopup({ isOpen, onClose }: EpicsPopupProps) {
  const [epics, setEpics] = useState<Epic[]>([]);
  const [selectedEpic, setSelectedEpic] = useState<string | null>(null);
  const [selectedEpicData, setSelectedEpicData] = useState<Epic | null>(null);
  const [loadingList, setLoadingList] = useState(false);
  const [loadingDetail, setLoadingDetail] = useState(false);

  // ── load epic list on open ────────────────────────────────────────────────

  const loadEpics = useCallback(async () => {
    setLoadingList(true);
    try {
      const names = await fetchEpicList();
      const resolved: Epic[] = await Promise.all(
        names.map(async (name) => {
          const raw = await fetchPlanContent(name);
          const parsed = parsePlanFrontmatter(raw);
          return { name, rawPlan: raw, ...parsed };
        })
      );
      // Sort: in-progress first, then todo, then done
      resolved.sort((a, b) => {
        const order: Record<string, number> = {
          IN_PROGRESS: 0,
          TODO: 1,
          DONE: 2,
        };
        return (order[a.status] ?? 3) - (order[b.status] ?? 3);
      });
      setEpics(resolved);
      if (!selectedEpic && resolved.length > 0) {
        setSelectedEpic(resolved[0].name);
      }
    } catch {
      setEpics([]);
    } finally {
      setLoadingList(false);
    }
  }, [selectedEpic]);

  useEffect(() => {
    if (isOpen) {
      loadEpics();
    }
  }, [isOpen]);

  // ── load plan detail when selection changes ───────────────────────────────

  useEffect(() => {
    if (!selectedEpic) {
      setSelectedEpicData(null);
      return;
    }
    const found = epics.find((e) => e.name === selectedEpic);
    if (found) {
      setSelectedEpicData(found);
      return;
    }
    // Not yet in list — fetch directly
    setLoadingDetail(true);
    fetchPlanContent(selectedEpic).then((raw) => {
      const parsed = parsePlanFrontmatter(raw);
      setSelectedEpicData({ name: selectedEpic, rawPlan: raw, ...parsed });
      setLoadingDetail(false);
    });
  }, [selectedEpic, epics]);

  // ── keyboard: close on Escape ─────────────────────────────────────────────

  useEffect(() => {
    if (!isOpen) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [isOpen, onClose]);

  // ── render ────────────────────────────────────────────────────────────────

  return (
    <AnimatePresence>
      {isOpen && (
        <>
          {/* Backdrop */}
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.15 }}
            className="fixed inset-0 z-50 bg-black/40 dark:bg-black/60 backdrop-blur-sm"
            onClick={onClose}
            aria-hidden
          />

          {/* Panel */}
          <motion.div
            initial={{ opacity: 0, scale: 0.97, y: 12 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.97, y: 12 }}
            transition={{ duration: 0.18, ease: "easeOut" }}
            className="
              fixed inset-4 sm:inset-8 z-50
              flex flex-col
              bg-white/90 dark:bg-slate-900/90
              backdrop-blur-xl
              border border-stone-200 dark:border-slate-700/60
              rounded-2xl shadow-2xl shadow-black/20 dark:shadow-black/60
              overflow-hidden
            "
            role="dialog"
            aria-modal
            aria-label="Agent epics board"
            onClick={(e) => e.stopPropagation()}
          >
            {/* ── Header ─────────────────────────────────────────────────── */}
            <div className="flex items-center justify-between px-5 py-3.5 border-b border-stone-200/80 dark:border-slate-700/40 bg-stone-50/60 dark:bg-slate-800/40 flex-shrink-0">
              <div className="flex items-center gap-2.5">
                <GitBranch size={16} className="text-accent-500 dark:text-accent-400" />
                <h2 className="text-sm font-semibold text-stone-800 dark:text-slate-100">
                  Agent Epics
                </h2>
                {epics.length > 0 && (
                  <span className="px-1.5 py-0.5 rounded bg-stone-200/80 dark:bg-slate-700/60 text-[10px] font-mono text-stone-500 dark:text-slate-400">
                    {epics.length}
                  </span>
                )}
              </div>
              <div className="flex items-center gap-2">
                <button
                  onClick={loadEpics}
                  disabled={loadingList}
                  className="p-1.5 rounded-lg text-stone-400 hover:text-stone-700 dark:text-slate-500 dark:hover:text-slate-300 hover:bg-stone-200/60 dark:hover:bg-slate-700/60 transition-colors disabled:opacity-40"
                  title="Refresh"
                  aria-label="Refresh epics"
                >
                  <RefreshCw size={14} className={loadingList ? "animate-spin" : ""} />
                </button>
                <button
                  onClick={onClose}
                  className="p-1.5 rounded-lg text-stone-400 hover:text-stone-700 dark:text-slate-500 dark:hover:text-slate-300 hover:bg-stone-200/60 dark:hover:bg-slate-700/60 transition-colors"
                  aria-label="Close"
                >
                  <X size={16} />
                </button>
              </div>
            </div>

            {/* ── Body ───────────────────────────────────────────────────── */}
            <div className="flex flex-1 min-h-0">

              {/* Left: epic list */}
              <div className="w-56 sm:w-64 flex-shrink-0 border-r border-stone-200/60 dark:border-slate-700/40 overflow-y-auto">
                {loadingList && epics.length === 0 ? (
                  <div className="flex items-center justify-center p-8 text-stone-400 dark:text-slate-500">
                    <Loader2 size={18} className="animate-spin" />
                  </div>
                ) : epics.length === 0 ? (
                  <div className="p-6 text-center text-xs text-stone-400 dark:text-slate-500">
                    No epics found in .tasks/
                  </div>
                ) : (
                  <nav className="p-2 space-y-0.5" aria-label="Epics">
                    {epics.map((epic) => {
                      const isActive = epic.name === selectedEpic;
                      return (
                        <button
                          key={epic.name}
                          onClick={() => setSelectedEpic(epic.name)}
                          className={`
                            w-full text-left px-3 py-2 rounded-lg transition-colors
                            ${isActive
                              ? "bg-accent-500/10 dark:bg-accent-400/10 border border-accent-500/20 dark:border-accent-400/20"
                              : "hover:bg-stone-100/80 dark:hover:bg-slate-800/60 border border-transparent"
                            }
                          `}
                          aria-current={isActive ? "page" : undefined}
                        >
                          <div className="flex items-start justify-between gap-2">
                            <span
                              className={`text-xs font-medium font-mono truncate ${
                                isActive
                                  ? "text-accent-600 dark:text-accent-400"
                                  : "text-stone-700 dark:text-slate-300"
                              }`}
                            >
                              {epic.name}
                            </span>
                            <EpicStatusBadge status={epic.status} />
                          </div>
                          {epic.phases.length > 0 && (
                            <div className="mt-1.5 flex gap-1 flex-wrap">
                              {epic.phases.map((ph) => (
                                <PhaseStatusIcon key={ph.id} status={ph.status} />
                              ))}
                            </div>
                          )}
                        </button>
                      );
                    })}
                  </nav>
                )}
              </div>

              {/* Right: phases + execution log */}
              <div className="flex-1 flex flex-col min-w-0 min-h-0">
                {loadingDetail ? (
                  <div className="flex-1 flex items-center justify-center text-stone-400 dark:text-slate-500">
                    <Loader2 size={20} className="animate-spin" />
                  </div>
                ) : selectedEpicData ? (
                  <>
                    {/* Phases summary strip */}
                    <div className="px-4 py-3 border-b border-stone-200/60 dark:border-slate-700/40 flex-shrink-0">
                      <div className="flex items-center gap-1.5 mb-2">
                        <span className="text-xs font-semibold text-stone-700 dark:text-slate-300">
                          {selectedEpicData.name}
                        </span>
                        <EpicStatusBadge status={selectedEpicData.status} />
                      </div>
                      {selectedEpicData.phases.length > 0 && (
                        <div className="space-y-1">
                          {selectedEpicData.phases.map((ph) => (
                            <div key={ph.id} className="flex items-center gap-2">
                              <PhaseStatusIcon status={ph.status} />
                              <span className="text-xs text-stone-600 dark:text-slate-400 truncate">
                                {ph.title}
                              </span>
                              <span className="text-[10px] font-mono text-stone-400 dark:text-slate-600 ml-auto flex-shrink-0">
                                @{ph.persona}
                              </span>
                            </div>
                          ))}
                        </div>
                      )}
                    </div>

                    {/* Execution log — smart scroll lives here */}
                    <ExecutionLogPanel epicName={selectedEpic} />
                  </>
                ) : (
                  <div className="flex-1 flex items-center justify-center text-stone-400 dark:text-slate-500 text-sm">
                    <div className="text-center">
                      <ChevronDown
                        size={24}
                        className="mx-auto mb-2 rotate-[-90deg] opacity-30"
                      />
                      <p>Select an epic from the list</p>
                    </div>
                  </div>
                )}
              </div>
            </div>
          </motion.div>
        </>
      )}
    </AnimatePresence>
  );
}
