import { useState, useCallback } from "react";
import { motion, AnimatePresence } from "framer-motion";
import {
  Cat,
  Plug,
  Sparkles,
  ChevronRight,
  ChevronLeft,
  Check,
  Server,
  Bot,
  Clock,
  Shield,
  GitCompare,
  MessageSquare,
  RefreshCw,
  ExternalLink,
  X,
} from "lucide-react";

// ─── Storage helpers ──────────────────────────────────────────────────────────

const STORAGE_KEY = "kubecat-onboarding-v1";

interface OnboardingState {
  completed: boolean;
  completedAt?: string;
  skipped?: boolean;
}

export function getOnboardingState(): OnboardingState {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) return JSON.parse(raw);
  } catch {
    // ignore parse errors
  }
  return { completed: false };
}

export function markOnboardingComplete(skipped = false) {
  const state: OnboardingState = {
    completed: true,
    completedAt: new Date().toISOString(),
    skipped,
  };
  localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
}

export function resetOnboarding() {
  localStorage.removeItem(STORAGE_KEY);
}

// ─── Types ────────────────────────────────────────────────────────────────────

interface OnboardingWizardProps {
  /** Available kubeconfig context names — fetched by the parent before mount */
  contexts: string[];
  /** Called when the user clicks a context card to connect */
  onConnect: (contextName: string) => Promise<void>;
  /** Called when the wizard is fully finished (or skipped) */
  onFinish: () => void;
  /** Whether a connection is actively being established */
  connecting?: boolean;
  /** The currently active context after a successful connect */
  activeContext?: string;
  /** Whether we are currently connected */
  isConnected?: boolean;
  /** Called to refresh the context list from kubeconfig */
  onRefreshContexts?: () => Promise<void>;
}

// ─── Tour step definition ─────────────────────────────────────────────────────

const TOUR_STEPS = [
  {
    icon: MessageSquare,
    title: "AI Query",
    description:
      "Ask natural-language questions about your cluster. \"Why is my pod crashlooping?\" gets you an AI-powered root-cause analysis in seconds.",
    accentClass: "text-cyan-400",
    glowClass: "shadow-cyan-500/20",
  },
  {
    icon: Shield,
    title: "Security Scan",
    description:
      "Run a one-click RBAC and misconfiguration audit across every namespace. Get a letter grade and actionable recommendations.",
    accentClass: "text-amber-400",
    glowClass: "shadow-amber-500/20",
  },
  {
    icon: Clock,
    title: "Time Travel",
    description:
      "Browse your cluster's history with snapshots. Compare pod states from 10 minutes ago to now without touching kubectl.",
    accentClass: "text-purple-400",
    glowClass: "shadow-purple-500/20",
  },
  {
    icon: GitCompare,
    title: "Cluster Diff",
    description:
      "Compare two clusters, two snapshots, or a snapshot against live state. Great for catching config drift in staging vs. production.",
    accentClass: "text-green-400",
    glowClass: "shadow-green-500/20",
  },
];

// ─── Sub-step components ──────────────────────────────────────────────────────

function WelcomeStep({ onNext }: { onNext: () => void }) {
  return (
    <div className="flex flex-col items-center text-center gap-8 py-4">
      {/* Animated logo area */}
      <div className="relative">
        <motion.div
          className="absolute inset-0 rounded-full bg-accent-500/20 blur-2xl"
          animate={{ scale: [1, 1.2, 1] }}
          transition={{ repeat: Infinity, duration: 3, ease: "easeInOut" }}
        />
        <div className="relative w-28 h-28 rounded-full bg-slate-800/80 border border-slate-600/50 flex items-center justify-center shadow-2xl">
          <Cat size={56} className="text-accent-400" />
        </div>
      </div>

      <div className="space-y-3 max-w-sm">
        <h1 className="text-3xl font-bold text-slate-100 tracking-tight">
          Welcome to Kubecat
        </h1>
        <p className="text-slate-400 leading-relaxed">
          Your AI-powered Kubernetes cockpit. Let's get you connected to a
          cluster and run your first AI query in under 2 minutes.
        </p>
      </div>

      <div className="grid grid-cols-2 gap-3 w-full max-w-xs text-sm">
        {[
          { icon: Plug, label: "Connect cluster" },
          { icon: Bot, label: "Configure AI" },
          { icon: Sparkles, label: "Quick tour" },
          { icon: Check, label: "You're ready!" },
        ].map(({ icon: Icon, label }, i) => (
          <div
            key={label}
            className="flex items-center gap-2 px-3 py-2 rounded-lg bg-slate-800/60 border border-slate-700/50 text-slate-400"
          >
            <Icon size={14} className="text-accent-400 flex-shrink-0" />
            <span>{label}</span>
            <span className="ml-auto text-xs text-slate-600 font-mono">
              {i + 1}
            </span>
          </div>
        ))}
      </div>

      <button
        onClick={onNext}
        className="group flex items-center gap-2 px-8 py-3 rounded-xl bg-accent-500 hover:bg-accent-400 text-slate-900 font-semibold text-sm transition-all shadow-lg shadow-accent-500/25 hover:shadow-accent-500/40 hover:scale-[1.02] active:scale-[0.99]"
      >
        Let's get started
        <ChevronRight
          size={16}
          className="group-hover:translate-x-0.5 transition-transform"
        />
      </button>
    </div>
  );
}

