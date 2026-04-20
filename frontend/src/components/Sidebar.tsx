/**
 * Sidebar — Kubecat navigation rail
 *
 * Design language: Pixelabs "cockpit rail"
 * - Frosted glass panel flush to the left edge
 * - Active nav item has an accent glow bar + highlighted background
 * - Collapsed mode shows icon-only with tooltip
 * - Logo area with gradient underline separator
 * - Footer shows connection status + version in a compact status row
 */

import { AnimatePresence, motion } from "framer-motion";
import { ChevronRight, type LucideIcon } from "lucide-react";
import { shortClusterName } from "../utils/displayName";

interface NavItem {
  id: string;
  label: string;
  icon: LucideIcon;
  shortcut: string;
}

interface SidebarProps {
  navItems: NavItem[];
  activeView: string;
  sidebarCollapsed: boolean;
  isConnected: boolean;
  appVersion: string;
  contextQueueCount: number;
  /** Currently connected kubeconfig context. Optional for backward-compat
   *  until callers are updated. When present, the footer status line shows
   *  the cluster short-name with the full value on hover. */
  activeContext?: string;

  onNavigate: (view: any) => void;
  onToggleCollapse: () => void;
}

export function Sidebar({
  navItems,
  activeView,
  sidebarCollapsed,
  isConnected,
  appVersion,
  contextQueueCount,
  activeContext,
  onNavigate,
  onToggleCollapse,
}: SidebarProps) {
  const connectedLabel =
    isConnected && activeContext ? shortClusterName(activeContext) : isConnected ? "Connected" : "No cluster";
  const connectedTitle =
    isConnected && activeContext ? activeContext : isConnected ? "Connected" : "No cluster connected";
  return (
    <motion.aside
      className="
        relative
        bg-stone-50/80 dark:bg-slate-900/70
        border-r border-stone-200/80 dark:border-slate-700/40
        flex flex-col
        backdrop-blur-sm
        transition-colors duration-200
        overflow-hidden
      "
      animate={{ width: sidebarCollapsed ? 64 : 224 }}
      transition={{ duration: 0.2, ease: "easeInOut" }}
    >
      {/* Subtle top accent line */}
      <div className="absolute top-0 left-0 right-0 h-px bg-gradient-to-r from-transparent via-accent-500/40 to-transparent" />

      {/* Logo / Brand area */}
      <div className="relative flex-shrink-0">
        <div
          className={`
            flex items-center justify-center
            transition-all duration-200
            ${sidebarCollapsed ? "h-20 px-2" : "h-24 px-4"}
          `}
        >
          <motion.div
            className="flex items-center justify-center w-full h-full"
            animate={{ opacity: 1 }}
          >
            {/* Light mode logos */}
            <img
              src={sidebarCollapsed ? "/kubecat-logo-small-light.png" : "/kubecat-logo-light.png"}
              alt="Kubecat"
              className="max-w-full max-h-full object-contain block dark:hidden"
            />
            {/* Dark mode logos */}
            <img
              src={sidebarCollapsed ? "/kubecat-logo-small.png" : "/kubecat-logo.png"}
              alt="Kubecat"
              className="max-w-full max-h-full object-contain hidden dark:block"
            />
          </motion.div>
        </div>

        {/* Gradient divider */}
        <div className="h-px bg-gradient-to-r from-transparent via-stone-300 dark:via-slate-600/80 to-transparent" />
      </div>

      {/* Navigation items */}
      <nav className="flex-1 py-3 overflow-y-auto overflow-x-hidden" aria-label="Main navigation">
        {navItems.map((item) => {
          const Icon = item.icon;
          const isActive = activeView === item.id;
          const hasBadge = item.id === "query" && contextQueueCount > 0;

          return (
            <SidebarNavItem
              key={item.id}
              item={item}
              Icon={Icon}
              isActive={isActive}
              hasBadge={hasBadge}
              badgeCount={contextQueueCount}
              collapsed={sidebarCollapsed}
              onClick={() => onNavigate(item.id)}
            />
          );
        })}
      </nav>

      {/* Footer — status row + collapse toggle */}
      <div className="flex-shrink-0 border-t border-stone-200/60 dark:border-slate-700/40 p-2">
        <div className={`flex items-center ${sidebarCollapsed ? "justify-center" : "justify-between"} gap-2`}>

          {/* Connection status */}
          {!sidebarCollapsed && (
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              className="flex items-center gap-2 min-w-0"
            >
              <span
                className={`
                  w-2 h-2 rounded-full flex-shrink-0 transition-colors
                  ${isConnected
                    ? "bg-emerald-500 dark:bg-emerald-400 shadow-[0_0_6px_rgba(52,211,153,0.7)]"
                    : "bg-stone-400 dark:bg-slate-600"
                  }
                `}
              />
              <div className="flex flex-col min-w-0">
                <span
                  className={`text-xs font-medium truncate leading-tight ${
                    isConnected
                      ? "text-emerald-700 dark:text-emerald-400"
                      : "text-stone-500 dark:text-slate-500"
                  }`}
                  title={connectedTitle}
                >
                  {connectedLabel}
                </span>
                <span
                  className="text-[10px] text-stone-400 dark:text-slate-600 font-mono leading-tight truncate"
                  title={appVersion}
                >
                  {appVersion}
                </span>
              </div>
            </motion.div>
          )}

          {/* Collapse toggle */}
          <button
            onClick={onToggleCollapse}
            className={`
              p-1.5 rounded-lg transition-all duration-150
              text-stone-400 hover:text-stone-700
              dark:text-slate-500 dark:hover:text-slate-300
              hover:bg-stone-200/60 dark:hover:bg-slate-700/50
              focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500/50
              flex-shrink-0
            `}
            title={sidebarCollapsed ? "Expand sidebar ([)" : "Collapse sidebar ([)"}
            aria-label={sidebarCollapsed ? "Expand sidebar" : "Collapse sidebar"}
          >
            <motion.span
              animate={{ rotate: sidebarCollapsed ? 0 : 180 }}
              transition={{ duration: 0.2 }}
              className="flex items-center"
            >
              <ChevronRight size={15} />
            </motion.span>
          </button>
        </div>
      </div>
    </motion.aside>
  );
}

