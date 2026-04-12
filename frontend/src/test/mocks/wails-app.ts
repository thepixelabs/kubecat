import { vi } from 'vitest';
import type { ClusterNode, ClusterEdge } from '../../components/cluster-visualizer/types';

// Mock data generators
export const mockPodResource = (name: string, namespace: string, status = 'Running') => ({
  kind: 'Pod',
  name,
  namespace,
  status,
  age: '1d',
  labels: {},
  restarts: 0,
  node: 'node-1',
  readyContainers: '1/1',
});

export const mockServiceResource = (name: string, namespace: string) => ({
  kind: 'Service',
  name,
  namespace,
  status: 'Active',
  age: '1d',
  serviceType: 'ClusterIP',
  clusterIP: '10.0.0.1',
  ports: '80/TCP',
});

export const mockDeploymentResource = (name: string, namespace: string) => ({
  kind: 'Deployment',
  name,
  namespace,
  status: '1/1',
  age: '1d',
  labels: {},
});

export const mockClusterNode = (type: ClusterNode['type'], name: string, namespace: string): ClusterNode => ({
  id: `${type}/${namespace}/${name}`,
  type,
  name,
  namespace,
  status: type === 'Pod' ? 'Running' : 'Active',
});

export const mockClusterEdge = (
  source: string,
  target: string,
  edgeType: ClusterEdge['edgeType']
): ClusterEdge => ({
  id: `${source}-${target}`,
  source,
  target,
  edgeType,
});

// Default mock responses
const defaultMockResponses = {
  pods: [
    mockPodResource('web-pod-1', 'default'),
    mockPodResource('web-pod-2', 'default'),
    mockPodResource('api-pod-1', 'default'),
  ],
  services: [
    mockServiceResource('web-service', 'default'),
    mockServiceResource('api-service', 'default'),
  ],
  deployments: [mockDeploymentResource('web-deployment', 'default')],
  statefulsets: [],
  daemonsets: [],
  replicasets: [],
  jobs: [],
  cronjobs: [],
  nodes: [
    {
      kind: 'Node',
      name: 'node-1',
      namespace: '',
      status: 'Ready',
      cpuCapacity: '4',
      memCapacity: '8Gi',
      cpuAllocatable: '3.5',
      memAllocatable: '7Gi',
      nodeConditions: ['Ready'],
    },
  ],
  ingresses: [],
};

const defaultEdges: ClusterEdge[] = [
  mockClusterEdge('Service/default/web-service', 'Pod/default/web-pod-1', 'service-to-pod'),
  mockClusterEdge('Service/default/web-service', 'Pod/default/web-pod-2', 'service-to-pod'),
  mockClusterEdge(
    'Deployment/default/web-deployment',
    'Pod/default/web-pod-1',
    'controller-to-pod'
  ),
];

// Wails App binding mocks
export const createWailsAppMocks = (overrides: Partial<typeof defaultMockResponses> = {}) => {
  const responses = { ...defaultMockResponses, ...overrides };

  return {
    ListResources: vi.fn().mockImplementation((kind: string, _namespace: string) => {
      const resourceMap: Record<string, any[]> = {
        pods: responses.pods,
        services: responses.services,
        deployments: responses.deployments,
        statefulsets: responses.statefulsets,
        daemonsets: responses.daemonsets,
        replicasets: responses.replicasets,
        jobs: responses.jobs,
        cronjobs: responses.cronjobs,
        nodes: responses.nodes,
        ingresses: responses.ingresses,
      };
      return Promise.resolve(resourceMap[kind] || []);
    }),

    GetClusterEdges: vi.fn().mockResolvedValue(defaultEdges),

    GetContexts: vi.fn().mockResolvedValue([
      { name: 'minikube', current: true },
      { name: 'production', current: false },
    ]),

    Connect: vi.fn().mockResolvedValue(true),
    Disconnect: vi.fn().mockResolvedValue(true),
    IsConnected: vi.fn().mockResolvedValue(true),

    GetClusterInfo: vi.fn().mockResolvedValue({
      name: 'test-cluster',
      version: 'v1.28.0',
      nodes: 3,
    }),

    GetResource: vi.fn().mockResolvedValue({ spec: {}, status: {} }),
    GetResourceYAML: vi.fn().mockResolvedValue('apiVersion: v1\nkind: Pod'),

    ComputeDiff: vi.fn().mockResolvedValue({
      leftYaml: 'apiVersion: v1\nkind: Deployment',
      rightYaml: 'apiVersion: v1\nkind: Deployment',
      differences: [],
    }),

    GetSnapshots: vi.fn().mockResolvedValue([]),
    IsTimelineAvailable: vi.fn().mockResolvedValue(false),

    AnalyzeResource: vi.fn().mockResolvedValue({ suggestions: [] }),
    AIAnalyzeResource: vi.fn().mockResolvedValue({ analysis: '' }),
    AIQuery: vi.fn().mockResolvedValue({ response: '' }),

    ApplyResourceToCluster: vi.fn().mockResolvedValue({ success: true }),
    GenerateDiffReport: vi.fn().mockResolvedValue({ content: '', filename: '' }),

    GetSecuritySummary: vi.fn().mockResolvedValue({ issues: [] }),
    GetSecurityIssues: vi.fn().mockResolvedValue([]),
    ScanCluster: vi.fn().mockResolvedValue({ findings: [] }),

    GetRBACAnalysis: vi.fn().mockResolvedValue({ roles: [], bindings: [] }),
    GetSubjectPermissions: vi.fn().mockResolvedValue({ permissions: [] }),
  };
};

// Setup global mock
export const setupWailsAppMock = (overrides: Partial<typeof defaultMockResponses> = {}) => {
  const mocks = createWailsAppMocks(overrides);

  vi.mock('../../../wailsjs/go/main/App', () => mocks);

  return mocks;
};

