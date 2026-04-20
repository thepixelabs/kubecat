import React from "react";
import { motion } from "framer-motion";
import {
  Box,
  Hexagon,
  Globe,
  Layers,
  Database,
  Server,
  Link,
  Cpu,
  PlayCircle,
  Clock,
  Copy,
  GripHorizontal,
} from "lucide-react";
import { useDragControls } from "framer-motion";
import type { ToggleState } from "../types";

interface TogglePanelProps {
  toggles: ToggleState;
  onToggle: (key: keyof ToggleState) => void;
}

export function TogglePanel({ toggles, onToggle }: TogglePanelProps) {
  const [isMinimized, setIsMinimized] = React.useState(false);
  const dragControls = useDragControls();

  return (
    <motion.div
      drag
      dragListener={false}
      dragControls={dragControls}
      dragMomentum={false}
      initial={{ y: -20, opacity: 0 }}
      animate={{ y: 0, opacity: 1, height: isMinimized ? "auto" : undefined }}
      className={`absolute top-20 left-4 bg-white/40 dark:bg-slate-800/40 backdrop-blur-xl border border-stone-200 dark:border-slate-700 rounded-lg p-3 shadow-xl z-10 overflow-hidden resize-none ${
        isMinimized
          ? "w-auto min-w-[150px]"
          : "resize min-w-[220px] max-w-[280px] max-h-[80vh] overflow-auto"
      }`}
    >
      {/* Drag Handle */}
      <div
        className="flex items-center justify-center p-1 mb-2 cursor-grab active:cursor-grabbing text-stone-400 dark:text-slate-500 hover:text-stone-600 dark:hover:text-slate-300 transition-colors"
        onPointerDown={(e) => dragControls.start(e)}
        onDoubleClick={() => setIsMinimized(!isMinimized)}
      >
        <GripHorizontal size={16} />
      </div>

      {!isMinimized && (
        <>
          {/* Node Type Toggles */}
          <div className="mb-3 pb-3 border-b border-stone-200 dark:border-slate-700">
            <label className="text-xs text-stone-500 dark:text-slate-400 uppercase tracking-wider block mb-2">
              Resources
            </label>
            <div className="flex flex-wrap gap-1.5">
              <ToggleButton
                active={toggles.showPods}
                onClick={() => onToggle("showPods")}
                icon={<Box size={12} />}
                label="Pods"
                color="text-stone-500 dark:text-slate-400"
              />
              <ToggleButton
                active={toggles.showServices}
                onClick={() => onToggle("showServices")}
                icon={<Hexagon size={12} />}
                label="Services"
                color="text-cyan-600 dark:text-cyan-400"
              />
              <ToggleButton
                active={toggles.showIngresses}
                onClick={() => onToggle("showIngresses")}
                icon={<Globe size={12} />}
                label="Ingress"
                color="text-purple-600 dark:text-purple-400"
              />
              <ToggleButton
                active={toggles.showDeployments}
                onClick={() => onToggle("showDeployments")}
                icon={<Layers size={12} />}
                label="Deploy"
                color="text-green-600 dark:text-green-400"
              />
              <ToggleButton
                active={toggles.showStatefulSets}
                onClick={() => onToggle("showStatefulSets")}
                icon={<Database size={12} />}
                label="STS"
                color="text-amber-600 dark:text-amber-400"
              />
              <ToggleButton
                active={toggles.showDaemonSets}
                onClick={() => onToggle("showDaemonSets")}
                icon={<Server size={12} />}
                label="DS"
                color="text-rose-600 dark:text-rose-400"
              />
            </div>
          </div>

          {/* Advanced Resources */}
          <div className="mb-3 pb-3 border-b border-stone-200 dark:border-slate-700">
            <label className="text-xs text-stone-500 dark:text-slate-400 uppercase tracking-wider block mb-2">
              Infrastructure & Logic
            </label>
            <div className="flex flex-wrap gap-1.5">
              <ToggleButton
                active={toggles.showNodes}
                onClick={() => onToggle("showNodes")}
                icon={<Server size={12} />}
                label="Nodes"
                color="text-indigo-600 dark:text-indigo-400"
              />
              <ToggleButton
                active={toggles.showOperators}
                onClick={() => onToggle("showOperators")}
                icon={<Cpu size={12} />}
                label="Operators"
                color="text-indigo-600 dark:text-indigo-400"
              />
              <ToggleButton
                active={toggles.showReplicaSets}
                onClick={() => onToggle("showReplicaSets")}
                icon={<Copy size={12} />}
                label="RS"
                color="text-stone-500 dark:text-slate-400"
              />
              <ToggleButton
                active={toggles.showJobs}
                onClick={() => onToggle("showJobs")}
                icon={<PlayCircle size={12} />}
                label="Jobs"
                color="text-blue-600 dark:text-blue-400"
              />
              <ToggleButton
                active={toggles.showCronJobs}
                onClick={() => onToggle("showCronJobs")}
                icon={<Clock size={12} />}
                label="CronJobs"
                color="text-blue-600 dark:text-blue-400"
              />
            </div>
          </div>

          {/* Edge Type Toggles */}
          <div>
            <label className="text-xs text-stone-500 dark:text-slate-400 uppercase tracking-wider block mb-2">
              Connections
            </label>
            <div className="flex flex-wrap gap-1.5">
              <ToggleButton
                active={toggles.showServiceToPod}
                onClick={() => onToggle("showServiceToPod")}
                icon={<Link size={12} />}
                label="Svc→Pod"
                color="text-cyan-600 dark:text-cyan-400"
              />
              <ToggleButton
                active={toggles.showIngressToService}
                onClick={() => onToggle("showIngressToService")}
                icon={<Link size={12} />}
                label="Ing→Svc"
                color="text-purple-600 dark:text-purple-400"
              />
              <ToggleButton
                active={toggles.showControllerToPod}
                onClick={() => onToggle("showControllerToPod")}
                icon={<Link size={12} />}
                label="Ctrl→Pod"
                color="text-green-600 dark:text-green-400"
              />
              <ToggleButton
                active={toggles.showNodeToPod}
                onClick={() => onToggle("showNodeToPod")}
                icon={<Link size={12} />}
                label="Node→Pod"
                color="text-indigo-600 dark:text-indigo-400"
              />
            </div>
          </div>
        </>
      )}
    </motion.div>
  );
}

interface ToggleButtonProps {
  active: boolean;
  onClick: () => void;
  icon: React.ReactNode;
  label: string;
  color: string;
}

function ToggleButton({
  active,
  onClick,
  icon,
  label,
  color,
}: ToggleButtonProps) {
  return (
    <button
      onClick={onClick}
      className={`
        flex items-center gap-1 px-2 py-1 rounded text-xs font-medium transition-all
        ${
          active
            ? `bg-stone-200 dark:bg-slate-700 ${color}`
            : "bg-stone-100 dark:bg-slate-800 text-stone-500 dark:text-slate-500 hover:bg-stone-200 dark:hover:bg-slate-700/50"
        }
      `}
    >
      {icon}
      <span>{label}</span>
    </button>
  );
}
