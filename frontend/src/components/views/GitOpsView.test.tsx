/**
 * GitOpsView — ArgoCD/Flux resource browser with tabs, namespace filter,
 * sortable table, and keyboard shortcuts.
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { GitOpsView } from "./GitOpsView";

const mockListResources = vi.fn();

vi.mock("../../../wailsjs/go/main/App", () => ({
  ListResources: (...a: unknown[]) => mockListResources(...a),
}));

const RESOURCES = {
  applications: [] as any[],
  helmreleases: [
    { name: "release-a", namespace: "default", status: "Ready", age: "1h" },
    { name: "release-b", namespace: "kube-system", status: "Failed", age: "2h" },
  ],
  kustomizations: [
    { name: "kust-x", namespace: "default", status: "Ready", age: "10m" },
  ],
  namespaces: [{ name: "default" }, { name: "kube-system" }],
};

beforeEach(() => {
  vi.clearAllMocks();
  mockListResources.mockImplementation((kind: string) => {
    // @ts-expect-error indexing by string
    return Promise.resolve(RESOURCES[kind] ?? []);
  });
});

describe("GitOpsView", () => {
  it("shows the connect-prompt when disconnected", () => {
    render(<GitOpsView isConnected={false} />);
    expect(
      screen.getByText(/Connect to a cluster to view GitOps resources/)
    ).toBeDefined();
  });

  it("renders the Helm Releases tab as the default and shows populated rows", async () => {
    render(<GitOpsView isConnected />);
    await waitFor(() => {
      expect(screen.getByText("release-a")).toBeDefined();
    });
    // All three helm release rows: 2 entries shown with their names.
    expect(screen.getByText("release-b")).toBeDefined();
  });

  it("switches tabs when ArgoCD Applications / Kustomizations are clicked", async () => {
    render(<GitOpsView isConnected />);
    await screen.findByText("release-a");
    fireEvent.click(screen.getByRole("button", { name: /ArgoCD Applications/ }));
    await waitFor(() => {
      expect(screen.getByText(/No ArgoCD applications found/)).toBeDefined();
    });
    fireEvent.click(screen.getByRole("button", { name: /Kustomizations/ }));
    await waitFor(() => {
      expect(screen.getByText("kust-x")).toBeDefined();
    });
  });

  it("shows the namespace dropdown and filters by selected namespace", async () => {
    render(<GitOpsView isConnected />);
    await screen.findByText("release-a");
    fireEvent.click(screen.getByText("All Namespaces"));
    // Select 'kube-system'. There could be multiple "kube-system" matches
    // (once in table, once in dropdown). Find buttons only.
    const ksBtn = screen.getAllByRole("button", { name: "kube-system" })[0];
    fireEvent.click(ksBtn);
    await waitFor(() => {
      // The filter triggers a re-fetch of resources with the chosen namespace
      // — we assert ListResources was called with ns=kube-system at least once.
      const calls = mockListResources.mock.calls.filter(
        (c: unknown[]) => c[1] === "kube-system"
      );
      expect(calls.length).toBeGreaterThan(0);
    });
  });
});
