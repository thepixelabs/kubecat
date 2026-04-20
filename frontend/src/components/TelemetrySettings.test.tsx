import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TelemetrySettings } from "./TelemetrySettings";

const { mockOpenUrl } = vi.hoisted(() => ({ mockOpenUrl: vi.fn() }));
vi.mock("../../wailsjs/runtime/runtime", () => ({
  BrowserOpenURL: (...a: unknown[]) => mockOpenUrl(...a),
}));

beforeEach(() => {
  localStorage.clear();
  vi.clearAllMocks();
});

describe("TelemetrySettings", () => {
  it("defaults to 'not enabled' on a fresh install (pending consent)", () => {
    render(<TelemetrySettings />);
    const toggle = screen.getByRole("switch");
    expect(toggle).toHaveAttribute("aria-checked", "false");
    // Pending nudge visible
    expect(
      screen.getByText(/You haven't made a choice yet/i)
    ).toBeInTheDocument();
  });

  it("flips the toggle to enabled when the user grants consent", async () => {
    const user = userEvent.setup();
    render(<TelemetrySettings />);
    await user.click(screen.getByRole("switch"));
    expect(screen.getByRole("switch")).toHaveAttribute("aria-checked", "true");
    expect(localStorage.getItem("kubecat-telemetry-consent")).toBe("granted");
  });

  it("toggles back off when the user revokes", async () => {
    const user = userEvent.setup();
    render(<TelemetrySettings />);
    // Grant then revoke
    const toggle = screen.getByRole("switch");
    await user.click(toggle);
    await user.click(toggle);
    expect(toggle).toHaveAttribute("aria-checked", "false");
    expect(localStorage.getItem("kubecat-telemetry-consent")).toBe("denied");
  });

  it("shows the anonymous ID once the user has made a choice", async () => {
    const user = userEvent.setup();
    render(<TelemetrySettings />);
    await user.click(screen.getByRole("switch"));
    expect(screen.getByText("Anonymous ID")).toBeInTheDocument();
  });

  it("opens the privacy policy in the system browser when clicked", async () => {
    const user = userEvent.setup();
    render(<TelemetrySettings />);
    await user.click(screen.getByRole("button", { name: /Privacy policy/i }));
    expect(mockOpenUrl).toHaveBeenCalledWith("https://kubecat.app/privacy");
  });
});
