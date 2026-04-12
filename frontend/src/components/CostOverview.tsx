import { useState, useEffect } from "react";
import { DollarSign, RefreshCw, Loader2, TrendingUp, Cpu, MemoryStick } from "lucide-react";
import { GetNamespaceCostSummary } from "../../wailsjs/go/main/App";

interface CostEstimate {
  workloadName: string;
  cpuCost: number;
  memoryCost: number;
  totalCost: number;
  monthlyTotal: number;
  currency: string;
  source: string;
}

interface NamespaceCostSummary {
  namespace: string;
  totalPerHour: number;
  totalPerMonth: number;
  currency: string;
  source: string;
  workloads: CostEstimate[];
}

function Bar({ value, max }: { value: number; max: number }) {
  const pct = max > 0 ? Math.min((value / max) * 100, 100) : 0;
  return (
    <div className="w-full bg-stone-100 dark:bg-slate-700 rounded-full h-1.5">
      <div
        className="bg-accent-500 h-1.5 rounded-full transition-all duration-300"
        style={{ width: `${pct}%` }}
      />
    </div>
  );
}

export function CostOverview({
  activeContext,
  namespace,
}: {
  activeContext: string;
  namespace: string;
}) {
  const [summary, setSummary] = useState<NamespaceCostSummary | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const s = await GetNamespaceCostSummary(activeContext, namespace);
      setSummary(s as NamespaceCostSummary);
    } catch (e: any) {
      setError(e?.message ?? String(e));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, [activeContext, namespace]);

  const sorted = summary?.workloads
    ?.slice()
    .sort((a, b) => b.monthlyTotal - a.monthlyTotal)
    .slice(0, 10) ?? [];

  const maxCost = sorted[0]?.monthlyTotal ?? 1;

  return (
    <div className="h-full flex flex-col">
      <div className="flex items-center gap-3 mb-4">
        <TrendingUp className="w-5 h-5 text-accent-400" />
        <h2 className="text-lg font-semibold text-stone-800 dark:text-slate-100">Cost Overview</h2>
        {summary && (
          <span className="ml-auto text-xs text-stone-500 dark:text-slate-400">
            Source: {summary.source}
          </span>
        )}
        <button
          onClick={load}
          disabled={loading}
          className="p-2 rounded-lg hover:bg-stone-100 dark:hover:bg-slate-700 text-stone-500 dark:text-slate-400 transition-colors"
        >
          {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <RefreshCw className="w-4 h-4" />}
        </button>
      </div>

      {error && (
        <div className="mb-4 px-3 py-2 bg-red-500/10 border border-red-500/30 rounded-lg text-sm text-red-500">
          {error}
        </div>
      )}

      {/* Summary cards */}
      {summary && (
        <div className="grid grid-cols-2 gap-3 mb-4">
          <div className="bg-white dark:bg-slate-800 border border-stone-200 dark:border-slate-700 rounded-xl p-4">
            <div className="flex items-center gap-2 text-xs text-stone-500 dark:text-slate-400 mb-1">
              <DollarSign className="w-3.5 h-3.5" />
              Monthly Est.
            </div>
            <div className="text-2xl font-bold text-stone-800 dark:text-slate-100">
              ${summary.totalPerMonth.toFixed(2)}
            </div>
            <div className="text-xs text-stone-400 dark:text-slate-500 mt-0.5">{summary.currency}</div>
          </div>
          <div className="bg-white dark:bg-slate-800 border border-stone-200 dark:border-slate-700 rounded-xl p-4">
            <div className="flex items-center gap-2 text-xs text-stone-500 dark:text-slate-400 mb-1">
              <DollarSign className="w-3.5 h-3.5" />
              Hourly Est.
            </div>
            <div className="text-2xl font-bold text-stone-800 dark:text-slate-100">
              ${summary.totalPerHour.toFixed(4)}
            </div>
            <div className="text-xs text-stone-400 dark:text-slate-500 mt-0.5">per hour</div>
          </div>
        </div>
      )}

      {/* Top workloads */}
      <div className="flex-1 overflow-auto">
        <h3 className="text-xs font-medium text-stone-500 dark:text-slate-400 uppercase tracking-wide mb-2">
          Top workloads by cost
        </h3>
        {loading && !summary && (
          <div className="flex items-center justify-center py-12">
            <Loader2 className="w-5 h-5 animate-spin text-stone-400" />
          </div>
        )}
        <div className="space-y-2">
          {sorted.map((w, i) => (
            <div key={i} className="bg-white dark:bg-slate-800 border border-stone-200 dark:border-slate-700 rounded-lg p-3">
              <div className="flex items-center justify-between mb-1.5">
                <span className="text-sm font-mono text-stone-700 dark:text-slate-200 truncate max-w-[60%]">
                  {w.workloadName}
                </span>
                <span className="text-sm font-medium text-stone-800 dark:text-slate-100">
                  ${w.monthlyTotal.toFixed(3)}/mo
                </span>
              </div>
              <Bar value={w.monthlyTotal} max={maxCost} />
              <div className="flex gap-3 mt-1.5 text-[10px] text-stone-400 dark:text-slate-500">
                <span className="flex items-center gap-0.5">
                  <Cpu className="w-2.5 h-2.5" /> ${(w.cpuCost * 730).toFixed(3)}
                </span>
                <span className="flex items-center gap-0.5">
                  <MemoryStick className="w-2.5 h-2.5" /> ${(w.memoryCost * 730).toFixed(3)}
                </span>
              </div>
            </div>
          ))}
        </div>
      </div>

      <p className="mt-2 text-xs text-stone-400 dark:text-slate-500">
        Estimates based on resource requests × pricing defaults. Enable OpenCost for accurate billing.
      </p>
    </div>
  );
}
