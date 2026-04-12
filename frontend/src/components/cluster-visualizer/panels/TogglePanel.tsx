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
  ChevronDown,
  Check,
  GripHorizontal,
} from "lucide-react";
import { AnimatePresence, useDragControls } from "framer-motion";
import type { ToggleState } from "../types";

interface TogglePanelProps {
  toggles: ToggleState;
  onToggle: (key: keyof ToggleState) => void;
  namespaces: string[];
  selectedNamespace: string;
  onNamespaceChange: (ns: string) => void;
}

export function TogglePanel({
  toggles,
  onToggle,
  namespaces,
  selectedNamespace,
  onNamespaceChange,
}: TogglePanelProps) {
  const [isOpen, setIsOpen] = React.useState(false);
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
      className={`absolute top-4 left-4 bg-white/40 dark:bg-slate-800/40 backdrop-blur-xl border border-stone-200 dark:border-slate-700 rounded-lg p-3 shadow-xl z-10 overflow-hidden resize-none ${
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
          <div className="mb-3 pb-3 border-b border-stone-200 dark:border-slate-700">
            <label className="text-xs text-stone-500 dark:text-slate-400 uppercase tracking-wider block mb-1.5">
              Namespace
            </label>
            <div className="relative">
              <button
                onClick={() => setIsOpen(!isOpen)}
                className="w-full flex items-center justify-between gap-2 px-3 py-2 bg-white/50 dark:bg-slate-900/50 
                     backdrop-blur-sm border border-stone-200 dark:border-slate-700 rounded-lg 
                     hover:border-stone-300 dark:hover:border-slate-600 transition-colors shadow-sm dark:shadow-none"
              >
                <span className="text-sm text-stone-700 dark:text-slate-200 truncate">
                  {selectedNamespace || "All Namespaces"}
                </span>
                <ChevronDown className="w-4 h-4 text-stone-400 dark:text-slate-500" />
              </button>
              <AnimatePresence>
                {isOpen && (
                  <motion.div
                    initial={{ opacity: 0, y: -4 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -4 }}
                    className="absolute z-20 top-full left-0 right-0 mt-1 py-1 bg-white dark:bg-slate-800 
                         border border-stone-200 dark:border-slate-700 rounded-lg shadow-xl max-h-48 overflow-auto"
                  >
                    <button
                      onClick={() => {
                        onNamespaceChange("");
                        setIsOpen(false);
                      }}
                      className={`w-full flex items-center justify-between px-3 py-2 text-sm 
                           hover:bg-stone-100 dark:hover:bg-slate-800 transition-colors ${
                             selectedNamespace === ""
                               ? "text-accent-600 dark:text-accent-400 bg-accent-50 dark:bg-accent-500/10"
                               : "text-stone-700 dark:text-slate-300"
                           }`}
                    >
                      <span>All Namespaces</span>
                      {selectedNamespace === "" && (
                        <Check className="w-4 h-4" />
                      )}
                    </button>
                    {namespaces.map((ns) => (
                      <button
                        key={ns}
                        onClick={() => {
                          onNamespaceChange(ns);
                          setIsOpen(false);
                        }}
                        className={`w-full flex items-center justify-between px-3 py-2 text-sm 
                             hover:bg-stone-100 dark:hover:bg-slate-800 transition-colors ${
                               selectedNamespace === ns
                                 ? "text-accent-600 dark:text-accent-400 bg-accent-50 dark:bg-accent-500/10"
                                 : "text-stone-700 dark:text-slate-300"
                             }`}
                      >
                        <span>{ns}</span>
                        {selectedNamespace === ns && (
                          <Check className="w-4 h-4" />
                        )}
                      </button>
                    ))}
                  </motion.div>
                )}
              </AnimatePresence>
            </div>
          </div>

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