/* ---- SidebarNavItem ---- */

interface SidebarNavItemProps {
  item: NavItem;
  Icon: LucideIcon;
  isActive: boolean;
  hasBadge: boolean;
  badgeCount: number;
  collapsed: boolean;
  onClick: () => void;
}

function SidebarNavItem({
  item,
  Icon,
  isActive,
  hasBadge,
  badgeCount,
  collapsed,
  onClick,
}: SidebarNavItemProps) {
  return (
    <div className="relative px-2 mb-0.5">
      <button
        onClick={onClick}
        className={`
          relative w-full flex items-center gap-3
          ${collapsed ? "justify-center px-2 py-2.5" : "px-3 py-2"}
          rounded-lg transition-all duration-150
          focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500/50
          group
          ${isActive
            ? "bg-accent-500/10 dark:bg-accent-400/10 text-accent-600 dark:text-accent-400"
            : "text-stone-600 dark:text-slate-400 hover:text-stone-900 dark:hover:text-slate-200 hover:bg-stone-200/50 dark:hover:bg-slate-700/40"
          }
        `}
        title={collapsed ? `${item.label} (${item.shortcut})` : undefined}
        aria-label={`${item.label}, shortcut ${item.shortcut}`}
        aria-current={isActive ? "page" : undefined}
      >
        {/* Active glow bar */}
        {isActive && (
          <motion.span
            layoutId="nav-active-bar"
            className="absolute left-0 top-1/2 -translate-y-1/2 w-0.5 h-5 rounded-r-full bg-accent-500 dark:bg-accent-400 shadow-[0_0_8px_rgba(var(--accent-500),0.8)]"
            transition={{ duration: 0.2 }}
          />
        )}

        {/* Icon */}
        <span className="relative flex-shrink-0">
          <Icon size={18} />
          {/* Badge dot when collapsed */}
          {hasBadge && collapsed && (
            <span className="absolute -top-1 -right-1 w-3.5 h-3.5 bg-violet-500 text-white text-[8px] rounded-full flex items-center justify-center font-bold leading-none">
              {badgeCount > 9 ? "9+" : badgeCount}
            </span>
          )}
        </span>

        {/* Label + shortcut — only when expanded */}
        <AnimatePresence initial={false}>
          {!collapsed && (
            <motion.span
              initial={{ opacity: 0, width: 0 }}
              animate={{ opacity: 1, width: "auto" }}
              exit={{ opacity: 0, width: 0 }}
              transition={{ duration: 0.15 }}
              className="flex flex-1 items-center justify-between overflow-hidden"
            >
              <span className="text-sm font-medium whitespace-nowrap">{item.label}</span>
              <span className="flex items-center gap-1.5 ml-2 flex-shrink-0">
                {hasBadge && (
                  <span className="px-1.5 py-0.5 bg-violet-500 text-white text-[9px] rounded-full font-bold leading-none">
                    {badgeCount > 9 ? "9+" : badgeCount}
                  </span>
                )}
                <kbd
                  className="
                    text-[9px] px-1 py-0.5 rounded
                    bg-stone-200/70 dark:bg-slate-700/50
                    text-stone-400 dark:text-slate-500
                    font-mono leading-none
                    border border-stone-300/40 dark:border-slate-600/40
                  "
                >
                  {item.shortcut}
                </kbd>
              </span>
            </motion.span>
          )}
        </AnimatePresence>
      </button>
    </div>
  );
}
