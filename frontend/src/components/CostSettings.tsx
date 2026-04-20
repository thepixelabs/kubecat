/**
 * CostSettings — Cost backend configuration (OpenCost / Kubecost).
 *
 * Mirrors the pattern used by AIProviderSettings:
 *   - Load on mount via GetCostSettings() + DetectCostBackend().
 *   - Local-state edits; persist on blur via SaveCostSettings().
 *   - Test-connection probes the endpoint without writing anything.
 *
 * The OpenCost in-cluster service URL is often unreachable from the desktop
 * (e.g. http://opencost.opencost.svc.cluster.local:9003). We surface that
 * scenario with a concrete kubectl port-forward hint so the user has an
 * obvious next step instead of staring at a generic "connection refused".
 *
 * Security: endpoints are user-supplied URLs that eventually cross the Wails
 * boundary. The Go side (cost.QueryOpenCost) performs the network call with
 * a short timeout; we do not log endpoint values anywhere on the frontend
 * other than the user's own input field.
 */

import { useEffect, useRef, useState } from "react";
import {
  Check,
  Coins,
  Loader2,
  Server,
  Terminal as TerminalIcon,
  XCircle,
} from "lucide-react";
import {
  DetectCostBackend,
  GetCostSettings,
  SaveCostSettings,
} from "../../wailsjs/go/main/App";
import { config as configModels } from "../../wailsjs/go/models";
import { useToastStore } from "../stores/toastStore";

// ── Types ────────────────────────────────────────────────────────────────────

// The Wails-generated CostConfig uses Go-style field names (PascalCase).
// Keep the local shape aligned with that so a round-trip through Save/Get
// doesn't silently drop fields.
interface CostConfigShape {
  CPUCostPerCoreHour: number;
  MemCostPerGBHour: number;
  Currency: string;
  OpenCostEndpoint: string;
}

type DetectedBackend = "opencost" | "kubecost" | "none" | "unknown";

type TestStatus =
  | { state: "idle" }
  | { state: "testing" }
  | { state: "ok"; message: string }
  | { state: "error"; message: string; hint?: string };

// Sensible default the user can one-click copy.
const OPENCOST_INCLUSTER_DEFAULT =
  "http://opencost.opencost.svc.cluster.local:9003";
const OPENCOST_PORTFORWARD_DEFAULT = "http://localhost:9003";

// ── Component ────────────────────────────────────────────────────────────────

