/**
 * AIProviderSettings — First-class AI provider configuration in Settings.
 *
 * Moved from a per-window dropdown to Settings so that:
 *   - API keys, endpoints, model lists live in one canonical place.
 *   - The AI query view becomes a lightweight consumer, not the owner.
 *
 * Per provider we surface:
 *   - Enabled toggle
 *   - API key (password field, show/hide)
 *   - Endpoint (Ollama / LiteLLM only — SaaS providers are fixed)
 *   - Default model dropdown + "Refresh from API"
 *   - Test connection action
 *
 * Security: API keys are rendered via type="password" by default and never
 * logged. Values go straight to SaveAISettings → Go config.  Endpoints are
 * SSRF-validated server-side.
 *
 * UX: Provider cards are collapsible; only the enabled / selected provider
 * auto-expands on mount to reduce visual noise.  Long endpoints and model ids
 * use `truncate + title` so hover reveals the full value, and keyboard focus
 * on the form field shows the raw text rather than cropping it.
 */

import { useEffect, useMemo, useState } from "react";
import {
  Bot,
  Check,
  ChevronDown,
  ChevronUp,
  Eye,
  EyeOff,
  ExternalLink,
  Loader2,
  RefreshCw,
  Server,
  ShieldCheck,
  XCircle,
} from "lucide-react";
import {
  GetAISettings,
  GetAvailableProviders,
  SaveAISettings,
  FetchProviderModels,
} from "../../wailsjs/go/main/App";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import { useToastStore } from "../stores/toastStore";

// ── Types ────────────────────────────────────────────────────────────────────

interface ProviderInfo {
  id: string;
  name: string;
  requiresApiKey: boolean;
  defaultEndpoint: string;
  defaultModel: string;
  models: string[];
}

interface ProviderConfig {
  enabled: boolean;
  apiKey: string;
  endpoint: string;
  models: string[];
}

interface AISettings {
  enabled: boolean;
  selectedProvider: string;
  selectedModel: string;
  providers: Record<string, ProviderConfig>;
}

// Providers where a custom endpoint is meaningful.  SaaS providers are locked
// to their canonical URLs and the server rejects any override.
const CUSTOM_ENDPOINT_PROVIDERS = new Set(["ollama", "litellm"]);

// Link-out documentation per provider (where to get an API key).
const PROVIDER_DOCS: Record<string, string> = {
  openai: "https://platform.openai.com/api-keys",
  anthropic: "https://console.anthropic.com/settings/keys",
  google: "https://aistudio.google.com/apikey",
  ollama: "https://ollama.com/download",
  litellm: "https://docs.litellm.ai/docs/",
};

type TestStatus =
  | { state: "idle" }
  | { state: "testing" }
  | { state: "ok"; message: string }
  | { state: "error"; message: string };

// ── Component ────────────────────────────────────────────────────────────────

