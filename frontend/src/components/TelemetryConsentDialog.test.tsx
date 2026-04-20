import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TelemetryConsentDialog } from "./TelemetryConsentDialog";

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

const baseProps = () => ({
  isOpen: true,
  onAccept: vi.fn(),
  onDecline: vi.fn(),
});

describe("TelemetryConsentDialog", () => {
  it("renders nothing when closed", () => {
    render(<TelemetryConsentDialog {...baseProps()} isOpen={false} />);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("renders title and both disclosure panels when open", () => {
    render(<TelemetryConsentDialog {...baseProps()} />);
    expect(
      screen.getByRole("dialog", { name: /Help improve Kubecat/i })
    ).toBeInTheDocument();
    expect(screen.getByText("What we collect")).toBeInTheDocument();
    expect(screen.getByText("We never collect")).toBeInTheDocument();
  });

  it("fires onAccept only when the user clicks 'Allow anonymous analytics'", async () => {
    const onAccept = vi.fn();
    const user = userEvent.setup();
    render(<TelemetryConsentDialog {...baseProps()} onAccept={onAccept} />);
    await user.click(
      screen.getByRole("button", { name: /Allow anonymous analytics/i })
    );
    expect(onAccept).toHaveBeenCalledTimes(1);
  });

  it("fires onDecline when 'No thanks' button is clicked", async () => {
    const onDecline = vi.fn();
    const user = userEvent.setup();
    render(<TelemetryConsentDialog {...baseProps()} onDecline={onDecline} />);
    // There are two elements matching /No thanks/ — the X (aria-label="No
    // thanks, close") and the text button. Use exact name match to get the
    // footer button only.
    await user.click(screen.getByRole("button", { name: "No thanks" }));
    expect(onDecline).toHaveBeenCalled();
  });

  // Privacy regression: the default path must be decline, not accept.
  // Backdrop click and Escape both count as decline.
  it("Escape triggers onDecline (default path = deny)", async () => {
    const onAccept = vi.fn();
    const onDecline = vi.fn();
    const user = userEvent.setup();
    render(
      <TelemetryConsentDialog
        {...baseProps()}
        onAccept={onAccept}
        onDecline={onDecline}
      />
    );
    await user.keyboard("{Escape}");
    expect(onDecline).toHaveBeenCalled();
    expect(onAccept).not.toHaveBeenCalled();
  });

  it("backdrop click triggers onDecline (default path = deny)", async () => {
    const onDecline = vi.fn();
    const user = userEvent.setup();
    const { container } = render(
      <TelemetryConsentDialog {...baseProps()} onDecline={onDecline} />
    );
    const backdrop = container.querySelector('[aria-hidden="true"]');
    await user.click(backdrop!);
    expect(onDecline).toHaveBeenCalled();
  });
});
