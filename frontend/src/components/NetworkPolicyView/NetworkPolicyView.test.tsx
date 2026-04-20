/**
 * NetworkPolicyView — renders pod/service graph with NetworkPolicy overlays.
 *
 * We mock the heavy @xyflow/react component since Flow rendering requires a
 * layout engine; the unit we care about here is the control surface (header,
 * namespace selector, status strip) and the data-fetch side-effects.
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { NetworkPolicyView } from "./NetworkPolicyView";

const mockAnalyzeNetworkPolicies = vi.fn();

vi.mock("../../../wailsjs/go/main/App", () => ({
  AnalyzeNetworkPolicies: (...a: unknown[]) => mockAnalyzeNetworkPolicies(...a),
}));

// Shim xyflow to a cheap div so we don't require a layout engine during tests.
vi.mock("@xyflow/react", async () => {
  const actual = await vi.importActual<any>("@xyflow/react");
  return {
    ...actual,
    ReactFlow: ({ children }: any) => (
      <div data-testid="react-flow">{children}</div>
    ),
    Background: () => <div data-testid="bg" />,
    Controls: () => <div data-testid="ctrls" />,
    MiniMap: () => <div data-testid="minimap" />,
  };
});

const GRAPH_WITH_POLICIES = {
  nodes: [
    { id: "pod-a", name: "pod-a", namespace: "default", kind: "Pod", labels: {} },
    { id: "pod-b", name: "pod-b", namespace: "default", kind: "Pod", labels: {} },
  ],
  edges: [
    {
      id: "e1",
      source: "pod-a",
      target: "pod-b",
      allowed: true,
      direction: "ingress",
      policyName: "allow-a-to-b",
      ports: "tcp/80",
    },
  ],
  hasPolicies: true,
};

const EMPTY_GRAPH = {
  nodes: [],
  edges: [],
  hasPolicies: false,
};

beforeEach(() => {
  vi.clearAllMocks();
  mockAnalyzeNetworkPolicies.mockResolvedValue(GRAPH_WITH_POLICIES);
});

describe("NetworkPolicyView", () => {
  it("renders the header title and legend", async () => {
    render(
      <NetworkPolicyView activeCluster="ctx" namespaces={["default"]} />
    );
    expect(screen.getByText(/Network Policy Visualizer/)).toBeDefined();
    expect(screen.getByText(/Allowed/)).toBeDefined();
    expect(screen.getByText(/Blocked/)).toBeDefined();
  });

  it("fetches the graph for the default namespace on mount", async () => {
    render(
      <NetworkPolicyView activeCluster="ctx" namespaces={["default", "ns2"]} />
    );
    await waitFor(() => {
      expect(mockAnalyzeNetworkPolicies).toHaveBeenCalledWith("ctx", "default");
    });
  });

  it("shows policy count on the status strip when NetworkPolicies exist", async () => {
    render(<NetworkPolicyView activeCluster="ctx" namespaces={["default"]} />);
    await waitFor(() => {
      expect(
        screen.getByText(/1 allowed connections from NetworkPolicies/)
      ).toBeDefined();
    });
  });

  it("shows permissive-mode banner when no NetworkPolicies are present", async () => {
    mockAnalyzeNetworkPolicies.mockResolvedValue(EMPTY_GRAPH);
    render(<NetworkPolicyView activeCluster="ctx" namespaces={["default"]} />);
    await waitFor(() => {
      expect(
        screen.getByText(/No NetworkPolicies — all pods can communicate freely/)
      ).toBeDefined();
    });
  });

  it("re-fetches when namespace changes", async () => {
    render(
      <NetworkPolicyView activeCluster="ctx" namespaces={["default", "ns2"]} />
    );
    await waitFor(() => {
      expect(mockAnalyzeNetworkPolicies).toHaveBeenCalledTimes(1);
    });
    fireEvent.change(screen.getByRole("combobox"), { target: { value: "ns2" } });
    await waitFor(() => {
      expect(mockAnalyzeNetworkPolicies).toHaveBeenLastCalledWith("ctx", "ns2");
    });
  });

  it("surfaces errors", async () => {
    mockAnalyzeNetworkPolicies.mockRejectedValue(new Error("rbac forbidden"));
    render(<NetworkPolicyView activeCluster="ctx" namespaces={["default"]} />);
    await waitFor(() => {
      expect(screen.getByText(/rbac forbidden/)).toBeDefined();
    });
  });
});
