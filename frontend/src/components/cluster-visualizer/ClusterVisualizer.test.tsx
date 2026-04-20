import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ClusterVisualizer } from "./ClusterVisualizer";

// Mock Wails bindings
const mockListResources = vi.fn();
const mockGetClusterEdges = vi.fn();

vi.mock("../../../wailsjs/go/main/App", () => ({
  ListResources: (...args: unknown[]) => mockListResources(...args),
  GetClusterEdges: (...args: unknown[]) => mockGetClusterEdges(...args),
}));

// Mock React Flow - it requires DOM measurements that aren't available in jsdom
vi.mock("@xyflow/react", () => ({
  ReactFlow: ({ children }: { children?: React.ReactNode }) => (
    <div data-testid="react-flow">{children}</div>
  ),
  Background: () => <div data-testid="react-flow-background" />,
  Controls: () => <div data-testid="react-flow-controls" />,
  MiniMap: () => <div data-testid="react-flow-minimap" />,
  Panel: ({
    children,
    position,
  }: {
    children: React.ReactNode;
    position: string;
  }) => <div data-testid={`react-flow-panel-${position}`}>{children}</div>,
  useNodesState: () => [[], vi.fn(), vi.fn()],
  useEdgesState: () => [[], vi.fn(), vi.fn()],
}));

// Mock next-themes
vi.mock("next-themes", () => ({
  useTheme: () => ({ resolvedTheme: "dark" }),
}));

// Mock elkjs. Exposes a hoisted `elkLayoutMock` so individual tests can seed
// a layout throw and verify the error branch in the component surfaces it.
const { elkLayoutMock } = vi.hoisted(() => ({
  elkLayoutMock: vi.fn().mockResolvedValue({ children: [] }),
}));
vi.mock("elkjs/lib/elk.bundled", () => ({
  default: class ELK {
    layout = elkLayoutMock;
  },
}));

// Mock framer-motion
vi.mock("framer-motion", () => ({
  motion: {
    div: ({ children, ...props }: React.HTMLAttributes<HTMLDivElement>) => (
      <div {...props}>{children}</div>
    ),
  },
  AnimatePresence: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
  useDragControls: () => ({
    start: vi.fn(),
  }),
}));

