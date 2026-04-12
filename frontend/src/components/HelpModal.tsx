/**
 * HelpModal — Keyboard shortcut reference sheet.
 *
 * - role="dialog" with aria-modal, aria-labelledby
 * - Focus trap + Escape to close
 * - Backdrop click closes
 * - Groups: Navigation, Explorer, App, AI Query
 */

import { useEffect, useRef } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { X, Keyboard } from "lucide-react";

// ── Types ────────────────────────────────────────────────────────────────────

interface ShortcutGroup {
  title: string;
  shortcuts: { keys: string[]; description: string }[];
}

interface HelpModalProps {
  isOpen: boolean;
  onClose: () => void;
  /** Current active view — used to highlight relevant group */
  activeView?: string;
}

// ── Shortcut catalog ──────────────────────────────────────────────────────────

const SHORTCUT_GROUPS: ShortcutGroup[] = [
  {
    title: "Navigation",
    shortcuts: [
      { keys: ["1"], description: "Dashboard" },
      { keys: ["2"], description: "Explorer" },
      { keys: ["3"], description: "Timeline" },
      { keys: ["4"], description: "Visualizer" },
      { keys: ["5"], description: "Analyzer" },
      { keys: ["6"], description: "GitOps" },
      { keys: ["7"], description: "Security" },
      { keys: ["8"], description: "Port Forwards" },
      { keys: ["9"], description: "Cluster Diff" },
      { keys: ["0"], description: "AI Query" },
      { keys: ["["], description: "Toggle sidebar" },
    ],
  },
  {
    title: "Global",
    shortcuts: [
      { keys: [","], description: "Open Settings" },
      { keys: ["?"], description: "Show this help" },
      { keys: ["c"], description: "Open cluster switcher" },
      { keys: ["Esc"], description: "Close modal / blur input" },
      { keys: ["⌘", "="], description: "Zoom in" },
      { keys: ["⌘", "-"], description: "Zoom out" },
      { keys: ["⌘", "0"], description: "Reset zoom" },
    ],
  },
  {
    title: "Explorer",
    shortcuts: [
      { keys: ["/"], description: "Focus search" },
      { keys: ["↑", "↓"], description: "Navigate resources" },
      { keys: ["Enter"], description: "Inspect selected resource" },
      { keys: ["r"], description: "Toggle CPU/Memory columns" },
      { keys: ["i"], description: "Toggle container images column" },
      { keys: ["o"], description: "Toggle owner column" },
      { keys: ["p"], description: "Toggle probes column" },
      { keys: ["v"], description: "Toggle volumes column" },
      { keys: ["s"], description: "Toggle security issues column" },
      { keys: ["!"], description: "Show only resources with issues" },
    ],
  },
  {
    title: "Context Switcher",
    shortcuts: [
      { keys: ["↑", "↓"], description: "Navigate contexts" },
      { keys: ["Enter"], description: "Connect to selected context" },
      { keys: ["Esc"], description: "Close context menu" },
    ],
  },
];

// ── Component ─────────────────────────────────────────────────────────────────