function ClusterStep({
  contexts,
  onConnect,
  onRefresh,
  connecting,
  activeContext,
  isConnected,
  onNext,
}: {
  contexts: string[];
  onConnect: (ctx: string) => Promise<void>;
  onRefresh?: () => Promise<void>;
  connecting: boolean;
  activeContext: string;
  isConnected: boolean;
  onNext: () => void;
}) {
  const [refreshing, setRefreshing] = useState(false);
  const [connectingCtx, setConnectingCtx] = useState<string | null>(null);

  const handleRefresh = async () => {
    if (!onRefresh) return;
    setRefreshing(true);
    try {
      await onRefresh();
    } finally {
      setRefreshing(false);
    }
  };

  const handleConnect = async (ctx: string) => {
    setConnectingCtx(ctx);
    try {
      await onConnect(ctx);
    } finally {
      setConnectingCtx(null);
    }
  };

  return (
    <div className="flex flex-col gap-6">
      <div className="space-y-1">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded-lg bg-accent-500/20 flex items-center justify-center">
            <Server size={16} className="text-accent-400" />
          </div>
          <h2 className="text-xl font-semibold text-slate-100">
            Connect to a cluster
          </h2>
        </div>
        <p className="text-sm text-slate-400 ml-11">
          Kubecat auto-detected these contexts from your kubeconfig.
        </p>
      </div>

      {/* Context cards */}
      <div className="space-y-2">
        {contexts.length === 0 ? (
          <div className="flex flex-col items-center gap-3 py-8 text-center">
            <Server size={32} className="text-slate-600" />
            <p className="text-slate-400 text-sm">
              No kubeconfig contexts found.
            </p>
            <p className="text-slate-500 text-xs max-w-xs">
              Make sure you have a valid{" "}
              <code className="font-mono bg-slate-800 px-1 rounded">
                ~/.kube/config
              </code>{" "}
              or set the{" "}
              <code className="font-mono bg-slate-800 px-1 rounded">
                KUBECONFIG
              </code>{" "}
              env var.
            </p>
            {onRefresh && (
              <button
                onClick={handleRefresh}
                disabled={refreshing}
                className="flex items-center gap-2 text-sm text-accent-400 hover:text-accent-300 transition-colors disabled:opacity-50"
              >
                <RefreshCw
                  size={14}
                  className={refreshing ? "animate-spin" : ""}
                />
                Refresh
              </button>
            )}
          </div>
        ) : (
          <div className="max-h-52 overflow-y-auto space-y-2 pr-1">
            {contexts.map((ctx) => {
              const isActive = ctx === activeContext && isConnected;
              const isConnectingThis = connectingCtx === ctx || (connecting && connectingCtx === ctx);
              return (
                <motion.button
                  key={ctx}
                  onClick={() => !isActive && handleConnect(ctx)}
                  disabled={connecting || isActive}
                  whileHover={!isActive ? { scale: 1.01 } : undefined}
                  whileTap={!isActive ? { scale: 0.99 } : undefined}
                  className={`
                    w-full flex items-center gap-3 px-4 py-3 rounded-xl border text-left transition-all
                    ${
                      isActive
                        ? "border-emerald-500/50 bg-emerald-500/10 text-emerald-400 cursor-default"
                        : "border-slate-700/50 bg-slate-800/40 hover:border-accent-500/50 hover:bg-accent-500/5 text-slate-300 hover:text-slate-100"
                    }
                    disabled:opacity-60
                  `}
                >
                  <div
                    className={`w-2 h-2 rounded-full flex-shrink-0 ${
                      isActive ? "bg-emerald-400" : "bg-slate-600"
                    }`}
                  />
                  <span className="flex-1 font-mono text-sm truncate">
                    {ctx}
                  </span>
                  {isConnectingThis ? (
                    <div className="w-4 h-4 border-2 border-accent-400 border-t-transparent rounded-full animate-spin" />
                  ) : isActive ? (
                    <Check size={14} className="text-emerald-400" />
                  ) : (
                    <span className="text-xs text-slate-500">Connect</span>
                  )}
                </motion.button>
              );
            })}
          </div>
        )}

        {/* Refresh link */}
        {contexts.length > 0 && onRefresh && (
          <button
            onClick={handleRefresh}
            disabled={refreshing}
            className="flex items-center gap-1.5 text-xs text-slate-500 hover:text-slate-300 transition-colors disabled:opacity-50 mt-1"
          >
            <RefreshCw
              size={11}
              className={refreshing ? "animate-spin" : ""}
            />
            Refresh from kubeconfig
          </button>
        )}
      </div>

      {/* Connected success banner */}
      <AnimatePresence>
        {isConnected && activeContext && (
          <motion.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: "auto" }}
            exit={{ opacity: 0, height: 0 }}
            className="flex items-center gap-3 px-4 py-3 rounded-xl bg-emerald-500/10 border border-emerald-500/30"
          >
            <Check size={16} className="text-emerald-400 flex-shrink-0" />
            <div>
              <p className="text-sm font-medium text-emerald-300">
                Connected to{" "}
                <span className="font-mono">{activeContext}</span>
              </p>
              <p className="text-xs text-emerald-600">
                Ready to explore your cluster
              </p>
            </div>
          </motion.div>
        )}
      </AnimatePresence>

      <div className="flex justify-end">
        <button
          onClick={onNext}
          disabled={!isConnected}
          className="group flex items-center gap-2 px-6 py-2.5 rounded-xl bg-accent-500 hover:bg-accent-400 text-slate-900 font-semibold text-sm transition-all disabled:opacity-40 disabled:cursor-not-allowed shadow-lg shadow-accent-500/25 hover:shadow-accent-500/40 hover:scale-[1.02] active:scale-[0.99]"
        >
          Continue
          <ChevronRight
            size={16}
            className="group-hover:translate-x-0.5 transition-transform"
          />
        </button>
      </div>
    </div>
  );
}

