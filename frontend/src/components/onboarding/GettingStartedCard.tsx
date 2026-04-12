import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import {
  X,
  Plug,
  MessageSquare,
  Shield,
  Clock,
  GitCompare,
  ChevronRight,
  Sparkles,
} from "lucide-react";

const GETTING_STARTED_KEY = "kubecat-getting-started-dismissed";

export function useGettingStartedDismissed() {
  const [dismissed, setDismissed] = useState(() =>
    localStorage.getItem(GETTING_STARTED_KEY) === "true"
  );

  const dismiss = () => {
    localStorage.setItem(GETTING_STARTED_KEY, "true");
    setDismissed(true);
  };

  return { dismissed, dismiss };
}

interface GettingStartedCardProps {
  isConnected: boolean;
  onOpenOnboarding?: () => void;
  onNavigate?: (view: string) => void;
}

const QUICK_ACTIONS = [
  {
    icon: Plug,
    label: "Connect a cluster",
    description: "Select a kubeconfig context from the header",
    done: (_isConnected: boolean) => _isConnected,
    view: null,
    accentClass: "text-accent-400",
    bgClass: "bg-accent-500/10",
    borderClass: "border-accent-500/20",
  },
  {
    icon: MessageSquare,
    label: "Run your first AI query",
    description: "Ask anything about your cluster in plain English",
    done: () => false,
    view: "query",
    accentClass: "text-purple-400",
    bgClass: "bg-purple-500/10",
    borderClass: "border-purple-500/20",
  },
  {
    icon: Shield,
    label: "Security scan",
    description: "Audit RBAC and misconfigurations with one click",
    done: () => false,
    view: "security",
    accentClass: "text-amber-400",
    bgClass: "bg-amber-500/10",
    borderClass: "border-amber-500/20",
  },
  {
    icon: Clock,
    label: "Explore the timeline",
    description: "Time-travel through cluster history with snapshots",
    done: () => false,
    view: "timeline",
    accentClass: "text-cyan-400",
    bgClass: "bg-cyan-500/10",
    borderClass: "border-cyan-500/20",
  },
  {
    icon: GitCompare,
    label: "Compare clusters",
    description: "Diff two clusters or snapshots side by side",
    done: () => false,
    view: "diff",
    accentClass: "text-green-400",
    bgClass: "bg-green-500/10",
    borderClass: "border-green-500/20",
  },
];

export function GettingStartedCard({
  isConnected,
  onOpenOnboarding,
  onNavigate,
}: GettingStartedCardProps) {
  const { dismissed, dismiss } = useGettingStartedDismissed();
  const [hovered, setHovered] = useState<number | null>(null);

  const completedCount = QUICK_ACTIONS.filter((a) => a.done(isConnected)).length;
  const progressPct = (completedCount / QUICK_ACTIONS.length) * 100;

  return (
    <AnimatePresence>
      {!dismissed && (
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, scale: 0.98 }}
          transition={{ duration: 0.25 }}
          className="col-span-full relative rounded-2xl bg-slate-800/40 border border-slate-700/50 backdrop-blur-sm overflow-hidden"
        >
          {/* Top accent glow */}
          <div className="absolute top-0 inset-x-0 h-px bg-gradient-to-r from-transparent via-accent-500/50 to-transparent" />

          <div className="p-6">
            {/* Header row */}
            <div className="flex items-start justify-between gap-4 mb-5">
              <div className="space-y-1">
                <div className="flex items-center gap-2">
                  <Sparkles size={16} className="text-accent-400" />
                  <h3 className="text-base font-semibold text-slate-100">
                    Getting Started
                  </h3>
                </div>
                <p className="text-xs text-slate-400">
                  Complete these steps to get the most out of Kubecat.
                </p>
              </div>
              <div className="flex items-center gap-3 flex-shrink-0">
                {onOpenOnboarding && (
                  <button
                    onClick={onOpenOnboarding}
                    className="text-xs text-accent-400 hover:text-accent-300 transition-colors"
                  >
                    Reopen wizard
                  </button>
                )}
                <button
                  onClick={dismiss}
                  className="p-1.5 rounded-lg text-slate-500 hover:text-slate-300 hover:bg-slate-700/50 transition-colors"
                  aria-label="Dismiss getting started card"
                >
                  <X size={14} />
                </button>
              </div>
            </div>

            {/* Progress bar */}
            <div className="mb-5 space-y-1.5">
              <div className="flex items-center justify-between text-xs">
                <span className="text-slate-500">Progress</span>
                <span className="text-slate-400 font-medium">
                  {completedCount}/{QUICK_ACTIONS.length}
                </span>
              </div>
              <div className="h-1.5 rounded-full bg-slate-700/60 overflow-hidden">
                <motion.div
                  className="h-full rounded-full bg-gradient-to-r from-accent-600 to-accent-400"
                  initial={{ width: 0 }}
                  animate={{ width: `${progressPct}%` }}
                  transition={{ duration: 0.5, ease: "easeOut" }}
                />
              </div>
            </div>

            {/* Action list */}
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5 gap-2">
              {QUICK_ACTIONS.map((action, i) => {
                const Icon = action.icon;
                const isDone = action.done(isConnected);
                const isHovered = hovered === i;
                const clickable = !isDone && (action.view || onOpenOnboarding);

                return (
                  <motion.button
                    key={action.label}
                    onClick={() => {
                      if (isDone) return;
                      if (action.view && onNavigate) {
                        onNavigate(action.view);
                      } else if (!action.view && onOpenOnboarding) {
                        onOpenOnboarding();
                      }
                    }}
                    onMouseEnter={() => setHovered(i)}
                    onMouseLeave={() => setHovered(null)}
                    disabled={!clickable}
                    whileHover={clickable ? { scale: 1.02 } : undefined}
                    whileTap={clickable ? { scale: 0.99 } : undefined}
                    className={`
                      relative flex flex-col gap-2 p-3 rounded-xl border text-left transition-all
                      ${
                        isDone
                          ? "border-emerald-500/30 bg-emerald-500/5 opacity-70"
                          : `${action.bgClass} ${action.borderClass} hover:border-opacity-60`
                      }
                      ${clickable ? "cursor-pointer" : "cursor-default"}
                    `}
                  >
                    {/* Done checkmark */}
                    {isDone && (
                      <div className="absolute top-2 right-2 w-4 h-4 rounded-full bg-emerald-500/20 border border-emerald-500/40 flex items-center justify-center">
                        <svg
                          viewBox="0 0 12 12"
                          className="w-2.5 h-2.5 text-emerald-400"
                          fill="none"
                          stroke="currentColor"
                          strokeWidth={2}
                        >
                          <polyline points="2,6 5,9 10,3" />
                        </svg>
                      </div>
                    )}

                    <Icon
                      size={16}
                      className={isDone ? "text-emerald-400" : action.accentClass}
                    />

                    <div className="space-y-0.5">
                      <p
                        className={`text-xs font-medium leading-snug ${
                          isDone ? "text-slate-400 line-through" : "text-slate-200"
                        }`}
                      >
                        {action.label}
                      </p>
                      <p className="text-[11px] text-slate-500 leading-snug">
                        {action.description}
                      </p>
                    </div>

                    {!isDone && clickable && isHovered && (
                      <ChevronRight
                        size={12}
                        className={`${action.accentClass} self-end`}
                      />
                    )}
                  </motion.button>
                );
              })}
            </div>
          </div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