export function HelpModal({ isOpen, onClose, activeView }: HelpModalProps) {
  const dialogRef = useRef<HTMLDivElement>(null);
  const closeBtnRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!isOpen) return;
    const previouslyFocused = document.activeElement as HTMLElement;
    closeBtnRef.current?.focus();

    const trap = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        onClose();
        return;
      }
      if (e.key !== "Tab" || !dialogRef.current) return;
      const focusable = dialogRef.current.querySelectorAll<HTMLElement>(
        'button, [href], input, [tabindex]:not([tabindex="-1"])'
      );
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (e.shiftKey) {
        if (document.activeElement === first) {
          e.preventDefault();
          last.focus();
        }
      } else {
        if (document.activeElement === last) {
          e.preventDefault();
          first.focus();
        }
      }
    };

    document.addEventListener("keydown", trap);
    return () => {
      document.removeEventListener("keydown", trap);
      previouslyFocused?.focus();
    };
  }, [isOpen, onClose]);

  // Determine which group to highlight based on active view
  const highlightedGroup =
    activeView === "explorer"
      ? "Explorer"
      : undefined;

  return (
    <AnimatePresence>
      {isOpen && (
        <>
          {/* Backdrop */}
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm"
            onClick={onClose}
            aria-hidden="true"
          />

          {/* Dialog */}
          <motion.div
            initial={{ opacity: 0, scale: 0.96, y: 16 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.96, y: 16 }}
            transition={{ duration: 0.2, ease: "easeOut" }}
            className="fixed inset-0 z-50 flex items-center justify-center p-4 pointer-events-none"
          >
            <div
              ref={dialogRef}
              className="
                relative w-full max-w-2xl max-h-[85vh]
                bg-white/95 dark:bg-slate-900/95
                backdrop-blur-xl
                border border-stone-200/80 dark:border-slate-700/50
                rounded-2xl shadow-2xl shadow-black/20
                pointer-events-auto
                overflow-hidden flex flex-col
              "
              role="dialog"
              aria-modal="true"
              aria-labelledby="help-title"
              onClick={(e) => e.stopPropagation()}
            >
              {/* Accent stripe */}
              <div className="h-0.5 bg-gradient-to-r from-transparent via-accent-500/50 to-transparent flex-shrink-0" />

              {/* Header */}
              <div className="flex items-center gap-3 px-5 py-4 border-b border-stone-100 dark:border-slate-700/40 flex-shrink-0">
                <div className="w-8 h-8 rounded-lg bg-accent-500/10 border border-accent-500/20 flex items-center justify-center">
                  <Keyboard
                    size={15}
                    className="text-accent-500 dark:text-accent-400"
                    aria-hidden="true"
                  />
                </div>
                <h2
                  id="help-title"
                  className="flex-1 text-base font-semibold text-stone-900 dark:text-slate-100"
                >
                  Keyboard Shortcuts
                </h2>
                <button
                  ref={closeBtnRef}
                  onClick={onClose}
                  className="
                    p-1.5 rounded-lg
                    text-stone-400 hover:text-stone-700
                    dark:text-slate-500 dark:hover:text-slate-300
                    hover:bg-stone-100 dark:hover:bg-slate-700/60
                    transition-colors
                    focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500/50
                  "
                  aria-label="Close keyboard shortcuts"
                >
                  <X size={16} aria-hidden="true" />
                </button>
              </div>

              {/* Shortcut groups */}
              <div className="overflow-y-auto flex-1 p-5">
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                  {SHORTCUT_GROUPS.map((group) => {
                    const isHighlighted = group.title === highlightedGroup;
                    return (
                      <div
                        key={group.title}
                        className={`
                          rounded-xl border p-4 transition-colors
                          ${isHighlighted
                            ? "border-accent-500/30 dark:border-accent-400/20 bg-accent-500/5 dark:bg-accent-400/5"
                            : "border-stone-100 dark:border-slate-700/40 bg-stone-50/50 dark:bg-slate-800/30"
                          }
                        `}
                      >
                        <p
                          className={`
                            text-[10px] font-semibold uppercase tracking-widest mb-3
                            ${isHighlighted
                              ? "text-accent-500 dark:text-accent-400"
                              : "text-stone-400 dark:text-slate-500"
                            }
                          `}
                        >
                          {group.title}
                        </p>
                        <ul className="space-y-2" role="list">
                          {group.shortcuts.map((shortcut) => (
                            <li
                              key={shortcut.description}
                              className="flex items-center justify-between gap-3"
                            >
                              <span className="text-xs text-stone-600 dark:text-slate-400">
                                {shortcut.description}
                              </span>
                              <div className="flex items-center gap-1 flex-shrink-0">
                                {shortcut.keys.map((key, i) => (
                                  <kbd
                                    key={i}
                                    className="
                                      inline-flex items-center justify-center
                                      min-w-[22px] px-1.5 py-0.5
                                      text-[10px] font-mono font-medium
                                      bg-white dark:bg-slate-700
                                      border border-stone-200 dark:border-slate-600
                                      rounded shadow-sm
                                      text-stone-700 dark:text-slate-300
                                    "
                                  >
                                    {key}
                                  </kbd>
                                ))}
                              </div>
                            </li>
                          ))}
                        </ul>
                      </div>
                    );
                  })}
                </div>
              </div>

              {/* Footer hint */}
              <div className="flex-shrink-0 px-5 py-3 border-t border-stone-100 dark:border-slate-700/40">
                <p className="text-[11px] text-stone-400 dark:text-slate-600 text-center">
                  Press{" "}
                  <kbd className="inline px-1 py-0.5 text-[10px] bg-stone-100 dark:bg-slate-700 border border-stone-200 dark:border-slate-600 rounded font-mono">
                    ?
                  </kbd>{" "}
                  anywhere to show this help
                </p>
              </div>
            </div>
          </motion.div>
        </>
      )}
    </AnimatePresence>
  );
}
