import { useState, useEffect, useRef } from "react";
import { WindowToggleMaximise } from "../wailsjs/runtime/runtime";
import { motion, AnimatePresence } from "framer-motion";
import {
  Layers,
  Server,
  MessageSquare,
  Clock,
  GitBranch,
  Shield,
  Network,
  AlertCircle,
  Share2,
  GitCompare,
  KeyRound,
  DollarSign,
} from "lucide-react";
import { type ColorTheme } from "./components/ThemeSettings";
import { AIQueryView } from "./components/AIQueryView";
import { ErrorBoundary } from "./components/ErrorBoundary";
import { OnboardingWizard, getOnboardingState } from "./components/onboarding/OnboardingWizard";
import { ToastContainer } from "./components/ToastContainer";
import { TelemetryConsentDialog } from "./components/TelemetryConsentDialog";
import { useTelemetry } from "./hooks/useTelemetry";
import { useAIStore } from "./stores/aiStore";
import { ErrorModal } from "./components/ErrorModal";
import { ClusterVisualizer } from "./components/cluster-visualizer/ClusterVisualizer";
import { Navbar } from "./components/Navbar";
import { Sidebar } from "./components/Sidebar";
import { Dashboard } from "./components/Dashboard";
import { EpicsPopup } from "./components/EpicsPopup";
import { SettingsModal, type SettingsTab } from "./components/SettingsModal";
import { HelpModal } from "./components/HelpModal";
import { ExplorerView } from "./components/views/ExplorerView";
import { LogsView } from "./components/views/LogsView";
import { TimelineView } from "./components/views/TimelineView";
import { GitOpsView } from "./components/views/GitOpsView";
import { SecurityView } from "./components/views/SecurityView";
import { PortForwardsView } from "./components/views/PortForwardsView";
import { AnalyzerView } from "./components/views/AnalyzerView";
import { ClusterDiffView } from "./components/views/ClusterDiffView";
import { RBACView } from "./components/views/RBACView";
import { CostOverview } from "./components/CostOverview";
import type { ClusterDiffPersistentState } from "./components/cluster-diff/types";
import { useAppKeyboard } from "./hooks/useAppKeyboard";
import {
  GetContexts,
  Connect,
  Disconnect,
  IsConnected,
  GetActiveContext,
  ListResources,
  IsTimelineAvailable,
  RefreshContexts,
  GetAppVersion,
} from "../wailsjs/go/main/App";
import type { View, SelectedPod, ResourceInfo } from "./types/resources";

