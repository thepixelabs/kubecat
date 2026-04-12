/**
 * SettingsModal — Extracted settings dialog with AI, Theme, and Telemetry tabs.
 *
 * - Tabs: Appearance, AI Provider, Telemetry
 * - Focus trap + Escape to close + backdrop click
 * - Full a11y: role="dialog", aria-modal, aria-labelledby
 */

import { useEffect, useRef, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { X, Palette, Bot, BarChart2 } from "lucide-react";
import { ThemeSettings, type ColorTheme } from "./ThemeSettings";
import { TelemetrySettings } from "./TelemetrySettings";

// ── Types ────────────────────────────────────────────────────────────────────

type SettingsTab = "appearance" | "ai" | "telemetry";

interface SettingsModalProps {
  isOpen: boolean;
  onClose: () => void;
  colorTheme: ColorTheme;
  setColorTheme: (theme: ColorTheme) => void;
  selectionColor: string;
  setSelectionColor: (color: string) => void;
}

// ── Tab config ────────────────────────────────────────────────────────────────

const TABS: { id: SettingsTab; label: string; icon: typeof Palette }[] = [
  { id: "appearance", label: "Appearance", icon: Palette },
  { id: "ai", label: "AI Provider", icon: Bot },
  { id: "telemetry", label: "Analytics", icon: BarChart2 },
];

// ── Component ─────────────────────────────────────────────────────────────────

export function SettingsModal({
  isOpen,
  onClose,
  colorTheme,
  setColorTheme,
  selectionColor,
  setSelectionColor,
}: SettingsModalProps) {
  const [activeTab, setActiveTab] = useState<SettingsTab>("appearance");
  const dialogRef = useRef<HTMLDivElement>(null);
  const closeBtnRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!isOpen) return;
    const previouslyFocused = document.activeElement as HTMLElement;
    closeBtnRef.current?.focus();

    const trap = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        onClose();
        return;
      }
      if (e.key !== "Tab" || !dialogRef.current) return;
      const focusable = dialogRef.current.querySelectorAll<HTMLElement>(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
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

    document.addEventListener("keydown", trap);
    return () => {
      document.removeEventListener("keydown", trap);
      previouslyFocused?.focus();
    };
  }, [isOpen, onClose]);

  return (
    <AnimatePresence>
      {isOpen && (
        <>
          {/* Backdrop */}
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm"
            onClick={onClose}
            aria-hidden="true"
          />

          {/* Dialog */}
          <motion.div
            initial={{ opacity: 0, scale: 0.96, y: 16 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.96, y: 16 }}
            transition={{ duration: 0.2, ease: "easeOut" }}
            className="fixed inset-0 z-50 flex items-center justify-center p-4 pointer-events-none"
          >
            <div
              ref={dialogRef}
              className="
                relative w-full max-w-2xl max-h-[85vh]
                bg-white/95 dark:bg-slate-900/95
                backdrop-blur-xl
                border border-stone-200/80 dark:border-slate-700/50
                rounded-2xl shadow-2xl shadow-black/20
                pointer-events-auto
                overflow-hidden flex flex-col
              "
              role="dialog"
              aria-modal="true"
              aria-labelledby="settings-title"
              onClick={(e) => e.stopPropagation()}
            >
              {/* Accent top stripe */}
              <div className="h-0.5 bg-gradient-to-r from-transparent via-accent-500/50 to-transparent flex-shrink-0" />

              {/* Header */}
              <div className="flex items-center gap-3 px-5 py-4 border-b border-stone-100 dark:border-slate-700/40 flex-shrink-0">
                <h2
                  id="settings-title"
                  className="flex-1 text-base font-semibold text-stone-900 dark:text-slate-100"
                >
                  Settings
                </h2>
                <button
                  ref={closeBtnRef}
                  onClick={onClose}
                  className="
                    p-1.5 rounded-lg
                    text-stone-400 hover:text-stone-700
                    dark:text-slate-500 dark:hover:text-slate-300
                    hover:bg-stone-100 dark:hover:bg-slate-700/60
                    transition-colors
                    focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500/50
                  "
                  aria-label="Close settings"
                >
                  <X size={16} aria-hidden="true" />
                </button>
              </div>

              {/* Tab bar */}
              <div
                className="flex gap-1 px-5 pt-3 pb-0 flex-shrink-0"
                role="tablist"
                aria-label="Settings sections"
              >
                {TABS.map((tab) => {
                  const Icon = tab.icon;
                  const isActive = activeTab === tab.id;
                  return (
                    <button
                      key={tab.id}
                      role="tab"
                      aria-selected={isActive}
                      aria-controls={`settings-panel-${tab.id}`}
                      id={`settings-tab-${tab.id}`}
                      onClick={() => setActiveTab(tab.id)}
                      className={`
                        flex items-center gap-1.5 px-3 py-2 rounded-t-lg
                        text-xs font-medium transition-colors duration-150
                        border-b-2 -mb-px
                        focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500/50
                        ${isActive
                          ? "border-accent-500 dark:border-accent-400 text-accent-600 dark:text-accent-400 bg-accent-500/5 dark:bg-accent-400/5"
                          : "border-transparent text-stone-500 dark:text-slate-500 hover:text-stone-700 dark:hover:text-slate-300 hover:bg-stone-100/60 dark:hover:bg-slate-700/40"
                        }
                      `}
                    >
                      <Icon size={13} aria-hidden="true" />
                      {tab.label}
                    </button>
                  );
                })}
              </div>

              {/* Divider */}
              <div className="h-px bg-stone-100 dark:bg-slate-700/40 flex-shrink-0" />

              {/* Tab panels */}
              <div className="flex-1 overflow-y-auto">
                <AnimatePresence mode="wait">
                  <motion.div
                    key={activeTab}
                    initial={{ opacity: 0, y: 8 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -8 }}
                    transition={{ duration: 0.15 }}
                    role="tabpanel"
                    id={`settings-panel-${activeTab}`}
                    aria-labelledby={`settings-tab-${activeTab}`}
                    className="p-5"
                  >
                    {activeTab === "appearance" && (
                      <ThemeSettings
                        colorTheme={colorTheme}
                        setColorTheme={setColorTheme}
                        selectionColor={selectionColor}
                        setSelectionColor={setSelectionColor}
                      />
                    )}

                    {activeTab === "ai" && <AIProviderSettings />}

                    {activeTab === "telemetry" && <TelemetrySettings />}
                  </motion.div>
                </AnimatePresence>
              </div>
            </div>
          </motion.div>
        </>
      )}
    </AnimatePresence>
  );
}

