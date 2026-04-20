import { useEffect, useMemo, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import {
  X,
  Plug,
  MessageSquare,
  Shield,
  Clock,
  Sparkles,
  ChevronRight,
  KeyRound,
} from "lucide-react";
import { useAIStore } from "../../stores/aiStore";
import { useOnboardingStore } from "../../stores/onboardingStore";

// ---------------------------------------------------------------------------
// Legacy dismiss migration
// ---------------------------------------------------------------------------
// The old card stored its dismissed flag under this key in localStorage. Read
// it once on module load and migrate into the onboarding store, then delete —
// that way users who dismissed before this refactor stay dismissed.
const LEGACY_DISMISS_KEY = "kubecat-getting-started-dismissed";
try {
  if (
    typeof localStorage !== "undefined" &&
    localStorage.getItem(LEGACY_DISMISS_KEY) === "true"
  ) {
    useOnboardingStore.getState().dismiss();
    localStorage.removeItem(LEGACY_DISMISS_KEY);
  }
} catch {
  // localStorage unavailable (SSR, privacy mode) — fine, nothing to migrate.
}

// ---------------------------------------------------------------------------
// Public hook (kept for backwards compat with any outside caller)
// ---------------------------------------------------------------------------
export function useGettingStartedDismissed() {
  const dismissed = useOnboardingStore((s) => s.dismissed);
  const dismiss = useOnboardingStore((s) => s.dismiss);
  return { dismissed, dismiss };
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------
interface GettingStartedCardProps {
  isConnected: boolean;
  onOpenOnboarding?: () => void;
  onNavigate?: (view: string) => void;
}

type Flags = {
  connected: boolean;
  aiConfigured: boolean;
  aiQuerySent: boolean;
  snapshotTaken: boolean;
  securityScanRun: boolean;
};

type StepView = "query" | "security" | "timeline" | null;

interface Step {
  id: keyof Flags;
  icon: typeof Plug;
  label: string;
  description: string;
  /** Where to send the user when they click. `null` → open the wizard. */
  view: StepView;
  accentClass: string;
  bgClass: string;
  borderClass: string;
}

// ---------------------------------------------------------------------------
// Step definitions — done predicates live outside, keyed by step id, so the
// card stays declarative and predicates can evolve without re-arranging JSX.
// ---------------------------------------------------------------------------
const STEPS: Step[] = [
  {
    id: "connected",
    icon: Plug,
    label: "Connect a cluster",
    description: "Select a kubeconfig context from the header",
    view: null,
    accentClass: "text-accent-400",
    bgClass: "bg-accent-500/10",
    borderClass: "border-accent-500/20",
  },
  {
    id: "aiConfigured",
    icon: KeyRound,
    label: "Configure an AI model",
    description: "Pick a provider and model in Settings",
    view: null,
    accentClass: "text-fuchsia-400",
    bgClass: "bg-fuchsia-500/10",
    borderClass: "border-fuchsia-500/20",
  },
  {
    id: "aiQuerySent",
    icon: MessageSquare,
    label: "Run your first AI query",
    description: "Ask anything about your cluster in plain English",
    view: "query",
    accentClass: "text-purple-400",
    bgClass: "bg-purple-500/10",
    borderClass: "border-purple-500/20",
  },
  {
    id: "snapshotTaken",
    icon: Clock,
    label: "Take your first snapshot",
    description: "Capture cluster state for time-travel diffs",
    view: "timeline",
    accentClass: "text-cyan-400",
    bgClass: "bg-cyan-500/10",
    borderClass: "border-cyan-500/20",
  },
  {
    id: "securityScanRun",
    icon: Shield,
    label: "Run a security scan",
    description: "Audit RBAC and misconfigurations with one click",
    view: "security",
    accentClass: "text-amber-400",
    bgClass: "bg-amber-500/10",
    borderClass: "border-amber-500/20",
  },
];

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------
export function GettingStartedCard({
  isConnected,
  onOpenOnboarding,
  onNavigate,
}: GettingStartedCardProps) {
  const dismissed = useOnboardingStore((s) => s.dismissed);
  const dismiss = useOnboardingStore((s) => s.dismiss);

  const everConnected = useOnboardingStore((s) => s.everConnected);
  const aiQuerySentFlag = useOnboardingStore((s) => s.aiQuerySent);
  const snapshotTaken = useOnboardingStore((s) => s.snapshotTaken);
  const securityScanRun = useOnboardingStore((s) => s.securityScanRun);

  const markEverConnected = useOnboardingStore((s) => s.markEverConnected);
  const markAIQuerySent = useOnboardingStore((s) => s.markAIQuerySent);

  const selectedModel = useAIStore((s) => s.selectedModel);
  const conversations = useAIStore((s) => s.conversations);

  // Latch "ever connected" the first time we observe a live connection.
  useEffect(() => {
    if (isConnected) markEverConnected();
  }, [isConnected, markEverConnected]);

  // Derive "AI query sent" from existing conversation state, then latch it so
  // the flag survives a conversation wipe. This avoids needing to edit
  // AIQueryView.tsx (owned by another engineer). If any future query pipeline
  // bypasses `conversations`, set `markAIQuerySent()` at that site directly.
  const derivedAIQuerySent = useMemo(
    () =>
      conversations.some((c) =>
        c.messages.some((m) => m.role === "user")
      ),
    [conversations]
  );
  useEffect(() => {
    if (derivedAIQuerySent) markAIQuerySent();
  }, [derivedAIQuerySent, markAIQuerySent]);

  const flags: Flags = {
    connected: isConnected || everConnected,
    aiConfigured: !!selectedModel,
    aiQuerySent: aiQuerySentFlag || derivedAIQuerySent,
    snapshotTaken,
    securityScanRun,
  };

  const [hovered, setHovered] = useState<number | null>(null);

  const completedCount = STEPS.reduce(
    (n, step) => (flags[step.id] ? n + 1 : n),
    0
  );
  const progressPct = (completedCount / STEPS.length) * 100;
  const allDone = completedCount === STEPS.length;

  // Auto-hide once everything is done. Separate from explicit dismiss so the
  // card can re-appear if a future step is added and its flag is still false.
  const hidden = dismissed || allDone;

  return (
    <AnimatePresence>
      {!hidden && (
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
                  {completedCount}/{STEPS.length}
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
              {STEPS.map((step, i) => {
                const Icon = step.icon;
                const isDone = flags[step.id];
                const isHovered = hovered === i;
                const clickable =
                  !isDone && (step.view !== null || onOpenOnboarding);

                return (
                  <motion.button
                    key={step.id}
                    onClick={() => {
                      if (isDone) return;
                      if (step.view && onNavigate) {
                        onNavigate(step.view);
                      } else if (step.view === null && onOpenOnboarding) {
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
                          : `${step.bgClass} ${step.borderClass} hover:border-opacity-60`
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
                      className={isDone ? "text-emerald-400" : step.accentClass}
                    />

                    <div className="space-y-0.5">
                      <p
                        className={`text-xs font-medium leading-snug ${
                          isDone ? "text-slate-400 line-through" : "text-slate-200"
                        }`}
                      >
                        {step.label}
                      </p>
                      <p className="text-[11px] text-slate-500 leading-snug">
                        {step.description}
                      </p>
                    </div>

                    {!isDone && clickable && isHovered && (
                      <ChevronRight
                        size={12}
                        className={`${step.accentClass} self-end`}
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
