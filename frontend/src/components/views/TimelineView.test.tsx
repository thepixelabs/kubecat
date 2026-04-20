/**
 * TimelineView — events + snapshots + diff modes.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { TimelineView } from "./TimelineView";

const mockGetTimelineEvents = vi.fn();
const mockGetSnapshots = vi.fn();
const mockGetSnapshotDiff = vi.fn();
const mockTakeSnapshot = vi.fn();
const mockIsTimelineAvailable = vi.fn();
const mockListResources = vi.fn();

vi.mock("../../../wailsjs/go/main/App", () => ({
  GetTimelineEvents: (...a: unknown[]) => mockGetTimelineEvents(...a),
  GetSnapshots: (...a: unknown[]) => mockGetSnapshots(...a),
  GetSnapshotDiff: (...a: unknown[]) => mockGetSnapshotDiff(...a),
  TakeSnapshot: () => mockTakeSnapshot(),
  IsTimelineAvailable: () => mockIsTimelineAvailable(),
  ListResources: (...a: unknown[]) => mockListResources(...a),
}));

beforeEach(() => {
  vi.clearAllMocks();
  mockGetTimelineEvents.mockResolvedValue([]);
  mockGetSnapshots.mockResolvedValue([]);
  mockIsTimelineAvailable.mockResolvedValue(true);
  mockListResources.mockResolvedValue([]);
  vi.useFakeTimers({ shouldAdvanceTime: true });
});

afterEach(() => {
  vi.useRealTimers();
});

describe("TimelineView", () => {
  it("shows the connect prompt when not connected", async () => {
    render(<TimelineView isConnected={false} namespaces={["default"]} />);
    await waitFor(() => {
      expect(screen.getByText(/Connect to a cluster to view events/)).toBeDefined();
    });
  });

  it("shows the 'No events found' empty state", async () => {
    render(<TimelineView isConnected namespaces={["default"]} />);
    await waitFor(() => {
      expect(screen.getByText(/No events found/)).toBeDefined();
    });
  });

  it("renders events when backend returns data", async () => {
    mockGetTimelineEvents.mockResolvedValue([
      {
        id: 1,
        cluster: "prod",
        namespace: "default",
        kind: "Pod",
        name: "crashing-pod",
        type: "Warning",
        reason: "BackOff",
        message: "crash",
        firstSeen: new Date().toISOString(),
        lastSeen: new Date().toISOString(),
        count: 2,
        sourceComponent: "kubelet",
      },
    ]);
    render(<TimelineView isConnected namespaces={["default"]} />);
    expect(await screen.findByText("crashing-pod")).toBeDefined();
  });

  it("switches to Snapshots mode", async () => {
    render(<TimelineView isConnected namespaces={["default"]} />);
    await waitFor(() => {
      expect(mockGetTimelineEvents).toHaveBeenCalled();
    });
    fireEvent.click(screen.getByRole("button", { name: /^Snapshots$/ }));
    await waitFor(() => {
      expect(mockGetSnapshots).toHaveBeenCalled();
    });
  });
});
