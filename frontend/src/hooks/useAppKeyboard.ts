import { useEffect } from "react";
import type { View } from "../types/resources";

interface NavItem {
  id: View;
}

interface UseAppKeyboardOptions {
  showHelp: boolean;
  showSettings: boolean;
  showEpics: boolean;
  showContextMenu: boolean;
  sidebarCollapsed: boolean;
  activeView: View;
  contexts: string[];
  contextMenuIndex: number;
  navItems: NavItem[];
  onCloseHelp: () => void;
  onCloseSettings: () => void;
  onCloseEpics: () => void;
  onCloseContextMenu: () => void;
  onToggleSidebar: () => void;
  onOpenSettings: () => void;
  onOpenHelp: () => void;
  onToggleContextMenu: () => void;
  onNavigate: (view: View) => void;
  onConnect: (ctx: string) => void;
  onSetContextMenuIndex: (idx: number) => void;
  onZoomIn: () => void;
  onZoomOut: () => void;
  onZoomReset: () => void;
}

export function useAppKeyboard({
  showHelp,
  showSettings,
  showEpics,
  showContextMenu,
  sidebarCollapsed,
  activeView,
  contexts,
  contextMenuIndex,
  navItems,
  onCloseHelp,
  onCloseSettings,
  onCloseEpics,
  onCloseContextMenu,
  onToggleSidebar,
  onOpenSettings,
  onOpenHelp,
  onToggleContextMenu,
  onNavigate,
  onConnect,
  onSetContextMenuIndex,
  onZoomIn,
  onZoomOut,
  onZoomReset,
}: UseAppKeyboardOptions) {
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement;
      if (target.tagName === "INPUT" || target.tagName === "TEXTAREA" || target.isContentEditable) {
        if (e.key === "Escape") target.blur();
        return;
      }

      if (e.key === "Escape") {
        e.preventDefault();
        if (showHelp) onCloseHelp();
        else if (showSettings) onCloseSettings();
        else if (showEpics) onCloseEpics();
        else if (showContextMenu) onCloseContextMenu();
        return;
      }

      if (showContextMenu && contexts.length > 0) {
        if (e.key === "ArrowDown") {
          e.preventDefault(); e.stopImmediatePropagation();
          onSetContextMenuIndex((contextMenuIndex + 1) % contexts.length);
          return;
        }
        if (e.key === "ArrowUp") {
          e.preventDefault(); e.stopImmediatePropagation();
          onSetContextMenuIndex((contextMenuIndex - 1 + contexts.length) % contexts.length);
          return;
        }
        if (e.key === "Enter") {
          e.preventDefault(); e.stopImmediatePropagation();
          onConnect(contexts[contextMenuIndex]);
          return;
        }
      }

      if (e.metaKey || e.ctrlKey) {
        if (e.key === "=" || e.key === "+") { e.preventDefault(); onZoomIn(); return; }
        if (e.key === "-") { e.preventDefault(); onZoomOut(); return; }
        if (e.key === "0") { e.preventDefault(); onZoomReset(); return; }
      }

      if (showHelp || showSettings) return;

      if (e.key >= "0" && e.key <= "9") {
        const num = parseInt(e.key);
        const index = num === 0 ? 9 : num - 1;
        if (index < navItems.length) {
          e.preventDefault();
          onNavigate(navItems[index].id);
          return;
        }
      }

      switch (e.key) {
        case "[": e.preventDefault(); onToggleSidebar(); break;
        case ",": e.preventDefault(); onOpenSettings(); break;
        case "?": e.preventDefault(); onOpenHelp(); break;
        case "/":
          e.preventDefault();
          if (activeView === "explorer") {
            const searchEl = document.querySelector('input[placeholder*="Search"]') as HTMLInputElement;
            searchEl?.focus();
          }
          break;
        case "c":
          e.preventDefault();
          onToggleContextMenu();
          if (!showContextMenu) onSetContextMenuIndex(0);
          break;
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [
    showHelp, showSettings, showEpics, showContextMenu,
    sidebarCollapsed, activeView, contexts, contextMenuIndex,
  ]);
}
