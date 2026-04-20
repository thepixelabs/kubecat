import { memo } from "react";
import { Handle, Position } from "@xyflow/react";
import { Server as ServerIcon, Cpu, HardDrive } from "lucide-react";

interface ServerNodeData {
  label: string;
  status: string;
  cpuCapacity?: string;
  memCapacity?: string;
  cpuAllocatable?: string;
  memAllocatable?: string;
  isHighlighted?: boolean;
  isUpstream?: boolean;
  isDownstream?: boolean;
  isDimmed?: boolean;
}

interface ServerNodeProps {
  data: ServerNodeData;
  selected?: boolean;
}

function ServerNodeComponent({ data, selected }: ServerNodeProps) {
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

  const isReady = data.status?.toLowerCase().includes("ready");
  const borderColor = isReady ? "border-indigo-500" : "border-red-500";
  const bgColor = isReady ? "bg-indigo-500/10" : "bg-red-500/10";

  return (
    <div
      className={`
        relative px-4 py-3 rounded-md border-2 ${borderColor} ${bgColor}
        min-w-[140px] text-center transition-all duration-200
        hover:shadow-lg dark:hover:shadow-indigo-900/30
        ${getHighlightStyle()}
      `}
    >
      <Handle
        type="target"
        position={Position.Top}
        className="!bg-indigo-500 !w-2 !h-2"
      />

      <div className="flex flex-col items-center gap-2">
        <div className="flex items-center gap-2 border-b border-indigo-500/30 pb-1 w-full justify-center">
          <ServerIcon
            size={16}
            className="text-indigo-600 dark:text-indigo-400"
          />
          <span
            className="text-sm font-semibold text-slate-700 dark:text-slate-200 truncate max-w-[120px]"
            title={data.label}
          >
            {data.label}
          </span>
        </div>

        <div className="flex gap-3 text-[10px] text-slate-600 dark:text-slate-400 w-full justify-between px-1">
          <div className="flex items-center gap-1" title="CPU Capacity">
            <Cpu size={10} />
            <span>{data.cpuCapacity || "N/A"}</span>
          </div>
          <div className="flex items-center gap-1" title="Memory Capacity">
            <HardDrive size={10} />
            <span>{data.memCapacity || "N/A"}</span>
          </div>
        </div>
      </div>

      <Handle
        type="source"
        position={Position.Bottom}
        className="!bg-indigo-500 !w-2 !h-2"
      />
    </div>
  );
}

export const ServerNode = memo(ServerNodeComponent);
