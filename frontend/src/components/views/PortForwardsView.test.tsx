/**
 * PortForwardsView — manages kubectl port-forward sessions.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { PortForwardsView } from "./PortForwardsView";

const mockListPortForwards = vi.fn();
const mockCreatePortForward = vi.fn();
const mockStopPortForward = vi.fn();
const mockListResources = vi.fn();

vi.mock("../../../wailsjs/go/main/App", () => ({
  ListPortForwards: () => mockListPortForwards(),
  CreatePortForward: (...a: unknown[]) => mockCreatePortForward(...a),
  StopPortForward: (...a: unknown[]) => mockStopPortForward(...a),
  ListResources: (...a: unknown[]) => mockListResources(...a),
}));

beforeEach(() => {
  vi.clearAllMocks();
  mockListPortForwards.mockResolvedValue([]);
  mockListResources.mockResolvedValue([]);
  vi.useFakeTimers({ shouldAdvanceTime: true });
});

afterEach(() => {
  vi.useRealTimers();
});

describe("PortForwardsView", () => {
  it("shows the connect prompt when not connected", () => {
    render(<PortForwardsView isConnected={false} />);
    expect(
      screen.getByText(/Connect to a cluster to manage port forwards/)
    ).toBeDefined();
  });

  it("shows the empty state when connected with zero forwards", async () => {
    render(<PortForwardsView isConnected />);
    await waitFor(() => {
      expect(mockListPortForwards).toHaveBeenCalled();
    });
    expect(screen.getByText(/No active port forwards/)).toBeDefined();
  });

  it("renders rows for active forwards", async () => {
    mockListPortForwards.mockResolvedValue([
      {
        id: "pf-1",
        pod: "api-abc",
        namespace: "default",
        localPort: 8080,
        remotePort: 80,
        status: "active",
      },
    ]);
    render(<PortForwardsView isConnected />);
    expect(await screen.findByText("api-abc")).toBeDefined();
  });

  it("toggles the create form when 'New Port Forward' is clicked", async () => {
    render(<PortForwardsView isConnected />);
    await waitFor(() => {
      expect(mockListPortForwards).toHaveBeenCalled();
    });
    const btn = screen.getByRole("button", { name: /New Port Forward/ });
    fireEvent.click(btn);
    // The form renders an <h3> "Create Port Forward" heading when open.
    expect(
      screen.getByRole("heading", { name: /Create Port Forward/ })
    ).toBeDefined();
    fireEvent.click(screen.getByRole("button", { name: /Cancel/ }));
    expect(
      screen.queryByRole("heading", { name: /Create Port Forward/ })
    ).toBeNull();
  });
});