export function AIProviderSettings() {
  const [providers, setProviders] = useState<ProviderInfo[]>([]);
  const [settings, setSettings] = useState<AISettings | null>(null);
  const [loading, setLoading] = useState(true);
  const [expanded, setExpanded] = useState<string | null>(null);
  const addToast = useToastStore((s) => s.addToast);

  // ── Load on mount ─────────────────────────────────────────────────────────
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const [p, s] = await Promise.all([
          GetAvailableProviders(),
          GetAISettings(),
        ]);
        if (cancelled) return;
        // Normalize into our local shape (strip any extras from Go).
        const providerList: ProviderInfo[] = (p as unknown as ProviderInfo[]).map(
          (x) => ({
            id: x.id,
            name: x.name,
            requiresApiKey: x.requiresApiKey,
            defaultEndpoint: x.defaultEndpoint,
            defaultModel: x.defaultModel,
            models: x.models ?? [],
          })
        );
        const rawSettings = s as unknown as AISettings;
        // Ensure every provider has an entry — makes the controlled-form code below simpler.
        const providersMap: Record<string, ProviderConfig> = {};
        for (const prov of providerList) {
          const existing = rawSettings.providers?.[prov.id];
          providersMap[prov.id] = {
            enabled: existing?.enabled ?? false,
            apiKey: existing?.apiKey ?? "",
            endpoint: existing?.endpoint ?? prov.defaultEndpoint,
            models:
              existing?.models && existing.models.length > 0
                ? existing.models
                : prov.models,
          };
        }
        setProviders(providerList);
        setSettings({
          enabled: rawSettings.enabled ?? false,
          selectedProvider: rawSettings.selectedProvider ?? "",
          selectedModel: rawSettings.selectedModel ?? "",
          providers: providersMap,
        });
        // Auto-expand the currently selected provider, or the first enabled one.
        const initialOpen =
          rawSettings.selectedProvider ||
          Object.entries(providersMap).find(([, v]) => v.enabled)?.[0] ||
          null;
        setExpanded(initialOpen);
      } catch (err) {
        console.error("Failed to load AI settings:", err);
        addToast({
          type: "error",
          message: "Failed to load AI settings",
        });
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [addToast]);

  // ── Persist to backend (debounced through explicit handlers) ───────────────
  const persist = async (next: AISettings) => {
    try {
      await SaveAISettings(next as unknown as any);
    } catch (err) {
      console.error("Failed to save AI settings:", err);
      addToast({
        type: "error",
        message:
          err instanceof Error ? err.message : "Failed to save AI settings",
      });
      throw err;
    }
  };

  // Build the next settings object for a patch without persisting.  Used by
  // text-field handlers that hold edits locally and only flush on blur.
  const buildProviderPatch = (
    state: AISettings,
    providerId: string,
    patch: Partial<ProviderConfig>
  ): AISettings => ({
    ...state,
    providers: {
      ...state.providers,
      [providerId]: {
        ...state.providers[providerId],
        ...patch,
      },
    },
  });

  // Immediate update + persist.  Use for toggles and select changes — those
  // are single atomic user actions, so every change should hit disk.
  const updateProviderConfig = (
    providerId: string,
    patch: Partial<ProviderConfig>
  ) => {
    if (!settings) return;
    const next = buildProviderPatch(settings, providerId, patch);
    setSettings(next);
    persist(next).catch(() => {
      /* already toasted */
    });
  };

  // Local-only update (no persist).  Use for per-keystroke edits to text
  // fields like API key / endpoint — the caller is responsible for flushing
  // the final value to disk via onBlur.
  const updateProviderConfigLocal = (
    providerId: string,
    patch: Partial<ProviderConfig>
  ) => {
    if (!settings) return;
    setSettings((prev) => (prev ? buildProviderPatch(prev, providerId, patch) : prev));
  };

  // Flush the currently-held settings to disk.  Used by text-field onBlur.
  const persistCurrent = () => {
    if (!settings) return;
    persist(settings).catch(() => {
      /* already toasted */
    });
  };

  const toggleEnabled = (providerId: string) => {
    if (!settings) return;
    const current = settings.providers[providerId];
    const nextEnabled = !current.enabled;

    let nextSelectedProvider = settings.selectedProvider;
    let nextSelectedModel = settings.selectedModel;

    if (nextEnabled && !settings.selectedProvider) {
      // First provider enabled → auto-select it.
      nextSelectedProvider = providerId;
      nextSelectedModel = current.models[0] ?? "";
    } else if (!nextEnabled && settings.selectedProvider === providerId) {
      // User disabled the currently selected provider → pick another one.
      const fallback = Object.entries(settings.providers).find(
        ([id, cfg]) => id !== providerId && cfg.enabled
      );
      if (fallback) {
        nextSelectedProvider = fallback[0];
        nextSelectedModel = fallback[1].models[0] ?? "";
      } else {
        nextSelectedProvider = "";
        nextSelectedModel = "";
      }
    }

    const next: AISettings = {
      ...settings,
      enabled:
        nextEnabled ||
        Object.entries(settings.providers).some(
          ([id, c]) => id !== providerId && c.enabled
        ),
      selectedProvider: nextSelectedProvider,
      selectedModel: nextSelectedModel,
      providers: {
        ...settings.providers,
        [providerId]: { ...current, enabled: nextEnabled },
      },
    };
    setSettings(next);
    persist(next).catch(() => {});
  };

  const setDefaultModel = (providerId: string, model: string) => {
    if (!settings) return;
    const next: AISettings = {
      ...settings,
      enabled: true,
      selectedProvider: providerId,
      selectedModel: model,
    };
    setSettings(next);
    persist(next).catch(() => {});
  };

  // ── Render ────────────────────────────────────────────────────────────────
  if (loading || !settings) {
    return (
      <section className="space-y-4">
        <SectionHeader />
        <div className="flex items-center gap-2 p-6 text-sm text-stone-500 dark:text-slate-400">
          <Loader2 size={14} className="animate-spin" aria-hidden="true" />
          Loading providers…
        </div>
      </section>
    );
  }

  return (
    <section aria-labelledby="ai-settings-title" className="space-y-4">
      <SectionHeader />

      <p className="text-xs text-stone-500 dark:text-slate-400 leading-relaxed">
        Configure which AI providers can be used by the Copilot. API keys and
        endpoints are stored locally — never uploaded or logged.
      </p>

      {/* Active selection banner */}
      <ActiveSelectionBanner
        settings={settings}
        providers={providers}
      />

      <div className="space-y-2">
        {providers.map((prov) => {
          const cfg = settings.providers[prov.id];
          const isExpanded = expanded === prov.id;
          const isActive = settings.selectedProvider === prov.id;
          return (
            <ProviderCard
              key={prov.id}
              provider={prov}
              config={cfg}
              isExpanded={isExpanded}
              isActive={isActive}
              activeModel={isActive ? settings.selectedModel : ""}
              onToggleExpand={() =>
                setExpanded((prev) => (prev === prov.id ? null : prov.id))
              }
              onToggleEnabled={() => toggleEnabled(prov.id)}
              onUpdateConfig={(patch) => updateProviderConfig(prov.id, patch)}
              onUpdateConfigLocal={(patch) =>
                updateProviderConfigLocal(prov.id, patch)
              }
              onFlush={persistCurrent}
              onSetDefaultModel={(m) => setDefaultModel(prov.id, m)}
            />
          );
        })}
      </div>
    </section>
  );
}

