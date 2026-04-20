/**
 * Dashboard — Kubecat operational command center
 *
 * Design language: Pixelabs "cockpit" aesthetic
 * - Dark glass panels with backdrop blur
 * - Accent glow on status indicators (emerald=healthy, amber=warning, red=critical)
 * - Responsive grid that collapses gracefully at narrow widths
 * - All data via existing Wails Go bindings — no new backend endpoints
 */

import { useEffect, useState, useCallback } from "react";
import { motion, AnimatePresence } from "framer-motion";
import {
  Activity,
  AlertTriangle,
  Bot,
  Camera,
  CheckCircle2,
  ChevronRight,
  CircleDot,
  GitBranch,
  Heart,
  Plug,
  RefreshCw,
  Server,
  Shield,
  ShieldCheck,
  Wifi,
  WifiOff,
  XCircle,
  Zap,
  type LucideIcon,
} from "lucide-react";

// Wails bindings
import {
  GetMultiClusterHealth,
  GetTimelineEvents,
  ListResources,
  GetGitOpsStatus,
  GetSecuritySummary,
  TakeSnapshot,
} from "../../wailsjs/go/main/App";
import { GettingStartedCard } from "./onboarding/GettingStartedCard";
import { useOnboardingStore } from "../stores/onboardingStore";

// ---------------------------------------------------------------------------
// Types (mirroring Go structs)
// ---------------------------------------------------------------------------

interface ClusterHealthInfo {
  context: string;
  status: string;
  nodeCount: number;
  podCount: number;
  cpuPercent: number;
  memPercent: number;
  issues: number;
  lastChecked: string;
}

interface TimelineEvent {
  id: number;
  cluster: string;
  namespace: string;
  kind: string;
  name: string;
  type: string;
  reason: string;
  message: string;
  firstSeen: string;
  lastSeen: string;
  count: number;
  sourceComponent: string;
}

interface ResourceInfo {
  kind: string;
  name: string;
  namespace: string;
  status: string;
  age: string;
  restarts?: number;
}

interface GitOpsApp {
  name: string;
  namespace: string;
  provider: string;
  kind: string;
  syncStatus: string;
  healthStatus: string;
  message?: string;
  lastSyncTime?: string;
}

interface GitOpsStatus {
  provider: string;
  detected: boolean;
  applications: GitOpsApp[];
  summary: {
    total: number;
    synced: number;
    outOfSync: number;
    healthy: number;
    degraded: number;
    progressing: number;
  };
}

