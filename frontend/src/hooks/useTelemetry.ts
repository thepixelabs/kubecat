/**
 * useTelemetry — Privacy-first telemetry hook.
 *
 * Events catalog: 25 pre-approved pairs (action + outcome).
 * All events are anonymous: no PII, no cluster names, no resource names.
 * Gate: localStorage "kubecat-telemetry-consent" must be "granted".
 *
 * Anonymous ID: generated once, stored in localStorage, never rotated.
 * Used only for deduplicating session counts server-side.
 */

import { useCallback, useEffect, useState } from "react";

// ── Types ────────────────────────────────────────────────────────────────────

export type TelemetryConsent = "granted" | "denied" | "pending";

export type TelemetryEventName =
  // App lifecycle
  | "app_launched"
  | "app_update_available"
  | "app_update_dismissed"
  | "app_update_downloaded"
  // Cluster
  | "cluster_connect_started"
  | "cluster_connect_success"
  | "cluster_connect_failed"
  | "cluster_disconnected"
  | "cluster_context_switched"
  // Explorer
  | "explorer_resource_kind_changed"
  | "explorer_resource_inspected"
  | "explorer_resource_yaml_copied"
  | "explorer_search_used"
  // AI Query
  | "ai_query_submitted"
  | "ai_query_completed"
  | "ai_query_failed"
  | "ai_command_approved"
  | "ai_command_rejected"
  | "ai_agent_session_started"
  | "ai_agent_session_completed"
  // Settings
  | "settings_theme_changed"
  | "settings_ai_provider_changed"
  | "settings_zoom_changed"
  // Security
  | "security_scan_started"
  | "security_scan_completed";

export interface TelemetryEvent {
  name: TelemetryEventName;
  properties?: Record<string, string | number | boolean>;
}

// ── Constants ────────────────────────────────────────────────────────────────

const CONSENT_KEY = "kubecat-telemetry-consent";
const ANON_ID_KEY = "kubecat-anon-id";

// Telemetry endpoint — replace with real ingestion URL when available
const TELEMETRY_ENDPOINT =
  (import.meta as any).env?.VITE_TELEMETRY_ENDPOINT ?? "";

// ── Helpers ──────────────────────────────────────────────────────────────────

function getOrCreateAnonId(): string {
  const existing = localStorage.getItem(ANON_ID_KEY);
  if (existing) return existing;

  const id = crypto.randomUUID
    ? crypto.randomUUID()
    : Math.random().toString(36).slice(2) + Date.now().toString(36);

  localStorage.setItem(ANON_ID_KEY, id);
  return id;
}

function readConsent(): TelemetryConsent {
  const raw = localStorage.getItem(CONSENT_KEY);
  if (raw === "granted" || raw === "denied") return raw;
  return "pending";
}

async function sendEvent(
  event: TelemetryEvent,
  anonId: string
): Promise<void> {
  if (!TELEMETRY_ENDPOINT) return;

  try {
    await fetch(TELEMETRY_ENDPOINT, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        anonId,
        event: event.name,
        properties: event.properties ?? {},
        ts: new Date().toISOString(),
      }),
      // Non-blocking best-effort fire-and-forget
      keepalive: true,
    });
  } catch {
    // Telemetry failures are silent — never affect the user
  }
}

// ── Hook ─────────────────────────────────────────────────────────────────────

export interface UseTelemetryReturn {
  consent: TelemetryConsent;
  anonId: string;
  track: (event: TelemetryEvent) => void;
  grantConsent: () => void;
  denyConsent: () => void;
  revokeConsent: () => void;
}

export function useTelemetry(): UseTelemetryReturn {
  const [consent, setConsent] = useState<TelemetryConsent>(readConsent);
  const [anonId] = useState(getOrCreateAnonId);

  // Sync consent state when localStorage changes in another tab
  useEffect(() => {
    const handler = (e: StorageEvent) => {
      if (e.key === CONSENT_KEY) {
        setConsent(readConsent());
      }
    };
    window.addEventListener("storage", handler);
    return () => window.removeEventListener("storage", handler);
  }, []);

  const grantConsent = useCallback(() => {
    localStorage.setItem(CONSENT_KEY, "granted");
    setConsent("granted");
  }, []);

  const denyConsent = useCallback(() => {
    localStorage.setItem(CONSENT_KEY, "denied");
    setConsent("denied");
  }, []);

  const revokeConsent = useCallback(() => {
    localStorage.removeItem(CONSENT_KEY);
    setConsent("pending");
  }, []);

  const track = useCallback(
    (event: TelemetryEvent) => {
      if (consent !== "granted") return;
      // Fire-and-forget
      sendEvent(event, anonId);
    },
    [consent, anonId]
  );

  return { consent, anonId, track, grantConsent, denyConsent, revokeConsent };
}