export function CostSettings() {
  const [cfg, setCfg] = useState<CostConfigShape | null>(null);
  const [backend, setBackend] = useState<DetectedBackend>("unknown");
  const [detecting, setDetecting] = useState(true);
  const [loading, setLoading] = useState(true);
  const [testStatus, setTestStatus] = useState<TestStatus>({ state: "idle" });
  const addToast = useToastStore((s) => s.addToast);

  // Track the last persisted endpoint so onBlur only writes when the value
  // actually changed — avoids redundant YAML writes on every focus/blur.
  const lastPersistedEndpoint = useRef<string>("");

  // ── Load on mount ──────────────────────────────────────────────────────────
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const settings = await GetCostSettings();
        if (cancelled) return;
        // Wails returns an instance of the generated class; normalise to a
        // plain object for controlled-form purposes.
        const raw = settings as unknown as CostConfigShape;
        const next: CostConfigShape = {
          CPUCostPerCoreHour: raw.CPUCostPerCoreHour ?? 0.048,
          MemCostPerGBHour: raw.MemCostPerGBHour ?? 0.006,
          Currency: raw.Currency ?? "USD",
          OpenCostEndpoint: raw.OpenCostEndpoint ?? "",
        };
        setCfg(next);
        lastPersistedEndpoint.current = next.OpenCostEndpoint;
      } catch (err) {
        console.error("Failed to load cost settings:", err);
        addToast({
          type: "error",
          message: "Failed to load cost settings",
        });
      } finally {
        if (!cancelled) setLoading(false);
      }

      // Detect the cluster-side backend in parallel. A failure here is non-fatal
      // — the panel still works for typing in an endpoint by hand.
      try {
        const detected = await DetectCostBackend();
        if (cancelled) return;
        if (detected === "opencost" || detected === "kubecost" || detected === "none") {
          setBackend(detected);
        } else {
          setBackend("unknown");
        }
      } catch {
        if (!cancelled) setBackend("unknown");
      } finally {
        if (!cancelled) setDetecting(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [addToast]);

  // ── Persist (full CostConfig — preserves CPU/mem defaults) ────────────────
  const persist = async (next: CostConfigShape) => {
    try {
      // The Wails binding expects an instance of the generated class so that
      // JSON marshalling picks up the exported Go field names. createFrom
      // just wraps a plain object.
      await SaveCostSettings(
        configModels.CostConfig.createFrom(next) as unknown as never
      );
      lastPersistedEndpoint.current = next.OpenCostEndpoint;
    } catch (err) {
      console.error("Failed to save cost settings:", err);
      addToast({
        type: "error",
        message:
          err instanceof Error ? err.message : "Failed to save cost settings",
      });
      throw err;
    }
  };

  const onEndpointBlur = () => {
    if (!cfg) return;
    if (cfg.OpenCostEndpoint === lastPersistedEndpoint.current) return;
    persist(cfg).catch(() => {
      /* already toasted */
    });
  };

  const onEndpointChange = (value: string) => {
    setCfg((prev) => (prev ? { ...prev, OpenCostEndpoint: value } : prev));
    // Any edit invalidates the previous connection result.
    setTestStatus({ state: "idle" });
  };

  const useSuggestedEndpoint = (suggestion: string) => {
    if (!cfg) return;
    const next = { ...cfg, OpenCostEndpoint: suggestion };
    setCfg(next);
    setTestStatus({ state: "idle" });
    persist(next).catch(() => {});
  };

  // ── Test connection ───────────────────────────────────────────────────────
  const handleTest = async () => {
    if (!cfg) return;
    const endpoint = cfg.OpenCostEndpoint.trim();
    if (!endpoint) {
      setTestStatus({
        state: "error",
        message: "Enter an OpenCost endpoint first.",
      });
      return;
    }

    // Validate the URL shape before we fire anything off the renderer.
    let parsed: URL;
    try {
      parsed = new URL(endpoint);
    } catch {
      setTestStatus({
        state: "error",
        message: "That doesn't look like a valid URL.",
      });
      return;
    }
    if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
      setTestStatus({
        state: "error",
        message: `Only http/https endpoints are supported (got ${parsed.protocol}).`,
      });
      return;
    }

    // If the user edited the endpoint since the last persist, flush first so
    // the backend Test button and the backend cost query are testing the same
    // value the user is looking at.
    if (endpoint !== lastPersistedEndpoint.current) {
      try {
        await persist({ ...cfg, OpenCostEndpoint: endpoint });
      } catch {
        return; // already toasted
      }
    }

    setTestStatus({ state: "testing" });

    // Try /healthz first — OpenCost exposes this as a fast liveness probe.
    // Fall through to /allocation/compute with a tiny window if /healthz is
    // missing (some Kubecost installs don't expose it).
    const base = endpoint.replace(/\/+$/, "");
    const healthURL = `${base}/healthz`;
    const fallbackURL = `${base}/allocation/compute?window=1m&aggregate=namespace`;

    // AbortController — keep the probe snappy; in-cluster DNS will fail fast.
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 4000);

    const looksLikeInCluster =
      parsed.hostname.endsWith(".svc") ||
      parsed.hostname.endsWith(".svc.cluster.local") ||
      parsed.hostname.endsWith(".cluster.local");

    const unreachableHint = looksLikeInCluster
      ? `Tip: in-cluster service URLs are not reachable from your desktop. Try kubectl port-forward svc/opencost -n opencost 9003:9003 and use ${OPENCOST_PORTFORWARD_DEFAULT}.`
      : "Tip: check the endpoint is reachable from this machine, and that CORS permits requests from the desktop app.";

    try {
      let res: Response;
      try {
        res = await fetch(healthURL, { signal: controller.signal });
      } catch {
        // /healthz might not exist — try the data endpoint as a fallback probe.
        res = await fetch(fallbackURL, { signal: controller.signal });
      }
      clearTimeout(timeout);

      if (res.ok) {
        setTestStatus({
          state: "ok",
          message: `Connected (${res.status}).`,
        });
        return;
      }

      setTestStatus({
        state: "error",
        message: `Endpoint responded ${res.status} ${res.statusText || ""}`.trim(),
        hint: unreachableHint,
      });
    } catch (err) {
      clearTimeout(timeout);
      // Network error, DNS failure, CORS block, or timeout — the error object
      // doesn't distinguish reliably in the browser, so a single hint covers it.
      const message =
        err instanceof DOMException && err.name === "AbortError"
          ? "Endpoint did not respond within 4 seconds."
          : "Endpoint not reachable from this machine.";
      setTestStatus({
        state: "error",
        message,
        hint: unreachableHint,
      });
    }
  };

  // ── Render ────────────────────────────────────────────────────────────────
  if (loading || !cfg) {
    return (
      <section className="space-y-4">
        <SectionHeader />
        <div className="flex items-center gap-2 p-6 text-sm text-stone-500 dark:text-slate-400">
          <Loader2 size={14} className="animate-spin" aria-hidden="true" />
          Loading cost settings…
        </div>
      </section>
    );
  }

  const endpointConfigured = cfg.OpenCostEndpoint.trim() !== "";
  const currentSource = endpointConfigured ? "opencost" : "heuristic";

  return (
    <section aria-labelledby="cost-settings-title" className="space-y-4">
      <SectionHeader />

      <p className="text-xs text-stone-500 dark:text-slate-400 leading-relaxed">
        Configure an OpenCost or Kubecost endpoint for authoritative billing.
        When unset, Kubecat falls back to a local heuristic (resource requests
        × default rates).
      </p>

      {/* Detected backend */}
      <BackendStatusCard
        backend={backend}
        detecting={detecting}
        endpointConfigured={endpointConfigured}
        onUseSuggestion={useSuggestedEndpoint}
      />

      {/* Endpoint input */}
      <Field
        label="OpenCost / Kubecost Endpoint"
        description={
          cfg.OpenCostEndpoint ? null : (
            <span className="text-[11px] text-stone-400 dark:text-slate-500">
              Leave empty to use the local heuristic.
            </span>
          )
        }
      >
        <input
          type="url"
          spellCheck={false}
          value={cfg.OpenCostEndpoint}
          onChange={(e) => onEndpointChange(e.target.value)}
          onBlur={onEndpointBlur}
          placeholder={OPENCOST_INCLUSTER_DEFAULT}
          aria-label="OpenCost endpoint URL"
          className="
            w-full px-3 py-1.5
            text-xs font-mono
            bg-white dark:bg-slate-900
            border border-stone-200 dark:border-slate-700
            rounded-lg
            text-stone-800 dark:text-slate-200
            placeholder:text-stone-400 dark:placeholder:text-slate-600
            focus:outline-none focus:ring-2 focus:ring-accent-500/50
            truncate
          "
          title={cfg.OpenCostEndpoint || OPENCOST_INCLUSTER_DEFAULT}
        />
      </Field>

      {/* Test connection */}
      <div className="space-y-2">
        <div className="flex items-center gap-2 pt-1">
          <button
            type="button"
            onClick={handleTest}
            disabled={testStatus.state === "testing"}
            className="
              inline-flex items-center gap-1.5 px-3 py-1.5
              text-xs font-medium
              bg-accent-500/10 border border-accent-500/30
              text-accent-600 dark:text-accent-400
              rounded-lg
              hover:bg-accent-500/20
              focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500/50
              disabled:opacity-50 disabled:cursor-not-allowed
              transition-colors
            "
          >
            {testStatus.state === "testing" ? (
              <Loader2 size={12} className="animate-spin" aria-hidden="true" />
            ) : (
              <Server size={12} aria-hidden="true" />
            )}
            <span>Test connection</span>
          </button>

          {testStatus.state === "ok" && (
            <span
              className="inline-flex items-center gap-1 text-xs text-emerald-600 dark:text-emerald-400 min-w-0"
              role="status"
            >
              <Check size={12} className="flex-shrink-0" aria-hidden="true" />
              <span className="truncate" title={testStatus.message}>
                {testStatus.message}
              </span>
            </span>
          )}

          {testStatus.state === "error" && (
            <span
              className="inline-flex items-center gap-1 text-xs text-red-500 dark:text-red-400 min-w-0"
              role="alert"
            >
              <XCircle size={12} className="flex-shrink-0" aria-hidden="true" />
              <span className="truncate" title={testStatus.message}>
                {testStatus.message}
              </span>
            </span>
          )}
        </div>

        {testStatus.state === "error" && testStatus.hint && (
          <div
            className="flex items-start gap-2 p-2 rounded-lg bg-amber-500/10 border border-amber-500/30 text-[11px] text-amber-700 dark:text-amber-300 leading-relaxed"
            role="note"
          >
            <TerminalIcon
              size={12}
              className="flex-shrink-0 mt-0.5"
              aria-hidden="true"
            />
            <span>{testStatus.hint}</span>
          </div>
        )}
      </div>

      {/* Source-of-cost preview */}
      <div className="flex items-center gap-2 p-3 rounded-xl bg-stone-100/50 dark:bg-slate-800/40 border border-stone-200/60 dark:border-slate-700/40">
        <span
          className={`w-1.5 h-1.5 rounded-full flex-shrink-0 ${
            currentSource === "opencost"
              ? "bg-emerald-500"
              : "bg-stone-400 dark:bg-slate-500"
          }`}
          aria-hidden="true"
        />
        <p className="text-xs text-stone-600 dark:text-slate-300">
          <span className="font-mono text-[10px] uppercase tracking-widest text-stone-400 dark:text-slate-500 mr-2">
            Current source
          </span>
          <span className="font-mono">{currentSource}</span>
          {currentSource === "heuristic" && (
            <span className="text-stone-400 dark:text-slate-500">
              {" "}
              — will switch to <span className="font-mono">opencost</span> once
              an endpoint is set.
            </span>
          )}
        </p>
      </div>
    </section>
  );
}

