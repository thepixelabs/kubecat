/**
 * CloudAIConsentDialog — Consent gate before sending data to cloud AI providers.
 *
 * - Dark glass panel with amber warning aesthetic
 * - Shows exactly what data is sent: resource YAML, namespace names, cluster context
 * - Data residency and provider DPA links
 * - Mandatory "I understand" checkbox before proceeding
 * - "Use local Ollama instead" escape hatch
 * - Full a11y: focus trap, aria-modal, keyboard dismiss
 */

import { useEffect, useRef, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import {
  AlertTriangle,
  X,
  ExternalLink,
  Shield,
  CheckSquare,
  Square,
  Server,
} from "lucide-react";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";

// ── Types ────────────────────────────────────────────────────────────────────

export interface CloudAIProvider {
  id: string;
  name: string;
  dpaUrl: string;
  dataResidency: string;
}

interface CloudAIConsentDialogProps {
  isOpen: boolean;
  provider: CloudAIProvider;
  onAccept: () => void;
  onDecline: () => void;
  onUseLocal: () => void;
}

// ── Data disclosure items ─────────────────────────────────────────────────────

const DATA_SENT = [
  "Kubernetes resource YAML (the resources you explicitly share)",
  "Namespace names visible in your selected context",
  "Your natural-language query text",
  "Cluster resource kinds (e.g. Pod, Deployment) — not cluster names",
];

const DATA_NOT_SENT = [
  "Secrets or Secret values",
  "Cluster endpoint URLs or kubeconfig credentials",
  "Your identity or any personal information",
  "Data from other namespaces or clusters",
];

// ── Component ─────────────────────────────────────────────────────────────────

export function CloudAIConsentDialog({
  isOpen,
  provider,
  onAccept,
  onDecline,
  onUseLocal,
}: CloudAIConsentDialogProps) {
  const [understood, setUnderstood] = useState(false);
  const dialogRef = useRef<HTMLDivElement>(null);
  const acceptBtnRef = useRef<HTMLButtonElement>(null);

  // Focus trap + focus on open
  useEffect(() => {
    if (!isOpen) return;

    const previouslyFocused = document.activeElement as HTMLElement;
    acceptBtnRef.current?.focus();

    const trap = (e: KeyboardEvent) => {
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

    const escClose = (e: KeyboardEvent) => {
      if (e.key === "Escape") onDecline();
    };

    document.addEventListener("keydown", trap);
    document.addEventListener("keydown", escClose);

    return () => {
      document.removeEventListener("keydown", trap);
      document.removeEventListener("keydown", escClose);
      previouslyFocused?.focus();
    };
  }, [isOpen, onDecline]);

  // Reset checkbox on open
  useEffect(() => {
    if (isOpen) setUnderstood(false);
  }, [isOpen]);

  return (
    <AnimatePresence>
      {isOpen && (
        <>
          {/* Backdrop */}
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
            ref={dialogRef}
            initial={{ opacity: 0, scale: 0.95, y: 16 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.95, y: 16 }}
            transition={{ duration: 0.2, ease: "easeOut" }}
            className="
              fixed inset-0 z-50 flex items-center justify-center p-4
              pointer-events-none
            "
          >
            <div
              className="
                relative w-full max-w-lg
                bg-slate-900/95 dark:bg-slate-950/95
                backdrop-blur-xl
                border border-amber-500/30
                rounded-2xl shadow-2xl shadow-black/50
                pointer-events-auto
                overflow-hidden
              "
              role="dialog"
              aria-modal="true"
              aria-labelledby="consent-title"
              aria-describedby="consent-description"
              onClick={(e) => e.stopPropagation()}
            >
              {/* Amber top stripe */}
              <div className="h-0.5 bg-gradient-to-r from-transparent via-amber-500/60 to-transparent" />

              {/* Header */}
              <div className="flex items-start gap-3 p-5 pb-4">
                <div className="flex-shrink-0 w-9 h-9 rounded-xl bg-amber-500/15 border border-amber-500/25 flex items-center justify-center">
                  <AlertTriangle
                    size={18}
                    className="text-amber-400"
                    aria-hidden="true"
                  />
                </div>

                <div className="flex-1 min-w-0">
                  <h2
                    id="consent-title"
                    className="text-base font-semibold text-slate-100"
                  >
                    Send data to {provider.name}?
                  </h2>
                  <p
                    id="consent-description"
                    className="text-xs text-slate-400 mt-0.5"
                  >
                    Cloud AI requires sending some cluster data to{" "}
                    {provider.name}&apos;s servers. Review what will be shared.
                  </p>
                </div>

                <button
                  onClick={onDecline}
                  className="
                    flex-shrink-0 p-1.5 rounded-lg
                    text-slate-500 hover:text-slate-300
                    hover:bg-slate-700/60
                    transition-colors
                    focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-amber-500/50
                  "
                  aria-label="Close"
                >
                  <X size={16} aria-hidden="true" />
                </button>
              </div>

              {/* Data disclosure */}
              <div className="px-5 pb-4 space-y-3">
                {/* What IS sent */}
                <DataColumn
                  title="What we send"
                  icon="amber"
                  items={DATA_SENT}
                />

                {/* What is NOT sent */}
                <DataColumn
                  title="We never send"
                  icon="emerald"
                  items={DATA_NOT_SENT}
                />

                {/* Provider details */}
                <div className="flex items-center justify-between px-3 py-2.5 rounded-xl bg-slate-800/60 border border-slate-700/40">
                  <div>
                    <p className="text-xs font-medium text-slate-300">
                      Data residency
                    </p>
                    <p className="text-[11px] text-slate-500 mt-0.5">
                      {provider.dataResidency}
                    </p>
                  </div>
                  <button
                    onClick={() => BrowserOpenURL(provider.dpaUrl)}
                    className="
                      flex items-center gap-1
                      text-[11px] text-amber-400 hover:text-amber-300
                      transition-colors
                      focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-amber-500/50 rounded
                    "
                    aria-label={`View ${provider.name} data processing agreement (opens in browser)`}
                  >
                    <Shield size={11} aria-hidden="true" />
                    DPA
                    <ExternalLink size={10} aria-hidden="true" />
                  </button>
                </div>

                {/* Confirmation checkbox */}
                <button
                  onClick={() => setUnderstood(!understood)}
                  className="
                    w-full flex items-start gap-2.5 px-3 py-2.5
                    rounded-xl border transition-colors duration-150
                    focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-amber-500/50
                    text-left
                    ${understood
                      ? 'border-amber-500/40 bg-amber-500/10'
                      : 'border-slate-700/40 bg-slate-800/40 hover:bg-slate-800/60'
                    }
                  "
                  role="checkbox"
                  aria-checked={understood}
                >
                  {understood ? (
                    <CheckSquare
                      size={15}
                      className="text-amber-400 flex-shrink-0 mt-0.5"
                      aria-hidden="true"
                    />
                  ) : (
                    <Square
                      size={15}
                      className="text-slate-500 flex-shrink-0 mt-0.5"
                      aria-hidden="true"
                    />
                  )}
                  <span className="text-xs text-slate-300 leading-relaxed">
                    I understand my cluster data will be sent to{" "}
                    <span className="text-amber-400 font-medium">
                      {provider.name}
                    </span>{" "}
                    and processed under their terms.
                  </span>
                </button>
              </div>

              {/* Actions */}
              <div className="px-5 pb-5 flex items-center gap-3">
                {/* Use local instead */}
                <button
                  onClick={onUseLocal}
                  className="
                    flex items-center gap-1.5
                    px-3 py-2 rounded-xl
                    text-xs font-medium
                    text-slate-400 hover:text-slate-200
                    bg-slate-800/60 hover:bg-slate-700/60
                    border border-slate-700/40 hover:border-slate-600/40
                    transition-colors duration-150
                    focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-slate-500/50
                  "
                >
                  <Server size={12} aria-hidden="true" />
                  Use local Ollama instead
                </button>

                <div className="flex-1" />

                {/* Decline */}
                <button
                  onClick={onDecline}
                  className="
                    px-3 py-2 rounded-xl
                    text-xs font-medium
                    text-slate-400 hover:text-slate-200
                    hover:bg-slate-700/60
                    transition-colors duration-150
                    focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-slate-500/50
                  "
                >
                  Cancel
                </button>

                {/* Accept */}
                <button
                  ref={acceptBtnRef}
                  onClick={understood ? onAccept : undefined}
                  disabled={!understood}
                  className="
                    px-4 py-2 rounded-xl
                    text-xs font-semibold
                    bg-amber-500 hover:bg-amber-400
                    disabled:bg-amber-500/30 disabled:cursor-not-allowed
                    text-white dark:text-slate-900
                    transition-colors duration-150
                    focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-amber-500/50
                  "
                  aria-disabled={!understood}
                >
                  Send to {provider.name}
                </button>
              </div>
            </div>
          </motion.div>
        </>
      )}
    </AnimatePresence>
  );
}

// ── DataColumn helper ─────────────────────────────────────────────────────────

function DataColumn({
  title,
  icon,
  items,
}: {
  title: string;
  icon: "amber" | "emerald";
  items: string[];
}) {
  const isAmber = icon === "amber";
  return (
    <div
      className={`
        rounded-xl border p-3
        ${isAmber
          ? "bg-amber-500/5 border-amber-500/20"
          : "bg-emerald-500/5 border-emerald-500/20"}
      `}
    >
      <p
        className={`text-[10px] font-semibold uppercase tracking-widest mb-2 ${isAmber ? "text-amber-400" : "text-emerald-400"}`}
      >
        {title}
      </p>
      <ul className="space-y-1" role="list">
        {items.map((item) => (
          <li key={item} className="flex items-start gap-1.5">
            <span
              className={`mt-1 w-1 h-1 rounded-full flex-shrink-0 ${isAmber ? "bg-amber-400/60" : "bg-emerald-400/60"}`}
              aria-hidden="true"
            />
            <span className="text-[11px] text-slate-400 leading-relaxed">
              {item}
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}