// ── AIProviderSettings (inline lightweight section) ───────────────────────────

function AIProviderSettings() {
  return (
    <div className="space-y-4">
      <p className="text-sm text-stone-600 dark:text-slate-400">
        Configure your AI provider in the{" "}
        <span className="font-medium text-stone-800 dark:text-slate-200">
          AI Query
        </span>{" "}
        view via the model selector in the top bar. Supported providers:
        Anthropic Claude, OpenAI, Google Gemini, and local Ollama.
      </p>

      <div className="grid grid-cols-2 gap-3">
        {[
          { name: "Anthropic Claude", type: "cloud", color: "text-amber-500" },
          { name: "OpenAI GPT", type: "cloud", color: "text-emerald-500" },
          { name: "Google Gemini", type: "cloud", color: "text-blue-500" },
          { name: "Ollama (local)", type: "local", color: "text-violet-500" },
        ].map((provider) => (
          <div
            key={provider.name}
            className="flex items-center gap-2.5 px-3 py-2.5 rounded-xl bg-stone-100/60 dark:bg-slate-800/40 border border-stone-200/60 dark:border-slate-700/40"
          >
            <span
              className={`w-2 h-2 rounded-full flex-shrink-0 ${provider.color.replace("text-", "bg-")}`}
              aria-hidden="true"
            />
            <div className="min-w-0">
              <p className="text-xs font-medium text-stone-700 dark:text-slate-300 truncate">
                {provider.name}
              </p>
              <p className="text-[10px] text-stone-400 dark:text-slate-500">
                {provider.type}
              </p>
            </div>
          </div>
        ))}
      </div>

      <p className="text-xs text-stone-400 dark:text-slate-500">
        API keys and model preferences are saved locally in your app data
        directory and are never uploaded.
      </p>
    </div>
  );
}
