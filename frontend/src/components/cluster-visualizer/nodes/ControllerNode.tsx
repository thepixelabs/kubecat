import { memo } from "react";
import { Handle, Position } from "@xyflow/react";
import { Layers, Database, Server, PlayCircle, Clock, Cpu } from "lucide-react";

interface ControllerNodeData {
  label: string;
  resourceType:
    | "Deployment"
    | "StatefulSet"
    | "DaemonSet"
    | "ReplicaSet"
    | "Job"
    | "CronJob"
    | "Operator";
  isHighlighted?: boolean;
  isUpstream?: boolean;
  isDownstream?: boolean;
  isDimmed?: boolean;
}

interface ControllerNodeProps {
  data: ControllerNodeData;
  selected?: boolean;
}

function ControllerNodeComponent({ data, selected }: ControllerNodeProps) {
  const getIcon = () => {
    switch (data.resourceType) {
      case "StatefulSet":
        return (
          <Database size={16} className="text-amber-600 dark:text-amber-400" />
        );
      case "DaemonSet":
        return (
          <Server size={16} className="text-rose-600 dark:text-rose-400" />
        );
      case "ReplicaSet":
        return (
          <Layers size={16} className="text-slate-600 dark:text-slate-400" />
        );
      case "Job":
        return (
          <PlayCircle size={16} className="text-blue-600 dark:text-blue-400" />
        );
      case "CronJob":
        return <Clock size={16} className="text-blue-600 dark:text-blue-400" />;
      case "Operator":
        return (
          <Cpu size={16} className="text-indigo-600 dark:text-indigo-400" />
        );
      case "Deployment":
      default:
        return (
          <Layers size={16} className="text-green-600 dark:text-green-400" />
        );
    }
  };

  const getBorderColor = () => {
    switch (data.resourceType) {
      case "StatefulSet":
        return "border-amber-500 bg-amber-500/10";
      case "DaemonSet":
        return "border-rose-500 bg-rose-500/10";
      case "ReplicaSet":
        return "border-slate-500 bg-slate-500/10";
      case "Job":
      case "CronJob":
        return "border-blue-500 bg-blue-500/10";
      case "Operator":
        return "border-indigo-500 bg-indigo-500/10";
      case "Deployment":
      default:
        return "border-green-500 bg-green-500/10";
    }
  };

  const getHighlightStyle = () => {
    if (data.isUpstream)
      return "ring-2 ring-cyan-400 ring-offset-2 ring-offset-white dark:ring-offset-slate-900";
    if (data.isDownstream)
      return "ring-2 ring-orange-400 ring-offset-2 ring-offset-white dark:ring-offset-slate-900";
    if (data.isDimmed) return "opacity-30";
    if (selected)
      return "ring-2 ring-accent-400 ring-offset-2 ring-offset-white dark:ring-offset-slate-900";
    return "";
  };

  return (
    <div
      className={`
        relative px-4 py-2 rounded-lg border-2 ${getBorderColor()}
        min-w-[100px] text-center transition-all duration-200
        hover:shadow-lg dark:hover:shadow-slate-900/50
        ${getHighlightStyle()}
      `}
    >
      <Handle
        type="target"
        position={Position.Top}
        className="!bg-slate-500 !w-2 !h-2"
      />

      <div className="flex flex-col items-center gap-1">
        {getIcon()}
        <span className="text-xs font-medium text-slate-700 dark:text-slate-200 truncate max-w-[80px]">
          {data.label}
        </span>
        <span className="text-[10px] text-slate-500 dark:text-slate-400">
          {data.resourceType}
        </span>
      </div>

      <Handle
        type="source"
        position={Position.Bottom}
        className="!bg-slate-500 !w-2 !h-2"
      />
    </div>
  );
}

export const ControllerNode = memo(ControllerNodeComponent);
