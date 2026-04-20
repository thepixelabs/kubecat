/**
 * RBACView — namespace RBAC matrix with expandable subject rows.
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { RBACView } from "./RBACView";

const mockGetNamespaceRBAC = vi.fn();

vi.mock("../../../wailsjs/go/main/App", () => ({
  GetNamespaceRBAC: (...a: unknown[]) => mockGetNamespaceRBAC(...a),
}));

const MATRIX = {
  namespace: "default",
  subjects: [
    {
      subject: "alice@example.com",
      kind: "User",
      bindings: [
        {
          name: "alice-binding",
          kind: "RoleBinding",
          roleName: "admin",
          roleKind: "ClusterRole",
          namespace: "default",
          clusterWide: false,
        },
      ],
      rules: [
        {
          verbs: ["get", "list"],
          resources: ["pods"],
          apiGroups: [""],
          isWildcard: false,
        },
      ],
    },
    {
      subject: "system:serviceaccount:default:builder",
      kind: "ServiceAccount",
      bindings: [],
      rules: [
        {
          verbs: ["*"],
          resources: ["*"],
          apiGroups: ["*"],
          isWildcard: true,
        },
      ],
    },
  ],
};

beforeEach(() => {
  vi.clearAllMocks();
  mockGetNamespaceRBAC.mockResolvedValue(MATRIX);
});

describe("RBACView", () => {
  it("fetches RBAC on mount and renders subjects", async () => {
    render(
      <RBACView isConnected activeContext="ctx" namespaces={["default"]} />
    );
    await waitFor(() => {
      expect(mockGetNamespaceRBAC).toHaveBeenCalledWith("ctx", "default");
    });
    expect(await screen.findByText("alice@example.com")).toBeDefined();
    expect(
      screen.getByText("system:serviceaccount:default:builder")
    ).toBeDefined();
  });

  it("highlights wildcard (cluster-admin) rules in red", async () => {
    render(
      <RBACView isConnected activeContext="ctx" namespaces={["default"]} />
    );
    expect(await screen.findByText(/cluster-admin \/ \*/)).toBeDefined();
  });

  it("expands a row to show bindings and rules on click", async () => {
    render(
      <RBACView isConnected activeContext="ctx" namespaces={["default"]} />
    );
    const row = await screen.findByText("alice@example.com");
    fireEvent.click(row);
    // After expansion the detail <h4>Bindings</h4> appears alongside the
    // binding chip with the role name.
    expect(await screen.findByRole("heading", { name: "Bindings" })).toBeDefined();
    expect(screen.getByText("admin")).toBeDefined();
  });

  it("surfaces errors", async () => {
    mockGetNamespaceRBAC.mockRejectedValue(new Error("rbac call failed"));
    render(
      <RBACView isConnected activeContext="ctx" namespaces={["default"]} />
    );
    expect(await screen.findByText(/rbac call failed/)).toBeDefined();
  });

  it("skips fetch when disconnected", () => {
    render(
      <RBACView isConnected={false} activeContext="ctx" namespaces={["default"]} />
    );
    expect(mockGetNamespaceRBAC).not.toHaveBeenCalled();
  });

  it("re-fetches when the namespace is changed via the selector", async () => {
    render(
      <RBACView
        isConnected
        activeContext="ctx"
        namespaces={["default", "kube-system"]}
      />
    );
    await waitFor(() => {
      expect(mockGetNamespaceRBAC).toHaveBeenCalledTimes(1);
    });
    fireEvent.change(screen.getByRole("combobox"), {
      target: { value: "kube-system" },
    });
    await waitFor(() => {
      expect(mockGetNamespaceRBAC).toHaveBeenLastCalledWith("ctx", "kube-system");
    });
  });
});
