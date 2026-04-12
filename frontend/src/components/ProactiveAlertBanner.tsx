/**
 * ProactiveAlertBanner — Stacked notification cards (max 3, top-right).
 *
 * - Color-coded by severity: critical=red, high=amber, medium=yellow, low=blue, info=slate
 * - "Want me to investigate?" CTA fires a callback with the alert context
 * - Overflow counter badge when > 3 alerts are queued
 * - Each card dismissable individually or via "investigate" action
 * - Smooth enter/exit via framer-motion
 */

import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { X, AlertTriangle, Info, AlertCircle, Sparkles } from "lucide-react";

// ── Types ────────────────────────────────────────────────────────────────────

export type AlertSeverity = "critical" | "high" | "medium" | "low" | "info";

export interface ProactiveAlert {
  id: string;
  severity: AlertSeverity;
  title: string;
  message: string;
  /** Optional context passed to the investigate callback */
  context?: Record<string, string>;
}

interface ProactiveAlertBannerProps {
  alerts: ProactiveAlert[];
  onDismiss: (id: string) => void;
  onInvestigate: (alert: ProactiveAlert) => void;
}

// ── Severity config ───────────────────────────────────────────────────────────

const severityConfig: Record<
  AlertSeverity,
  {
    icon: typeof AlertCircle;
    border: string;
    bg: string;
    text: string;
    badge: string;
  }
> = {
  critical: {
    icon: AlertCircle,
    border: "border-red-500/40 dark:border-red-400/30",
    bg: "bg-red-500/10 dark:bg-red-400/10",
    text: "text-red-700 dark:text-red-300",
    badge: "bg-red-500 text-white",
  },
  high: {
    icon: AlertTriangle,
    border: "border-amber-500/40 dark:border-amber-400/30",
    bg: "bg-amber-500/10 dark:bg-amber-400/10",
    text: "text-amber-700 dark:text-amber-300",
    badge: "bg-amber-500 text-white",
  },
  medium: {
    icon: AlertTriangle,
    border: "border-yellow-500/40 dark:border-yellow-400/30",
    bg: "bg-yellow-500/10 dark:bg-yellow-400/10",
    text: "text-yellow-700 dark:text-yellow-300",
    badge: "bg-yellow-500 text-white",
  },
  low: {
    icon: Info,
    border: "border-blue-500/40 dark:border-blue-400/30",
    bg: "bg-blue-500/10 dark:bg-blue-400/10",
    text: "text-blue-700 dark:text-blue-300",
    badge: "bg-blue-500 text-white",
  },
  info: {
    icon: Info,
    border: "border-slate-400/40 dark:border-slate-500/30",
    bg: "bg-slate-100/60 dark:bg-slate-800/40",
    text: "text-slate-600 dark:text-slate-400",
    badge: "bg-slate-500 text-white",
  },
};

const MAX_VISIBLE = 3;

// ── Component ─────────────────────────────────────────────────────────────────

export function ProactiveAlertBanner({
  alerts,
  onDismiss,
  onInvestigate,
}: ProactiveAlertBannerProps) {
  const visible = alerts.slice(0, MAX_VISIBLE);
  const overflow = alerts.length - MAX_VISIBLE;

  if (alerts.length === 0) return null;

  return (
    <div
      className="fixed top-20 right-4 z-50 flex flex-col gap-2 w-80"
      role="region"
      aria-label="Proactive alerts"
      aria-live="polite"
    >
      <AnimatePresence initial={false} mode="popLayout">
        {visible.map((alert) => (
          <AlertCard
            key={alert.id}
            alert={alert}
            onDismiss={() => onDismiss(alert.id)}
            onInvestigate={() => onInvestigate(alert)}
          />
        ))}

        {overflow > 0 && (
          <motion.div
            key="overflow"
            initial={{ opacity: 0, scale: 0.95 }}
            animate={{ opacity: 1, scale: 1 }}
            exit={{ opacity: 0, scale: 0.95 }}
            className="
              flex items-center justify-center
              px-3 py-1.5
              bg-slate-800/80 dark:bg-slate-900/80
              backdrop-blur-sm
              border border-slate-600/40
              rounded-lg
              text-xs text-slate-400 font-medium
            "
          >
            +{overflow} more alert{overflow === 1 ? "" : "s"}
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

// ── AlertCard ─────────────────────────────────────────────────────────────────

interface AlertCardProps {
  alert: ProactiveAlert;
  onDismiss: () => void;
  onInvestigate: () => void;
}

function AlertCard({ alert, onDismiss, onInvestigate }: AlertCardProps) {
  const [expanded, setExpanded] = useState(false);
  const config = severityConfig[alert.severity];
  const Icon = config.icon;

  return (
    <motion.div
      initial={{ opacity: 0, x: 60, scale: 0.95 }}
      animate={{ opacity: 1, x: 0, scale: 1 }}
      exit={{ opacity: 0, x: 60, scale: 0.9 }}
      transition={{ duration: 0.2, ease: "easeOut" }}
      className={`
        relative rounded-xl overflow-hidden
        border ${config.border}
        ${config.bg}
        backdrop-blur-sm
        shadow-lg shadow-black/20
      `}
      role="alert"
      aria-label={`${alert.severity} alert: ${alert.title}`}
    >
      {/* Severity stripe */}
      <div
        className={`absolute left-0 top-0 bottom-0 w-0.5 ${config.badge.split(" ")[0]}`}
        aria-hidden="true"
      />

      <div className="px-3 pt-3 pb-2 pl-4">
        {/* Header */}
        <div className="flex items-start gap-2">
          <Icon
            size={14}
            className={`${config.text} flex-shrink-0 mt-0.5`}
            aria-hidden="true"
          />
          <div className="flex-1 min-w-0">
            <p className={`text-xs font-semibold ${config.text} truncate`}>
              {alert.title}
            </p>
            <p
              className={`text-xs ${config.text} opacity-80 mt-0.5 ${expanded ? "" : "line-clamp-2"} leading-relaxed`}
            >
              {alert.message}
            </p>
            {alert.message.length > 80 && (
              <button
                onClick={() => setExpanded(!expanded)}
                className={`text-[10px] ${config.text} opacity-60 hover:opacity-100 transition-opacity mt-0.5`}
                aria-expanded={expanded}
              >
                {expanded ? "Show less" : "Show more"}
              </button>
            )}
          </div>

          {/* Dismiss */}
          <button
            onClick={onDismiss}
            className={`
              flex-shrink-0 p-0.5 rounded
              ${config.text} opacity-50 hover:opacity-100
              transition-opacity
              focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-current
            `}
            aria-label={`Dismiss: ${alert.title}`}
          >
            <X size={12} aria-hidden="true" />
          </button>
        </div>

        {/* CTA */}
        <div className="flex justify-end mt-2">
          <button
            onClick={onInvestigate}
            className={`
              flex items-center gap-1
              px-2 py-0.5 rounded-md
              text-[10px] font-medium
              ${config.text}
              bg-current/10
              hover:bg-current/20
              transition-colors duration-150
              focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-current
            `}
            aria-label={`Investigate: ${alert.title}`}
          >
            <Sparkles size={10} aria-hidden="true" />
            Want me to investigate?
          </button>
        </div>
      </div>
    </motion.div>
  );
}
