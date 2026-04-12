import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { TogglePanel } from "./TogglePanel";
import type { ToggleState } from "../types";

// Mock framer-motion to avoid animation issues in tests
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

describe("TogglePanel", () => {
  const defaultToggles: ToggleState = {
    showPods: true,
    showServices: true,
    showIngresses: true,
    showDeployments: true,
    showStatefulSets: true,
    showDaemonSets: true,
    showServiceToPod: true,
    showIngressToService: true,
    showControllerToPod: true,
    showNodes: true,
    showReplicaSets: false,
    showJobs: false,
    showCronJobs: false,
    showOperators: true,
    showNodeToPod: true,
    showOperatorToManaged: true,
  };

  const namespaces = ["default", "kube-system", "monitoring"];

  const renderTogglePanel = (
    props: Partial<{
      toggles: ToggleState;
      onToggle: (key: keyof ToggleState) => void;
      namespaces: string[];
      selectedNamespace: string;
      onNamespaceChange: (ns: string) => void;
    }> = {}
  ) => {
    const defaultProps = {
      toggles: defaultToggles,
      onToggle: vi.fn(),
      namespaces,
      selectedNamespace: "default",
      onNamespaceChange: vi.fn(),
    };

    return render(<TogglePanel {...defaultProps} {...props} />);
  };

  describe("namespace selector", () => {
    it("should render namespace dropdown with options", () => {
      renderTogglePanel();

      // Find the dropdown trigger button
      const dropdownButton = screen.getByText("default");
      expect(dropdownButton).toBeInTheDocument();

      // Open the dropdown
      fireEvent.click(dropdownButton);

      // Should have "All Namespaces" option plus our namespaces
      expect(screen.getByText("All Namespaces")).toBeInTheDocument();
      expect(screen.getAllByText("default").length).toBeGreaterThan(0); // One in button, one in list
      expect(screen.getByText("kube-system")).toBeInTheDocument();
      expect(screen.getByText("monitoring")).toBeInTheDocument();
    });

    it("should show selected namespace as value", () => {
      renderTogglePanel({ selectedNamespace: "kube-system" });

      expect(screen.getByText("kube-system")).toBeInTheDocument();
    });

    it("should call onNamespaceChange when selection changes", () => {
      const onNamespaceChange = vi.fn();
      renderTogglePanel({ onNamespaceChange });

      // Open dropdown
      const dropdownButton = screen.getByText("default");
      fireEvent.click(dropdownButton);

      // Click option
      const monitoringOption = screen.getByText("monitoring");
      fireEvent.click(monitoringOption);

      expect(onNamespaceChange).toHaveBeenCalledWith("monitoring");
    });

    it('should allow selecting "All Namespaces" (empty value)', () => {
      const onNamespaceChange = vi.fn();
      renderTogglePanel({ onNamespaceChange });

      // Open dropdown
      const dropdownButton = screen.getByText("default");
      fireEvent.click(dropdownButton);

      // Click "All Namespaces" option
      // The first one is likely the "All Namespaces"
      const allNamespacesOption = screen.getByText("All Namespaces");
      fireEvent.click(allNamespacesOption);

      expect(onNamespaceChange).toHaveBeenCalledWith("");
    });
  });

  describe("resource toggles", () => {
    it("should render all resource toggle buttons", () => {
      renderTogglePanel();

      expect(screen.getByText("Pods")).toBeInTheDocument();
      expect(screen.getByText("Services")).toBeInTheDocument();
      expect(screen.getByText("Ingress")).toBeInTheDocument();
      expect(screen.getByText("Deploy")).toBeInTheDocument();
      expect(screen.getByText("STS")).toBeInTheDocument(); // StatefulSets
      expect(screen.getByText("DS")).toBeInTheDocument(); // DaemonSets
    });

    it("should call onToggle when a resource toggle is clicked", () => {
      const onToggle = vi.fn();
      renderTogglePanel({ onToggle });

      const podsButton = screen.getByText("Pods").closest("button");
      fireEvent.click(podsButton!);

      expect(onToggle).toHaveBeenCalledWith("showPods");
    });

    it("should call onToggle with correct key for Services", () => {
      const onToggle = vi.fn();
      renderTogglePanel({ onToggle });

      const servicesButton = screen.getByText("Services").closest("button");
      fireEvent.click(servicesButton!);

      expect(onToggle).toHaveBeenCalledWith("showServices");
    });

    it("should call onToggle with correct key for Ingress", () => {
      const onToggle = vi.fn();
      renderTogglePanel({ onToggle });

      const ingressButton = screen.getByText("Ingress").closest("button");
      fireEvent.click(ingressButton!);

      expect(onToggle).toHaveBeenCalledWith("showIngresses");
    });
  });

  describe("infrastructure toggles", () => {
    it("should render infrastructure toggle buttons", () => {
      renderTogglePanel();

      expect(screen.getByText("Nodes")).toBeInTheDocument();
      expect(screen.getByText("Operators")).toBeInTheDocument();
      expect(screen.getByText("RS")).toBeInTheDocument(); // ReplicaSets
      expect(screen.getByText("Jobs")).toBeInTheDocument();
      expect(screen.getByText("CronJobs")).toBeInTheDocument();
    });

    it("should call onToggle for Nodes", () => {
      const onToggle = vi.fn();
      renderTogglePanel({ onToggle });

      const nodesButton = screen.getByText("Nodes").closest("button");
      fireEvent.click(nodesButton!);

      expect(onToggle).toHaveBeenCalledWith("showNodes");
    });

    it("should call onToggle for Operators", () => {
      const onToggle = vi.fn();
      renderTogglePanel({ onToggle });

      const operatorsButton = screen.getByText("Operators").closest("button");
      fireEvent.click(operatorsButton!);

      expect(onToggle).toHaveBeenCalledWith("showOperators");
    });
  });

  describe("connection toggles", () => {
    it("should render connection toggle buttons", () => {
      renderTogglePanel();

      expect(screen.getByText("Svc→Pod")).toBeInTheDocument();
      expect(screen.getByText("Ing→Svc")).toBeInTheDocument();
      expect(screen.getByText("Ctrl→Pod")).toBeInTheDocument();
      expect(screen.getByText("Node→Pod")).toBeInTheDocument();
    });

    it("should call onToggle for Service to Pod connections", () => {
      const onToggle = vi.fn();
      renderTogglePanel({ onToggle });

      const svcToPodButton = screen.getByText("Svc→Pod").closest("button");
      fireEvent.click(svcToPodButton!);

      expect(onToggle).toHaveBeenCalledWith("showServiceToPod");
    });

    it("should call onToggle for Ingress to Service connections", () => {
      const onToggle = vi.fn();
      renderTogglePanel({ onToggle });

      const ingToSvcButton = screen.getByText("Ing→Svc").closest("button");
      fireEvent.click(ingToSvcButton!);

      expect(onToggle).toHaveBeenCalledWith("showIngressToService");
    });

    it("should call onToggle for Controller to Pod connections", () => {
      const onToggle = vi.fn();
      renderTogglePanel({ onToggle });

      const ctrlToPodButton = screen.getByText("Ctrl→Pod").closest("button");
      fireEvent.click(ctrlToPodButton!);

      expect(onToggle).toHaveBeenCalledWith("showControllerToPod");
    });
  });

  describe("visual state", () => {
    it("should visually indicate active toggles", () => {
      const toggles = { ...defaultToggles, showPods: true };
      renderTogglePanel({ toggles });

      const podsButton = screen.getByText("Pods").closest("button");
      // Active buttons should have the active class
      expect(podsButton?.className).toContain("bg-slate-700");
    });

    it("should visually indicate inactive toggles", () => {
      const toggles = { ...defaultToggles, showReplicaSets: false };
      renderTogglePanel({ toggles });

      const rsButton = screen.getByText("RS").closest("button");
      // Inactive buttons should have the inactive styling
      expect(rsButton?.className).toContain("text-slate-500");
    });
  });

  describe("section labels", () => {
    it("should render section headers", () => {
      renderTogglePanel();

      expect(screen.getByText("Namespace")).toBeInTheDocument();
      expect(screen.getByText("Resources")).toBeInTheDocument();
      expect(screen.getByText("Infrastructure & Logic")).toBeInTheDocument();
      expect(screen.getByText("Connections")).toBeInTheDocument();
    });
  });
});
