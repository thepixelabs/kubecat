import { ErrorBoundary } from "../ErrorBoundary";
import { ClusterDiff } from "../cluster-diff";
import type { ClusterDiffPersistentState } from "../cluster-diff/types";
import {
  GetSnapshots,
  ListResources,
} from "../../../wailsjs/go/main/App";

interface ClusterDiffViewProps {
  contexts: string[];
  activeContext: string;
  namespaces: string[];
  isTimelineAvailable: boolean;
  initialState: ClusterDiffPersistentState | undefined;
  onStateChange: (state: ClusterDiffPersistentState) => void;
}

export function ClusterDiffView({
  contexts,
  activeContext,
  namespaces,
  isTimelineAvailable,
  initialState,
  onStateChange,
}: ClusterDiffViewProps) {
  return (
    <ErrorBoundary componentName="Cluster Diff">
      <ClusterDiff
        contexts={contexts}
        activeContext={activeContext}
        namespaces={namespaces}
        isTimelineAvailable={isTimelineAvailable}
        initialState={initialState}
        onStateChange={onStateChange}
        onComputeDiff={async (req) => {
          const { ComputeDiff } = await import("../../../wailsjs/go/main/App");
          return ComputeDiff(req);
        }}
        onGetSnapshots={async (limit) => {
          const snapshots = await GetSnapshots(limit);
          return snapshots.map((s: any) => ({ timestamp: s.timestamp }));
        }}
        onListResources={async (kind, namespace) => {
          const resources = await ListResources(kind, namespace);
          return resources || [];
        }}
        onApplyResource={async (context, kind, namespace, name, yaml, dryRun) => {
          const { ApplyResourceToCluster } = await import("../../../wailsjs/go/main/App");
          return ApplyResourceToCluster(context, kind, namespace, name, yaml, dryRun);
        }}
        onGenerateReport={async (result, format) => {
          const { GenerateDiffReport } = await import("../../../wailsjs/go/main/App");
          return GenerateDiffReport(result, format);
        }}
      />
    </ErrorBoundary>
  );
}
