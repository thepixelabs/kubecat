/**
 * ClusterDiffView — thin wrapper around <ClusterDiff /> inside an ErrorBoundary.
 * We assert the wiring layer: the wrapper forwards props and renders the
 * ErrorBoundary-wrapped child without crashing.
 */
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { ClusterDiffView } from "./ClusterDiffView";

// Replace the heavy ClusterDiff subtree with a sentinel component that
// records the props it received.
const clusterDiffSpy = vi.fn();
vi.mock("../cluster-diff", () => ({
  ClusterDiff: (props: any) => {
    clusterDiffSpy(props);
    return <div data-testid="cluster-diff">ClusterDiff stub</div>;
  },
}));

vi.mock("../../../wailsjs/go/main/App", () => ({
  GetSnapshots: vi.fn(),
  ListResources: vi.fn(),
  ComputeDiff: vi.fn(),
  ApplyResourceToCluster: vi.fn(),
  GenerateDiffReport: vi.fn(),
}));

describe("ClusterDiffView", () => {
  it("renders the ClusterDiff subtree inside the error boundary", () => {
    const onStateChange = vi.fn();
    render(
      <ClusterDiffView
        contexts={["ctx-a", "ctx-b"]}
        activeContext="ctx-a"
        namespaces={["default"]}
        isTimelineAvailable
        initialState={undefined}
        onStateChange={onStateChange}
      />
    );
    expect(screen.getByTestId("cluster-diff")).toBeDefined();
    expect(clusterDiffSpy).toHaveBeenCalled();
    const passed = clusterDiffSpy.mock.calls[0][0];
    expect(passed.contexts).toEqual(["ctx-a", "ctx-b"]);
    expect(passed.activeContext).toBe("ctx-a");
    expect(passed.namespaces).toEqual(["default"]);
    expect(passed.isTimelineAvailable).toBe(true);
  });
});