// ── Subcomponents ────────────────────────────────────────────────────────────

function SectionHeader() {
  return (
    <div className="flex items-center gap-2">
      <Bot
        size={15}
        className="text-accent-500 dark:text-accent-400"
        aria-hidden="true"
      />
      <h3
        id="ai-settings-title"
        className="text-sm font-semibold text-stone-800 dark:text-slate-200"
      >
        AI Providers
      </h3>
    </div>
  );
}

function ActiveSelectionBanner({
  settings,
  providers,
}: {
  settings: AISettings;
  providers: ProviderInfo[];
}) {
  const active = providers.find((p) => p.id === settings.selectedProvider);
  if (!active || !settings.selectedModel) {
    return (
      <div className="flex items-start gap-2 p-3 rounded-xl bg-amber-500/10 border border-amber-500/30">
        <ShieldCheck
          size={14}
          className="text-amber-500 mt-0.5 flex-shrink-0"
          aria-hidden="true"
        />
        <p className="text-xs text-amber-700 dark:text-amber-300 leading-relaxed">
          No default model selected yet. Enable a provider below and pick a
          default model to start using the Copilot.
        </p>
      </div>
    );
  }

  return (
    <div className="flex items-center gap-2 p-3 rounded-xl bg-accent-500/5 border border-accent-500/20">
      <span
        className="w-1.5 h-1.5 rounded-full bg-accent-500 flex-shrink-0"
        aria-hidden="true"
      />
      <div className="flex-1 min-w-0 flex items-baseline gap-2">
        <span className="text-[10px] font-mono font-semibold uppercase tracking-widest text-stone-400 dark:text-slate-500 flex-shrink-0">
          Active
        </span>
        <span
          className="text-xs font-medium text-stone-700 dark:text-slate-200 truncate"
          title={`${active.name} — ${settings.selectedModel}`}
        >
          {active.name}
          <span className="text-stone-400 dark:text-slate-500 mx-1">/</span>
          <code className="font-mono">{settings.selectedModel}</code>
        </span>
      </div>
    </div>
  );
}

// ── ProviderCard ─────────────────────────────────────────────────────────────

