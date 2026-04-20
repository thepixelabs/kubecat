/**
 * Dashboard — operational command center.
 *
 * Coverage priorities:
 * - DisconnectedState CTA: wired to onSelectCluster when provided,
 *   falls back to DOM click of the Navbar cluster button.
 * - Tiles render populated data (cluster health, recent events, unhealthy
 *   pods, GitOps, security).
 * - Successful Snapshot + Scan actions hit the onboarding setters so the
 *   onboarding flow can advance.
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { Dashboard } from "./Dashboard";
import { useOnboardingStore } from "../stores/onboardingStore";

// ---------------------------------------------------------------------------
// Wails mocks — each backend call is a named mock so tests can override per-case.
// ---------------------------------------------------------------------------

const mockGetMultiClusterHealth = vi.fn();
const mockGetTimelineEvents = vi.fn();
const mockListResources = vi.fn();
const mockGetGitOpsStatus = vi.fn();
const mockGetSecuritySummary = vi.fn();
const mockTakeSnapshot = vi.fn();

vi.mock("../../wailsjs/go/main/App", () => ({
  GetMultiClusterHealth: () => mockGetMultiClusterHealth(),
  GetTimelineEvents: (...args: unknown[]) => mockGetTimelineEvents(...args),
  ListResources: (...args: unknown[]) => mockListResources(...args),
  GetGitOpsStatus: () => mockGetGitOpsStatus(),
  GetSecuritySummary: (...args: unknown[]) => mockGetSecuritySummary(...args),
  TakeSnapshot: () => mockTakeSnapshot(),
}));

// Keep GettingStartedCard as a cheap stub — it pulls onboarding state we
// don't care about for these tests.
vi.mock("./onboarding/GettingStartedCard", () => ({
  GettingStartedCard: () => <div data-testid="getting-started" />,
}));

// ---------------------------------------------------------------------------
// Default fixtures
// ---------------------------------------------------------------------------

const HEALTHY_CLUSTER = {
  context: "prod",
  status: "healthy",
  nodeCount: 5,
  podCount: 120,
  cpuPercent: 42,
  memPercent: 55,
  issues: 0,
  lastChecked: new Date().toISOString(),
};

const WARNING_EVENT = {
  id: 1,
  cluster: "prod",
  namespace: "default",
  kind: "Pod",
  name: "bad-pod",
  type: "Warning",
  reason: "BackOff",
  message: "Container is crashing",
  firstSeen: "",
  lastSeen: new Date().toISOString(),
  count: 2,
  sourceComponent: "kubelet",
};

const UNHEALTHY_POD = {
  kind: "Pod",
  name: "api-5d4",
  namespace: "default",
  status: "CrashLoopBackOff",
  age: "1h",
  restarts: 12,
};

const GITOPS_OK = {
  provider: "argocd",
  detected: true,
  applications: [
    {
      name: "checkout",
      namespace: "default",
      provider: "argocd",
      kind: "Application",
      syncStatus: "Synced",
      healthStatus: "Healthy",
    },
  ],
  summary: {
    total: 1,
    synced: 1,
    outOfSync: 0,
    healthy: 1,
    degraded: 0,
    progressing: 0,
  },
};

const SECURITY_OK = {
  score: { overall: 92, grade: "A", scannedAt: new Date().toISOString() },
  totalIssues: 0,
  criticalCount: 0,
  highCount: 0,
  mediumCount: 1,
  lowCount: 3,
};

beforeEach(() => {
  vi.clearAllMocks();
  mockGetMultiClusterHealth.mockResolvedValue([HEALTHY_CLUSTER]);
  mockGetTimelineEvents.mockResolvedValue([WARNING_EVENT]);
  mockListResources.mockResolvedValue([UNHEALTHY_POD]);
  mockGetGitOpsStatus.mockResolvedValue(GITOPS_OK);
  mockGetSecuritySummary.mockResolvedValue(SECURITY_OK);
  mockTakeSnapshot.mockResolvedValue(undefined);
  // Reset onboarding store flags we care about.
  useOnboardingStore.setState({
    snapshotTaken: false,
    securityScanRun: false,
  } as any);
});

// ---------------------------------------------------------------------------
// Disconnected state
// ---------------------------------------------------------------------------

describe("Dashboard — disconnected state", () => {
  it("renders the 'Select cluster' CTA when not connected", () => {
    render(<Dashboard isConnected={false} />);
    expect(
      screen.getByRole("button", { name: /Select cluster/ })
    ).toBeDefined();
  });

  it("invokes onSelectCluster prop when the CTA is clicked", () => {
    const onSelectCluster = vi.fn();
    render(
      <Dashboard isConnected={false} onSelectCluster={onSelectCluster} />
    );
    fireEvent.click(screen.getByRole("button", { name: /Select cluster/ }));
    expect(onSelectCluster).toHaveBeenCalledTimes(1);
  });

  it("falls back to clicking the navbar cluster picker when onSelectCluster is not provided", () => {
    // Stage a fake navbar with the expected ARIA contract.
    document.body.innerHTML = `
      <header>
        <button aria-haspopup="listbox" data-testid="fake-navbar-cluster-btn"></button>
      </header>
    `;
    const fakeClick = vi.spyOn(HTMLButtonElement.prototype, "click");
    render(<Dashboard isConnected={false} />);
    fireEvent.click(screen.getByRole("button", { name: /Select cluster/ }));
    expect(fakeClick).toHaveBeenCalled();
    fakeClick.mockRestore();
    // Clean up the staged header for downstream tests.
    document.body.innerHTML = "";
  });
});

// ---------------------------------------------------------------------------
// Connected / populated state
// ---------------------------------------------------------------------------

describe("Dashboard — populated tiles", () => {
  it("renders Cluster Health with the seeded context name", async () => {
    render(<Dashboard isConnected />);
    await waitFor(() => {
      expect(screen.getByText("prod")).toBeDefined();
    });
  });

  it("renders Recent Warning Events with the seeded reason/message", async () => {
    render(<Dashboard isConnected />);
    await waitFor(() => {
      expect(
        screen.getByText(/BackOff: Container is crashing/)
      ).toBeDefined();
    });
  });

  it("renders Unhealthy Pods with restart count", async () => {
    render(<Dashboard isConnected />);
    await waitFor(() => {
      expect(screen.getByText("api-5d4")).toBeDefined();
    });
    expect(screen.getByText(/12x restarts/)).toBeDefined();
  });

  it("renders GitOps Sync Status when ArgoCD/Flux is detected", async () => {
    render(<Dashboard isConnected />);
    await waitFor(() => {
      expect(screen.getByText("checkout")).toBeDefined();
    });
    expect(screen.getByText(/1 synced/)).toBeDefined();
  });

  it("renders GitOps empty state when not detected", async () => {
    mockGetGitOpsStatus.mockResolvedValue({
      provider: "",
      detected: false,
      applications: [],
      summary: {
        total: 0, synced: 0, outOfSync: 0,
        healthy: 0, degraded: 0, progressing: 0,
      },
    });
    render(<Dashboard isConnected />);
    await waitFor(() => {
      expect(screen.getByText(/No ArgoCD or Flux detected/)).toBeDefined();
    });
  });
});

// ---------------------------------------------------------------------------
// Quick actions → onboarding side-effects
// ---------------------------------------------------------------------------

describe("Dashboard — quick action onboarding setters", () => {
  it("marks snapshot taken in the onboarding store on successful TakeSnapshot", async () => {
    render(<Dashboard isConnected />);
    // Wait for initial load to settle.
    await screen.findByText("prod");
    fireEvent.click(screen.getByRole("button", { name: /Take Snapshot/ }));
    await waitFor(() => {
      expect(mockTakeSnapshot).toHaveBeenCalled();
    });
    await waitFor(() => {
      expect(useOnboardingStore.getState().snapshotTaken).toBe(true);
    });
  });

  it("does NOT mark snapshot taken when TakeSnapshot rejects", async () => {
    mockTakeSnapshot.mockRejectedValue(new Error("io error"));
    render(<Dashboard isConnected />);
    await screen.findByText("prod");
    fireEvent.click(screen.getByRole("button", { name: /Take Snapshot/ }));
    await waitFor(() => {
      expect(mockTakeSnapshot).toHaveBeenCalled();
    });
    expect(useOnboardingStore.getState().snapshotTaken).toBe(false);
  });

  it("marks security scan run on successful Scan Now", async () => {
    render(<Dashboard isConnected />);
    await screen.findByText("prod");
    // Two scan entry points render; click the first one.
    const scanBtns = screen.getAllByRole("button", { name: /Scan Now|Run Security Scan/ });
    fireEvent.click(scanBtns[0]);
    await waitFor(() => {
      // At least one call during mount plus one explicit scan.
      expect(mockGetSecuritySummary.mock.calls.length).toBeGreaterThanOrEqual(2);
    });
    await waitFor(() => {
      expect(useOnboardingStore.getState().securityScanRun).toBe(true);
    });
  });

  it("navigates to AI query when 'New AI Query' is clicked", async () => {
    const onNavigate = vi.fn();
    render(<Dashboard isConnected onNavigate={onNavigate} />);
    await screen.findByText("prod");
    fireEvent.click(screen.getByRole("button", { name: /New AI Query/ }));
    expect(onNavigate).toHaveBeenCalledWith("query");
  });
});