function AISetupStep({ onNext }: { onNext: () => void }) {
  return (
    <div className="flex flex-col gap-6">
      <div className="space-y-1">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded-lg bg-purple-500/20 flex items-center justify-center">
            <Bot size={16} className="text-purple-400" />
          </div>
          <h2 className="text-xl font-semibold text-slate-100">
            Configure AI
          </h2>
        </div>
        <p className="text-sm text-slate-400 ml-11">
          Kubecat supports multiple AI providers. We recommend Ollama for
          privacy — everything stays local.
        </p>
      </div>

      {/* Provider cards */}
      <div className="space-y-3">
        {/* Ollama — recommended */}
        <div className="relative flex gap-4 p-4 rounded-xl bg-slate-800/60 border border-accent-500/40 ring-1 ring-accent-500/20">
          <div className="absolute -top-2 right-3">
            <span className="px-2 py-0.5 rounded-full bg-accent-500/20 border border-accent-500/30 text-xs font-medium text-accent-400">
              Recommended
            </span>
          </div>
          <div className="w-10 h-10 rounded-lg bg-slate-900/80 border border-slate-700/50 flex items-center justify-center flex-shrink-0 mt-0.5">
            <Bot size={20} className="text-accent-400" />
          </div>
          <div className="space-y-1 flex-1 min-w-0">
            <p className="font-medium text-slate-100 text-sm">Ollama (Local)</p>
            <p className="text-xs text-slate-400 leading-relaxed">
              Run models like Llama 3.2 or Mistral locally. 100% private — your
              cluster data never leaves your machine.
            </p>
            <div className="flex items-center gap-3 pt-1">
              <span className="text-xs text-slate-500">
                Default endpoint:{" "}
                <code className="font-mono bg-slate-900/80 px-1 rounded text-slate-400">
                  http://localhost:11434
                </code>
              </span>
            </div>
            <a
              href="https://ollama.ai"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-1.5 text-xs text-accent-400 hover:text-accent-300 transition-colors pt-0.5"
            >
              <ExternalLink size={11} />
              Install Ollama
            </a>
          </div>
        </div>

        {/* Cloud providers */}
        <div className="grid grid-cols-3 gap-2">
          {[
            { name: "OpenAI", color: "text-green-400" },
            { name: "Anthropic", color: "text-orange-400" },
            { name: "Gemini", color: "text-blue-400" },
          ].map(({ name, color }) => (
            <div
              key={name}
              className="flex flex-col items-center gap-2 p-3 rounded-xl bg-slate-800/40 border border-slate-700/50"
            >
              <Sparkles size={16} className={color} />
              <span className="text-xs text-slate-400">{name}</span>
            </div>
          ))}
        </div>

        <p className="text-xs text-slate-500 text-center">
          Cloud providers can be configured in{" "}
          <span className="text-slate-400 font-medium">Settings</span> after
          setup.
        </p>
      </div>

      <div className="flex justify-end">
        <button
          onClick={onNext}
          className="group flex items-center gap-2 px-6 py-2.5 rounded-xl bg-accent-500 hover:bg-accent-400 text-slate-900 font-semibold text-sm transition-all shadow-lg shadow-accent-500/25 hover:shadow-accent-500/40 hover:scale-[1.02] active:scale-[0.99]"
        >
          Continue
          <ChevronRight
            size={16}
            className="group-hover:translate-x-0.5 transition-transform"
          />
        </button>
      </div>
    </div>
  );
}

