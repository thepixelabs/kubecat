/**
 * SecurityView — RBAC analysis, resources browser, and AI insights modes.
 *
 * Focus on mode routing + top-level loading paths. The full RBAC table UI
 * and AI-insight prompts are large and interactive; deeper coverage would
 * live alongside the RBAC analysis backend contract.
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { SecurityView } from "./SecurityView";

const mockListResources = vi.fn();
const mockGetRBACAnalysis = vi.fn();
const mockGetResourceYAML = vi.fn();

vi.mock("../../../wailsjs/go/main/App", () => ({
  ListResources: (...a: unknown[]) => mockListResources(...a),
  GetRBACAnalysis: () => mockGetRBACAnalysis(),
  GetResourceYAML: (...a: unknown[]) => mockGetResourceYAML(...a),
}));

// Keep the analysis modal inert.
vi.mock("../AnalysisModal", () => ({
  AnalysisModal: () => null,
}));

const RBAC_SUMMARY = {
  bindings: [],
  subjectSummary: { "User:alice": ["default"] },
  dangerousAccess: [
    {
      subject: { kind: "User", name: "alice" },
      reason: "cluster-admin",
      binding: "admin-binding",
      permissions: ["*"],
    },
  ],
};

beforeEach(() => {
  vi.clearAllMocks();
  mockListResources.mockResolvedValue([]);
  mockGetRBACAnalysis.mockResolvedValue(RBAC_SUMMARY);
});

describe("SecurityView", () => {
  it("opens on RBAC mode by default and fetches RBAC analysis", async () => {
    render(<SecurityView isConnected />);
    await waitFor(() => {
      expect(mockGetRBACAnalysis).toHaveBeenCalled();
    });
  });

  it("shows the connect prompt for RBAC mode when disconnected", () => {
    render(<SecurityView isConnected={false} />);
    expect(screen.getByText(/Connect to a cluster to analyze RBAC/)).toBeDefined();
  });

  it("switches to Resources mode and fetches security resources", async () => {
    render(<SecurityView isConnected />);
    await waitFor(() => {
      expect(mockGetRBACAnalysis).toHaveBeenCalled();
    });
    fireEvent.click(screen.getByRole("button", { name: /^Resources$/ }));
    await waitFor(() => {
      // Resources mode triggers six parallel ListResources calls.
      expect(mockListResources).toHaveBeenCalled();
    });
  });

  it("switches to AI Insights mode and shows the suggestion prompts", async () => {
    render(<SecurityView isConnected />);
    fireEvent.click(screen.getByRole("button", { name: /AI Insights/ }));
    expect(
      await screen.findByText(/Security & Contextual Analysis/)
    ).toBeDefined();
    expect(
      screen.getByRole("button", {
        name: /Which ports are open to the world\?/,
      })
    ).toBeDefined();
  });

  it("surfaces RBAC errors when GetRBACAnalysis rejects", async () => {
    mockGetRBACAnalysis.mockRejectedValue(new Error("rbac cluster denied"));
    render(<SecurityView isConnected />);
    expect(await screen.findByText(/rbac cluster denied/)).toBeDefined();
  });

  it("shows the dangerous-access badge when dangerous entries exist", async () => {
    render(<SecurityView isConnected />);
    // Wait for the summary to load.
    await waitFor(() => {
      expect(mockGetRBACAnalysis).toHaveBeenCalled();
    });
    const dangerousTab = await screen.findByRole("button", {
      name: /Dangerous Access/,
    });
    expect(dangerousTab.textContent).toContain("1");
  });
});
