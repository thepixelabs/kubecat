/**
 * TelemetrySettings — In-settings telemetry toggle + privacy summary.
 *
 * Shows:
 *  - Toggle to grant/deny consent
 *  - One-sentence privacy summary with link to full policy
 *  - Anonymous ID display (read-only, for transparency)
 */

import { BarChart2, ExternalLink, RefreshCw } from "lucide-react";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import { useTelemetry } from "../hooks/useTelemetry";

const PRIVACY_POLICY_URL = "https://kubecat.app/privacy";

export function TelemetrySettings() {
  const { consent, anonId, grantConsent, denyConsent } = useTelemetry();

  const isEnabled = consent === "granted";

  const handleToggle = () => {
    if (isEnabled) {
      denyConsent();
    } else {
      grantConsent();
    }
  };

  return (
    <section
      aria-labelledby="telemetry-settings-title"
      className="space-y-4"
    >
      {/* Section header */}
      <div className="flex items-center gap-2">
        <BarChart2
          size={15}
          className="text-accent-500 dark:text-accent-400"
          aria-hidden="true"
        />
        <h3
          id="telemetry-settings-title"
          className="text-sm font-semibold text-stone-800 dark:text-slate-200"
        >
          Usage Analytics
        </h3>
      </div>

      {/* Toggle row */}
      <div className="flex items-start justify-between gap-4 p-4 rounded-xl bg-stone-100/60 dark:bg-slate-800/40 border border-stone-200/60 dark:border-slate-700/40">
        <div className="flex-1 min-w-0">
          <p className="text-sm font-medium text-stone-800 dark:text-slate-200">
            Anonymous usage data
          </p>
          <p className="text-xs text-stone-500 dark:text-slate-400 mt-0.5 leading-relaxed">
            Send anonymous, aggregated data about feature usage to help us
            improve Kubecat. No cluster data, no PII.{" "}
            <button
              onClick={() => BrowserOpenURL(PRIVACY_POLICY_URL)}
              className="
                inline-flex items-center gap-0.5
                text-accent-500 dark:text-accent-400 hover:underline
                focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent-500/50 rounded
              "
              aria-label="View privacy policy (opens in browser)"
            >
              Privacy policy
              <ExternalLink size={10} aria-hidden="true" />
            </button>
          </p>
        </div>

        {/* Toggle switch */}
        <button
          role="switch"
          aria-checked={isEnabled}
          aria-label="Toggle anonymous usage analytics"
          onClick={handleToggle}
          className={`
            relative inline-flex items-center flex-shrink-0
            w-10 h-5.5 rounded-full transition-colors duration-200
            focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500/50
            ${isEnabled
              ? "bg-accent-500 dark:bg-accent-400"
              : "bg-stone-300 dark:bg-slate-600"
            }
          `}
        >
          <span
            className={`
              inline-block w-4 h-4 rounded-full bg-white
              shadow-sm transition-transform duration-200
              ${isEnabled ? "translate-x-5" : "translate-x-0.5"}
            `}
            aria-hidden="true"
          />
        </button>
      </div>

      {/* Anonymous ID display */}
      {consent !== "pending" && (
        <div className="px-4 py-3 rounded-xl bg-stone-100/40 dark:bg-slate-800/30 border border-stone-200/40 dark:border-slate-700/30">
          <p className="text-[10px] font-mono font-semibold uppercase tracking-widest text-stone-400 dark:text-slate-500 mb-1">
            Anonymous ID
          </p>
          <div className="flex items-center gap-2">
            <code className="text-[11px] font-mono text-stone-500 dark:text-slate-500 truncate flex-1">
              {anonId}
            </code>
            <RefreshCw
              size={10}
              className="text-stone-400 dark:text-slate-600 flex-shrink-0"
              aria-hidden="true"
            />
          </div>
          <p className="text-[10px] text-stone-400 dark:text-slate-600 mt-1">
            This random ID is stored only on your device and is used solely to
            deduplicate session counts. It contains no personal information.
          </p>
        </div>
      )}

      {/* Pending state nudge */}
      {consent === "pending" && (
        <p className="text-xs text-stone-400 dark:text-slate-500 italic">
          You haven't made a choice yet. Enable above to help improve Kubecat.
        </p>
      )}
    </section>
  );
}