function TourStep({ onFinish }: { onFinish: () => void }) {
  const [active, setActive] = useState(0);
  const step = TOUR_STEPS[active];
  const Icon = step.icon;

  return (
    <div className="flex flex-col gap-6">
      <div className="space-y-1">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded-lg bg-slate-700/60 flex items-center justify-center">
            <Sparkles size={16} className="text-slate-300" />
          </div>
          <h2 className="text-xl font-semibold text-slate-100">Quick tour</h2>
        </div>
        <p className="text-sm text-slate-400 ml-11">
          Here's what Kubecat does best. You can always come back via the{" "}
          <kbd className="font-mono text-xs bg-slate-800 px-1.5 py-0.5 rounded border border-slate-700">
            ?
          </kbd>{" "}
          shortcut.
        </p>
      </div>

      {/* Feature card */}
      <AnimatePresence mode="wait">
        <motion.div
          key={active}
          initial={{ opacity: 0, x: 20 }}
          animate={{ opacity: 1, x: 0 }}
          exit={{ opacity: 0, x: -20 }}
          transition={{ duration: 0.2 }}
          className={`p-5 rounded-xl bg-slate-800/60 border border-slate-700/50 shadow-xl ${step.glowClass}`}
        >
          <div className="flex gap-4">
            <div className="w-12 h-12 rounded-xl bg-slate-900/80 border border-slate-700/50 flex items-center justify-center flex-shrink-0">
              <Icon size={22} className={step.accentClass} />
            </div>
            <div className="space-y-1.5">
              <p className={`font-semibold text-base ${step.accentClass}`}>
                {step.title}
              </p>
              <p className="text-sm text-slate-400 leading-relaxed">
                {step.description}
              </p>
            </div>
          </div>
        </motion.div>
      </AnimatePresence>

      {/* Step dots */}
      <div className="flex items-center gap-2 justify-center">
        {TOUR_STEPS.map((_, i) => (
          <button
            key={i}
            onClick={() => setActive(i)}
            className={`transition-all rounded-full ${
              i === active
                ? "w-6 h-2 bg-accent-400"
                : "w-2 h-2 bg-slate-600 hover:bg-slate-500"
            }`}
            aria-label={`Go to step ${i + 1}`}
          />
        ))}
      </div>

      {/* Navigation */}
      <div className="flex items-center justify-between">
        <button
          onClick={() => setActive((a) => Math.max(a - 1, 0))}
          disabled={active === 0}
          className="flex items-center gap-1.5 px-4 py-2 rounded-lg text-sm text-slate-400 hover:text-slate-200 hover:bg-slate-800/60 transition-colors disabled:opacity-0"
        >
          <ChevronLeft size={14} />
          Back
        </button>

        {active < TOUR_STEPS.length - 1 ? (
          <button
            onClick={() => setActive((a) => a + 1)}
            className="group flex items-center gap-2 px-6 py-2.5 rounded-xl bg-slate-700/60 hover:bg-slate-700 text-slate-200 font-medium text-sm transition-all border border-slate-600/50"
          >
            Next
            <ChevronRight
              size={14}
              className="group-hover:translate-x-0.5 transition-transform"
            />
          </button>
        ) : (
          <button
            onClick={onFinish}
            className="group flex items-center gap-2 px-6 py-2.5 rounded-xl bg-accent-500 hover:bg-accent-400 text-slate-900 font-semibold text-sm transition-all shadow-lg shadow-accent-500/25 hover:shadow-accent-500/40 hover:scale-[1.02] active:scale-[0.99]"
          >
            <Check size={14} />
            Let's go!
          </button>
        )}
      </div>
    </div>
  );
}

// ─── Step indicator ───────────────────────────────────────────────────────────

