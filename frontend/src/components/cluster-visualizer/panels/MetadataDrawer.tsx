import { motion, AnimatePresence } from "framer-motion";
import {
  X,
  Box,
  Hexagon,
  Globe,
  Layers,
  Server,
  Cpu,
  PlayCircle,
  Clock,
  Sparkles,
} from "lucide-react";
import type { ClusterNode } from "../types";

interface MetadataDrawerProps {
  node: ClusterNode | null;
  onClose: () => void;
  onAnalyze: () => void;
}

export function MetadataDrawer({
  node,
  onClose,
  onAnalyze,
}: MetadataDrawerProps) {
  const getIcon = () => {
    switch (node?.type) {
      case "Pod":
        return <Box size={20} className="text-slate-400" />;
      case "Service":
        return <Hexagon size={20} className="text-cyan-400" />;
      case "Ingress":
        return <Globe size={20} className="text-purple-400" />;
      case "Node":
        return <Server size={20} className="text-indigo-400" />;
      case "Operator":
        return <Cpu size={20} className="text-indigo-400" />;
      case "Job":
        return <PlayCircle size={20} className="text-blue-400" />;
      case "CronJob":
        return <Clock size={20} className="text-blue-400" />;
      default:
        return <Layers size={20} className="text-green-400" />;
    }
  };

  const getStatusColor = () => {
    switch (node?.status?.toLowerCase()) {
      case "running":
      case "active":
        return "text-emerald-400";
      case "pending":
        return "text-amber-400";
      case "failed":
      case "error":
        return "text-red-400";
      default:
        return "text-slate-400";
    }
  };

  return (
    <AnimatePresence>
      {node && (
        <motion.div
          initial={{ x: "100%", opacity: 0 }}
          animate={{ x: 0, opacity: 1 }}
          exit={{ x: "100%", opacity: 0 }}
          transition={{ type: "spring", damping: 25, stiffness: 200 }}
          className="absolute right-0 top-0 h-full w-80 bg-white/80 dark:bg-slate-800/95 backdrop-blur-md border-l border-stone-200 dark:border-slate-700 shadow-xl overflow-y-auto"
        >
          {/* Header */}
          <div className="sticky top-0 bg-white/80 dark:bg-slate-800/95 backdrop-blur-md border-b border-stone-200 dark:border-slate-700 p-4 flex items-center justify-between">
            <div className="flex items-center gap-3">
              {getIcon()}
              <div>
                <h3 className="font-semibold text-stone-800 dark:text-slate-100 truncate max-w-[180px]">
                  {node.name}
                </h3>
                <p className="text-xs text-stone-500 dark:text-slate-400">
                  {node.type}
                </p>
              </div>
            </div>
            <div className="flex items-center gap-1">
              <button
                onClick={onAnalyze}
                className="p-1.5 hover:bg-stone-200 dark:hover:bg-slate-700 rounded transition-colors text-cyan-600 dark:text-cyan-400"
                title="Analyze with AI"
              >
                <Sparkles size={18} />
              </button>
              <button
                onClick={onClose}
                className="p-1.5 hover:bg-stone-200 dark:hover:bg-slate-700 rounded transition-colors"
              >
                <X size={18} className="text-stone-400 dark:text-slate-400" />
              </button>
            </div>
          </div>

          {/* Content */}
          <div className="p-4 space-y-4">
            {/* Status Section */}
            <Section title="Status">
              <InfoRow
                label="Status"
                value={node.status}
                valueClassName={getStatusColor()}
              />
              <InfoRow label="Namespace" value={node.namespace} />
              {node.age && <InfoRow label="Age" value={node.age} />}
            </Section>

            {/* Pod-specific */}
            {node.type === "Pod" && (
              <Section title="Pod Details">
                {node.node && <InfoRow label="Node" value={node.node} />}
                {node.readyContainers && (
                  <InfoRow label="Containers" value={node.readyContainers} />
                )}
                {node.restarts !== undefined && (
                  <InfoRow
                    label="Restarts"
                    value={String(node.restarts)}
                    valueClassName={
                      node.restarts > 0 ? "text-amber-400" : undefined
                    }
                  />
                )}
              </Section>
            )}

            {/* Service-specific */}
            {node.type === "Service" && (
              <Section title="Service Details">
                {node.serviceType && (
                  <InfoRow label="Type" value={node.serviceType} />
                )}
                {node.clusterIP && (
                  <InfoRow label="Cluster IP" value={node.clusterIP} />
                )}
                {node.ports && <InfoRow label="Ports" value={node.ports} />}
              </Section>
            )}

            {/* Node-specific */}
            {node.type === "Node" && (
              <Section title="Node Resources">
                {node.cpuCapacity && (
                  <InfoRow label="CPU Capacity" value={node.cpuCapacity} />
                )}
                {node.memCapacity && (
                  <InfoRow label="Mem Capacity" value={node.memCapacity} />
                )}
                {node.cpuAllocatable && (
                  <InfoRow
                    label="CPU Allocatable"
                    value={node.cpuAllocatable}
                  />
                )}
                {node.memAllocatable && (
                  <InfoRow
                    label="Mem Allocatable"
                    value={node.memAllocatable}
                  />
                )}
                {node.nodeConditions && node.nodeConditions.length > 0 && (
                  <div className="pt-1">
                    <span className="text-sm text-slate-400 block mb-1">
                      Conditions
                    </span>
                    <div className="flex flex-wrap gap-1">
                      {node.nodeConditions.map((cond, i) => (
                        <span
                          key={i}
                          className="text-xs bg-stone-200 dark:bg-slate-700 text-stone-600 dark:text-slate-300 px-2 py-0.5 rounded"
                        >
                          {cond}
                        </span>
                      ))}
                    </div>
                  </div>
                )}
              </Section>
            )}

            {/* Controller-specific */}
            {[
              "Deployment",
              "StatefulSet",
              "DaemonSet",
              "ReplicaSet",
              "Job",
              "CronJob",
              "Operator",
            ].includes(node.type) && (
              <Section title="Controller Details">
                <InfoRow label="Type" value={node.type} />
                {node.labels?.["app.kubernetes.io/managed-by"] && (
                  <InfoRow
                    label="Managed By"
                    value={node.labels["app.kubernetes.io/managed-by"]}
                  />
                )}
              </Section>
            )}

            {/* Labels */}
            {node.labels && Object.keys(node.labels).length > 0 && (
              <Section title="Labels">
                <div className="flex flex-wrap gap-1">
                  {Object.entries(node.labels)
                    .slice(0, 10)
                    .map(([key, value]) => (
                      <span
                        key={key}
                        className="text-xs bg-stone-200 dark:bg-slate-700 text-stone-600 dark:text-slate-300 px-2 py-0.5 rounded truncate max-w-full"
                        title={`${key}=${value}`}
                      >
                        {key.split("/").pop()}=
                        {value.length > 15 ? value.slice(0, 15) + "..." : value}
                      </span>
                    ))}
                  {Object.keys(node.labels).length > 10 && (
                    <span className="text-xs text-stone-500 dark:text-slate-500">
                      +{Object.keys(node.labels).length - 10} more
                    </span>
                  )}
                </div>
              </Section>
            )}
          </div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}

function Section({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div>
      <h4 className="text-xs font-medium text-stone-500 dark:text-slate-500 uppercase tracking-wider mb-2">
        {title}
      </h4>
      <div className="space-y-1">{children}</div>
    </div>
  );
}

function InfoRow({
  label,
  value,
  valueClassName,
}: {
  label: string;
  value: string;
  valueClassName?: string;
}) {
  return (
    <div className="flex justify-between items-center py-1">
      <span className="text-sm text-stone-500 dark:text-slate-400">
        {label}
      </span>
      <span
        className={`text-sm font-medium ${
          valueClassName || "text-stone-700 dark:text-slate-200"
        }`}
      >
        {value}
      </span>
    </div>
  );
}
