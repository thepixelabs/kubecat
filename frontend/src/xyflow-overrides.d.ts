// Type shim for @xyflow/react — the published npm package has broken
// declaration files that reference source paths (../../src/) not included in
// the bundle.  Until the upstream package is fixed, we declare the module as
// any to allow the TypeScript compiler to pass while Vite/ESM resolves the
// actual runtime code correctly at build/dev time.
declare module "@xyflow/react" {
  import type { ComponentType, CSSProperties, ReactNode } from "react";

  // Core component
  export const ReactFlow: ComponentType<Record<string, unknown>>;

  // Additional components
  export const Background: ComponentType<Record<string, unknown>>;
  export const Controls: ComponentType<Record<string, unknown>>;
  export const MiniMap: ComponentType<Record<string, unknown>>;
  export const Panel: ComponentType<Record<string, unknown>>;
  export const Handle: ComponentType<Record<string, unknown>>;
  export const BaseEdge: ComponentType<Record<string, unknown>>;
  export const ReactFlowProvider: ComponentType<{ children: ReactNode }>;
  export const EdgeLabelRenderer: ComponentType<{ children: ReactNode }>;
  export const ViewportPortal: ComponentType<{ children: ReactNode }>;

  // Hooks
  export function useNodesState<T extends Node = Node>(initialNodes: T[]): [T[], (nodes: T[]) => void, (changes: NodeChange[]) => void];
  export function useEdgesState<T extends Edge = Edge>(initialEdges: T[]): [T[], (edges: T[]) => void, (changes: EdgeChange[]) => void];
  export function useReactFlow(): Record<string, unknown>;
  export function useNodes(): Node[];
  export function useEdges(): Edge[];
  export function useStore<T>(selector: (store: Record<string, unknown>) => T): T;
  export function useStoreApi(): Record<string, unknown>;
  export function useNodeId(): string | null;
  export function useUpdateNodeInternals(): (nodeId: string) => void;
  export function useInternalNode(nodeId: string): Record<string, unknown> | undefined;
  export function useNodeConnections(params?: Record<string, unknown>): Record<string, unknown>[];
  export function useNodesData<T extends Record<string, unknown>>(nodeId: string): T | undefined;
  export function useNodesInitialized(): boolean;
  export function useKeyPress(keyOrCode: string | string[]): boolean;
  export function useViewport(): Viewport;
  export function useHandleConnections(params?: Record<string, unknown>): Record<string, unknown>[];
  export function useOnViewportChange(options: Record<string, unknown>): void;
  export function useOnSelectionChange(options: Record<string, unknown>): void;
  export function useConnection(): Record<string, unknown>;

  // Path utilities
  export function getSmoothStepPath(params: Record<string, unknown>): [string, number, number, number, number];
  export function getBezierPath(params: Record<string, unknown>): [string, number, number, number, number];
  export function getStraightPath(params: Record<string, unknown>): [string, number, number, number, number];
  export function getSimpleBezierPath(params: Record<string, unknown>): [string, number, number, number, number];

  // Change utilities
  export function applyNodeChanges(changes: NodeChange[], nodes: Node[]): Node[];
  export function applyEdgeChanges(changes: EdgeChange[], edges: Edge[]): Edge[];
  export function addEdge(edge: Partial<Edge>, edges: Edge[]): Edge[];

  // Types
  export type Node<
    NodeData extends Record<string, unknown> = Record<string, unknown>,
    NodeType extends string | undefined = string | undefined
  > = {
    id: string;
    position: { x: number; y: number };
    data: NodeData;
    type?: NodeType;
    style?: CSSProperties;
    className?: string;
    width?: number;
    height?: number;
    selected?: boolean;
    dragging?: boolean;
    draggable?: boolean;
    selectable?: boolean;
    connectable?: boolean;
    deletable?: boolean;
    hidden?: boolean;
    parentId?: string;
    zIndex?: number;
    extent?: "parent" | [[number, number], [number, number]];
    expandParent?: boolean;
    sourcePosition?: Position;
    targetPosition?: Position;
    [key: string]: unknown;
  };

  export type Edge<
    EdgeData extends Record<string, unknown> = Record<string, unknown>,
    EdgeType extends string | undefined = string | undefined
  > = {
    id: string;
    source: string;
    target: string;
    type?: EdgeType;
    data?: EdgeData;
    style?: CSSProperties;
    className?: string;
    label?: string;
    animated?: boolean;
    hidden?: boolean;
    selected?: boolean;
    deletable?: boolean;
    selectable?: boolean;
    sourceHandle?: string;
    targetHandle?: string;
    [key: string]: unknown;
  };

  export type NodeChange = Record<string, unknown>;
  export type EdgeChange = Record<string, unknown>;
  export type Connection = { source: string; target: string; sourceHandle?: string | null; targetHandle?: string | null };
  export type OnConnect = (connection: Connection) => void;
  export type OnNodesChange = (changes: NodeChange[]) => void;
  export type OnEdgesChange = (changes: EdgeChange[]) => void;
  export type Viewport = { x: number; y: number; zoom: number };
  export type FitViewOptions = Record<string, unknown>;
  export type ReactFlowInstance = Record<string, unknown>;
  export type ReactFlowProps = Record<string, unknown>;
  export type NodeProps<T extends Record<string, unknown> = Record<string, unknown>> = {
    id: string;
    data: T;
    selected?: boolean;
    dragging?: boolean;
    [key: string]: unknown;
  };
  export type EdgeProps<T extends Record<string, unknown> = Record<string, unknown>> = {
    id: string;
    source: string;
    target: string;
    data?: T;
    selected?: boolean;
    animated?: boolean;
    [key: string]: unknown;
  };

  // Enums
  export enum Position {
    Left = "left",
    Right = "right",
    Top = "top",
    Bottom = "bottom",
  }

  export enum BackgroundVariant {
    Lines = "lines",
    Dots = "dots",
    Cross = "cross",
  }

  export enum MarkerType {
    Arrow = "arrow",
    ArrowClosed = "arrowclosed",
  }
}
