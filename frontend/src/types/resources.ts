/**
 * Shared TypeScript interfaces for Kubernetes resource types.
 * Used across Explorer, Security, and other views.
 */

export type View =
  | "dashboard"
  | "explorer"
  | "logs"
  | "timeline"
  | "gitops"
  | "security"
  | "portforwards"
  | "query"
  | "analyzer"
  | "visualizer"
  | "diff"
  | "rbac"
  | "costs";

export interface ResourceInfo {
  kind: string;
  name: string;
  namespace: string;
  status: string;
  age: string;
  labels?: Record<string, string>;
  apiVersion?: string;
  // Workload info
  replicas?: string;
  restarts?: number;
  node?: string;
  qosClass?: string;
  readyContainers?: string;
  images?: string[];
  cpuRequest?: string;
  cpuLimit?: string;
  memRequest?: string;
  memLimit?: string;
  // Service info
  serviceType?: string;
  clusterIP?: string;
  externalIP?: string;
  ports?: string;
  // PVC info
  storageClass?: string;
  capacity?: string;
  accessModes?: string;
  // Ingress info
  ingressClass?: string;
  hosts?: string;
  paths?: string;
  tlsHosts?: string;
  backends?: string;
  // Service selectors
  selectors?: string;
  // ConfigMap/Secret data keys
  dataKeys?: string[];
  dataCount?: number;
  // Owner info
  ownerKind?: string;
  ownerName?: string;
  // Probes
  hasLiveness?: boolean;
  hasReadiness?: boolean;
  // Security
  securityIssues?: string[];
  volumes?: string[];
  // Node-specific info
  cpuAllocatable?: string;
  memAllocatable?: string;
  cpuCapacity?: string;
  memCapacity?: string;
  podCount?: number;
  podCapacity?: number;
  nodeConditions?: string[];
  kubeletVersion?: string;
  containerRuntime?: string;
  osImage?: string;
  architecture?: string;
  taints?: string[];
  unschedulable?: boolean;
  internalIP?: string;
  externalIPNode?: string;
  roles?: string;
}

export interface NodeMetricsInfo {
  nodeName: string;
  podCount: number;
  cpuRequests: string;
  cpuLimits: string;
  memRequests: string;
  memLimits: string;
  cpuAllocatable: string;
  memAllocatable: string;
  cpuRequestPct: number;
  memRequestPct: number;
  cpuLimitPct: number;
  memLimitPct: number;
}

export interface ProviderInfo {
  id: string;
  name: string;
  requiresApiKey: boolean;
  defaultEndpoint: string;
  defaultModel: string;
  models: string[];
}

export interface PortForwardInfo {
  id: string;
  namespace: string;
  pod: string;
  localPort: number;
  remotePort: number;
  status: string;
  error?: string;
}

export interface SelectedPod {
  name: string;
  namespace: string;
  container?: string;
  kind?: string;
}

export interface RBACBinding {
  kind: string;
  name: string;
  namespace?: string;
  role: string;
  subjects: RBACSubject[];
}

export interface RBACSubject {
  kind: string;
  name: string;
  namespace?: string;
}

export interface SecurityFinding {
  severity: "critical" | "high" | "medium" | "low" | "info";
  category: string;
  resource: string;
  namespace?: string;
  message: string;
  remediation?: string;
}

export interface ClusterScanResult {
  findings: SecurityFinding[];
  scannedAt: string;
  totalResources: number;
}

export interface TimelineEvent {
  id: string;
  timestamp: string;
  kind: string;
  name: string;
  namespace?: string;
  reason: string;
  message: string;
  type: "Normal" | "Warning";
}

export interface SnapshotInfo {
  timestamp: string;
  label?: string;
}

export interface AISettings {
  provider: string;
  apiKey?: string;
  endpoint?: string;
  model?: string;
  ollamaEndpoint?: string;
  ollamaModel?: string;
  cloudProvider?: string;
  cloudModel?: string;
  cloudApiKey?: string;
  localProvider?: string;
  localModel?: string;
  localEndpoint?: string;
}

export interface CommandResult {
  stdout: string;
  stderr: string;
  exitCode: number;
}

export type SortDirection = "asc" | "desc";

export type ResourceKind =
  | "pods"
  | "deployments"
  | "statefulsets"
  | "daemonsets"
  | "replicasets"
  | "services"
  | "ingresses"
  | "persistentvolumeclaims"
  | "configmaps"
  | "secrets"
  | "nodes"
  | "namespaces"
  | "events"
  | "jobs"
  | "cronjobs"
  | "serviceaccounts"
  | "roles"
  | "rolebindings"
  | "clusterroles"
  | "clusterrolebindings";
