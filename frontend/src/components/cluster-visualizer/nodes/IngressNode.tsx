import { memo } from "react";
import { Handle, Position } from "@xyflow/react";
import { Globe } from "lucide-react";

interface IngressNodeData {
  label: string;
  isHighlighted?: boolean;
  isUpstream?: boolean;
  isDownstream?: boolean;
  isDimmed?: boolean;
}

interface IngressNodeProps {
  data: IngressNodeData;
  selected?: boolean;
}

function IngressNodeComponent({ data, selected }: IngressNodeProps) {
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
        relative px-4 py-3 bg-purple-500/10 border-2 border-purple-500
        min-w-[100px] text-center transition-all duration-200
        hover:shadow-lg dark:hover:shadow-purple-900/30
        ${getHighlightStyle()}
      `}
      style={{
        clipPath: "polygon(50% 0%, 100% 50%, 50% 100%, 0% 50%)",
        padding: "16px 28px",
      }}
    >
      <Handle
        type="target"
        position={Position.Top}
        className="!bg-purple-500 !w-2 !h-2"
      />

      <div className="flex flex-col items-center gap-1">
        <Globe size={16} className="text-purple-600 dark:text-purple-400" />
        <span className="text-xs font-medium text-slate-700 dark:text-slate-200 truncate max-w-[80px]">
          {data.label}
        </span>
      </div>

      <Handle
        type="source"
        position={Position.Bottom}
        className="!bg-purple-500 !w-2 !h-2"
      />
    </div>
  );
}

export const IngressNode = memo(IngressNodeComponent);