function App() {
  const [clusterDiffState, setClusterDiffState] = useState<ClusterDiffPersistentState | undefined>(undefined);
  const [activeView, setActiveView] = useState<View>("dashboard");
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [contexts, setContexts] = useState<string[]>([]);
  const [activeContext, setActiveContext] = useState<string>("");
  const [isConnected, setIsConnected] = useState(false);
  const [showContextMenu, setShowContextMenu] = useState(false);
  const [contextMenuIndex, setContextMenuIndex] = useState(0);
  const contextMenuContainerRef = useRef<HTMLDivElement>(null);
  const [connecting, setConnecting] = useState(false);
  const [selectedPod, setSelectedPod] = useState<SelectedPod | null>(null);
  const [showSettings, setShowSettings] = useState(false);
  const [settingsInitialTab, setSettingsInitialTab] = useState<SettingsTab | undefined>(undefined);
  const [showHelp, setShowHelp] = useState(false);
  const [showEpics, setShowEpics] = useState(false);
  const [errorModal, setErrorModal] = useState<{ title: string; message: string } | null>(null);
  const [namespaces, setNamespaces] = useState<string[]>([]);
  const [isTimelineAvailable, setIsTimelineAvailable] = useState(false);
  const [explorerKind, setExplorerKind] = useState("pods");
  const [explorerNamespace, setExplorerNamespace] = useState("");
  const [explorerSearch, setExplorerSearch] = useState("");
  const [appVersion, setAppVersion] = useState("v0.1.0");
  const [colorTheme, setColorTheme] = useState<ColorTheme>(() => (localStorage.getItem("nexus-color-theme") as ColorTheme) || "rain");
  const [selectionColor, setSelectionColor] = useState<string>(() => localStorage.getItem("nexus-selection-color") || "");
  const [zoomLevel, setZoomLevel] = useState<number>(() => {
    const saved = localStorage.getItem("nexus-zoom");
    return saved ? parseFloat(saved) : 1;
  });

  const contextQueueCount = useAIStore((state) => state.contextQueue.length);
  const [showOnboarding, setShowOnboarding] = useState<boolean>(() => !getOnboardingState().completed);
  const { consent, track, grantConsent, denyConsent } = useTelemetry();
  const [showTelemetryConsent, setShowTelemetryConsent] = useState(false);

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", colorTheme);
    localStorage.setItem("nexus-color-theme", colorTheme);
    if (selectionColor) {
      document.documentElement.style.setProperty("--color-selection", selectionColor);
      localStorage.setItem("nexus-selection-color", selectionColor);
    } else {
      document.documentElement.style.removeProperty("--color-selection");
      localStorage.removeItem("nexus-selection-color");
    }
  }, [colorTheme, selectionColor]);

  useEffect(() => {
    document.documentElement.style.fontSize = `${zoomLevel * 16}px`;
    localStorage.setItem("nexus-zoom", zoomLevel.toString());
  }, [zoomLevel]);

  useEffect(() => {
    GetContexts().then((ctxs) => setContexts(ctxs || [])).catch(console.error);
    IsConnected().then(async (connected) => {
      setIsConnected(connected);
      if (connected) setActiveContext(await GetActiveContext());
    }).catch(console.error);
    GetAppVersion().then((v) => { if (v && v !== "dev") setAppVersion(v); });
  }, []);

  const handleConnect = async (contextName: string) => {
    setConnecting(true);
    setShowContextMenu(false);
    try {
      await Connect(contextName);
      const nsList = await ListResources("namespaces", "");
      setActiveContext(contextName);
      setIsConnected(true);
      setNamespaces((nsList || []).map((ns: ResourceInfo) => ns.name));
      track({ name: "cluster_connect_success" });
      IsTimelineAvailable().then(setIsTimelineAvailable).catch(() => setIsTimelineAvailable(false));
    } catch (err) {
      setActiveContext("");
      setIsConnected(false);
      setNamespaces([]);
      setErrorModal({
        title: "Connection Failed",
        message: typeof err === "string" ? err : (err as Error).message || "Failed to connect to cluster.",
      });
    } finally {
      setConnecting(false);
    }
  };

  const handleDisconnect = async () => {
    if (!activeContext) return;
    try {
      await Disconnect(activeContext);
      setActiveContext("");
      setIsConnected(false);
      setNamespaces([]);
    } catch (err) {
      console.error("Failed to disconnect:", err);
    }
  };

  const handleRefreshContexts = async (e: React.MouseEvent) => {
    e.stopPropagation();
    setConnecting(true);
    try {
      setContexts((await RefreshContexts()) || []);
    } catch (err) {
      console.error("Failed to refresh contexts:", err);
    } finally {
      setConnecting(false);
    }
  };

  const navItems = [
    { id: "dashboard" as View, label: "Dashboard", icon: Layers, shortcut: "1" },
    { id: "explorer" as View, label: "Explorer", icon: Server, shortcut: "2" },
    { id: "timeline" as View, label: "Timeline", icon: Clock, shortcut: "3" },
    { id: "visualizer" as View, label: "Visualizer", icon: Share2, shortcut: "4" },
    { id: "analyzer" as View, label: "Analyzer", icon: AlertCircle, shortcut: "5" },
    { id: "gitops" as View, label: "GitOps", icon: GitBranch, shortcut: "6" },
    { id: "security" as View, label: "Security", icon: Shield, shortcut: "7" },
    { id: "portforwards" as View, label: "Port Forwards", icon: Network, shortcut: "8" },
    { id: "diff" as View, label: "Cluster Diff", icon: GitCompare, shortcut: "9" },
    { id: "rbac" as View, label: "RBAC", icon: KeyRound, shortcut: "" },
    { id: "costs" as View, label: "Costs", icon: DollarSign, shortcut: "" },
    { id: "query" as View, label: "AI Query", icon: MessageSquare, shortcut: "0" },
  ];

  useAppKeyboard({
    showHelp, showSettings, showEpics, showContextMenu,
    sidebarCollapsed, activeView, contexts, contextMenuIndex, navItems,
    onCloseHelp: () => setShowHelp(false),
    onCloseSettings: () => setShowSettings(false),
    onCloseEpics: () => setShowEpics(false),
    onCloseContextMenu: () => setShowContextMenu(false),
    onToggleSidebar: () => setSidebarCollapsed((c) => !c),
    onOpenSettings: () => setShowSettings(true),
    onOpenHelp: () => setShowHelp(true),
    onToggleContextMenu: () => setShowContextMenu((v) => !v),
    onNavigate: setActiveView,
    onConnect: handleConnect,
    onSetContextMenuIndex: setContextMenuIndex,
    onZoomIn: () => setZoomLevel((z) => Math.min(z + 0.1, 2)),
    onZoomOut: () => setZoomLevel((z) => Math.max(z - 0.1, 0.5)),
    onZoomReset: () => setZoomLevel(1),
  });

  // Scroll context menu item into view
  useEffect(() => {
    if (showContextMenu && contextMenuContainerRef.current) {
      const container = contextMenuContainerRef.current;
      const item = container.children[contextMenuIndex] as HTMLElement;
      if (item) {
        const cr = container.getBoundingClientRect();
        const ir = item.getBoundingClientRect();
        if (ir.bottom > cr.bottom) container.scrollTop += ir.bottom - cr.bottom;
        else if (ir.top < cr.top) container.scrollTop -= cr.top - ir.top;
      }
    }
  }, [contextMenuIndex, showContextMenu]);

  return (
    <div className="relative flex flex-col h-screen bg-transparent text-stone-900 dark:text-slate-100 transition-colors duration-200">
      <div className="iridescent-bg">
        <div className="iri-blob" /><div className="iri-blob-2" /><div className="iri-blob-3" /><div className="iri-blob-4" />
      </div>
      <div className="contour-bg" />
      <div className="h-8 w-full titlebar-drag flex-shrink-0 bg-stone-50/50 dark:bg-slate-900/50 transition-colors duration-200" onDoubleClick={WindowToggleMaximise} />
      <div className="flex flex-1 overflow-hidden">
        <Sidebar
          navItems={navItems} activeView={activeView} sidebarCollapsed={sidebarCollapsed}
          isConnected={isConnected} appVersion={appVersion} contextQueueCount={contextQueueCount}
          activeContext={activeContext}
          onNavigate={setActiveView} onToggleCollapse={() => setSidebarCollapsed(!sidebarCollapsed)}
        />
        <main className="flex-1 flex flex-col overflow-hidden">
          <Navbar
            activeView={activeView} isConnected={isConnected} connecting={connecting}
            activeContext={activeContext} contexts={contexts} contextMenuIndex={contextMenuIndex}
            showContextMenu={showContextMenu} appVersion={appVersion} contextMenuContainerRef={contextMenuContainerRef}
            onToggleContextMenu={() => setShowContextMenu(!showContextMenu)} onConnect={handleConnect}
            onDisconnect={handleDisconnect} onRefreshContexts={handleRefreshContexts}
            onSetContextMenuIndex={setContextMenuIndex} onShowHelp={() => setShowHelp(true)}
            onShowSettings={() => setShowSettings(true)} onShowEpics={() => setShowEpics(true)}
          />
          <div className={`flex-1 overflow-auto ${activeView === "diff" ? "" : "p-6"}`}>
            <AnimatePresence mode="wait">
              <motion.div key={activeView} initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -20 }} transition={{ duration: 0.2 }} className="h-full">
                {activeView === "dashboard" && <Dashboard isConnected={isConnected} onNavigate={(view) => setActiveView(view as View)} onOpenOnboarding={() => setShowOnboarding(true)} onSelectCluster={() => setShowContextMenu(true)} />}
                {activeView === "explorer" && (
                  <ExplorerView
                    isConnected={isConnected} onSelectPod={(pod) => { setSelectedPod(pod); setActiveView("logs"); }}
                    namespaces={namespaces} selectedKind={explorerKind} setSelectedKind={setExplorerKind}
                    namespaceFilter={explorerNamespace} setNamespaceFilter={setExplorerNamespace}
                    searchInput={explorerSearch} setSearchInput={setExplorerSearch}
                    contextMenuOpen={showContextMenu} activeContext={activeContext}
                  />
                )}
                {activeView === "logs" && (
                  <LogsView isConnected={isConnected} selectedPod={selectedPod} onClearPod={() => { setSelectedPod(null); setActiveView("explorer"); }} />
                )}
                {activeView === "timeline" && <TimelineView isConnected={isConnected} namespaces={namespaces} />}
                {activeView === "gitops" && <GitOpsView isConnected={isConnected} />}
                {activeView === "security" && <SecurityView isConnected={isConnected} />}
                {activeView === "portforwards" && <PortForwardsView isConnected={isConnected} />}
                {activeView === "analyzer" && <AnalyzerView isConnected={isConnected} />}
                {activeView === "visualizer" && (
                  <ErrorBoundary componentName="Cluster Visualizer">
                    <ClusterVisualizer isConnected={isConnected} namespaces={namespaces} />
                  </ErrorBoundary>
                )}
                {activeView === "diff" && (
                  <ClusterDiffView
                    contexts={contexts} activeContext={activeContext} namespaces={namespaces}
                    isTimelineAvailable={isTimelineAvailable} initialState={clusterDiffState} onStateChange={setClusterDiffState}
                  />
                )}
                {activeView === "rbac" && (
                  <RBACView isConnected={isConnected} namespaces={namespaces} activeContext={activeContext} />
                )}
                {activeView === "costs" && (
                  <CostOverview
                    activeContext={activeContext}
                    namespace={explorerNamespace || namespaces[0] || "default"}
                    onOpenSettings={() => {
                      setSettingsInitialTab("cost");
                      setShowSettings(true);
                    }}
                  />
                )}
                {activeView === "query" && (
                  <ErrorBoundary componentName="AI Copilot">
                    <AIQueryView
                      onOpenSettings={() => {
                        setSettingsInitialTab("ai");
                        setShowSettings(true);
                      }}
                    />
                  </ErrorBoundary>
                )}
              </motion.div>
            </AnimatePresence>
          </div>
        </main>
      </div>

      <SettingsModal
        isOpen={showSettings}
        onClose={() => {
          setShowSettings(false);
          // Clear the deep-link so the next open defaults to Appearance again.
          setSettingsInitialTab(undefined);
        }}
        colorTheme={colorTheme}
        setColorTheme={setColorTheme}
        selectionColor={selectionColor}
        setSelectionColor={setSelectionColor}
        initialTab={settingsInitialTab}
      />
      <HelpModal isOpen={showHelp} onClose={() => setShowHelp(false)} activeView={activeView} />
      <ErrorModal isOpen={errorModal !== null} onClose={() => setErrorModal(null)} title={errorModal?.title || "Error"} message={errorModal?.message || ""} />
      <EpicsPopup isOpen={showEpics} onClose={() => setShowEpics(false)} />
      <ToastContainer />

      <AnimatePresence>
        {showOnboarding && (
          <OnboardingWizard
            contexts={contexts} onConnect={handleConnect} onFinish={() => { setShowOnboarding(false); if (consent === "pending") setShowTelemetryConsent(true); }}
            connecting={connecting} activeContext={activeContext} isConnected={isConnected}
            onRefreshContexts={async () => setContexts((await RefreshContexts()) || [])}
          />
        )}
      </AnimatePresence>

      <TelemetryConsentDialog
        isOpen={showTelemetryConsent}
        onAccept={() => { grantConsent(); setShowTelemetryConsent(false); }}
        onDecline={() => { denyConsent(); setShowTelemetryConsent(false); }}
      />
    </div>
  );
}

export default App;
