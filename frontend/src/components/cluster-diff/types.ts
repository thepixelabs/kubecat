// TypeScript interfaces for Cluster Diff feature
// These match the Go types in gui/app.go

export interface DiffSource {
  context: string; // Cluster context name
  snapshot?: string; // RFC3339 timestamp for historical comparison
  isLive: boolean; // true = live cluster, false = snapshot
}

export interface DiffRequest {
  kind: string;
  namespace: string;
  name: string;
  left: DiffSource;
  right: DiffSource;
}

export interface FieldDifference {
  path: string; // JSONPath e.g., "spec.replicas"
  leftValue: string; // Value in left source (empty if not present)
  rightValue: string; // Value in right source (empty if not present)
  category: DiffCategory;
  severity: DiffSeverity;
  changeType: ChangeType;
}

export type DiffCategory = string;

export type DiffSeverity = string;

export type ChangeType = string;

export interface DiffResult {
  request: DiffRequest;
  leftYaml: string;
  rightYaml: string;
  leftExists: boolean;
  rightExists: boolean;
  differences: FieldDifference[];
  filteredPaths: string[];
  computedAt: string;
}

export interface ApplyResult {
  success: boolean;
  dryRun: boolean;
  message: string;
  changes: string[];
  warnings: string[];
}

export interface DiffReport {
  format: "markdown" | "json";
  content: string;
  filename: string;
}

export type DiffMode = "cross-cluster" | "historical";

export interface DiffState {
  mode: DiffMode;
  left: DiffSource;
  right: DiffSource;
  kind: string;
  namespace: string;
  name: string;
  result: DiffResult | null;
  loading: boolean;
  error: string | null;
}

export interface ClusterDiffPersistentState {
  leftSource: DiffSource;
  rightSource: DiffSource;
  namespace: string;
  resourceName: string;
  resourceKind: string;
  diffResult: DiffResult | null;
}

// UI-specific types
export interface SnapshotInfo {
  timestamp: string;
}

export interface CategoryIcon {
  icon: React.ReactNode;
  label: string;
  color: string;
}

// Props for sub-components
export interface DiffViewerProps {
  leftYaml: string;
  rightYaml: string;
  leftLabel: string;
  rightLabel: string;
  differences: FieldDifference[];
}

export interface SourceSelectorProps {
  contexts: string[];
  snapshots: SnapshotInfo[];
  value: DiffSource;
  onChange: (source: DiffSource) => void;
  label: string;
  isTimelineAvailable: boolean;
  readOnly?: boolean;
}

export interface DiffSummaryProps {
  differences: FieldDifference[];
  onJumpTo: (path: string) => void;
}

export interface ApplyConfirmModalProps {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: (dryRun: boolean) => void;
  targetContext: string;
  resourceInfo: {
    kind: string;
    namespace: string;
    name: string;
  };
  differences: FieldDifference[];
  isApplying: boolean;
  applyResult: ApplyResult | null;
}

export interface ActionBarProps {
  onExport: (format: "markdown" | "json") => void;
  onApply: () => void;
  hasResult: boolean;
  isExporting: boolean;
}