interface SecuritySummary {
  score: {
    overall: number;
    grade: string;
    scannedAt: string;
  };
  totalIssues: number;
  criticalCount: number;
  highCount: number;
  mediumCount: number;
  lowCount: number;
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function gradeColor(grade: string): string {
  switch (grade) {
    case "A":
      return "text-emerald-500 dark:text-emerald-400";
    case "B":
      return "text-cyan-500 dark:text-cyan-400";
    case "C":
      return "text-amber-500 dark:text-amber-400";
    case "D":
      return "text-orange-500 dark:text-orange-400";
    default:
      return "text-red-500 dark:text-red-400";
  }
}

function gradeGlow(grade: string): string {
  switch (grade) {
    case "A":
      return "shadow-[0_0_20px_rgba(52,211,153,0.3)]";
    case "B":
      return "shadow-[0_0_20px_rgba(34,211,238,0.25)]";
    case "C":
      return "shadow-[0_0_20px_rgba(251,191,36,0.25)]";
    case "D":
      return "shadow-[0_0_20px_rgba(249,115,22,0.25)]";
    default:
      return "shadow-[0_0_20px_rgba(239,68,68,0.3)]";
  }
}

function statusColor(status: string): string {
  const s = status.toLowerCase();
  if (s === "running" || s === "healthy" || s === "ready" || s === "connected")
    return "text-emerald-500 dark:text-emerald-400";
  if (s === "pending" || s === "progressing" || s === "syncing")
    return "text-amber-500 dark:text-amber-400";
  if (
    s === "failed" ||
    s === "error" ||
    s === "crashloopbackoff" ||
    s === "oomkilled" ||
    s === "degraded" ||
    s === "outofsync"
  )
    return "text-red-500 dark:text-red-400";
  return "text-stone-500 dark:text-slate-400";
}

function statusDot(status: string): string {
  const s = status.toLowerCase();
  if (s === "running" || s === "healthy" || s === "ready" || s === "connected")
    return "bg-emerald-500 shadow-[0_0_6px_rgba(52,211,153,0.8)]";
  if (s === "pending" || s === "progressing" || s === "syncing")
    return "bg-amber-500 shadow-[0_0_6px_rgba(245,158,11,0.7)]";
  if (
    s === "failed" ||
    s === "error" ||
    s === "crashloopbackoff" ||
    s === "oomkilled" ||
    s === "degraded" ||
    s === "outofsync"
  )
    return "bg-red-500 shadow-[0_0_6px_rgba(239,68,68,0.7)]";
  return "bg-stone-400 dark:bg-slate-500";
}

function syncIcon(syncStatus: string) {
  const s = syncStatus.toLowerCase();
  if (s === "synced") return <CheckCircle2 size={13} className="text-emerald-500 dark:text-emerald-400" />;
  if (s === "outofsync") return <XCircle size={13} className="text-red-500 dark:text-red-400" />;
  return <RefreshCw size={13} className="text-amber-500 dark:text-amber-400 animate-spin" />;
}

function healthIcon(healthStatus: string) {
  const s = healthStatus.toLowerCase();
  if (s === "healthy") return <Heart size={13} className="text-emerald-500 dark:text-emerald-400" />;
  if (s === "degraded") return <AlertTriangle size={13} className="text-red-500 dark:text-red-400" />;
  if (s === "progressing") return <Activity size={13} className="text-amber-500 dark:text-amber-400" />;
  return <CircleDot size={13} className="text-stone-400 dark:text-slate-500" />;
}

function relativeTime(isoString: string): string {
  if (!isoString) return "—";
  try {
    const then = new Date(isoString).getTime();
    const delta = Math.floor((Date.now() - then) / 1000);
    if (delta < 60) return `${delta}s ago`;
    if (delta < 3600) return `${Math.floor(delta / 60)}m ago`;
    if (delta < 86400) return `${Math.floor(delta / 3600)}h ago`;
    return `${Math.floor(delta / 86400)}d ago`;
  } catch {
    return isoString;
  }
}

function isUnhealthy(pod: ResourceInfo): boolean {
  const s = (pod.status || "").toLowerCase();
  return (
    s.includes("crashloopbackoff") ||
    s.includes("oomkilled") ||
    s === "pending" ||
    s === "failed" ||
    s === "error" ||
    s === "imagepullbackoff" ||
    s === "errimagepull"
  );
}

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

/** Glass card wrapper with optional top-accent gradient line */
function GlassCard({
  children,
  className = "",
  accent = false,
}: {
  children: React.ReactNode;
  className?: string;
  accent?: boolean;
}) {
  return (
    <div
      className={`
        relative rounded-xl overflow-hidden
        bg-white/50 dark:bg-slate-800/40
        border border-stone-200/80 dark:border-slate-700/40
        backdrop-blur-sm
        shadow-sm dark:shadow-none
        transition-colors duration-200
        ${className}
      `}
    >
      {accent && (
        <div className="absolute top-0 left-0 right-0 h-px bg-gradient-to-r from-transparent via-accent-500/50 to-transparent" />
      )}
      {children}
    </div>
  );
}

/** Section header inside a glass card */
function CardHeader({
  icon: Icon,
  title,
  badge,
  action,
}: {
  icon: LucideIcon;
  title: string;
  badge?: React.ReactNode;
  action?: React.ReactNode;
}) {
  return (
    <div className="flex items-center justify-between px-4 py-3 border-b border-stone-200/60 dark:border-slate-700/40">
      <div className="flex items-center gap-2.5">
        <Icon size={15} className="text-accent-500 dark:text-accent-400 flex-shrink-0" />
        <span className="text-sm font-semibold text-stone-800 dark:text-slate-200">
          {title}
        </span>
        {badge}
      </div>
      {action}
    </div>
  );
}

/** Small status badge pill */
function StatusBadge({ label, variant = "neutral" }: { label: string; variant?: "ok" | "warn" | "crit" | "neutral" }) {
  const colors = {
    ok: "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20",
    warn: "bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20",
    crit: "bg-red-500/10 text-red-600 dark:text-red-400 border-red-500/20",
    neutral: "bg-stone-100 dark:bg-slate-700/60 text-stone-500 dark:text-slate-400 border-stone-200/60 dark:border-slate-600/40",
  };
  return (
    <span className={`inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-semibold font-mono border ${colors[variant]}`}>
      {label}
    </span>
  );
}

// ---------------------------------------------------------------------------
// Panel: Cluster Health
// ---------------------------------------------------------------------------

function ClusterHealthPanel({ clusters, loading }: { clusters: ClusterHealthInfo[]; loading: boolean }) {
  if (loading) {
    return (
      <GlassCard accent>
        <CardHeader icon={Server} title="Cluster Health" />
        <div className="p-4 space-y-3">
          {[1, 2].map((i) => (
            <div key={i} className="h-16 rounded-lg bg-stone-100/60 dark:bg-slate-700/40 animate-pulse" />
          ))}
        </div>
      </GlassCard>
    );
  }

  if (clusters.length === 0) {
    return (
      <GlassCard accent>
        <CardHeader icon={Server} title="Cluster Health" />
        <div className="p-8 text-center">
          <WifiOff size={24} className="mx-auto mb-2 text-stone-300 dark:text-slate-600" />
          <p className="text-sm text-stone-500 dark:text-slate-400">No clusters connected</p>
        </div>
      </GlassCard>
    );
  }

  return (
    <GlassCard accent>
      <CardHeader icon={Server} title="Cluster Health" badge={
        <StatusBadge
          label={`${clusters.length} cluster${clusters.length !== 1 ? "s" : ""}`}
          variant="neutral"
        />
      } />
      <div className="p-3 space-y-2">
        {clusters.map((c) => (
          <div
            key={c.context}
            className="flex items-center gap-3 px-3 py-2.5 rounded-lg bg-stone-50/60 dark:bg-slate-900/30 border border-stone-100 dark:border-slate-700/30"
          >
            {/* Status dot */}
            <span className={`w-2 h-2 rounded-full flex-shrink-0 ${statusDot(c.status)}`} />

            {/* Context name */}
            <span
              className="flex-1 text-xs font-mono font-medium text-stone-700 dark:text-slate-300 truncate"
              title={c.context}
            >
              {c.context}
            </span>

            {/* Stats */}
            <div className="flex items-center gap-3 flex-shrink-0 text-xs text-stone-500 dark:text-slate-400">
              <span title="Nodes" className="flex items-center gap-1">
                <Server size={10} />
                {c.nodeCount}
              </span>
              <span title="Pods" className="flex items-center gap-1">
                <CircleDot size={10} />
                {c.podCount}
              </span>
              {c.issues > 0 ? (
                <StatusBadge label={`${c.issues} issue${c.issues !== 1 ? "s" : ""}`} variant="crit" />
              ) : (
                <StatusBadge label="Healthy" variant="ok" />
              )}
            </div>
          </div>
        ))}
      </div>
    </GlassCard>
  );
}

// ---------------------------------------------------------------------------
// Panel: Recent Warning Events
// ---------------------------------------------------------------------------

function EventsFeedPanel({ events, loading }: { events: TimelineEvent[]; loading: boolean }) {
  const warnings = events.filter((e) => e.type === "Warning").slice(0, 20);

  if (loading) {
    return (
      <GlassCard accent className="row-span-2">
        <CardHeader icon={AlertTriangle} title="Recent Warning Events" />
        <div className="p-4 space-y-2">
          {[1, 2, 3, 4, 5].map((i) => (
            <div key={i} className="h-10 rounded bg-stone-100/60 dark:bg-slate-700/40 animate-pulse" />
          ))}
        </div>
      </GlassCard>
    );
  }

  return (
    <GlassCard accent className="row-span-2">
      <CardHeader
        icon={AlertTriangle}
        title="Recent Warning Events"
        badge={
          warnings.length > 0 ? (
            <StatusBadge label={String(warnings.length)} variant="warn" />
          ) : undefined
        }
      />
      <div className="overflow-y-auto max-h-72">
        {warnings.length === 0 ? (
          <div className="p-8 text-center">
            <CheckCircle2 size={24} className="mx-auto mb-2 text-emerald-400 dark:text-emerald-500" />
            <p className="text-sm text-stone-500 dark:text-slate-400">No recent warning events</p>
          </div>
        ) : (
          <ul role="list" className="divide-y divide-stone-100/60 dark:divide-slate-700/30">
            {warnings.map((ev) => (
              <li key={ev.id} className="px-4 py-2.5 hover:bg-stone-50/60 dark:hover:bg-slate-700/20 transition-colors">
                <div className="flex items-start gap-2.5">
                  <span className="w-1.5 h-1.5 rounded-full bg-amber-500 shadow-[0_0_6px_rgba(245,158,11,0.7)] flex-shrink-0 mt-1.5" />
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-0.5">
                      <span
                        className="text-[10px] font-mono font-semibold text-stone-500 dark:text-slate-400 truncate"
                        title={`${ev.kind}/${ev.name}`}
                      >
                        {ev.kind}/{ev.name}
                      </span>
                      {ev.namespace && (
                        <span
                          className="text-[9px] px-1 py-0.5 rounded bg-stone-100 dark:bg-slate-700/50 text-stone-400 dark:text-slate-500 font-mono flex-shrink-0"
                          title={ev.namespace}
                        >
                          {ev.namespace}
                        </span>
                      )}
                    </div>
                    <p className="text-xs text-stone-600 dark:text-slate-300 leading-tight line-clamp-2">
                      {ev.reason}: {ev.message}
                    </p>
                  </div>
                  <span className="text-[10px] text-stone-400 dark:text-slate-500 flex-shrink-0 tabular-nums">
                    {relativeTime(ev.lastSeen)}
                  </span>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
    </GlassCard>
  );
}

// ---------------------------------------------------------------------------
// Panel: Unhealthy Resources
// ---------------------------------------------------------------------------

function UnhealthyResourcesPanel({ pods, loading }: { pods: ResourceInfo[]; loading: boolean }) {
  const unhealthy = pods.filter(isUnhealthy).slice(0, 15);

  if (loading) {
    return (
      <GlassCard accent>
        <CardHeader icon={XCircle} title="Unhealthy Pods" />
        <div className="p-4 space-y-2">
          {[1, 2, 3].map((i) => (
            <div key={i} className="h-10 rounded bg-stone-100/60 dark:bg-slate-700/40 animate-pulse" />
          ))}
        </div>
      </GlassCard>
    );
  }

  return (
    <GlassCard accent>
      <CardHeader
        icon={XCircle}
        title="Unhealthy Pods"
        badge={
          unhealthy.length > 0 ? (
            <StatusBadge label={String(unhealthy.length)} variant="crit" />
          ) : undefined
        }
      />
      <div className="overflow-y-auto max-h-52">
        {unhealthy.length === 0 ? (
          <div className="p-6 text-center">
            <ShieldCheck size={22} className="mx-auto mb-2 text-emerald-400 dark:text-emerald-500" />
            <p className="text-sm text-stone-500 dark:text-slate-400">All pods healthy</p>
          </div>
        ) : (
          <ul role="list" className="divide-y divide-stone-100/60 dark:divide-slate-700/30">
            {unhealthy.map((pod) => (
              <li key={`${pod.namespace}/${pod.name}`} className="px-4 py-2 hover:bg-stone-50/60 dark:hover:bg-slate-700/20 transition-colors">
                <div className="flex items-center gap-2.5">
                  <span className={`w-1.5 h-1.5 rounded-full flex-shrink-0 ${statusDot(pod.status)}`} />
                  <div className="flex-1 min-w-0">
                    <span
                      className="text-xs font-mono font-medium text-stone-700 dark:text-slate-300 truncate block"
                      title={`${pod.namespace}/${pod.name}`}
                    >
                      {pod.name}
                    </span>
                    <span
                      className="text-[10px] text-stone-400 dark:text-slate-500"
                      title={pod.namespace}
                    >
                      {pod.namespace}
                    </span>
                  </div>
                  <div className="flex items-center gap-2 flex-shrink-0">
                    {(pod.restarts ?? 0) > 0 && (
                      <span className="text-[10px] text-amber-600 dark:text-amber-400 font-mono">
                        {pod.restarts}x restarts
                      </span>
                    )}
                    <span className={`text-[10px] font-semibold ${statusColor(pod.status)}`}>
                      {pod.status}
                    </span>
                  </div>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
    </GlassCard>
  );
}

// ---------------------------------------------------------------------------
// Panel: Security Score Card
// ---------------------------------------------------------------------------

function SecurityScorePanel({
  summary,
  loading,
  scanning,
  onScan,
}: {
  summary: SecuritySummary | null;
  loading: boolean;
  scanning: boolean;
  onScan: () => void;
}) {
  const grade = summary?.score.grade ?? "—";
  const overall = summary?.score.overall ?? 0;

  return (
    <GlassCard accent>
      <CardHeader
        icon={Shield}
        title="Security Score"
        action={
          <button
            onClick={onScan}
            disabled={scanning || loading}
            className="
              flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-xs font-medium
              bg-accent-500/10 dark:bg-accent-400/10
              text-accent-600 dark:text-accent-400
              border border-accent-500/20 dark:border-accent-400/20
              hover:bg-accent-500/20 dark:hover:bg-accent-400/20
              transition-colors disabled:opacity-50 disabled:cursor-not-allowed
            "
            aria-label="Run security scan"
          >
            {scanning ? (
              <RefreshCw size={11} className="animate-spin" />
            ) : (
              <Zap size={11} />
            )}
            {scanning ? "Scanning…" : "Scan Now"}
          </button>
        }
      />

      {loading ? (
        <div className="p-6 flex items-center justify-center">
          <div className="w-16 h-16 rounded-full bg-stone-100/60 dark:bg-slate-700/40 animate-pulse" />
        </div>
      ) : summary ? (
        <div className="p-4">
          {/* Grade + Score ring area */}
          <div className="flex items-center gap-4 mb-4">
            <div className={`
              w-16 h-16 rounded-full flex items-center justify-center flex-shrink-0
              bg-white/60 dark:bg-slate-900/60
              border-2 border-stone-200 dark:border-slate-700/60
              ${gradeGlow(grade)}
            `}>
              <span className={`text-3xl font-bold font-mono ${gradeColor(grade)}`}>{grade}</span>
            </div>
            <div className="flex-1 min-w-0">
              <div className="flex items-baseline gap-2 mb-1">
                <span className="text-2xl font-bold text-stone-800 dark:text-slate-100 tabular-nums">{overall}</span>
                <span className="text-sm text-stone-400 dark:text-slate-500">/ 100</span>
              </div>
              {/* Score bar */}
              <div className="h-2 rounded-full bg-stone-100 dark:bg-slate-700/60 overflow-hidden">
                <motion.div
                  className={`h-full rounded-full ${
                    overall >= 90 ? "bg-emerald-500" :
                    overall >= 75 ? "bg-cyan-500" :
                    overall >= 60 ? "bg-amber-500" :
                    "bg-red-500"
                  }`}
                  initial={{ width: 0 }}
                  animate={{ width: `${overall}%` }}
                  transition={{ duration: 0.8, ease: "easeOut" }}
                />
              </div>
              <p className="text-[10px] text-stone-400 dark:text-slate-500 mt-1">
                Scanned {relativeTime(summary.score.scannedAt)}
              </p>
            </div>
          </div>

          {/* Issue breakdown */}
          <div className="grid grid-cols-4 gap-1.5">
            {[
              { label: "Critical", count: summary.criticalCount, color: "text-red-500 dark:text-red-400", bg: "bg-red-500/8" },
              { label: "High", count: summary.highCount, color: "text-orange-500 dark:text-orange-400", bg: "bg-orange-500/8" },
              { label: "Medium", count: summary.mediumCount, color: "text-amber-500 dark:text-amber-400", bg: "bg-amber-500/8" },
              { label: "Low", count: summary.lowCount, color: "text-sky-500 dark:text-sky-400", bg: "bg-sky-500/8" },
            ].map(({ label, count, color, bg }) => (
              <div key={label} className={`rounded-lg p-2 text-center ${bg} border border-stone-100/60 dark:border-slate-700/30`}>
                <p className={`text-lg font-bold tabular-nums ${color}`}>{count}</p>
                <p className="text-[9px] text-stone-400 dark:text-slate-500 font-medium uppercase tracking-wide">{label}</p>
              </div>
            ))}
          </div>
        </div>
      ) : (
        <div className="p-6 text-center">
          <Shield size={24} className="mx-auto mb-2 text-stone-300 dark:text-slate-600" />
          <p className="text-sm text-stone-500 dark:text-slate-400 mb-3">No scan results yet</p>
          <button
            onClick={onScan}
            disabled={scanning}
            className="
              inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg
              text-xs font-medium
              bg-accent-500/10 text-accent-600 dark:text-accent-400
              border border-accent-500/20
              hover:bg-accent-500/20 transition-colors
              disabled:opacity-50
            "
          >
            <Zap size={12} />
            Run First Scan
          </button>
        </div>
      )}
    </GlassCard>
  );
}

// ---------------------------------------------------------------------------
// Panel: GitOps Sync Status
// ---------------------------------------------------------------------------

function GitOpsSyncPanel({ gitops, loading }: { gitops: GitOpsStatus | null; loading: boolean }) {
  if (loading) {
    return (
      <GlassCard accent>
        <CardHeader icon={GitBranch} title="GitOps Sync Status" />
        <div className="p-4 space-y-2">
          {[1, 2, 3].map((i) => (
            <div key={i} className="h-9 rounded bg-stone-100/60 dark:bg-slate-700/40 animate-pulse" />
          ))}
        </div>
      </GlassCard>
    );
  }

  if (!gitops || !gitops.detected) {
    return (
      <GlassCard accent>
        <CardHeader icon={GitBranch} title="GitOps Sync Status" />
        <div className="p-8 text-center">
          <GitBranch size={24} className="mx-auto mb-2 text-stone-300 dark:text-slate-600" />
          <p className="text-sm text-stone-500 dark:text-slate-400">No ArgoCD or Flux detected</p>
          <p className="text-[11px] text-stone-400 dark:text-slate-500 mt-1">
            Install ArgoCD or Flux to see sync status here
          </p>
        </div>
      </GlassCard>
    );
  }

  const apps = (gitops.applications || []).slice(0, 10);
  const { summary } = gitops;

  return (
    <GlassCard accent>
      <CardHeader
        icon={GitBranch}
        title="GitOps Sync Status"
        badge={
          <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-mono font-semibold bg-stone-100 dark:bg-slate-700/60 text-stone-500 dark:text-slate-400 border border-stone-200/60 dark:border-slate-600/40 capitalize">
            {gitops.provider}
          </span>
        }
      />

      {/* Summary row */}
      <div className="px-4 py-2.5 border-b border-stone-100/60 dark:border-slate-700/30 flex items-center gap-4 text-xs">
        <span className="text-emerald-600 dark:text-emerald-400 font-medium">{summary.synced} synced</span>
        {summary.outOfSync > 0 && (
          <span className="text-red-500 dark:text-red-400 font-medium">{summary.outOfSync} out of sync</span>
        )}
        {summary.degraded > 0 && (
          <span className="text-orange-500 dark:text-orange-400 font-medium">{summary.degraded} degraded</span>
        )}
        {summary.progressing > 0 && (
          <span className="text-amber-500 dark:text-amber-400 font-medium">{summary.progressing} progressing</span>
        )}
      </div>

      <div className="overflow-y-auto max-h-48">
        <ul role="list" className="divide-y divide-stone-100/60 dark:divide-slate-700/30">
          {apps.map((app) => (
            <li key={`${app.namespace}/${app.name}`} className="px-4 py-2 hover:bg-stone-50/60 dark:hover:bg-slate-700/20 transition-colors">
              <div className="flex items-center gap-2.5">
                <div className="flex items-center gap-1.5 flex-shrink-0">
                  {syncIcon(app.syncStatus)}
                  {healthIcon(app.healthStatus)}
                </div>
                <div className="flex-1 min-w-0">
                  <span
                    className="text-xs font-mono font-medium text-stone-700 dark:text-slate-300 truncate block"
                    title={`${app.namespace}/${app.name}`}
                  >
                    {app.name}
                  </span>
                  <span
                    className="text-[10px] text-stone-400 dark:text-slate-500"
                    title={app.namespace}
                  >
                    {app.namespace}
                  </span>
                </div>
                <div className="flex items-center gap-2 flex-shrink-0">
                  <span className={`text-[10px] font-medium ${statusColor(app.syncStatus)}`}>
                    {app.syncStatus}
                  </span>
                  {app.lastSyncTime && (
                    <span className="text-[10px] text-stone-400 dark:text-slate-500 tabular-nums">
                      {relativeTime(app.lastSyncTime)}
                    </span>
                  )}
                </div>
              </div>
            </li>
          ))}
        </ul>
        {(gitops.applications || []).length > 10 && (
          <div className="px-4 py-2 text-[11px] text-stone-400 dark:text-slate-500 text-center border-t border-stone-100/60 dark:border-slate-700/30">
            +{gitops.applications.length - 10} more applications
          </div>
        )}
      </div>
    </GlassCard>
  );
}

// ---------------------------------------------------------------------------
// Panel: Quick Actions
// ---------------------------------------------------------------------------

function QuickActionsPanel({
  onNewAIQuery,
  onTakeSnapshot,
  onRunSecurityScan,
  scanning,
  snapshotLoading,
  isConnected,
}: {
  onNewAIQuery: () => void;
  onTakeSnapshot: () => void;
  onRunSecurityScan: () => void;
  scanning: boolean;
  snapshotLoading: boolean;
  isConnected: boolean;
}) {
  const actions = [
    {
      icon: Bot,
      label: "New AI Query",
      description: "Ask anything about your cluster",
      onClick: onNewAIQuery,
      loading: false,
      disabled: false,
      accent: true,
    },
    {
      icon: Camera,
      label: "Take Snapshot",
      description: "Capture current cluster state",
      onClick: onTakeSnapshot,
      loading: snapshotLoading,
      disabled: !isConnected || snapshotLoading,
    },
    {
      icon: Shield,
      label: "Run Security Scan",
      description: "Scan for vulnerabilities and issues",
      onClick: onRunSecurityScan,
      loading: scanning,
      disabled: !isConnected || scanning,
    },
  ];

  return (
    <GlassCard accent>
      <CardHeader icon={Zap} title="Quick Actions" />
      <div className="p-3 space-y-2">
        {actions.map(({ icon: Icon, label, description, onClick, loading, disabled, accent }) => (
          <button
            key={label}
            onClick={onClick}
            disabled={disabled}
            className={`
              w-full flex items-center gap-3 px-3 py-2.5 rounded-lg
              border transition-all duration-150 text-left
              focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500/50
              disabled:opacity-50 disabled:cursor-not-allowed
              ${accent
                ? "bg-accent-500/10 dark:bg-accent-400/10 border-accent-500/20 dark:border-accent-400/20 hover:bg-accent-500/20 dark:hover:bg-accent-400/20 text-accent-600 dark:text-accent-400"
                : "bg-stone-50/60 dark:bg-slate-900/30 border-stone-100 dark:border-slate-700/30 hover:bg-stone-100/60 dark:hover:bg-slate-800/40 text-stone-700 dark:text-slate-300"
              }
            `}
            aria-label={label}
          >
            <span className={`flex-shrink-0 ${accent ? "text-accent-500 dark:text-accent-400" : "text-stone-500 dark:text-slate-400"}`}>
              {loading ? <RefreshCw size={16} className="animate-spin" /> : <Icon size={16} />}
            </span>
            <div className="flex-1 min-w-0">
              <p className="text-sm font-medium leading-tight">{label}</p>
              <p className="text-[11px] opacity-60 leading-tight mt-0.5">{description}</p>
            </div>
            <ChevronRight size={14} className="flex-shrink-0 opacity-40" />
          </button>
        ))}
      </div>
    </GlassCard>
  );
}

// ---------------------------------------------------------------------------
// Disconnected state
// ---------------------------------------------------------------------------

function DisconnectedState({ onSelectCluster }: { onSelectCluster?: () => void }) {
  /**
   * Open the Navbar cluster picker. Prefer an explicitly-passed handler; fall
   * back to finding the Navbar's cluster button via its ARIA contract
   * (`aria-haspopup="listbox"` on a <button>). This keeps the CTA functional
   * without requiring App.tsx changes right now, while still allowing a
   * cleaner wiring later.
   */
  const handleSelectCluster = () => {
    if (onSelectCluster) {
      onSelectCluster();
      return;
    }
    const btn = document.querySelector<HTMLButtonElement>(
      'header button[aria-haspopup="listbox"]'
    );
    btn?.click();
    btn?.focus();
  };

  return (
    <div className="flex flex-col items-center justify-center h-full min-h-[400px] gap-4">
      <motion.div
        initial={{ scale: 0.9, opacity: 0 }}
        animate={{ scale: 1, opacity: 1 }}
        transition={{ duration: 0.3 }}
        className="
          w-20 h-20 rounded-2xl flex items-center justify-center
          bg-stone-100/80 dark:bg-slate-800/60
          border border-stone-200 dark:border-slate-700/60
        "
      >
        <WifiOff size={32} className="text-stone-400 dark:text-slate-500" />
      </motion.div>
      <div className="text-center">
        <h2 className="text-lg font-semibold text-stone-700 dark:text-slate-300 mb-1">
          No cluster connected
        </h2>
        <p className="text-sm text-stone-500 dark:text-slate-400 max-w-xs">
          Pick a cluster to see live health, events, and GitOps status.
        </p>
      </div>
      <button
        type="button"
        onClick={handleSelectCluster}
        className="
          inline-flex items-center gap-2 px-4 py-2 rounded-xl
          text-sm font-semibold
          bg-accent-500 hover:bg-accent-400 text-slate-900
          shadow-lg shadow-accent-500/25 hover:shadow-accent-500/40
          transition-all hover:scale-[1.02] active:scale-[0.99]
          focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500/50
        "
      >
        <Plug size={14} />
        Select cluster
      </button>
      <div className="flex items-center gap-2 text-xs text-stone-400 dark:text-slate-500">
        <Wifi size={13} />
        <span>Or use the cluster selector in the top navbar</span>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main Dashboard component
// ---------------------------------------------------------------------------

interface DashboardProps {
  isConnected: boolean;
  onNavigate?: (view: string) => void;
  onOpenOnboarding?: () => void;
  /** Optional: hook to open the Navbar cluster picker. If omitted, the
   *  disconnected-state CTA falls back to clicking the navbar button via DOM. */
  onSelectCluster?: () => void;
}

export function Dashboard({ isConnected, onNavigate, onOpenOnboarding, onSelectCluster }: DashboardProps) {
  // Data state
  const [clusters, setClusters] = useState<ClusterHealthInfo[]>([]);
  const [events, setEvents] = useState<TimelineEvent[]>([]);
  const [pods, setPods] = useState<ResourceInfo[]>([]);
  const [gitops, setGitops] = useState<GitOpsStatus | null>(null);
  const [security, setSecurity] = useState<SecuritySummary | null>(null);

  // Loading state per panel (independent fetches)
  const [loadingClusters, setLoadingClusters] = useState(false);
  const [loadingEvents, setLoadingEvents] = useState(false);
  const [loadingPods, setLoadingPods] = useState(false);
  const [loadingGitops, setLoadingGitops] = useState(false);
  const [loadingSecurity, setLoadingSecurity] = useState(false);

  // Action state
  const [scanning, setScanning] = useState(false);
  const [snapshotLoading, setSnapshotLoading] = useState(false);
  const [toasts, setToasts] = useState<{ id: number; msg: string; ok: boolean }[]>([]);

  const addToast = (msg: string, ok: boolean) => {
    const id = Date.now();
    setToasts((prev) => [...prev, { id, msg, ok }]);
    setTimeout(() => setToasts((prev) => prev.filter((t) => t.id !== id)), 4000);
  };

  // Fetch all dashboard data
  const fetchAll = useCallback(async () => {
    if (!isConnected) return;

    // Cluster health
    setLoadingClusters(true);
    GetMultiClusterHealth()
      .then(setClusters)
      .catch(() => setClusters([]))
      .finally(() => setLoadingClusters(false));

    // Warning events (last 60 min, limit 50)
    setLoadingEvents(true);
    GetTimelineEvents({ namespace: "", kind: "", type: "Warning", sinceMinutes: 60, limit: 50 })
      .then(setEvents)
      .catch(() => setEvents([]))
      .finally(() => setLoadingEvents(false));

    // Pods (all namespaces for unhealthy check)
    setLoadingPods(true);
    ListResources("pods", "")
      .then((res) => setPods((res ?? []) as ResourceInfo[]))
      .catch(() => setPods([]))
      .finally(() => setLoadingPods(false));

    // GitOps
    setLoadingGitops(true);
    GetGitOpsStatus()
      .then((res) => setGitops(res as unknown as GitOpsStatus))
      .catch(() => setGitops(null))
      .finally(() => setLoadingGitops(false));

    // Security
    setLoadingSecurity(true);
    GetSecuritySummary("")
      .then((res) => setSecurity(res as unknown as SecuritySummary))
      .catch(() => setSecurity(null))
      .finally(() => setLoadingSecurity(false));
  }, [isConnected]);

  useEffect(() => {
    if (isConnected) {
      fetchAll();
    } else {
      // Clear all data when disconnected
      setClusters([]);
      setEvents([]);
      setPods([]);
      setGitops(null);
      setSecurity(null);
    }
  }, [isConnected, fetchAll]);

  // Action handlers
  const handleScanNow = async () => {
    setScanning(true);
    try {
      const result = await GetSecuritySummary("");
      setSecurity(result as unknown as SecuritySummary);
      useOnboardingStore.getState().markSecurityScanRun();
      addToast("Security scan complete", true);
    } catch {
      addToast("Security scan failed", false);
    } finally {
      setScanning(false);
    }
  };

  const handleTakeSnapshot = async () => {
    setSnapshotLoading(true);
    try {
      await TakeSnapshot();
      useOnboardingStore.getState().markSnapshotTaken();
      addToast("Snapshot taken successfully", true);
    } catch {
      addToast("Failed to take snapshot", false);
    } finally {
      setSnapshotLoading(false);
    }
  };

  const handleNewAIQuery = () => {
    onNavigate?.("query");
  };

  if (!isConnected) {
    return <DisconnectedState onSelectCluster={onSelectCluster} />;
  }

  return (
    <div className="relative h-full overflow-auto">
      {/* Refresh button */}
      <div className="absolute top-0 right-0 z-10 p-1">
        <button
          onClick={fetchAll}
          className="
            flex items-center gap-1.5 px-2 py-1 rounded-lg text-xs
            text-stone-500 dark:text-slate-400
            hover:text-stone-700 dark:hover:text-slate-200
            hover:bg-stone-100/60 dark:hover:bg-slate-700/40
            transition-colors
          "
          title="Refresh dashboard"
          aria-label="Refresh dashboard"
        >
          <RefreshCw size={12} />
          <span className="hidden sm:inline">Refresh</span>
        </button>
      </div>

      {/* Dashboard grid */}
      <div
        className="p-4 sm:p-6 grid gap-4"
        style={{
          gridTemplateColumns: "repeat(auto-fill, minmax(min(100%, 340px), 1fr))",
        }}
      >
        {/* Getting started card — shown until dismissed */}
        <div className="col-span-full">
          <GettingStartedCard
            isConnected={isConnected}
            onOpenOnboarding={onOpenOnboarding}
            onNavigate={onNavigate}
          />
        </div>

        {/* Row 1 left: Cluster Health */}
        <ClusterHealthPanel clusters={clusters} loading={loadingClusters} />

        {/* Row 1 right: Recent Events — spans 2 rows via CSS grid */}
        <div className="row-span-2 sm:row-span-2">
          <EventsFeedPanel events={events} loading={loadingEvents} />
        </div>

        {/* Row 2 left: Quick Actions */}
        <QuickActionsPanel
          onNewAIQuery={handleNewAIQuery}
          onTakeSnapshot={handleTakeSnapshot}
          onRunSecurityScan={handleScanNow}
          scanning={scanning}
          snapshotLoading={snapshotLoading}
          isConnected={isConnected}
        />

        {/* Row 3: Unhealthy Pods */}
        <UnhealthyResourcesPanel pods={pods} loading={loadingPods} />

        {/* Row 3: Security Score */}
        <SecurityScorePanel
          summary={security}
          loading={loadingSecurity}
          scanning={scanning}
          onScan={handleScanNow}
        />

        {/* Row 4: GitOps */}
        <div className="col-span-full sm:col-span-2 lg:col-span-1" style={{ gridColumn: "1 / -1" }}>
          <GitOpsSyncPanel gitops={gitops} loading={loadingGitops} />
        </div>
      </div>

      {/* Toast notifications */}
      <div
        className="fixed bottom-4 right-4 z-50 flex flex-col gap-2 pointer-events-none"
        aria-live="polite"
        aria-atomic="false"
      >
        <AnimatePresence>
          {toasts.map((t) => (
            <motion.div
              key={t.id}
              initial={{ opacity: 0, x: 40, scale: 0.95 }}
              animate={{ opacity: 1, x: 0, scale: 1 }}
              exit={{ opacity: 0, x: 40, scale: 0.95 }}
              transition={{ duration: 0.2 }}
              role="status"
              className={`
                flex items-center gap-2.5 px-4 py-2.5 rounded-xl
                backdrop-blur-md shadow-lg
                border text-sm font-medium
                ${t.ok
                  ? "bg-emerald-500/15 border-emerald-500/30 text-emerald-700 dark:text-emerald-300"
                  : "bg-red-500/15 border-red-500/30 text-red-700 dark:text-red-300"
                }
              `}
            >
              {t.ok ? <CheckCircle2 size={15} /> : <XCircle size={15} />}
              {t.msg}
            </motion.div>
          ))}
        </AnimatePresence>
      </div>
    </div>
  );
}


