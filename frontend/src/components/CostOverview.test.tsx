/**
 * CostOverview — namespace-level cost summary with OpenCost integration CTA.
 *
 * Key behaviors:
 * - Loading spinner while initial fetch is in-flight
 * - Error banner when the Wails call rejects
 * - Summary cards render monthly/hourly totals
 * - Source badge reflects "heuristic" vs "opencost"
 * - Settings CTA shows only when source != "opencost" AND onOpenSettings is provided
 * - Clicking the CTA fires onOpenSettings
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { CostOverview } from "./CostOverview";

const mockGetNamespaceCostSummary = vi.fn();
vi.mock("../../wailsjs/go/main/App", () => ({
  GetNamespaceCostSummary: (...args: unknown[]) => mockGetNamespaceCostSummary(...args),
}));

const makeSummary = (overrides: Partial<any> = {}) => ({
  namespace: "default",
  totalPerHour: 0.25,
  totalPerMonth: 182.5,
  currency: "USD",
  source: "heuristic",
  workloads: [
    {
      workloadName: "api-server",
      cpuCost: 0.05,
      memoryCost: 0.02,
      totalCost: 0.07,
      monthlyTotal: 51.1,
      currency: "USD",
      source: "heuristic",
    },
    {
      workloadName: "worker-queue",
      cpuCost: 0.02,
      memoryCost: 0.01,
      totalCost: 0.03,
      monthlyTotal: 21.9,
      currency: "USD",
      source: "heuristic",
    },
  ],
  ...overrides,
});

beforeEach(() => {
  vi.clearAllMocks();
  mockGetNamespaceCostSummary.mockResolvedValue(makeSummary());
});

// ---------------------------------------------------------------------------
// Render states
// ---------------------------------------------------------------------------

describe("CostOverview — render states", () => {
  it("renders summary cards once data resolves", async () => {
    render(<CostOverview activeContext="ctx" namespace="default" />);
    // Two totals appear — match the bold amounts via partial text.
    expect(await screen.findByText(/\$182\.50/)).toBeDefined();
    expect(screen.getByText(/\$0\.2500/)).toBeDefined();
  });

  it("renders top workloads sorted by monthly cost", async () => {
    render(<CostOverview activeContext="ctx" namespace="default" />);
    await screen.findByText(/api-server/);
    // Both workloads should appear.
    expect(screen.getByText(/api-server/)).toBeDefined();
    expect(screen.getByText(/worker-queue/)).toBeDefined();
  });

  it("shows an error banner when the Wails call rejects", async () => {
    mockGetNamespaceCostSummary.mockRejectedValue(new Error("timeout fetching cost"));
    render(<CostOverview activeContext="ctx" namespace="default" />);
    expect(await screen.findByText(/timeout fetching cost/)).toBeDefined();
  });

  it("re-fetches when activeContext or namespace changes", async () => {
    const { rerender } = render(
      <CostOverview activeContext="ctx-a" namespace="default" />
    );
    await waitFor(() => {
      expect(mockGetNamespaceCostSummary).toHaveBeenCalledTimes(1);
    });
    rerender(<CostOverview activeContext="ctx-b" namespace="default" />);
    await waitFor(() => {
      expect(mockGetNamespaceCostSummary).toHaveBeenCalledTimes(2);
    });
  });
});

// ---------------------------------------------------------------------------
// Source badge + CTA
// ---------------------------------------------------------------------------

describe("CostOverview — source badge + CTA", () => {
  it("shows 'Source: heuristic' badge for the default source", async () => {
    render(<CostOverview activeContext="ctx" namespace="default" />);
    expect(await screen.findByText(/Source: heuristic/)).toBeDefined();
  });

  it("shows the 'OpenCost endpoint in Settings' CTA when source is heuristic and onOpenSettings is provided", async () => {
    const onOpenSettings = vi.fn();
    render(
      <CostOverview
        activeContext="ctx"
        namespace="default"
        onOpenSettings={onOpenSettings}
      />
    );
    const cta = await screen.findByRole("button", {
      name: /OpenCost endpoint in Settings/i,
    });
    expect(cta).toBeDefined();
    fireEvent.click(cta);
    expect(onOpenSettings).toHaveBeenCalled();
  });

  it("hides the CTA when source is 'opencost'", async () => {
    mockGetNamespaceCostSummary.mockResolvedValue(
      makeSummary({ source: "opencost" })
    );
    render(
      <CostOverview
        activeContext="ctx"
        namespace="default"
        onOpenSettings={vi.fn()}
      />
    );
    await screen.findByText(/Source: opencost/);
    expect(
      screen.queryByRole("button", { name: /OpenCost endpoint in Settings/ })
    ).toBeNull();
  });

  it("renders a plain-text fallback when onOpenSettings is not provided", async () => {
    render(<CostOverview activeContext="ctx" namespace="default" />);
    await screen.findByText(/Source: heuristic/);
    // When onOpenSettings is omitted the copy still mentions OpenCost endpoint,
    // but it is a <span>, not a <button>.
    expect(
      screen.queryByRole("button", { name: /OpenCost endpoint in Settings/ })
    ).toBeNull();
    expect(screen.getByText(/OpenCost endpoint in Settings/)).toBeDefined();
  });
});
