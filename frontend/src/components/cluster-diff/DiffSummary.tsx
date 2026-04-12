import { motion } from "framer-motion";
import {
  Layers,
  Image,
  Settings,
  Cpu,
  Tag,
  FileText,
  Box,
  AlertTriangle,
  Info,
  AlertCircle,
  ArrowRight,
  Check,
} from "lucide-react";
import type { DiffSummaryProps, FieldDifference, DiffCategory } from "./types";

const categoryConfig: Record<
  DiffCategory,
  { icon: React.ReactNode; label: string; color: string }
> = {
  replicas: {
    icon: <Layers className="w-4 h-4" />,
    label: "Replicas",
    color: "text-blue-400",
  },
  image: {
    icon: <Image className="w-4 h-4" />,
    label: "Container Image",
    color: "text-purple-400",
  },
  env: {
    icon: <Settings className="w-4 h-4" />,
    label: "Environment",
    color: "text-amber-400",
  },
  limits: {
    icon: <Cpu className="w-4 h-4" />,
    label: "Resource Limits",
    color: "text-teal-400",
  },
  labels: {
    icon: <Tag className="w-4 h-4" />,
    label: "Labels",
    color: "text-pink-400",
  },
  annotations: {
    icon: <FileText className="w-4 h-4" />,
    label: "Annotations",
    color: "text-cyan-400",
  },
  container: {
    icon: <Box className="w-4 h-4" />,
    label: "Container",
    color: "text-green-400",
  },
  config: {
    icon: <Settings className="w-4 h-4" />,
    label: "Configuration",
    color: "text-orange-400",
  },
  existence: {
    icon: <AlertCircle className="w-4 h-4" />,
    label: "Resource Existence",
    color: "text-red-400",
  },
  other: {
    icon: <FileText className="w-4 h-4" />,
    label: "Other",
    color: "text-stone-400 dark:text-slate-400",
  },
};

const severityConfig = {
  critical: {
    icon: <AlertCircle className="w-3.5 h-3.5" />,
    color: "text-red-400 bg-red-500/10 border-red-500/30",
  },
  warning: {
    icon: <AlertTriangle className="w-3.5 h-3.5" />,
    color: "text-amber-400 bg-amber-500/10 border-amber-500/30",
  },
  info: {
    icon: <Info className="w-3.5 h-3.5" />,
    color: "text-blue-400 bg-blue-500/10 border-blue-500/30",
  },
};

