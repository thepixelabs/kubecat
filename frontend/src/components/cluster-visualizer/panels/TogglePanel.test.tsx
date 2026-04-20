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

  const renderTogglePanel = (
    props: Partial<{
      toggles: ToggleState;
      onToggle: (key: keyof ToggleState) => void;
    }> = {}
  ) => {
    const defaultProps = {
      toggles: defaultToggles,
      onToggle: vi.fn(),
    };

    return render(<TogglePanel {...defaultProps} {...props} />);
  };

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

      expect(screen.getByText("Resources")).toBeInTheDocument();
      expect(screen.getByText("Infrastructure & Logic")).toBeInTheDocument();
      expect(screen.getByText("Connections")).toBeInTheDocument();
    });
  });
});
