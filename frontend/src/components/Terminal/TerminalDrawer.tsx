import { X, Terminal, Maximize2, Minimize2 } from "lucide-react";
import { useState } from "react";
import { XTerm } from "./XTerm";
import { ResizeTerminal, WriteTerminal } from "../../../wailsjs/go/main/App";

interface TerminalDrawerProps {
  isOpen: boolean;
  onClose: () => void;
  sessionId: string | null;
  nodeName: string;
  namespace: string;
}

export function TerminalDrawer({
  isOpen,
  onClose,
  sessionId,
  nodeName,
  namespace,
}: TerminalDrawerProps) {
  const [isExpanded, setIsExpanded] = useState(false);

  const handleClose = () => {
    setIsExpanded(false); // Reset to drawer mode on close
    onClose();
  };

  const toggleExpanded = () => {
    setIsExpanded(!isExpanded);
  };

  // Drawer mode (bottom panel)
  if (!isExpanded) {
    return (
      <div
        className={`fixed bottom-0 left-0 right-0 bg-[#1e1e1e] border-t border-stone-200 dark:border-slate-700 transition-transform duration-300 transform z-[60] flex flex-col shadow-2xl ${
          isOpen ? "translate-y-0" : "translate-y-full"
        }`}
        style={{ height: "350px" }}
      >
        <div className="flex items-center justify-between px-4 py-2 bg-[#2d2d2d] border-b border-[#3e3e3e]">
          <div className="flex items-center gap-2 text-sm text-gray-300">
            <Terminal size={14} className="text-emerald-400" />
            <span>
              {namespace}/{nodeName}
            </span>
          </div>
          <div className="flex items-center gap-1">
            <button
              onClick={toggleExpanded}
              className="text-gray-400 hover:text-white transition-colors p-1 rounded hover:bg-white/10"
              title="Expand to popup"
            >
              <Maximize2 size={16} />
            </button>
            <button
              onClick={handleClose}
              className="text-gray-400 hover:text-white transition-colors p-1 rounded hover:bg-white/10"
              title="Close terminal"
            >
              <X size={16} />
            </button>
          </div>
        </div>
        <div className="flex-1 overflow-hidden relative p-0">
          {isOpen && sessionId && (
            <XTerm
              id={sessionId}
              className="h-full w-full"
              onData={(data) => {
                WriteTerminal(sessionId, data);
              }}
              onResize={(rows, cols) => {
                ResizeTerminal(sessionId, rows, cols);
              }}
            />
          )}
        </div>
      </div>
    );
  }

  // Expanded mode (popup)
  return (
    <div
      className={`fixed inset-0 bg-black/60 backdrop-blur-sm z-[70] flex items-center justify-center p-4 transition-opacity duration-300 ${
        isOpen ? "opacity-100" : "opacity-0 pointer-events-none"
      }`}
    >
      <div className="bg-[#1e1e1e] border border-stone-200 dark:border-slate-700 rounded-lg shadow-2xl w-[90vw] h-[80vh] flex flex-col">
        <div className="flex items-center justify-between px-4 py-2 bg-[#2d2d2d] border-b border-[#3e3e3e] rounded-t-lg">
          <div className="flex items-center gap-2 text-sm text-gray-300">
            <Terminal size={14} className="text-emerald-400" />
            <span>
              {namespace}/{nodeName}
            </span>
          </div>
          <div className="flex items-center gap-1">
            <button
              onClick={toggleExpanded}
              className="text-gray-400 hover:text-white transition-colors p-1 rounded hover:bg-white/10"
              title="Minimize to drawer"
            >
              <Minimize2 size={16} />
            </button>
            <button
              onClick={handleClose}
              className="text-gray-400 hover:text-white transition-colors p-1 rounded hover:bg-white/10"
              title="Close terminal"
            >
              <X size={16} />
            </button>
          </div>
        </div>
        <div className="flex-1 overflow-hidden relative p-0">
          {isOpen && sessionId && (
            <XTerm
              id={sessionId}
              className="h-full w-full"
              onData={(data) => {
                WriteTerminal(sessionId, data);
              }}
              onResize={(rows, cols) => {
                ResizeTerminal(sessionId, rows, cols);
              }}
            />
          )}
        </div>
      </div>
    </div>
  );
}
