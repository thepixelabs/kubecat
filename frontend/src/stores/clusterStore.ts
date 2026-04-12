/**
 * clusterStore — Zustand store for cluster connection state.
 *
 * Single source of truth for active cluster context, connection status,
 * available contexts, namespaces, and related metadata.
 */

import { create } from "zustand";
import { persist } from "zustand/middleware";

export interface ClusterState {
  // Connection status
  isConnected: boolean;
  connecting: boolean;
  activeContext: string;

  // Available data
  contexts: string[];
  namespaces: string[];
  appVersion: string;
  isTimelineAvailable: boolean;

  // UI state
  showContextMenu: boolean;
  contextMenuIndex: number;

  // Actions
  setIsConnected: (connected: boolean) => void;
  setConnecting: (connecting: boolean) => void;
  setActiveContext: (ctx: string) => void;
  setContexts: (contexts: string[]) => void;
  setNamespaces: (namespaces: string[]) => void;
  setAppVersion: (version: string) => void;
  setIsTimelineAvailable: (available: boolean) => void;
  setShowContextMenu: (show: boolean) => void;
  setContextMenuIndex: (index: number) => void;

  // Compound actions
  connectSuccess: (ctx: string, namespaces: string[]) => void;
  disconnectCluster: () => void;
  resetAll: () => void;
}

const initialState = {
  isConnected: false,
  connecting: false,
  activeContext: "",
  contexts: [],
  namespaces: [],
  appVersion: "v0.1.0",
  isTimelineAvailable: false,
  showContextMenu: false,
  contextMenuIndex: 0,
};

export const useClusterStore = create<ClusterState>()(
  persist(
    (set) => ({
      ...initialState,

      setIsConnected: (connected) => set({ isConnected: connected }),
      setConnecting: (connecting) => set({ connecting }),
      setActiveContext: (ctx) => set({ activeContext: ctx }),
      setContexts: (contexts) => set({ contexts }),
      setNamespaces: (namespaces) => set({ namespaces }),
      setAppVersion: (appVersion) => set({ appVersion }),
      setIsTimelineAvailable: (isTimelineAvailable) =>
        set({ isTimelineAvailable }),
      setShowContextMenu: (showContextMenu) => set({ showContextMenu }),
      setContextMenuIndex: (contextMenuIndex) => set({ contextMenuIndex }),

      connectSuccess: (ctx, namespaces) =>
        set({
          isConnected: true,
          connecting: false,
          activeContext: ctx,
          namespaces,
          showContextMenu: false,
        }),

      disconnectCluster: () =>
        set({
          isConnected: false,
          activeContext: "",
          namespaces: [],
          isTimelineAvailable: false,
        }),

      resetAll: () => set(initialState),
    }),
    {
      name: "kubecat-cluster-store",
      // Only persist non-volatile UI state — connection status is re-checked on load
      partialize: (state) => ({
        activeContext: state.activeContext,
        appVersion: state.appVersion,
      }),
    }
  )
);
