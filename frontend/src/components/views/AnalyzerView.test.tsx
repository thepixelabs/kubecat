/**
 * AnalyzerView — cluster diagnostic scan and issue triage.
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { AnalyzerView } from "./AnalyzerView";

const mockScanCluster = vi.fn();

vi.mock("../../../wailsjs/go/main/App", () => ({
  ScanCluster: (...a: unknown[]) => mockScanCluster(...a),
}));

const SUMMARY_WITH_ISSUES = {
  critical: 1,
  warning: 2,
  info: 0,
  issuesByCategory: {
    Scheduling: [
      {
        id: "sched-1",
        category: "Scheduling",
        severity: "Critical",
        title: "Pod pending due to insufficient CPU",
        message: "no nodes satisfy requests",
        resource: "api-5d4",
        namespace: "default",
        kind: "Pod",
        fixes: [
          { description: "Add more nodes", command: "kubectl scale nodepool" },
        ],
      },
    ],
    Storage: [
      {
        id: "stor-1",
        category: "Storage",
        severity: "Warning",
        title: "PVC pending",
        message: "no storage class",
        resource: "data",
        namespace: "default",
        kind: "PersistentVolumeClaim",
      },
      {
        id: "stor-2",
        category: "Storage",
        severity: "Warning",
        title: "PV orphaned",
        message: "reclaimed",
        resource: "pv-1",
        namespace: "",
        kind: "PersistentVolume",
      },
    ],
  },
  scannedAt: "2024-01-01T10:00:00Z",
};

const EMPTY_SUMMARY = {
  critical: 0,
  warning: 0,
  info: 0,
  issuesByCategory: {},
  scannedAt: "2024-01-01T10:00:00Z",
};

beforeEach(() => {
  vi.clearAllMocks();
  mockScanCluster.mockResolvedValue(SUMMARY_WITH_ISSUES);
});

describe("AnalyzerView", () => {
  it("shows 'connect to a cluster' when not connected", () => {
    render(<AnalyzerView isConnected={false} />);
    expect(screen.getByText(/Connect to a cluster to scan/)).toBeDefined();
  });

  it("renders issue count summary cards on successful scan", async () => {
    render(<AnalyzerView isConnected />);
    await waitFor(() => {
      expect(mockScanCluster).toHaveBeenCalled();
    });
    await waitFor(() => {
      // Count cards: 1 critical, 2 warning, 0 info.
      expect(screen.getByText("1")).toBeDefined();
      expect(screen.getByText("2")).toBeDefined();
    });
  });

  it("renders each issue title", async () => {
    render(<AnalyzerView isConnected />);
    await waitFor(() => {
      expect(
        screen.getByText(/Pod pending due to insufficient CPU/)
      ).toBeDefined();
    });
    expect(screen.getByText(/PVC pending/)).toBeDefined();
    expect(screen.getByText(/PV orphaned/)).toBeDefined();
  });

  it("shows the 'No issues found' state when summary is empty", async () => {
    mockScanCluster.mockResolvedValue(EMPTY_SUMMARY);
    render(<AnalyzerView isConnected />);
    await waitFor(() => {
      expect(screen.getByText(/No issues found/)).toBeDefined();
    });
  });

  it("surfaces scan errors in the results area", async () => {
    mockScanCluster.mockRejectedValue(new Error("scan RBAC denied"));
    render(<AnalyzerView isConnected />);
    await waitFor(() => {
      expect(screen.getByText(/scan RBAC denied/)).toBeDefined();
    });
  });

  it("clicking an issue expands its fix suggestions", async () => {
    render(<AnalyzerView isConnected />);
    const title = await screen.findByText(
      /Pod pending due to insufficient CPU/
    );
    fireEvent.click(title);
    expect(await screen.findByText(/Suggested Fixes/)).toBeDefined();
    expect(screen.getByText(/Add more nodes/)).toBeDefined();
  });

  it("re-runs the scan when 'Scan Cluster' is clicked", async () => {
    render(<AnalyzerView isConnected />);
    await waitFor(() => {
      expect(mockScanCluster).toHaveBeenCalledTimes(1);
    });
    fireEvent.click(screen.getByRole("button", { name: /Scan Cluster/ }));
    await waitFor(() => {
      expect(mockScanCluster).toHaveBeenCalledTimes(2);
    });
  });
});
