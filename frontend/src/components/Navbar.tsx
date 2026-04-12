/**
 * Navbar — Kubecat top header bar
 *
 * Design language: Pixelabs "cockpit" aesthetic
 * - Dark glass panel with iridescent accent glow
 * - Accent-colored cluster status pill (connected = glowing cyan)
 * - Icon buttons with frosted hover rings
 * - Clean wordmark slot (view title) in mono font
 * - Mobile-first: collapses to icon-only at small breakpoints
 */

import { AnimatePresence, motion } from "framer-motion";
import type { LucideProps } from "lucide-react";
import {
  ChevronDown,
  Unplug,
  RefreshCw,
  HelpCircle,
  Settings,
  Terminal,
  GitBranch,
} from "lucide-react";

interface NavbarProps {
  activeView: string;
  isConnected: boolean;
  connecting: boolean;
  activeContext: string;
  contexts: string[];
  contextMenuIndex: number;
  showContextMenu: boolean;
  appVersion: string;
  contextMenuContainerRef: React.RefObject<HTMLDivElement>;
  onToggleContextMenu: () => void;
  onConnect: (ctx: string) => void;
  onDisconnect: () => void;
  onRefreshContexts: (e: React.MouseEvent) => void;
  onSetContextMenuIndex: (i: number) => void;
  onShowHelp: () => void;
  onShowSettings: () => void;
  onShowEpics?: () => void;
  onToggleTerminal?: () => void;
  terminalOpen?: boolean;
}

