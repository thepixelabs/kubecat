/**
 * CostBadge — compact cost chip rendered next to workloads.
 *
 * Behaviors:
 * - returns null when estimate is null and not loading
 * - loading state renders the "$…" placeholder
 * - sub-dollar monthly totals render as cents (¢)
 * - >=$1 monthly totals render as dollars
 * - hover tooltip surfaces source, workload name, and breakdown
 */
import { describe, it, expect } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { CostBadge } from "./CostBadge";

const makeEstimate = (overrides: Partial<any> = {}) => ({
  workloadName: "nginx",
  namespace: "default",
  cpuCost: 0.002,
  memoryCost: 0.001,
  totalCost: 0.003,
  monthlyTotal: 2.19,
  currency: "USD",
  source: "heuristic",
  ...overrides,
});

describe("CostBadge", () => {
  it("returns nothing when estimate is null and not loading", () => {
    const { container } = render(<CostBadge estimate={null} />);
    expect(container.textContent).toBe("");
  });

  it("renders a placeholder when loading", () => {
    const { container } = render(<CostBadge estimate={null} loading />);
    expect(container.textContent).toContain("$…");
  });

  it("formats amounts under $1/mo in cents", () => {
    render(<CostBadge estimate={makeEstimate({ monthlyTotal: 0.42 })} />);
    // 0.42 * 100 = 42.0 → "$42.0¢/mo"
    expect(screen.getByText(/\$42\.0¢\/mo/)).toBeDefined();
  });

  it("formats amounts >= $1/mo as dollars", () => {
    render(<CostBadge estimate={makeEstimate({ monthlyTotal: 12.5 })} />);
    expect(screen.getByText(/\$12\.50\/mo/)).toBeDefined();
  });

  it("shows tooltip on hover with the source label", () => {
    render(<CostBadge estimate={makeEstimate({ source: "opencost" })} />);
    const chip = screen.getByText(/\$2\.19\/mo/);
    fireEvent.mouseEnter(chip.parentElement!);
    expect(screen.getByText(/Source: opencost/)).toBeDefined();
  });

  it("hides tooltip on mouse leave", () => {
    render(<CostBadge estimate={makeEstimate()} />);
    const chip = screen.getByText(/\$2\.19\/mo/);
    fireEvent.mouseEnter(chip.parentElement!);
    expect(screen.getByText(/Source: heuristic/)).toBeDefined();
    fireEvent.mouseLeave(chip.parentElement!);
    expect(screen.queryByText(/Source: heuristic/)).toBeNull();
  });
});