const STEP_LABELS = ["Welcome", "Cluster", "AI Setup", "Tour"];

function StepIndicator({ current }: { current: number }) {
  return (
    <div className="flex items-center gap-2">
      {STEP_LABELS.map((label, i) => {
        const done = i < current;
        const active = i === current;
        return (
          <div key={label} className="flex items-center gap-2">
            <div className="flex items-center gap-1.5">
              <div
                className={`w-6 h-6 rounded-full flex items-center justify-center text-xs font-bold transition-all ${
                  done
                    ? "bg-emerald-500/20 text-emerald-400 border border-emerald-500/40"
                    : active
                    ? "bg-accent-500/20 text-accent-400 border border-accent-500/40"
                    : "bg-slate-800/60 text-slate-600 border border-slate-700/50"
                }`}
              >
                {done ? <Check size={12} /> : i + 1}
              </div>
              <span
                className={`text-xs font-medium transition-colors hidden sm:block ${
                  active
                    ? "text-slate-200"
                    : done
                    ? "text-emerald-500"
                    : "text-slate-600"
                }`}
              >
                {label}
              </span>
            </div>
            {i < STEP_LABELS.length - 1 && (
              <div
                className={`h-px w-6 transition-colors ${
                  done ? "bg-emerald-500/40" : "bg-slate-700/60"
                }`}
              />
            )}
          </div>
        );
      })}
    </div>
  );
}

// ─── Main wizard ──────────────────────────────────────────────────────────────

export function OnboardingWizard({
  contexts,
  onConnect,
  onFinish,
  connecting = false,
  activeContext = "",
  isConnected = false,
  onRefreshContexts,
}: OnboardingWizardProps) {
  const [step, setStep] = useState(0);

  const next = useCallback(() => setStep((s) => s + 1), []);

  const handleFinish = useCallback(() => {
    markOnboardingComplete();
    onFinish();
  }, [onFinish]);

  const handleSkip = useCallback(() => {
    markOnboardingComplete(true);
    onFinish();
  }, [onFinish]);

  return (
    // Full-screen overlay
    <div
      className="fixed inset-0 z-50 flex items-center justify-center"
      role="dialog"
      aria-modal="true"
      aria-label="Kubecat onboarding wizard"
    >
      {/* Backdrop */}
      <motion.div
        className="absolute inset-0 bg-slate-950/80 backdrop-blur-md"
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
      />

      {/* Iridescent glow blobs in background */}
      <div className="absolute inset-0 overflow-hidden pointer-events-none opacity-10">
        <div className="iri-blob" style={{ top: "-10vh", left: "-10vw" }} />
        <div className="iri-blob-2" />
      </div>

      {/* Card */}
      <motion.div
        className="relative z-10 w-full max-w-lg mx-4 bg-slate-900/90 border border-slate-700/60 rounded-2xl shadow-2xl backdrop-blur-xl overflow-hidden"
        initial={{ opacity: 0, scale: 0.95, y: 20 }}
        animate={{ opacity: 1, scale: 1, y: 0 }}
        transition={{ type: "spring", duration: 0.5, bounce: 0.2 }}
      >
        {/* Top accent line */}
        <div className="h-px bg-gradient-to-r from-transparent via-accent-500/60 to-transparent" />

        {/* Header */}
        <div className="flex items-center justify-between px-6 pt-5 pb-4 border-b border-slate-800/80">
          <StepIndicator current={step} />
          {step > 0 && (
            <button
              onClick={handleSkip}
              className="flex items-center gap-1.5 text-xs text-slate-500 hover:text-slate-300 transition-colors"
              title="Skip onboarding"
            >
              <X size={12} />
              Skip
            </button>
          )}
        </div>

        {/* Step content */}
        <div className="px-6 py-6">
          <AnimatePresence mode="wait">
            <motion.div
              key={step}
              initial={{ opacity: 0, x: 30 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: -30 }}
              transition={{ duration: 0.25 }}
            >
              {step === 0 && <WelcomeStep onNext={next} />}
              {step === 1 && (
                <ClusterStep
                  contexts={contexts}
                  onConnect={onConnect}
                  onRefresh={onRefreshContexts}
                  connecting={connecting}
                  activeContext={activeContext}
                  isConnected={isConnected}
                  onNext={next}
                />
              )}
              {step === 2 && <AISetupStep onNext={next} />}
              {step === 3 && <TourStep onFinish={handleFinish} />}
            </motion.div>
          </AnimatePresence>
        </div>

        {/* Bottom accent line */}
        <div className="h-px bg-gradient-to-r from-transparent via-slate-700/40 to-transparent" />
      </motion.div>
    </div>
  );
}
