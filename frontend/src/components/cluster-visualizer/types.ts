// TypeScript types for ClusterVisualizer

export type NodeType =
  | "Pod"
  | "Service"
  | "Ingress"
  | "Deployment"
  | "StatefulSet"
  | "DaemonSet"
  | "Node"
  | "ReplicaSet"
  | "Job"
  | "CronJob"
  | "Operator"
  | "Namespace"
  | "Placement"; // Synthetic node for split attributes

export type EdgeType =
  | "service-to-pod"
  | "ingress-to-service"
  | "controller-to-pod"
  | "node-to-pod"
  | "operator-to-managed";

export interface ClusterNode {
  id: string; // "Kind/namespace/name"
  type: NodeType;
  name: string;
  namespace: string;
  status: string;
  labels?: Record<string, string>;
  // Pod-specific
  restarts?: number;
  node?: string;
  readyContainers?: string;
  // Service-specific
  serviceType?: string;
  clusterIP?: string;
  ports?: string;
  // Node-specific
  cpuCapacity?: string;
  memCapacity?: string;
  cpuAllocatable?: string;
  memAllocatable?: string;
  nodeConditions?: string[];
  // Common
  age?: string;

  // Nesting
  parentId?: string;
  extent?: "parent";
  expandParent?: boolean;
}

export interface ClusterEdge {
  id: string;
  source: string; // nodeId
  target: string; // nodeId
  edgeType: EdgeType;
  label?: string;
}

export interface ClusterGraphData {
  nodes: ClusterNode[];
  edges: ClusterEdge[];
}

// React Flow node data
export interface CustomNodeData {
  label: string;
  resourceType: NodeType;
  status: string;
  namespace: string;
  restarts?: number;
  serviceType?: string;
  // Node stats
  cpuCapacity?: string;
  memCapacity?: string;
  cpuAllocatable?: string;
  memAllocatable?: string;

  isHighlighted?: boolean;
  isUpstream?: boolean;
  isDownstream?: boolean;
  isDimmed?: boolean;
}

// Toggle state for filtering
export interface ToggleState {
  showPods: boolean;
  showServices: boolean;
  showIngresses: boolean;
  showDeployments: boolean;
  showStatefulSets: boolean;
  showDaemonSets: boolean;
  showServiceToPod: boolean;
  showIngressToService: boolean;
  showControllerToPod: boolean;
  showNodes: boolean;
  showReplicaSets: boolean;
  showJobs: boolean;
  showCronJobs: boolean;
  showOperators: boolean;
  showNodeToPod: boolean;
  showOperatorToManaged: boolean;
}

export const defaultToggleState: ToggleState = {
  showPods: true,
  showServices: true,
  showIngresses: true,
  showDeployments: true,
  showStatefulSets: true,
  showDaemonSets: true,
  showServiceToPod: true,
  showIngressToService: true,
  showControllerToPod: true,
  showNodes: true,
  showReplicaSets: false,
  showJobs: false,
  showCronJobs: false,
  showOperators: true,
  showNodeToPod: true,
  showOperatorToManaged: true,
};