interface ProviderCardProps {
  provider: ProviderInfo;
  config: ProviderConfig;
  isExpanded: boolean;
  isActive: boolean;
  activeModel: string;
  onToggleExpand: () => void;
  onToggleEnabled: () => void;
  /** Update config and persist immediately. Use for toggles / selects. */
  onUpdateConfig: (patch: Partial<ProviderConfig>) => void;
  /** Update local state only (no persist). Use with onFlush for text fields. */
  onUpdateConfigLocal: (patch: Partial<ProviderConfig>) => void;
  /** Flush the currently-held settings to disk (call on text-field blur). */
  onFlush: () => void;
  onSetDefaultModel: (model: string) => void;
}

function ProviderCard({
  provider,
  config,
  isExpanded,
  isActive,
  activeModel,
  onToggleExpand,
  onToggleEnabled,
  onUpdateConfig,
  onUpdateConfigLocal,
  onFlush,
  onSetDefaultModel,
}: ProviderCardProps) {
  const [showKey, setShowKey] = useState(false);
  const [testStatus, setTestStatus] = useState<TestStatus>({ state: "idle" });
  const [refreshing, setRefreshing] = useState(false);
  const addToast = useToastStore((s) => s.addToast);

  const canEdit = config.enabled;
  const allowsCustomEndpoint = CUSTOM_ENDPOINT_PROVIDERS.has(provider.id);

  const docUrl = PROVIDER_DOCS[provider.id];

  const handleTest = async () => {
    setTestStatus({ state: "testing" });
    try {
      const models = await FetchProviderModels(
        provider.id,
        config.endpoint,
        config.apiKey
      );
      setTestStatus({
        state: "ok",
        message: `Connected — ${models.length} model${models.length === 1 ? "" : "s"} found`,
      });
      // Auto-dismiss success after a few seconds.
      setTimeout(() => setTestStatus({ state: "idle" }), 4000);
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Connection failed";
      setTestStatus({ state: "error", message });
    }
  };

  const handleRefreshModels = async () => {
    setRefreshing(true);
    try {
      const models = await FetchProviderModels(
        provider.id,
        config.endpoint,
        config.apiKey
      );
      if (models && models.length > 0) {
        onUpdateConfig({ models });
        addToast({
          type: "success",
          message: `Fetched ${models.length} model${models.length === 1 ? "" : "s"} from ${provider.name}`,
          duration: 2500,
        });
      } else {
        addToast({
          type: "info",
          message: `No models returned by ${provider.name}`,
          duration: 3000,
        });
      }
    } catch (err) {
      addToast({
        type: "error",
        message: err instanceof Error ? err.message : "Failed to fetch models",
        duration: 4000,
      });
    } finally {
      setRefreshing(false);
    }
  };

  const dotColor = useMemo(() => {
    if (!config.enabled) return "bg-stone-300 dark:bg-slate-600";
    if (isActive) return "bg-emerald-500";
    return "bg-stone-400 dark:bg-slate-500";
  }, [config.enabled, isActive]);

  return (
    <div
      className={`
        rounded-xl border overflow-hidden transition-colors
        ${
          isActive
            ? "border-accent-500/40 bg-accent-500/5"
            : config.enabled
              ? "border-stone-200 dark:border-slate-700/60 bg-white/60 dark:bg-slate-800/40"
              : "border-stone-200/60 dark:border-slate-700/40 bg-stone-50/50 dark:bg-slate-800/20"
        }
      `}
    >
      {/* Header row */}
      <button
        type="button"
        onClick={onToggleExpand}
        aria-expanded={isExpanded}
        aria-controls={`provider-panel-${provider.id}`}
        className="
          w-full flex items-center gap-3 px-4 py-3
          hover:bg-stone-100/40 dark:hover:bg-slate-700/30
          focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500/50
          transition-colors
        "
      >
        <span
          className={`w-2 h-2 rounded-full flex-shrink-0 ${dotColor}`}
          aria-hidden="true"
        />
        <div className="flex-1 min-w-0 text-left">
          <div className="flex items-center gap-2">
            <p
              className="text-sm font-medium text-stone-800 dark:text-slate-200 truncate"
              title={provider.name}
            >
              {provider.name}
            </p>
            {isActive && (
              <span className="text-[10px] font-mono uppercase tracking-widest text-accent-600 dark:text-accent-400 flex-shrink-0">
                default
              </span>
            )}
          </div>
          <p
            className="text-[11px] text-stone-500 dark:text-slate-500 truncate"
            title={
              isActive && activeModel
                ? `Active model: ${activeModel}`
                : config.enabled
                  ? `Enabled — ${config.models.length} model${config.models.length === 1 ? "" : "s"}`
                  : "Not enabled"
            }
          >
            {isActive && activeModel
              ? activeModel
              : config.enabled
                ? `Enabled • ${config.models.length} model${config.models.length === 1 ? "" : "s"}`
                : "Not enabled"}
          </p>
        </div>

        {/* Enabled toggle — click should not propagate to header toggle. */}
        <span
          role="switch"
          tabIndex={0}
          aria-checked={config.enabled}
          aria-label={`${config.enabled ? "Disable" : "Enable"} ${provider.name}`}
          onClick={(e) => {
            e.stopPropagation();
            onToggleEnabled();
          }}
          onKeyDown={(e) => {
            if (e.key === " " || e.key === "Enter") {
              e.preventDefault();
              e.stopPropagation();
              onToggleEnabled();
            }
          }}
          className={`
            relative inline-flex items-center flex-shrink-0
            w-9 h-5 rounded-full transition-colors duration-200 cursor-pointer
            focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500/50
            ${
              config.enabled
                ? "bg-accent-500 dark:bg-accent-400"
                : "bg-stone-300 dark:bg-slate-600"
            }
          `}
        >
          <span
            className={`
              inline-block w-3.5 h-3.5 rounded-full bg-white
              shadow-sm transition-transform duration-200
              ${config.enabled ? "translate-x-5" : "translate-x-0.5"}
            `}
            aria-hidden="true"
          />
        </span>

        {isExpanded ? (
          <ChevronUp
            size={14}
            className="text-stone-400 dark:text-slate-500 flex-shrink-0"
            aria-hidden="true"
          />
        ) : (
          <ChevronDown
            size={14}
            className="text-stone-400 dark:text-slate-500 flex-shrink-0"
            aria-hidden="true"
          />
        )}
      </button>

      {/* Expanded body */}
      {isExpanded && (
        <div
          id={`provider-panel-${provider.id}`}
          className="px-4 pb-4 pt-1 space-y-3 border-t border-stone-100 dark:border-slate-700/40"
        >
          {/* API key */}
          {provider.requiresApiKey && (
            <Field
              label="API Key"
              description={
                docUrl ? (
                  <button
                    type="button"
                    onClick={() => BrowserOpenURL(docUrl)}
                    className="inline-flex items-center gap-0.5 text-accent-500 dark:text-accent-400 hover:underline"
                  >
                    Get a key
                    <ExternalLink size={10} aria-hidden="true" />
                  </button>
                ) : null
              }
            >
              <div className="relative flex items-stretch">
                <input
                  type={showKey ? "text" : "password"}
                  autoComplete="off"
                  spellCheck={false}
                  value={config.apiKey}
                  placeholder={
                    provider.id === "anthropic"
                      ? "sk-ant-..."
                      : provider.id === "openai"
                        ? "sk-..."
                        : provider.id === "google"
                          ? "AIza..."
                          : "••••••••••••••••"
                  }
                  disabled={!canEdit}
                  onChange={(e) =>
                    onUpdateConfigLocal({ apiKey: e.target.value })
                  }
                  onBlur={onFlush}
                  className="
                    flex-1 min-w-0 px-3 py-1.5 pr-10
                    text-xs font-mono
                    bg-white dark:bg-slate-900
                    border border-stone-200 dark:border-slate-700
                    rounded-lg
                    text-stone-800 dark:text-slate-200
                    placeholder:text-stone-400 dark:placeholder:text-slate-600
                    focus:outline-none focus:ring-2 focus:ring-accent-500/50
                    disabled:opacity-50 disabled:cursor-not-allowed
                    truncate
                  "
                  title={config.apiKey ? "API key configured" : ""}
                />
                <button
                  type="button"
                  onClick={() => setShowKey((v) => !v)}
                  disabled={!canEdit || !config.apiKey}
                  aria-label={showKey ? "Hide API key" : "Show API key"}
                  className="
                    absolute right-1 top-1/2 -translate-y-1/2
                    p-1.5 rounded
                    text-stone-400 hover:text-stone-700
                    dark:text-slate-500 dark:hover:text-slate-300
                    hover:bg-stone-100 dark:hover:bg-slate-700/60
                    disabled:opacity-30 disabled:hover:bg-transparent disabled:cursor-not-allowed
                    transition-colors
                  "
                >
                  {showKey ? (
                    <EyeOff size={12} aria-hidden="true" />
                  ) : (
                    <Eye size={12} aria-hidden="true" />
                  )}
                </button>
              </div>
            </Field>
          )}

          {/* Endpoint — only for ollama / litellm */}
          {allowsCustomEndpoint && (
            <Field
              label="Endpoint"
              description={
                provider.id === "ollama"
                  ? "Must be localhost — Ollama is local-only."
                  : "HTTPS required for remote endpoints."
              }
            >
              <input
                type="url"
                spellCheck={false}
                value={config.endpoint}
                disabled={!canEdit}
                placeholder={provider.defaultEndpoint}
                onChange={(e) =>
                  onUpdateConfigLocal({ endpoint: e.target.value })
                }
                onBlur={onFlush}
                className="
                  w-full px-3 py-1.5
                  text-xs font-mono
                  bg-white dark:bg-slate-900
                  border border-stone-200 dark:border-slate-700
                  rounded-lg
                  text-stone-800 dark:text-slate-200
                  placeholder:text-stone-400 dark:placeholder:text-slate-600
                  focus:outline-none focus:ring-2 focus:ring-accent-500/50
                  disabled:opacity-50 disabled:cursor-not-allowed
                  truncate
                "
                title={config.endpoint}
              />
            </Field>
          )}

          {/* Model selector */}
          <Field
            label="Default Model"
            description={
              isActive
                ? "Used as the default for new Copilot queries."
                : "Enable this provider and pick a model to make it the default."
            }
          >
            <div className="flex gap-2">
              <div className="relative flex-1 min-w-0">
                <select
                  value={isActive ? activeModel : config.models[0] ?? ""}
                  disabled={!canEdit || config.models.length === 0}
                  onChange={(e) => onSetDefaultModel(e.target.value)}
                  className="
                    w-full appearance-none
                    bg-white dark:bg-slate-900
                    border border-stone-200 dark:border-slate-700
                    rounded-lg
                    pl-3 pr-8 py-1.5
                    text-xs font-mono
                    text-stone-800 dark:text-slate-200
                    focus:outline-none focus:ring-2 focus:ring-accent-500/50
                    disabled:opacity-50 disabled:cursor-not-allowed
                    truncate
                  "
                  title={isActive ? activeModel : config.models[0] ?? ""}
                >
                  {config.models.length === 0 ? (
                    <option value="">No models — refresh first</option>
                  ) : (
                    config.models.map((m) => (
                      <option key={m} value={m} title={m}>
                        {m}
                      </option>
                    ))
                  )}
                </select>
                <ChevronDown
                  size={12}
                  className="absolute right-2 top-1/2 -translate-y-1/2 text-stone-400 dark:text-slate-500 pointer-events-none"
                  aria-hidden="true"
                />
              </div>
              <button
                type="button"
                onClick={handleRefreshModels}
                disabled={!canEdit || refreshing}
                title="Fetch latest models from the provider"
                className="
                  inline-flex items-center gap-1.5 px-3 py-1.5
                  text-xs font-medium
                  bg-stone-100 dark:bg-slate-800
                  border border-stone-200 dark:border-slate-700
                  text-stone-700 dark:text-slate-300
                  rounded-lg
                  hover:bg-stone-200 dark:hover:bg-slate-700
                  focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500/50
                  disabled:opacity-50 disabled:cursor-not-allowed
                  transition-colors
                "
              >
                {refreshing ? (
                  <Loader2 size={12} className="animate-spin" aria-hidden="true" />
                ) : (
                  <RefreshCw size={12} aria-hidden="true" />
                )}
                <span>Refresh</span>
              </button>
            </div>
          </Field>

          {/* Test connection */}
          <div className="flex items-center gap-2 pt-1">
            <button
              type="button"
              onClick={handleTest}
              disabled={!canEdit || testStatus.state === "testing"}
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
        </div>
      )}
    </div>
  );
}

// ── Field wrapper ────────────────────────────────────────────────────────────

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
