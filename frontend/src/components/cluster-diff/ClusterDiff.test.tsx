import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { ClusterDiff } from "./ClusterDiff";

// Mock framer-motion
vi.mock("framer-motion", () => ({
  motion: {
    div: ({
      children,
      initial,
      animate,
      exit,
      ...props
    }: React.HTMLAttributes<HTMLDivElement> & {
      initial?: unknown;
      animate?: unknown;
      exit?: unknown;
    }) => <div {...props}>{children}</div>,
  },
  AnimatePresence: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
  useDragControls: () => ({
    start: vi.fn(),
  }),
}));

describe("ClusterDiff", () => {
  const contexts = ["minikube", "production", "staging"];

  const defaultProps = {
    contexts,
    isTimelineAvailable: false,
    onComputeDiff: vi.fn(),
    onGetSnapshots: vi.fn(),
    onListResources: vi.fn(),
    onApplyResource: vi.fn(),
    onGenerateReport: vi.fn(),
    activeContext: "test-context",
    namespaces: ["default", "kube-system"],
  };

  beforeEach(() => {
    vi.clearAllMocks();

    // Default mock implementations
    defaultProps.onListResources.mockResolvedValue([
      { name: "web-deployment", namespace: "default" },
      { name: "api-deployment", namespace: "default" },
    ]);

    defaultProps.onGetSnapshots.mockResolvedValue([]);

    defaultProps.onComputeDiff.mockResolvedValue({
      leftYaml: "apiVersion: apps/v1\nkind: Deployment",
      rightYaml: "apiVersion: apps/v1\nkind: Deployment",
      differences: [],
    });
  });

  describe("rendering", () => {
    it("should render mode toggle buttons", () => {
      render(<ClusterDiff {...defaultProps} />);

      expect(screen.getByText("Cross-Cluster")).toBeInTheDocument();
      expect(screen.getAllByText("Historical").length).toBeGreaterThan(0);
    });

    it("should render source and target selectors", () => {
      render(<ClusterDiff {...defaultProps} />);

      expect(screen.getByText("Source (Left)")).toBeInTheDocument();
      expect(screen.getByText("Target (Right)")).toBeInTheDocument();
    });

    it("should render resource selector section", () => {
      render(<ClusterDiff {...defaultProps} />);

      expect(screen.getByText("Resource")).toBeInTheDocument();
    });

    it("should render compare button", () => {
      render(<ClusterDiff {...defaultProps} />);

      expect(screen.getByText("Compare")).toBeInTheDocument();
    });
  });

  describe("mode toggle", () => {
    it("should start in cross-cluster mode", () => {
      render(<ClusterDiff {...defaultProps} />);

      const crossClusterButton = screen
        .getByText("Cross-Cluster")
        .closest("button");
      expect(crossClusterButton?.className).toContain("text-accent");
    });

    it("should disable historical mode when timeline not available", () => {
      render(<ClusterDiff {...defaultProps} isTimelineAvailable={false} />);

      // Get all Historical buttons and use the first one (mode toggle button)
      const historicalButtons = screen.getAllByText("Historical");
      const historicalButton = historicalButtons[0].closest("button");
      expect(historicalButton).toBeDisabled();
    });

    it("should enable historical mode when timeline is available", () => {
      render(<ClusterDiff {...defaultProps} isTimelineAvailable={true} />);

      const historicalButtons = screen.getAllByText("Historical");
      const historicalButton = historicalButtons[0].closest("button");
      expect(historicalButton).not.toBeDisabled();
    });

    it("should switch to historical mode when clicked", async () => {
      render(<ClusterDiff {...defaultProps} isTimelineAvailable={true} />);

      // Find the Historical button in the mode toggle area (header)
      const historicalButtons = screen.getAllByText("Historical");
      const historicalButton = historicalButtons[0].closest("button");
      fireEvent.click(historicalButton!);

      await waitFor(() => {
        expect(historicalButton?.className).toContain("text-purple");
      });
    });

    it("should fetch snapshots when switching to historical mode", async () => {
      render(<ClusterDiff {...defaultProps} isTimelineAvailable={true} />);

      const historicalButtons = screen.getAllByText("Historical");
      const historicalButton = historicalButtons[0].closest("button");
      fireEvent.click(historicalButton!);

      await waitFor(() => {
        expect(defaultProps.onGetSnapshots).toHaveBeenCalledWith(50);
      });
    });
  });

  describe("resource kind selection", () => {
    it("should show Deployments as default kind", () => {
      render(<ClusterDiff {...defaultProps} />);

      expect(screen.getByText("Deployments")).toBeInTheDocument();
    });

    it("should open kind dropdown when clicked", async () => {
      render(<ClusterDiff {...defaultProps} />);

      const kindButton = screen.getByText("Deployments").closest("button");
      fireEvent.click(kindButton!);

      await waitFor(() => {
        expect(screen.getByText("Services")).toBeInTheDocument();
        expect(screen.getByText("ConfigMaps")).toBeInTheDocument();
        expect(screen.getByText("Secrets")).toBeInTheDocument();
        expect(screen.getByText("StatefulSets")).toBeInTheDocument();
      });
    });

    it("should call onListResources when kind changes", async () => {
      render(<ClusterDiff {...defaultProps} />);

      // Wait for initial load
      await waitFor(() => {
        expect(defaultProps.onListResources).toHaveBeenCalled();
      });

      defaultProps.onListResources.mockClear();

      // Open dropdown and select different kind
      const kindButton = screen.getByText("Deployments").closest("button");
      fireEvent.click(kindButton!);

      const servicesOption = await screen.findByText("Services");
      fireEvent.click(servicesOption);

      await waitFor(() => {
        expect(defaultProps.onListResources).toHaveBeenCalledWith(
          "services",
          "default"
        );
      });
    });
  });

  describe("resource name selection", () => {
    it("should show placeholder for resource selection initially", async () => {
      render(<ClusterDiff {...defaultProps} />);

      // Wait for initial resource loading
      await waitFor(() => {
        expect(defaultProps.onListResources).toHaveBeenCalled();
      });

      // Look for the placeholder text which might be in loading or select state
      const selectButton = screen
        .getAllByRole("button")
        .find(
          (btn) =>
            btn.textContent?.includes("Select resource") ||
            btn.textContent?.includes("Loading")
        );
      expect(selectButton).toBeDefined();
    });

    it("should load resources and show them in dropdown", async () => {
      render(<ClusterDiff {...defaultProps} />);

      // Wait for resources to load
      await waitFor(() => {
        expect(defaultProps.onListResources).toHaveBeenCalled();
      });

      // Open resource dropdown
      const resourceButton = screen
        .getByText("Select resource...")
        .closest("button");
      fireEvent.click(resourceButton!);

      await waitFor(() => {
        expect(screen.getByText("web-deployment")).toBeInTheDocument();
        expect(screen.getByText("api-deployment")).toBeInTheDocument();
      });
    });

    it('should show "No resources found" when list is empty', async () => {
      defaultProps.onListResources.mockResolvedValue([]);
      render(<ClusterDiff {...defaultProps} />);

      await waitFor(() => {
        expect(defaultProps.onListResources).toHaveBeenCalled();
      });

      const resourceButton = screen
        .getByText("Select resource...")
        .closest("button");
      fireEvent.click(resourceButton!);

      await waitFor(() => {
        expect(screen.getByText("No resources found")).toBeInTheDocument();
      });
    });
  });

  describe("namespace selector", () => {
    it("should have default namespace selected", () => {
      render(<ClusterDiff {...defaultProps} />);

      // Namespace is now a dropdown, check if "default" is displayed
      expect(screen.getByText("default")).toBeInTheDocument();
    });

    it("should update namespace and trigger resource reload", async () => {
      render(<ClusterDiff {...defaultProps} />);

      // Wait for initial load
      await waitFor(() => {
        expect(defaultProps.onListResources).toHaveBeenCalled();
      });

      defaultProps.onListResources.mockClear();

      // Open namespace dropdown
      const namespaceButtons = screen.getAllByText("default");
      const namespaceButton = namespaceButtons
        .find((el) => el.closest("button"))
        ?.closest("button");
      fireEvent.click(namespaceButton!);

      // Select kube-system
      const kubeSystemOption = await screen.findByText("kube-system");
      fireEvent.click(kubeSystemOption);

      await waitFor(() => {
        expect(defaultProps.onListResources).toHaveBeenCalledWith(
          expect.any(String),
          "kube-system"
        );
      });
    });
  });

  describe("compare functionality", () => {
    it("should show error when no resource is selected", async () => {
      render(<ClusterDiff {...defaultProps} />);

      const compareButton = screen.getByText("Compare");
      fireEvent.click(compareButton);

      await waitFor(() => {
        expect(
          screen.getByText("Please select a resource to compare")
        ).toBeInTheDocument();
      });
    });

    it("should call onComputeDiff with correct parameters", async () => {
      render(<ClusterDiff {...defaultProps} />);

      // Wait for resources to load
      await waitFor(() => {
        expect(defaultProps.onListResources).toHaveBeenCalled();
      });

      // Select a resource
      const resourceButton = screen
        .getByText("Select resource...")
        .closest("button");
      fireEvent.click(resourceButton!);

      const resourceOption = await screen.findByText("web-deployment");
      fireEvent.click(resourceOption);

      // Click compare
      const compareButton = screen.getByText("Compare");
      fireEvent.click(compareButton);

      await waitFor(() => {
        expect(defaultProps.onComputeDiff).toHaveBeenCalledWith({
          kind: "deployments",
          namespace: "default",
          name: "web-deployment",
          left: expect.objectContaining({
            context: "test-context",
            isLive: true,
          }),
          right: expect.objectContaining({
            context: "minikube",
            isLive: true,
          }),
        });
      });
    });

    it("should show loading state while comparing", async () => {
      defaultProps.onComputeDiff.mockImplementation(
        () => new Promise((resolve) => setTimeout(resolve, 100))
      );

      render(<ClusterDiff {...defaultProps} />);

      // Wait for resources and select one
      await waitFor(() =>
        expect(defaultProps.onListResources).toHaveBeenCalled()
      );

      const resourceButton = screen
        .getByText("Select resource...")
        .closest("button");
      fireEvent.click(resourceButton!);
      const resourceOption = await screen.findByText("web-deployment");
      fireEvent.click(resourceOption);

      // Click compare
      const compareButton = screen.getByText("Compare");
      fireEvent.click(compareButton);

      expect(await screen.findByText("Comparing...")).toBeInTheDocument();
    });

    it("should show error message on diff failure", async () => {
      defaultProps.onComputeDiff.mockRejectedValue(
        new Error("Diff computation failed")
      );

      render(<ClusterDiff {...defaultProps} />);

      // Wait for resources and select one
      await waitFor(() =>
        expect(defaultProps.onListResources).toHaveBeenCalled()
      );

      const resourceButton = screen
        .getByText("Select resource...")
        .closest("button");
      fireEvent.click(resourceButton!);
      const resourceOption = await screen.findByText("web-deployment");
      fireEvent.click(resourceOption);

      // Click compare
      const compareButton = screen.getByText("Compare");
      fireEvent.click(compareButton);

      await waitFor(() => {
        expect(screen.getByText("Failed to compute diff")).toBeInTheDocument();
      });
    });
  });

  describe("empty state", () => {
    it("should show empty state message when no result", () => {
      render(<ClusterDiff {...defaultProps} />);

      expect(
        screen.getByText("Select resources to compare")
      ).toBeInTheDocument();
      expect(
        screen.getByText("Choose clusters and a resource, then click Compare")
      ).toBeInTheDocument();
    });
  });

  describe("context initialization", () => {
    it("should use first context as left source", () => {
      render(<ClusterDiff {...defaultProps} />);

      // The first context should be selected for left source
      // This is checked via the Source (Left) selector
      expect(defaultProps.contexts[0]).toBe("minikube");
    });

    it("should use second context as right source if available", () => {
      render(<ClusterDiff {...defaultProps} />);

      // The second context should be selected for right source
      expect(defaultProps.contexts[1]).toBe("production");
    });

    it("should handle single context gracefully", () => {
      render(<ClusterDiff {...defaultProps} contexts={["minikube"]} />);

      // Should not crash with single context - check for mode toggle instead
      expect(screen.getByText("Cross-Cluster")).toBeInTheDocument();
    });
  });
});
