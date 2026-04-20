/**
 * ExplorerView — cluster resource explorer.
 *
 * The view is large (sorting, filtering, YAML viewer, AI analysis, terminal,
 * secrets). These tests focus on the stable skeleton: disconnected state,
 * initial pod fetch on mount, rendering fetched rows, and "no resources"
 * empty state. Deeper interactions live in higher-level integration tests.
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { ExplorerView } from "./ExplorerView";

const mockListResources = vi.fn();

vi.mock("../../../wailsjs/go/main/App", () => ({
  ListResources: (...a: unknown[]) => mockListResources(...a),
  GetResourceYAML: vi.fn(),
  GetSecretData: vi.fn(),
  GetNodeAllocation: vi.fn(() => Promise.resolve({})),
  StartTerminal: vi.fn(),
  CloseTerminal: vi.fn(),
}));

// Keep terminal + analysis subtrees cheap.
vi.mock("../Terminal/TerminalDrawer", () => ({
  TerminalDrawer: () => <div data-testid="terminal" />,
}));
vi.mock("../AnalysisModal", () => ({
  AnalysisModal: () => null,
}));

const baseProps = {
  isConnected: true,
  onSelectPod: vi.fn(),
  namespaces: ["default"],
  selectedKind: "pods",
  setSelectedKind: vi.fn(),
  namespaceFilter: "",
  setNamespaceFilter: vi.fn(),
  searchInput: "",
  setSearchInput: vi.fn(),
  contextMenuOpen: false,
  activeContext: "ctx",
} as any;

beforeEach(() => {
  vi.clearAllMocks();
  mockListResources.mockResolvedValue([
    {
      kind: "Pod",
      name: "api-5d4",
      namespace: "default",
      status: "Running",
      age: "1h",
      ready: "1/1",
      restarts: 0,
    },
  ]);
});

describe("ExplorerView", () => {
  it("shows the connect-prompt when not connected", () => {
    render(<ExplorerView {...baseProps} isConnected={false} />);
    expect(
      screen.getByText(/Connect to a cluster to explore resources/)
    ).toBeDefined();
  });

  it("calls ListResources on mount with the selected kind", async () => {
    render(<ExplorerView {...baseProps} />);
    await waitFor(() => {
      expect(mockListResources).toHaveBeenCalledWith("pods", "");
    });
  });

  it("renders returned rows", async () => {
    render(<ExplorerView {...baseProps} />);
    await waitFor(() => {
      expect(screen.getByText("api-5d4")).toBeDefined();
    });
  });

  it("shows 'No {kind} found' placeholder when list is empty", async () => {
    mockListResources.mockResolvedValue([]);
    render(<ExplorerView {...baseProps} />);
    await waitFor(() => {
      expect(screen.getByText(/No pods found/)).toBeDefined();
    });
  });
});