// ── Subcomponents ────────────────────────────────────────────────────────────

function SectionHeader() {
  return (
    <div className="flex items-center gap-2">
      <Coins
        size={15}
        className="text-accent-500 dark:text-accent-400"
        aria-hidden="true"
      />
      <h3
        id="cost-settings-title"
        className="text-sm font-semibold text-stone-800 dark:text-slate-200"
      >
        Cost
      </h3>
    </div>
  );
}

function BackendStatusCard({
  backend,
  detecting,
  endpointConfigured,
  onUseSuggestion,
}: {
  backend: DetectedBackend;
  detecting: boolean;
  endpointConfigured: boolean;
  onUseSuggestion: (endpoint: string) => void;
}) {
  if (detecting) {
    return (
      <div className="flex items-center gap-2 p-3 rounded-xl bg-stone-100/50 dark:bg-slate-800/40 border border-stone-200/60 dark:border-slate-700/40 text-xs text-stone-500 dark:text-slate-400">
        <Loader2 size={12} className="animate-spin" aria-hidden="true" />
        Detecting backend…
      </div>
    );
  }

  if (backend === "opencost") {
    return (
      <div className="flex items-start gap-2 p-3 rounded-xl bg-emerald-500/10 border border-emerald-500/30">
        <span
          className="w-1.5 h-1.5 rounded-full bg-emerald-500 mt-1.5 flex-shrink-0"
          aria-hidden="true"
        />
        <div className="flex-1 min-w-0 text-xs leading-relaxed text-emerald-700 dark:text-emerald-300">
          <p className="font-medium">OpenCost detected in the active cluster.</p>
          {!endpointConfigured && (
            <div className="mt-1.5 space-y-1 text-emerald-700/90 dark:text-emerald-300/90">
              <p>
                Use the in-cluster URL, or port-forward locally — whichever is
                reachable from this machine.
              </p>
              <div className="flex flex-wrap gap-1.5 pt-0.5">
                <SuggestionButton
                  onClick={() =>
                    onUseSuggestion(OPENCOST_INCLUSTER_DEFAULT)
                  }
                >
                  Use in-cluster URL
                </SuggestionButton>
                <SuggestionButton
                  onClick={() =>
                    onUseSuggestion(OPENCOST_PORTFORWARD_DEFAULT)
                  }
                >
                  Use port-forward URL
                </SuggestionButton>
              </div>
            </div>
          )}
        </div>
      </div>
    );
  }

  if (backend === "kubecost") {
    return (
      <div className="flex items-start gap-2 p-3 rounded-xl bg-accent-500/10 border border-accent-500/30">
        <span
          className="w-1.5 h-1.5 rounded-full bg-accent-500 mt-1.5 flex-shrink-0"
          aria-hidden="true"
        />
        <p className="text-xs leading-relaxed text-accent-700 dark:text-accent-300">
          Kubecost detected — set its OpenCost-compatible API URL below (the
          Kubecost cost-analyzer service exposes <code className="font-mono">/allocation/compute</code>).
        </p>
      </div>
    );
  }

  if (backend === "none") {
    return (
      <div className="flex items-start gap-2 p-3 rounded-xl bg-stone-100/50 dark:bg-slate-800/40 border border-stone-200/60 dark:border-slate-700/40">
        <span
          className="w-1.5 h-1.5 rounded-full bg-stone-400 dark:bg-slate-500 mt-1.5 flex-shrink-0"
          aria-hidden="true"
        />
        <p className="text-xs leading-relaxed text-stone-600 dark:text-slate-400">
          No OpenCost or Kubecost service found in the active cluster. You can
          still point at an endpoint manually (e.g. a port-forward, or another
          cluster).
        </p>
      </div>
    );
  }

  // unknown — detection failed but we don't want to block the user.
  return (
    <div className="flex items-start gap-2 p-3 rounded-xl bg-stone-100/50 dark:bg-slate-800/40 border border-stone-200/60 dark:border-slate-700/40">
      <span
        className="w-1.5 h-1.5 rounded-full bg-stone-400 dark:bg-slate-500 mt-1.5 flex-shrink-0"
        aria-hidden="true"
      />
      <p className="text-xs leading-relaxed text-stone-600 dark:text-slate-400">
        Backend detection unavailable (not connected to a cluster). Enter an
        endpoint URL below to enable OpenCost-backed billing.
      </p>
    </div>
  );
}

function SuggestionButton({
  onClick,
  children,
}: {
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="
        inline-flex items-center gap-1 px-2 py-0.5
        text-[10px] font-mono
        bg-white/60 dark:bg-slate-900/40
        border border-emerald-500/40
        text-emerald-700 dark:text-emerald-300
        rounded
        hover:bg-white dark:hover:bg-slate-900/80
        transition-colors
      "
    >
      {children}
    </button>
  );
}

function Field({
  label,
  description,
  children,
}: {
  label: string;
  description?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-1">
      <div className="flex items-baseline justify-between gap-2">
        <label className="text-[11px] font-semibold uppercase tracking-widest text-stone-500 dark:text-slate-400">
          {label}
        </label>
        {description ? (
          <span className="text-[11px] text-stone-400 dark:text-slate-500 truncate">
            {description}
          </span>
        ) : null}
      </div>
      {children}
    </div>
  );
}
