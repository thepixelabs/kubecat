import { memo } from "react";
import { Box } from "lucide-react";

interface NamespaceNodeProps {
  data: {
    label: string;
    isHighlighted?: boolean;
    isDimmed?: boolean;
  };
  selected?: boolean;
}

const NamespaceNodeComponent = ({
  data,
  selected: _selected,
}: NamespaceNodeProps) => {
  return (
    <div className="w-full h-full min-w-[200px] min-h-[100px]">
      {/* We don't really need handles for Namespace usually, but React Flow might complain or edges might target it if we define edges to it. 
            The Layout handles positioning. */}
      <div className="absolute top-0 left-0 bg-slate-100 dark:bg-slate-800 px-3 py-1 rounded-br-lg border-b border-r border-slate-200 dark:border-slate-700 flex items-center gap-2">
        <Box size={14} className="text-slate-500" />
        <span className="text-sm font-semibold text-slate-700 dark:text-slate-200">
          {data.label}
        </span>
      </div>
    </div>
  );
};

export const NamespaceNode = memo(NamespaceNodeComponent);