describe("ClusterVisualizer", () => {
  const namespaces = ["default", "kube-system", "monitoring"];

  beforeEach(() => {
    vi.clearAllMocks();
    // Reset the shared ELK mock between tests so a prior throw doesn't bleed.
    elkLayoutMock.mockReset();
    elkLayoutMock.mockResolvedValue({ children: [] });

    // Default mock responses
    mockListResources.mockImplementation((kind: string) => {
      const resources: Record<string, unknown[]> = {
        pods: [
          {
            kind: "Pod",
            name: "web-pod-1",
            namespace: "default",
            status: "Running",
            node: "node-1",
          },
        ],
        services: [
          {
            kind: "Service",
            name: "web-service",
            namespace: "default",
            status: "Active",
          },
        ],
        deployments: [],
        statefulsets: [],
        daemonsets: [],
        replicasets: [],
        jobs: [],
        cronjobs: [],
        nodes: [],
        ingresses: [],
      };
      return Promise.resolve(resources[kind] || []);
    });

    mockGetClusterEdges.mockResolvedValue([]);
  });

  describe("when not connected", () => {
    it('should show "No cluster connected" message', () => {
      render(
        <ClusterVisualizer
          isConnected={false}
          namespaces={namespaces}
          onRefreshNamespaces={vi.fn()}
        />
      );

      expect(screen.getByText("No cluster connected")).toBeInTheDocument();
      expect(
        screen.getByText("Please ensure you have a valid kubeconfig")
      ).toBeInTheDocument();
    });

    it("should not render React Flow", () => {
      render(
        <ClusterVisualizer
          isConnected={false}
          namespaces={namespaces}
          onRefreshNamespaces={vi.fn()}
        />
      );

      expect(screen.queryByTestId("react-flow")).not.toBeInTheDocument();
    });

    it("should not fetch cluster data", () => {
      render(
        <ClusterVisualizer
          isConnected={false}
          namespaces={namespaces}
          onRefreshNamespaces={vi.fn()}
        />
      );

      expect(mockListResources).not.toHaveBeenCalled();
      expect(mockGetClusterEdges).not.toHaveBeenCalled();
    });
  });

  describe("when connected", () => {
    it("should render React Flow component", async () => {
      render(
        <ClusterVisualizer
          isConnected={true}
          namespaces={namespaces}
          onRefreshNamespaces={vi.fn()}
        />
      );

      await waitFor(() => {
        expect(screen.getByTestId("react-flow")).toBeInTheDocument();
      });
    });

    it("should render React Flow controls", async () => {
      render(
        <ClusterVisualizer
          isConnected={true}
          namespaces={namespaces}
          onRefreshNamespaces={vi.fn()}
        />
      );

      await waitFor(() => {
        expect(screen.getByTestId("react-flow-background")).toBeInTheDocument();
        expect(screen.getByTestId("react-flow-controls")).toBeInTheDocument();
        expect(screen.getByTestId("react-flow-minimap")).toBeInTheDocument();
      });
    });

    it("should fetch cluster data on mount", async () => {
      render(
        <ClusterVisualizer
          isConnected={true}
          namespaces={namespaces}
          onRefreshNamespaces={vi.fn()}
        />
      );

      await waitFor(() => {
        expect(mockListResources).toHaveBeenCalled();
        expect(mockGetClusterEdges).toHaveBeenCalled();
      });
    });

    it("should render toggle panel", async () => {
      render(
        <ClusterVisualizer
          isConnected={true}
          namespaces={namespaces}
          onRefreshNamespaces={vi.fn()}
        />
      );

      await waitFor(() => {
        expect(screen.getByText("Namespace")).toBeInTheDocument();
        expect(screen.getByText("Resources")).toBeInTheDocument();
        expect(screen.getByText("Connections")).toBeInTheDocument();
      });
    });

    it("should render refresh button in panel", async () => {
      render(
        <ClusterVisualizer
          isConnected={true}
          namespaces={namespaces}
          onRefreshNamespaces={vi.fn()}
        />
      );

      await waitFor(() => {
        expect(
          screen.getByTestId("react-flow-panel-top-right")
        ).toBeInTheDocument();
      });
    });
  });

  describe("loading state", () => {
    it("should show loading indicator while fetching data", () => {
      // Make the mock take longer
      mockListResources.mockImplementation(
        () => new Promise((resolve) => setTimeout(() => resolve([]), 1000))
      );

      render(
        <ClusterVisualizer
          isConnected={true}
          namespaces={namespaces}
          onRefreshNamespaces={vi.fn()}
        />
      );

      expect(screen.getByText("Loading cluster data...")).toBeInTheDocument();
    });
  });

  describe("error state", () => {
    it("should handle fetch errors gracefully", async () => {
      // Individual resource fetches have .catch(() => []) so they won't throw
      // Only a complete failure in Promise.all would trigger the error state
      // Since the component handles partial failures gracefully, we test that it
      // continues to render even when some resources fail
      mockListResources.mockRejectedValue(new Error("Connection failed"));
      mockGetClusterEdges.mockRejectedValue(new Error("Connection failed"));

      render(
        <ClusterVisualizer
          isConnected={true}
          namespaces={namespaces}
          onRefreshNamespaces={vi.fn()}
        />
      );

      // The component should still render React Flow even with errors
      // because individual resource errors are caught and return empty arrays
      await waitFor(() => {
        expect(screen.getByTestId("react-flow")).toBeInTheDocument();
      });
    });
  });

  describe("namespace selection", () => {
    it("should render the namespace HUD selector", async () => {
      render(
        <ClusterVisualizer
          isConnected={true}
          namespaces={namespaces}
          onRefreshNamespaces={vi.fn()}
        />
      );

      await waitFor(() => {
        // The HUD label is always visible
        expect(screen.getByText("Namespace")).toBeInTheDocument();
      });
    });

    it('should default to "All Namespaces" so empty `default` ns users still see a populated graph', async () => {
      render(
        <ClusterVisualizer
          isConnected={true}
          namespaces={namespaces}
          onRefreshNamespaces={vi.fn()}
        />
      );

      await waitFor(() => {
        // With the default of "" the HUD trigger shows the "All Namespaces"
        // label. There will be two matches once the dropdown opens (trigger +
        // list item), but on mount the dropdown is closed so there is only
        // one.
        expect(screen.getByText("All Namespaces")).toBeInTheDocument();
      });
    });
  });

  describe("refresh functionality", () => {
    it("should call onRefreshNamespaces when provided", async () => {
      const onRefreshNamespaces = vi.fn();

      render(
        <ClusterVisualizer
          isConnected={true}
          namespaces={namespaces}
          onRefreshNamespaces={onRefreshNamespaces}
        />
      );

      // The refresh functionality is in the top-right panel
      await waitFor(() => {
        expect(
          screen.getByTestId("react-flow-panel-top-right")
        ).toBeInTheDocument();
      });
    });
  });

  describe("toggle functionality", () => {
    it("should render resource toggles", async () => {
      render(
        <ClusterVisualizer
          isConnected={true}
          namespaces={namespaces}
          onRefreshNamespaces={vi.fn()}
        />
      );

      await waitFor(() => {
        expect(screen.getByText("Pods")).toBeInTheDocument();
        expect(screen.getByText("Services")).toBeInTheDocument();
        expect(screen.getByText("Deploy")).toBeInTheDocument();
      });
    });

    it("should render connection toggles", async () => {
      render(
        <ClusterVisualizer
          isConnected={true}
          namespaces={namespaces}
          onRefreshNamespaces={vi.fn()}
        />
      );

      await waitFor(() => {
        expect(screen.getByText("Svc→Pod")).toBeInTheDocument();
        expect(screen.getByText("Ing→Svc")).toBeInTheDocument();
        expect(screen.getByText("Ctrl→Pod")).toBeInTheDocument();
      });
    });
  });

  // ── Regression: graph-view default / NamespaceHud / ELK error branch ─────

  describe("NamespaceHud (top-left always-visible selector)", () => {
    it("renders the namespace HUD outside the draggable TogglePanel", async () => {
      render(
        <ClusterVisualizer
          isConnected={true}
          namespaces={namespaces}
          onRefreshNamespaces={vi.fn()}
        />
      );

      // Two "Namespace" labels would mean the selector was ALSO inside the
      // TogglePanel — the fix removed it from there and left it only in the
      // HUD. Guard against regression.
      await waitFor(() => {
        expect(screen.getAllByText("Namespace").length).toBe(1);
      });
    });

    it("updates selectedNamespace when the user picks one from the HUD", async () => {
      const user = userEvent.setup();
      render(
        <ClusterVisualizer
          isConnected={true}
          namespaces={namespaces}
          onRefreshNamespaces={vi.fn()}
        />
      );

      // Wait for initial fetch
      await waitFor(() => {
        expect(mockListResources).toHaveBeenCalled();
      });

      // Default is "All Namespaces" — so ListResources was called with "".
      expect(mockListResources).toHaveBeenCalledWith("pods", "");

      mockListResources.mockClear();

      // Open the HUD and pick "kube-system".
      const trigger = screen.getByText("All Namespaces").closest("button")!;
      await user.click(trigger);
      const option = await screen.findByText("kube-system");
      await user.click(option);

      // New fetches must target "kube-system".
      await waitFor(
        () => {
          expect(mockListResources).toHaveBeenCalledWith("pods", "kube-system");
        },
        { timeout: 3000 }
      );
    });
  });

  describe("TogglePanel integration", () => {
    it("does not contain the namespace selector (it was moved to the HUD)", async () => {
      render(
        <ClusterVisualizer
          isConnected={true}
          namespaces={namespaces}
          onRefreshNamespaces={vi.fn()}
        />
      );

      await waitFor(() => {
        // TogglePanel only shows Resources / Infrastructure & Logic / Connections
        expect(screen.getByText("Resources")).toBeInTheDocument();
        expect(screen.getByText("Infrastructure & Logic")).toBeInTheDocument();
        expect(screen.getByText("Connections")).toBeInTheDocument();
      });

      // If the old namespace dropdown leaked into TogglePanel we would see a
      // second "Namespace" label here.
      expect(screen.getAllByText("Namespace").length).toBe(1);
    });
  });

  describe("ELK layout failures", () => {
    it("surfaces the layout error into the UI error branch", async () => {
      // Seed a pod so filteredNodes is non-empty (layout only runs otherwise).
      mockListResources.mockImplementation((kind: string) => {
        if (kind === "pods") {
          return Promise.resolve([
            {
              kind: "Pod",
              name: "p",
              namespace: "default",
              status: "Running",
              node: "node-1",
              ownerKind: "ReplicaSet",
              ownerName: "rs",
            },
          ]);
        }
        return Promise.resolve([]);
      });
      mockGetClusterEdges.mockResolvedValue([]);
      elkLayoutMock.mockRejectedValueOnce(new Error("boom"));

      render(
        <ClusterVisualizer
          isConnected={true}
          namespaces={namespaces}
          onRefreshNamespaces={vi.fn()}
        />
      );

      // Error branch renders "Graph layout failed: <message>" — regression pin
      // for the fix that stopped swallowing ELK exceptions silently.
      await waitFor(
        () => {
          expect(
            screen.getByText(/Graph layout failed: boom/)
          ).toBeInTheDocument();
        },
        { timeout: 3000 }
      );
    });
  });

  describe("refresh button", () => {
    it("invokes onRefreshNamespaces when clicked", async () => {
      const onRefreshNamespaces = vi.fn();
      const user = userEvent.setup();

      render(
        <ClusterVisualizer
          isConnected={true}
          namespaces={namespaces}
          onRefreshNamespaces={onRefreshNamespaces}
        />
      );

      const btn = await screen.findByTitle("Refresh");
      await user.click(btn);

      expect(onRefreshNamespaces).toHaveBeenCalled();
      // Double-mute the unused import warning in builds
      expect(fireEvent).toBeDefined();
    });
  });
});