export function DiffSummary({ differences, onJumpTo }: DiffSummaryProps) {
  // Group differences by category
  const groupedDiffs = differences.reduce((acc, diff) => {
    const category = diff.category as DiffCategory;
    if (!acc[category]) {
      acc[category] = [];
    }
    acc[category].push(diff);
    return acc;
  }, {} as Record<DiffCategory, FieldDifference[]>);

  // Count by severity
  const severityCounts = differences.reduce((acc, diff) => {
    acc[diff.severity] = (acc[diff.severity] || 0) + 1;
    return acc;
  }, {} as Record<string, number>);

  if (differences.length === 0) {
    return (
      <motion.div
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        className="flex items-center gap-2 p-3 bg-stone-50/50 dark:bg-slate-800/30 rounded-lg border border-stone-200 dark:border-slate-700 text-stone-600 dark:text-slate-400"
      >
        <Check className="w-5 h-5 text-emerald-500" />
        <span className="font-medium">No Differences Found</span>
        <span className="text-stone-400 dark:text-slate-500 mx-2">|</span>
        <span className="text-sm">The resources are identical</span>
      </motion.div>
    );
  }

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      className="flex flex-col gap-4"
    >
      {/* Summary header */}
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-medium text-stone-700 dark:text-slate-300">
          {differences.length} Difference{differences.length !== 1 ? "s" : ""}{" "}
          Found
        </h3>
        <div className="flex items-center gap-2">
          {Object.entries(severityCounts).map(([severity, count]) => (
            <span
              key={severity}
              className={`flex items-center gap-1 px-2 py-0.5 rounded-full text-xs border ${
                severityConfig[severity as keyof typeof severityConfig]?.color
              }`}
            >
              {severityConfig[severity as keyof typeof severityConfig]?.icon}
              {count}
            </span>
          ))}
        </div>
      </div>

      {/* Category cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
        {Object.entries(groupedDiffs).map(([category, diffs]) => {
          const config = categoryConfig[category as DiffCategory];
          return (
            <motion.div
              key={category}
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              className="p-3 bg-white dark:bg-slate-800/50 rounded-lg border border-stone-200 dark:border-slate-700 hover:border-stone-300 dark:hover:border-slate-600 transition-colors"
            >
              {/* Category header */}
              <div className="flex items-center gap-2 mb-2">
                <span className={config.color}>{config.icon}</span>
                <span className="text-sm font-medium text-stone-700 dark:text-slate-200">
                  {config.label}
                </span>
                <span className="ml-auto text-xs text-stone-500 dark:text-slate-500 bg-stone-100 dark:bg-slate-700 px-1.5 py-0.5 rounded">
                  {diffs.length}
                </span>
              </div>

              {/* Diff items */}
              <div className="flex flex-col gap-1.5">
                {diffs.slice(0, 3).map((diff, idx) => (
                  <button
                    key={idx}
                    onClick={() => onJumpTo(diff.path)}
                    className="flex items-center gap-2 text-xs text-left hover:bg-stone-100 dark:hover:bg-slate-700/50 
                               p-1.5 rounded transition-colors group"
                  >
                    <span
                      className={`flex-shrink-0 ${
                        severityConfig[
                          diff.severity as keyof typeof severityConfig
                        ]?.color.split(" ")[0]
                      }`}
                    >
                      {
                        severityConfig[
                          diff.severity as keyof typeof severityConfig
                        ]?.icon
                      }
                    </span>
                    <div className="flex-1 min-w-0">
                      <div className="truncate text-stone-600 dark:text-slate-400 font-mono text-[10px]">
                        {diff.path}
                      </div>
                      <div className="flex items-center gap-1 text-stone-500 dark:text-slate-500">
                        <span className="truncate max-w-[60px]">
                          {diff.leftValue || "(empty)"}
                        </span>
                        <ArrowRight className="w-3 h-3 text-stone-400 dark:text-slate-600 flex-shrink-0" />
                        <span className="truncate max-w-[60px]">
                          {diff.rightValue || "(empty)"}
                        </span>
                      </div>
                    </div>
                    <span className="text-stone-400 dark:text-slate-600 group-hover:text-stone-600 dark:group-hover:text-slate-400 transition-colors">
                      →
                    </span>
                  </button>
                ))}
                {diffs.length > 3 && (
                  <div className="text-xs text-stone-500 dark:text-slate-500 pl-6">
                    +{diffs.length - 3} more
                  </div>
                )}
              </div>
            </motion.div>
          );
        })}
      </div>

      {/* Jump to buttons */}
      <div className="flex items-center gap-2 flex-wrap">
        <span className="text-xs text-stone-500 dark:text-slate-500">
          Jump to:
        </span>
        {Object.keys(groupedDiffs).map((category) => {
          const config = categoryConfig[category as DiffCategory];
          return (
            <button
              key={category}
              onClick={() => {
                const firstDiff = groupedDiffs[category as DiffCategory][0];
                if (firstDiff) onJumpTo(firstDiff.path);
              }}
              className={`flex items-center gap-1 px-2 py-1 text-xs rounded-md 
                           bg-white dark:bg-slate-800 border border-stone-200 dark:border-slate-700 hover:border-stone-300 dark:hover:border-slate-600 
                           transition-colors ${config.color}`}
            >
              {config.icon}
              {config.label}
            </button>
          );
        })}
      </div>
    </motion.div>
  );
}