export function Navbar({
  activeView,
  isConnected,
  connecting,
  activeContext,
  contexts,
  contextMenuIndex,
  showContextMenu,
  appVersion,
  contextMenuContainerRef,
  onToggleContextMenu,
  onConnect,
  onDisconnect,
  onRefreshContexts,
  onSetContextMenuIndex,
  onShowHelp,
  onShowSettings,
  onShowEpics,
  onToggleTerminal,
  terminalOpen,
}: NavbarProps) {
  const viewLabel = activeView.charAt(0).toUpperCase() + activeView.slice(1).replace(/-/g, " ");

  return (
    <header
      className="
        relative z-20 h-14 flex items-center justify-between px-4 sm:px-6
        bg-white/40 dark:bg-slate-900/60
        border-b border-stone-200/80 dark:border-slate-700/40
        backdrop-blur-md
        transition-colors duration-200
      "
    >
      {/* Left — view breadcrumb */}
      <div className="flex items-center gap-3 min-w-0">
        {/* Pixelabs brand pill — small, subtle */}
        <span
          className="
            hidden sm:inline-flex items-center gap-1.5
            px-2 py-0.5 rounded-md
            text-[10px] font-mono font-semibold tracking-widest uppercase
            text-accent-500 dark:text-accent-400
            bg-accent-500/10 dark:bg-accent-400/10
            border border-accent-500/20 dark:border-accent-400/20
            select-none flex-shrink-0
          "
        >
          <span className="w-1.5 h-1.5 rounded-full bg-accent-500 dark:bg-accent-400 animate-pulse" />
          kubecat
        </span>

        {/* Separator — hidden on very small screens */}
        <span className="hidden sm:block text-stone-300 dark:text-slate-600 text-lg font-light select-none">
          /
        </span>

        {/* View title */}
        <h1
          className="
            text-sm sm:text-base font-semibold
            text-stone-800 dark:text-slate-100
            truncate capitalize
            font-sans
          "
        >
          {viewLabel}
        </h1>
      </div>

      {/* Right — actions */}
      <div className="flex items-center gap-1.5 sm:gap-2 flex-shrink-0">

        {/* Cluster selector */}
        <div className="relative">
          <button
            onClick={onToggleContextMenu}
            disabled={connecting}
            aria-haspopup="listbox"
            aria-expanded={showContextMenu}
            className={`
              group flex items-center gap-1.5 sm:gap-2
              h-8 px-2.5 sm:px-3 rounded-lg
              text-xs sm:text-sm font-medium
              border transition-all duration-150
              focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500/50
              disabled:opacity-50 disabled:cursor-not-allowed
              ${isConnected
                ? `
                  border-emerald-500/40 dark:border-emerald-400/30
                  bg-emerald-500/10 dark:bg-emerald-400/10
                  text-emerald-700 dark:text-emerald-300
                  hover:border-emerald-500/60 dark:hover:border-emerald-400/50
                  hover:bg-emerald-500/15 dark:hover:bg-emerald-400/15
                  shadow-[0_0_12px_rgba(52,211,153,0.15)] dark:shadow-[0_0_12px_rgba(52,211,153,0.1)]
                `
                : `
                  border-stone-300/80 dark:border-slate-600/60
                  bg-white/60 dark:bg-slate-800/60
                  text-stone-600 dark:text-slate-300
                  hover:border-stone-400 dark:hover:border-slate-500
                  hover:bg-white/80 dark:hover:bg-slate-800/80
                `
              }
            `}
          >
            {/* Status indicator dot */}
            <span
              className={`
                w-1.5 h-1.5 rounded-full flex-shrink-0 transition-colors
                ${isConnected
                  ? "bg-emerald-500 dark:bg-emerald-400 shadow-[0_0_6px_rgba(52,211,153,0.8)]"
                  : "bg-stone-400 dark:bg-slate-500"
                }
                ${connecting ? "animate-pulse" : ""}
              `}
            />

            {/* Context name — hidden on tiny screens */}
            <span className="hidden xs:inline max-w-[100px] sm:max-w-[140px] truncate">
              {connecting
                ? "Connecting…"
                : isConnected && activeContext
                ? activeContext
                : "Select cluster"}
            </span>

            {/* Loading spinner or chevron */}
            {connecting ? (
              <RefreshCw size={12} className="animate-spin flex-shrink-0 opacity-70" />
            ) : (
              <motion.span
                animate={{ rotate: showContextMenu ? 180 : 0 }}
                transition={{ duration: 0.15 }}
                className="flex-shrink-0"
              >
                <ChevronDown size={12} />
              </motion.span>
            )}
          </button>

          {/* Dropdown */}
          <AnimatePresence>
            {showContextMenu && (
              <motion.div
                initial={{ opacity: 0, y: -6, scale: 0.97 }}
                animate={{ opacity: 1, y: 0, scale: 1 }}
                exit={{ opacity: 0, y: -6, scale: 0.97 }}
                transition={{ duration: 0.12 }}
                className="
                  absolute right-0 top-full mt-2 w-64
                  bg-white/90 dark:bg-slate-900/90
                  backdrop-blur-xl
                  border border-stone-200 dark:border-slate-700/60
                  rounded-xl shadow-2xl shadow-black/20 dark:shadow-black/50
                  z-50 overflow-hidden
                "
                role="listbox"
                aria-label="Kubernetes contexts"
              >
                {/* Dropdown header */}
                <div className="px-3 py-2.5 border-b border-stone-100 dark:border-slate-700/60 flex items-center justify-between">
                  <span className="text-[10px] font-mono font-semibold uppercase tracking-widest text-stone-400 dark:text-slate-500">
                    Available Clusters
                  </span>
                  <button
                    onClick={onRefreshContexts}
                    className="
                      p-1 rounded-md transition-colors
                      text-stone-400 hover:text-stone-700
                      dark:text-slate-500 dark:hover:text-slate-300
                      hover:bg-stone-100 dark:hover:bg-slate-700/60
                    "
                    title="Refresh contexts"
                    aria-label="Refresh contexts"
                  >
                    <RefreshCw size={11} />
                  </button>
                </div>

                {/* Context list */}
                <div
                  ref={contextMenuContainerRef}
                  className="max-h-60 overflow-y-auto py-1"
                  role="group"
                >
                  {contexts.length === 0 ? (
                    <p className="text-xs text-stone-400 dark:text-slate-500 p-4 text-center">
                      No contexts found in kubeconfig
                    </p>
                  ) : (
                    contexts.map((ctx, index) => {
                      const isActive = ctx === activeContext;
                      const isHighlighted = index === contextMenuIndex;
                      return (
                        <button
                          key={ctx}
                          role="option"
                          aria-selected={isActive}
                          onClick={() => onConnect(ctx)}
                          onMouseEnter={() => onSetContextMenuIndex(index)}
                          className={`
                            w-full text-left px-3 py-2 text-sm transition-colors
                            flex items-center gap-2
                            ${isActive
                              ? "bg-accent-500/15 dark:bg-accent-400/10 text-accent-600 dark:text-accent-400"
                              : isHighlighted
                              ? "bg-stone-100 dark:bg-slate-700/60 text-stone-900 dark:text-slate-200"
                              : "text-stone-600 dark:text-slate-300 hover:bg-stone-50 dark:hover:bg-slate-800/60"
                            }
                          `}
                        >
                          <span
                            className={`w-1.5 h-1.5 rounded-full flex-shrink-0 ${
                              isActive
                                ? "bg-accent-500 dark:bg-accent-400"
                                : "bg-stone-300 dark:bg-slate-600"
                            }`}
                          />
                          <span className="truncate font-mono text-xs">{ctx}</span>
                          {isActive && (
                            <span className="ml-auto text-[10px] text-accent-500 dark:text-accent-400 font-medium">
                              active
                            </span>
                          )}
                        </button>
                      );
                    })
                  )}
                </div>

                {/* Disconnect */}
                {isConnected && (
                  <div className="px-2 py-2 border-t border-stone-100 dark:border-slate-700/60">
                    <button
                      onClick={onDisconnect}
                      className="
                        w-full flex items-center gap-2 px-3 py-1.5 rounded-lg
                        text-xs text-red-500 dark:text-red-400
                        hover:bg-red-500/10 dark:hover:bg-red-400/10
                        transition-colors
                      "
                    >
                      <Unplug size={12} />
                      Disconnect from cluster
                    </button>
                  </div>
                )}
              </motion.div>
            )}
          </AnimatePresence>
        </div>

        {/* Divider */}
        <div className="w-px h-5 bg-stone-200 dark:bg-slate-700/60 mx-0.5" />

        {/* Terminal toggle */}
        {onToggleTerminal && (
          <NavIconButton
            icon={Terminal}
            label={terminalOpen ? "Close terminal" : "Open terminal"}
            active={terminalOpen}
            onClick={onToggleTerminal}
          />
        )}

        {/* Agent epics board */}
        {onShowEpics && (
          <NavIconButton
            icon={GitBranch}
            label="Agent epics"
            onClick={onShowEpics}
          />
        )}

        {/* Help */}
        <NavIconButton
          icon={HelpCircle}
          label="Keyboard shortcuts (?)"
          onClick={onShowHelp}
        />

        {/* Settings */}
        <NavIconButton
          icon={Settings}
          label="Settings (,)"
          onClick={onShowSettings}
        />

        {/* Version badge — desktop only */}
        <span
          className="
            hidden lg:inline
            ml-1 px-1.5 py-0.5 rounded
            text-[9px] font-mono text-stone-400 dark:text-slate-600
            bg-stone-100 dark:bg-slate-800/60
            border border-stone-200 dark:border-slate-700/40
            select-none
          "
        >
          {appVersion}
        </span>
      </div>
    </header>
  );
}

/* ---- NavIconButton ---- */

interface NavIconButtonProps {
  icon: React.ComponentType<LucideProps>;
  label: string;
  onClick: () => void;
  active?: boolean;
}

function NavIconButton({ icon: Icon, label, onClick, active }: NavIconButtonProps) {
  return (
    <button
      onClick={onClick}
      title={label}
      aria-label={label}
      className={`
        relative flex items-center justify-center
        w-8 h-8 rounded-lg
        transition-all duration-150
        focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500/50
        ${active
          ? "bg-accent-500/15 dark:bg-accent-400/15 text-accent-600 dark:text-accent-400"
          : "text-stone-500 dark:text-slate-400 hover:text-stone-800 dark:hover:text-slate-200 hover:bg-stone-200/60 dark:hover:bg-slate-700/60"
        }
      `}
    >
      <Icon size={17} />
    </button>
  );
}
