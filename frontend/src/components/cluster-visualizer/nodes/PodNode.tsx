import { memo } from "react";
import { Handle, Position } from "@xyflow/react";
import { Box } from "lucide-react";

interface PodNodeData {
  label: string;
  status: string;
  restarts?: number;
  isHighlighted?: boolean;
  isUpstream?: boolean;
  isDownstream?: boolean;
  isDimmed?: boolean;
}

interface PodNodeProps {
  data: PodNodeData;
  selected?: boolean;
}

function PodNodeComponent({ data, selected }: PodNodeProps) {
  const getStatusColor = () => {
    switch (data.status?.toLowerCase()) {
      case "running":
        return "bg-emerald-500";
      case "pending":
        return "bg-amber-500";
      case "failed":
      case "error":
        return "bg-red-500";
      case "succeeded":
        return "bg-blue-500";
      default:
        return "bg-slate-500";
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
        relative px-3 py-2 rounded-full bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-600
        min-w-[80px] text-center transition-all duration-200
        hover:border-slate-400 dark:hover:border-slate-500 hover:shadow-lg dark:hover:shadow-slate-900/50
        ${getHighlightStyle()}
      `}
    >
      <Handle
        type="target"
        position={Position.Top}
        className="!bg-slate-500 !w-2 !h-2"
      />

      <div className="flex items-center gap-2 justify-center">
        <div className={`w-2 h-2 rounded-full ${getStatusColor()}`} />
        <Box size={14} className="text-slate-500 dark:text-slate-400" />
        <span className="text-xs font-medium text-slate-700 dark:text-slate-200 truncate max-w-[100px]">
          {data.label}
        </span>
      </div>

      {data.restarts !== undefined && data.restarts > 0 && (
        <div className="absolute -top-1 -right-1 bg-amber-500 text-xs text-white dark:text-slate-900 rounded-full w-4 h-4 flex items-center justify-center font-bold">
          {data.restarts}
        </div>
      )}

      <Handle
        type="source"
        position={Position.Bottom}
        className="!bg-slate-500 !w-2 !h-2"
      />
    </div>
  );
}

export const PodNode = memo(PodNodeComponent);
