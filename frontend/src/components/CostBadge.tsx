import { useState } from "react";
import { DollarSign } from "lucide-react";

interface CostEstimate {
  workloadName: string;
  namespace: string;
  cpuCost: number;
  memoryCost: number;
  totalCost: number;
  monthlyTotal: number;
  currency: string;
  source: string;
}

interface CostBadgeProps {
  estimate: CostEstimate | null;
  loading?: boolean;
}

export function CostBadge({ estimate, loading }: CostBadgeProps) {
  const [showTooltip, setShowTooltip] = useState(false);

  if (loading) {
    return (
      <span className="text-[10px] text-stone-400 dark:text-slate-500 animate-pulse">
        $…
      </span>
    );
  }

  if (!estimate) return null;

  const formatted = estimate.monthlyTotal < 1
    ? `$${(estimate.monthlyTotal * 100).toFixed(1)}¢/mo`
    : `$${estimate.monthlyTotal.toFixed(2)}/mo`;

  return (
    <div
      className="relative inline-flex items-center gap-0.5"
      onMouseEnter={() => setShowTooltip(true)}
      onMouseLeave={() => setShowTooltip(false)}
    >
      <span className="text-[10px] flex items-center gap-0.5 text-stone-400 dark:text-slate-500 cursor-default">
        <DollarSign className="w-2.5 h-2.5" />
        {formatted}
      </span>
      {showTooltip && (
        <div className="absolute bottom-full left-0 mb-1 z-50 w-44 bg-slate-900 text-white text-xs rounded-lg p-2 shadow-lg">
          <div className="font-medium mb-1">{estimate.workloadName}</div>
          <div className="flex justify-between">
            <span className="text-slate-400">CPU</span>
            <span>${(estimate.cpuCost * 730).toFixed(3)}/mo</span>
          </div>
          <div className="flex justify-between">
            <span className="text-slate-400">Memory</span>
            <span>${(estimate.memoryCost * 730).toFixed(3)}/mo</span>
          </div>
          <div className="flex justify-between border-t border-slate-700 mt-1 pt-1 font-medium">
            <span>Total</span>
            <span>${estimate.monthlyTotal.toFixed(3)}/mo</span>
          </div>
          <div className="text-slate-500 mt-1 text-[10px]">
            Source: {estimate.source}
          </div>
        </div>
      )}
    </div>
  );
}
