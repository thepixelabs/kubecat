import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SourceSelector } from "./SourceSelector";
import type { DiffSource, SnapshotInfo } from "./types";

// Mock framer-motion so AnimatePresence renders its children synchronously.
vi.mock("framer-motion", () => ({
  motion: {
    div: ({ children, ...props }: React.HTMLAttributes<HTMLDivElement>) => (
      <div {...props}>{children}</div>
    ),
  },
  AnimatePresence: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
}));

// ── Factory ──────────────────────────────────────────────────────────────────

const makeProps = (
  overrides: Partial<React.ComponentProps<typeof SourceSelector>> = {}
) => ({
  contexts: ["minikube", "prod", "staging"],
  snapshots: [] as SnapshotInfo[],
  value: { context: "minikube", isLive: true } as DiffSource,
  onChange: vi.fn(),
  label: "Source",
  isTimelineAvailable: true,
  ...overrides,
});

// ── Tests ────────────────────────────────────────────────────────────────────

describe("SourceSelector", () => {
  it("renders the label", () => {
    render(<SourceSelector {...makeProps({ label: "Source (Left)" })} />);
    expect(screen.getByText("Source (Left)")).toBeInTheDocument();
  });

  it("shows the currently selected context", () => {
    render(
      <SourceSelector
        {...makeProps({ value: { context: "prod", isLive: true } })}
      />
    );
    // The trigger shows "prod"; the "Live cluster state" status line also references prod mode.
    expect(screen.getByText("prod")).toBeInTheDocument();
  });

  // Regression pin: the long-context truncation bug. Both the trigger AND
  // every dropdown option must carry `title=` so hover reveals the full ARN.
  it("puts title={context} on the trigger button's label span", () => {
    const longCtx =
      "arn:aws:eks:us-east-1:123456789012:cluster/prod-api-legacy";
    const { container } = render(
      <SourceSelector
        {...makeProps({ value: { context: longCtx, isLive: true } })}
      />
    );
    // There will be multiple spans with title — the one inside the trigger
    // button is the critical regression pin.
    const spans = Array.from(container.querySelectorAll(`span[title="${longCtx}"]`));
    expect(spans.length).toBeGreaterThanOrEqual(1);
  });

  it("puts title={ctx} on each dropdown option (regression pin)", async () => {
    const user = userEvent.setup();
    const longCtx =
      "arn:aws:eks:us-east-1:123456789012:cluster/super-long-cluster-name";
    const { container } = render(
      <SourceSelector
        {...makeProps({
          contexts: [longCtx],
          value: { context: "", isLive: true },
        })}
      />
    );

    await user.click(screen.getByText("Select cluster...").closest("button")!);

    // Option span
    const optionSpan = container.querySelector(
      `.overflow-auto span[title="${longCtx}"]`
    );
    expect(optionSpan).toBeTruthy();
    expect(optionSpan?.textContent).toBe(longCtx);
  });

  it("calls onChange with the new context when an option is selected", async () => {
    const onChange = vi.fn();
    const user = userEvent.setup();
    render(<SourceSelector {...makeProps({ onChange })} />);

    await user.click(screen.getByText("minikube").closest("button")!);
    await user.click(screen.getByText("prod"));

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ context: "prod", isLive: true })
    );
  });

  it("does not open the dropdown in readOnly mode", async () => {
    const user = userEvent.setup();
    render(
      <SourceSelector
        {...makeProps({
          readOnly: true,
          value: { context: "minikube", isLive: true },
        })}
      />
    );

    const trigger = screen.getByText("minikube").closest("button")!;
    expect(trigger).toBeDisabled();
    await user.click(trigger);

    // "prod" should not appear in the DOM because the dropdown is closed.
    // The trigger already renders "minikube". Any extra "prod" would be an
    // option entry — there should be none.
    expect(screen.queryByText("prod")).toBeNull();
  });

  it("switches live→historical via the mode toggle and emits onChange", async () => {
    const onChange = vi.fn();
    const user = userEvent.setup();
    render(<SourceSelector {...makeProps({ onChange })} />);

    // "Historical" button in the toggle
    const historical = screen.getByRole("button", { name: /Historical/i });
    await user.click(historical);

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ isLive: false })
    );
  });

  it("disables the Historical toggle when isTimelineAvailable is false", () => {
    render(<SourceSelector {...makeProps({ isTimelineAvailable: false })} />);
    const historical = screen.getByRole("button", { name: /Historical/i });
    expect(historical).toBeDisabled();
    expect(historical.getAttribute("title")).toContain(
      "Timeline not available"
    );
  });

  it('renders "No clusters available" when contexts is empty', async () => {
    const user = userEvent.setup();
    render(
      <SourceSelector
        {...makeProps({ contexts: [], value: { context: "", isLive: true } })}
      />
    );
    await user.click(screen.getByText("Select cluster...").closest("button")!);
    expect(screen.getByText("No clusters available")).toBeInTheDocument();
  });

  it("shows snapshot picker in historical mode and lists available snapshots", async () => {
    const user = userEvent.setup();
    const snapshots: SnapshotInfo[] = [
      { timestamp: "2026-04-19T10:00:00Z" },
      { timestamp: "2026-04-19T11:30:00Z" },
    ];
    render(
      <SourceSelector
        {...makeProps({
          snapshots,
          value: { context: "minikube", isLive: false },
        })}
      />
    );

    // The snapshot picker button is visible now.
    await user.click(screen.getByText("Select snapshot...").closest("button")!);

    // Every snapshot timestamp should render (formatted).
    expect(screen.getAllByText(/2026/).length).toBeGreaterThanOrEqual(1);
  });

  it("shows fallback text when no snapshots are available", async () => {
    const user = userEvent.setup();
    render(
      <SourceSelector
        {...makeProps({
          snapshots: [],
          value: { context: "minikube", isLive: false },
        })}
      />
    );
    await user.click(screen.getByText("Select snapshot...").closest("button")!);
    expect(screen.getByText("No snapshots available")).toBeInTheDocument();
    // Silence unused-import lint
    expect(fireEvent).toBeDefined();
  });
});
