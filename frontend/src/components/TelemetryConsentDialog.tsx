/**
 * TelemetryConsentDialog — Post-onboarding telemetry consent gate.
 *
 * - Two-column layout: "What we collect" vs "We never collect"
 * - Backdrop click = "No thanks" (deny consent, close dialog)
 * - Keyboard: Escape = "No thanks"
 * - Full a11y: focus trap, aria-modal, role=dialog
 */

import { useEffect, useRef } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { BarChart2, ShieldOff, X } from "lucide-react";

// ── Types ────────────────────────────────────────────────────────────────────

interface TelemetryConsentDialogProps {
  isOpen: boolean;
  onAccept: () => void;
  onDecline: () => void;
}

// ── Data ─────────────────────────────────────────────────────────────────────

const WE_COLLECT = [
  "Which views are used most often (counts only)",
  "How long sessions last (rounded to nearest minute)",
  "Feature adoption (e.g. AI queries used: yes/no)",
  "Crash reports with stack trace (no user data)",
  "App version and OS type",
];

const WE_NEVER_COLLECT = [
  "Cluster names, endpoint URLs, or kubeconfig data",
  "Resource names, namespaces, or YAML content",
  "Query text or AI conversation content",
  "Any personally identifying information",
  "IP addresses (aggregated at ingestion, never stored)",
];

// ── Component ─────────────────────────────────────────────────────────────────

export function TelemetryConsentDialog({
  isOpen,
  onAccept,
  onDecline,
}: TelemetryConsentDialogProps) {
  const dialogRef = useRef<HTMLDivElement>(null);
  const acceptBtnRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!isOpen) return;
    const previouslyFocused = document.activeElement as HTMLElement;
    acceptBtnRef.current?.focus();

    const trap = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        onDecline();
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
  }, [isOpen, onDecline]);

  return (
    <AnimatePresence>
      {isOpen && (
        <>
          {/* Backdrop — click = decline */}
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm"
            onClick={onDecline}
            aria-hidden="true"
          />

          {/* Dialog */}
          <motion.div
            initial={{ opacity: 0, scale: 0.96, y: 20 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.96, y: 20 }}
            transition={{ duration: 0.22, ease: "easeOut" }}
            className="fixed inset-0 z-50 flex items-center justify-center p-4 pointer-events-none"
          >
            <div
              ref={dialogRef}
              className="
                relative w-full max-w-lg
                bg-white/95 dark:bg-slate-900/95
                backdrop-blur-xl
                border border-stone-200/80 dark:border-slate-700/50
                rounded-2xl shadow-2xl shadow-black/20
                pointer-events-auto
                overflow-hidden
              "
              role="dialog"
              aria-modal="true"
              aria-labelledby="telemetry-title"
              aria-describedby="telemetry-desc"
              onClick={(e) => e.stopPropagation()}
            >
              {/* Accent top stripe */}
              <div className="h-0.5 bg-gradient-to-r from-transparent via-accent-500/50 to-transparent" />

              {/* Header */}
              <div className="flex items-start gap-3 p-5 pb-4">
                <div className="flex-shrink-0 w-9 h-9 rounded-xl bg-accent-500/10 border border-accent-500/20 flex items-center justify-center">
                  <BarChart2
                    size={17}
                    className="text-accent-500 dark:text-accent-400"
                    aria-hidden="true"
                  />
                </div>
                <div className="flex-1 min-w-0">
                  <h2
                    id="telemetry-title"
                    className="text-base font-semibold text-stone-900 dark:text-slate-100"
                  >
                    Help improve Kubecat
                  </h2>
                  <p
                    id="telemetry-desc"
                    className="text-xs text-stone-500 dark:text-slate-400 mt-0.5"
                  >
                    Optional, anonymous usage data helps us prioritize features.
                    You can change this anytime in Settings.
                  </p>
                </div>
                <button
                  onClick={onDecline}
                  className="
                    flex-shrink-0 p-1.5 rounded-lg
                    text-stone-400 hover:text-stone-700
                    dark:text-slate-500 dark:hover:text-slate-300
                    hover:bg-stone-100 dark:hover:bg-slate-700/60
                    transition-colors
                    focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500/50
                  "
                  aria-label="No thanks, close"
                >
                  <X size={16} aria-hidden="true" />
                </button>
              </div>

              {/* Two-column disclosure */}
              <div className="px-5 pb-4 grid grid-cols-2 gap-3">
                <DisclosurePanel
                  title="What we collect"
                  accent="accent"
                  items={WE_COLLECT}
                />
                <DisclosurePanel
                  title="We never collect"
                  accent="emerald"
                  items={WE_NEVER_COLLECT}
                />
              </div>

              {/* Actions */}
              <div className="px-5 pb-5 flex items-center justify-end gap-3">
                <button
                  onClick={onDecline}
                  className="
                    px-4 py-2 rounded-xl
                    text-xs font-medium
                    text-stone-500 hover:text-stone-700
                    dark:text-slate-400 dark:hover:text-slate-200
                    hover:bg-stone-100 dark:hover:bg-slate-700/60
                    transition-colors duration-150
                    focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500/50
                  "
                >
                  No thanks
                </button>
                <button
                  ref={acceptBtnRef}
                  onClick={onAccept}
                  className="
                    px-5 py-2 rounded-xl
                    text-xs font-semibold
                    bg-accent-500 hover:bg-accent-600
                    text-white
                    transition-colors duration-150
                    focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500/50
                  "
                >
                  Allow anonymous analytics
                </button>
              </div>
            </div>
          </motion.div>
        </>
      )}
    </AnimatePresence>
  );
}

// ── DisclosurePanel ───────────────────────────────────────────────────────────

function DisclosurePanel({
  title,
  accent,
  items,
}: {
  title: string;
  accent: "accent" | "emerald";
  items: string[];
}) {
  const isAccent = accent === "accent";
  return (
    <div
      className={`
        rounded-xl border p-3
        ${isAccent
          ? "bg-accent-500/5 dark:bg-accent-400/5 border-accent-500/15 dark:border-accent-400/15"
          : "bg-emerald-500/5 border-emerald-500/15 dark:border-emerald-400/15"}
      `}
    >
      <div className="flex items-center gap-1.5 mb-2">
        {isAccent ? (
          <BarChart2
            size={11}
            className="text-accent-500 dark:text-accent-400"
            aria-hidden="true"
          />
        ) : (
          <ShieldOff size={11} className="text-emerald-500" aria-hidden="true" />
        )}
        <p
          className={`text-[10px] font-semibold uppercase tracking-widest ${isAccent ? "text-accent-500 dark:text-accent-400" : "text-emerald-600 dark:text-emerald-400"}`}
        >
          {title}
        </p>
      </div>
      <ul className="space-y-1.5" role="list">
        {items.map((item) => (
          <li key={item} className="flex items-start gap-1.5">
            <span
              className={`mt-1.5 w-1 h-1 rounded-full flex-shrink-0 ${isAccent ? "bg-accent-400/70" : "bg-emerald-400/70"}`}
              aria-hidden="true"
            />
            <span className="text-[11px] text-stone-600 dark:text-slate-400 leading-relaxed">
              {item}
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}
