import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { UpdateNotificationBar } from "./UpdateNotificationBar";

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

const { mockOpenUrl } = vi.hoisted(() => ({ mockOpenUrl: vi.fn() }));
vi.mock("../../wailsjs/runtime/runtime", () => ({
  BrowserOpenURL: (...a: unknown[]) => mockOpenUrl(...a),
}));

describe("UpdateNotificationBar", () => {
  const baseProps = () => ({
    version: "v1.2.3",
    releaseUrl: "https://github.com/foo/kubecat/releases/tag/v1.2.3",
    show: true,
    onDismiss: vi.fn(),
  });

  it("renders nothing when show=false", () => {
    render(<UpdateNotificationBar {...baseProps()} show={false} />);
    expect(screen.queryByText(/Kubecat v1.2.3/)).toBeNull();
  });

  it("renders the version banner when show=true", () => {
    render(<UpdateNotificationBar {...baseProps()} />);
    expect(screen.getByText(/Kubecat v1\.2\.3/)).toBeInTheDocument();
  });

  it("opens the release URL in the system browser when Download is clicked", async () => {
    const user = userEvent.setup();
    render(<UpdateNotificationBar {...baseProps()} />);
    await user.click(screen.getByRole("button", { name: /Download Kubecat/i }));
    expect(mockOpenUrl).toHaveBeenCalledWith(
      "https://github.com/foo/kubecat/releases/tag/v1.2.3"
    );
  });

  it("invokes onDismiss when the X is clicked", async () => {
    const onDismiss = vi.fn();
    const user = userEvent.setup();
    render(<UpdateNotificationBar {...baseProps()} onDismiss={onDismiss} />);
    await user.click(
      screen.getByRole("button", { name: /Dismiss update notification/i })
    );
    expect(onDismiss).toHaveBeenCalled();
  });
});
