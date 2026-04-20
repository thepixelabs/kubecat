import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  ProactiveAlertBanner,
  type ProactiveAlert,
} from "./ProactiveAlertBanner";

vi.mock("framer-motion", () => ({
  motion: {
    div: ({ children, ...props }: React.HTMLAttributes<HTMLDivElement>) => (
      <div {...props}>{children}</div>
    ),
    button: ({
      children,
      ...props
    }: React.ButtonHTMLAttributes<HTMLButtonElement>) => (
      <button {...props}>{children}</button>
    ),
  },
  AnimatePresence: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
}));

const alert = (overrides: Partial<ProactiveAlert> = {}): ProactiveAlert => ({
  id: "a1",
  severity: "high",
  title: "High memory",
  message: "Pod near memory limit",
  ...overrides,
});

describe("ProactiveAlertBanner", () => {
  it("renders nothing when there are no alerts", () => {
    const { container } = render(
      <ProactiveAlertBanner
        alerts={[]}
        onDismiss={vi.fn()}
        onInvestigate={vi.fn()}
      />
    );
    expect(container.firstChild).toBeNull();
  });

  it("renders up to 3 alerts at once", () => {
    const alerts: ProactiveAlert[] = [
      alert({ id: "1", title: "A" }),
      alert({ id: "2", title: "B" }),
      alert({ id: "3", title: "C" }),
    ];
    render(
      <ProactiveAlertBanner
        alerts={alerts}
        onDismiss={vi.fn()}
        onInvestigate={vi.fn()}
      />
    );
    expect(screen.getByText("A")).toBeInTheDocument();
    expect(screen.getByText("B")).toBeInTheDocument();
    expect(screen.getByText("C")).toBeInTheDocument();
  });

  it("shows an overflow counter when >3 alerts are queued", () => {
    const alerts: ProactiveAlert[] = Array.from({ length: 5 }).map((_, i) =>
      alert({ id: String(i), title: `T${i}` })
    );
    render(
      <ProactiveAlertBanner
        alerts={alerts}
        onDismiss={vi.fn()}
        onInvestigate={vi.fn()}
      />
    );
    // "+2 more alerts"
    expect(screen.getByText(/\+2 more alerts/)).toBeInTheDocument();
  });

  it("fires onDismiss with the alert id when X is clicked", async () => {
    const onDismiss = vi.fn();
    const user = userEvent.setup();
    render(
      <ProactiveAlertBanner
        alerts={[alert({ id: "xyz", title: "DismissMe" })]}
        onDismiss={onDismiss}
        onInvestigate={vi.fn()}
      />
    );
    await user.click(screen.getByRole("button", { name: /Dismiss: DismissMe/i }));
    expect(onDismiss).toHaveBeenCalledWith("xyz");
  });

  it("fires onInvestigate with the full alert when the CTA is clicked", async () => {
    const onInvestigate = vi.fn();
    const user = userEvent.setup();
    const a = alert({ id: "inv", title: "Inv" });
    render(
      <ProactiveAlertBanner
        alerts={[a]}
        onDismiss={vi.fn()}
        onInvestigate={onInvestigate}
      />
    );
    await user.click(screen.getByRole("button", { name: /Investigate: Inv/i }));
    expect(onInvestigate).toHaveBeenCalledWith(a);
  });

  it("expand/collapse toggle appears only for long messages (>80 chars)", async () => {
    const user = userEvent.setup();
    const long = "X".repeat(100);
    render(
      <ProactiveAlertBanner
        alerts={[alert({ message: long })]}
        onDismiss={vi.fn()}
        onInvestigate={vi.fn()}
      />
    );
    // Show more initially
    const btn = screen.getByRole("button", { name: /Show more/i });
    await user.click(btn);
    expect(
      screen.getByRole("button", { name: /Show less/i })
    ).toBeInTheDocument();
  });
});
