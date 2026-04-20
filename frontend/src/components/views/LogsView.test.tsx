/**
 * LogsView — pod/workload log streaming view.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { LogsView } from "./LogsView";

const mockStartLogStream = vi.fn();
const mockStopLogStream = vi.fn();
const mockGetBufferedLogs = vi.fn();
const mockStartWorkloadLogStream = vi.fn();
const mockGetBufferedWorkloadLogs = vi.fn();

vi.mock("../../../wailsjs/go/main/App", () => ({
  StartLogStream: (...a: unknown[]) => mockStartLogStream(...a),
  StopLogStream: () => mockStopLogStream(),
  GetBufferedLogs: () => mockGetBufferedLogs(),
  StartWorkloadLogStream: (...a: unknown[]) => mockStartWorkloadLogStream(...a),
  GetBufferedWorkloadLogs: () => mockGetBufferedWorkloadLogs(),
}));

beforeEach(() => {
  vi.clearAllMocks();
  mockStartLogStream.mockResolvedValue(undefined);
  mockGetBufferedLogs.mockResolvedValue([]);
  mockStartWorkloadLogStream.mockResolvedValue(undefined);
  mockGetBufferedWorkloadLogs.mockResolvedValue([]);
  vi.useFakeTimers({ shouldAdvanceTime: true });
});

afterEach(() => {
  vi.useRealTimers();
});

describe("LogsView", () => {
  it("shows connect-prompt when not connected", () => {
    render(<LogsView isConnected={false} selectedPod={null} onClearPod={vi.fn()} />);
    expect(screen.getByText(/Connect to a cluster first/)).toBeDefined();
  });

  it("shows no-selection message when connected but nothing selected", () => {
    render(<LogsView isConnected selectedPod={null} onClearPod={vi.fn()} />);
    expect(
      screen.getByText(/No resource selected\. Go to Explorer/)
    ).toBeDefined();
  });

  it("starts a pod log stream when a pod is selected and shows streaming indicator", async () => {
    render(
      <LogsView
        isConnected
        selectedPod={{ kind: "Pod", namespace: "ns", name: "nginx" } as any}
        onClearPod={vi.fn()}
      />
    );
    await waitFor(() => {
      expect(mockStartLogStream).toHaveBeenCalledWith("ns", "nginx", "", 100);
    });
    await waitFor(() => {
      expect(screen.getByText("Streaming")).toBeDefined();
    });
  });

  it("starts a workload log stream for Deployments and marks multi-pod", async () => {
    render(
      <LogsView
        isConnected
        selectedPod={
          { kind: "Deployment", namespace: "ns", name: "api" } as any
        }
        onClearPod={vi.fn()}
      />
    );
    await waitFor(() => {
      expect(mockStartWorkloadLogStream).toHaveBeenCalledWith(
        "Deployment",
        "ns",
        "api",
        100
      );
    });
    expect(screen.getByText("multi-pod")).toBeDefined();
  });

  it("calls StopLogStream + onClearPod when X is clicked", async () => {
    const onClearPod = vi.fn();
    render(
      <LogsView
        isConnected
        selectedPod={{ kind: "Pod", namespace: "ns", name: "nginx" } as any}
        onClearPod={onClearPod}
      />
    );
    await waitFor(() => {
      expect(mockStartLogStream).toHaveBeenCalled();
    });
    // The X button is the only button in the header.
    const clearBtn = screen.getByRole("button");
    fireEvent.click(clearBtn);
    expect(mockStopLogStream).toHaveBeenCalled();
    expect(onClearPod).toHaveBeenCalled();
  });

  it("Escape key clears the selected pod", async () => {
    const onClearPod = vi.fn();
    render(
      <LogsView
        isConnected
        selectedPod={{ kind: "Pod", namespace: "ns", name: "nginx" } as any}
        onClearPod={onClearPod}
      />
    );
    await waitFor(() => {
      expect(mockStartLogStream).toHaveBeenCalled();
    });
    fireEvent.keyDown(window, { key: "Escape" });
    expect(onClearPod).toHaveBeenCalled();
  });

  it("ignores 'context canceled' errors silently", async () => {
    mockStartLogStream.mockRejectedValue(new Error("stream context canceled"));
    render(
      <LogsView
        isConnected
        selectedPod={{ kind: "Pod", namespace: "ns", name: "nginx" } as any}
        onClearPod={vi.fn()}
      />
    );
    // Give the async path a chance to resolve without asserting error UI.
    await waitFor(() => {
      expect(mockStartLogStream).toHaveBeenCalled();
    });
    expect(screen.queryByText(/Failed to start log stream/)).toBeNull();
  });
});
