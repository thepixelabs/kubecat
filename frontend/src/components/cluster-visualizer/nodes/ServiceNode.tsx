import { memo } from "react";
import { Handle, Position } from "@xyflow/react";
import { Hexagon } from "lucide-react";

interface ServiceNodeData {
  label: string;
  serviceType?: string;
  isHighlighted?: boolean;
  isUpstream?: boolean;
  isDownstream?: boolean;
  isDimmed?: boolean;
}

interface ServiceNodeProps {
  data: ServiceNodeData;
  selected?: boolean;
}

function ServiceNodeComponent({ data, selected }: ServiceNodeProps) {
  const getTypeColor = () => {
    switch (data.serviceType?.toLowerCase()) {
      case "loadbalancer":
        return "border-purple-500 bg-purple-500/10";
      case "nodeport":
        return "border-blue-500 bg-blue-500/10";
      case "clusterip":
      default:
        return "border-cyan-500 bg-cyan-500/10";
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
        relative px-4 py-2 rounded-lg border-2 ${getTypeColor()}
        min-w-[100px] text-center transition-all duration-200
        hover:shadow-lg dark:hover:shadow-cyan-900/30
        ${getHighlightStyle()}
      `}
      style={{
        clipPath:
          "polygon(25% 0%, 75% 0%, 100% 50%, 75% 100%, 25% 100%, 0% 50%)",
        padding: "12px 24px",
      }}
    >
      <Handle
        type="target"
        position={Position.Top}
        className="!bg-cyan-500 !w-2 !h-2"
      />

      <div className="flex flex-col items-center gap-1">
        <Hexagon size={16} className="text-cyan-600 dark:text-cyan-400" />
        <span
          className="text-xs font-medium text-slate-700 dark:text-slate-200 truncate max-w-[80px]"
          title={data.label}
        >
          {data.label}
        </span>
        {data.serviceType && (
          <span className="text-[10px] text-slate-500 dark:text-slate-400 uppercase">
            {data.serviceType}
          </span>
        )}
      </div>

      <Handle
        type="source"
        position={Position.Bottom}
        className="!bg-cyan-500 !w-2 !h-2"
      />
    </div>
  );
}

export const ServiceNode = memo(ServiceNodeComponent);
