import { motion } from "framer-motion";
import {
  Download,
  FileText,
  FileJson,
  Rocket,
  Loader2,
  Copy,
  Check,
} from "lucide-react";
import { useState } from "react";
import type { ActionBarProps } from "./types";

export function ActionBar({
  onExport,
  onApply,
  hasResult,
  isExporting,
}: ActionBarProps) {
  const [showExportMenu, setShowExportMenu] = useState(false);
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    // This would copy the diff content to clipboard
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <motion.div
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      className="flex items-center justify-between px-4 py-3 bg-white dark:bg-slate-800/50 border-t border-stone-200 dark:border-slate-700"
    >
      {/* Left side - Export options */}
      <div className="flex items-center gap-2">
        <div className="relative">
          <button
            onClick={() => setShowExportMenu(!showExportMenu)}
            disabled={!hasResult || isExporting}
            className="flex items-center gap-2 px-3 py-2 text-sm bg-stone-100 dark:bg-slate-700 
                     hover:bg-stone-200 dark:hover:bg-slate-600 text-stone-700 dark:text-slate-200 rounded-md transition-colors
                     disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {isExporting ? (
              <Loader2 className="w-4 h-4 animate-spin" />
            ) : (
              <Download className="w-4 h-4" />
            )}
            Export
          </button>

          {showExportMenu && (
            <motion.div
              initial={{ opacity: 0, y: -4 }}
              animate={{ opacity: 1, y: 0 }}
              className="absolute bottom-full left-0 mb-1 py-1 bg-white dark:bg-slate-900 border border-stone-200 dark:border-slate-700 
                       rounded-md shadow-xl min-w-[140px]"
            >
              <button
                onClick={() => {
                  onExport("markdown");
                  setShowExportMenu(false);
                }}
                className="w-full flex items-center gap-2 px-3 py-2 text-sm text-stone-700 dark:text-slate-300 
                         hover:bg-stone-100 dark:hover:bg-slate-800 transition-colors"
              >
                <FileText className="w-4 h-4" />
                Markdown
              </button>
              <button
                onClick={() => {
                  onExport("json");
                  setShowExportMenu(false);
                }}
                className="w-full flex items-center gap-2 px-3 py-2 text-sm text-stone-700 dark:text-slate-300 
                         hover:bg-stone-100 dark:hover:bg-slate-800 transition-colors"
              >
                <FileJson className="w-4 h-4" />
                JSON
              </button>
            </motion.div>
          )}
        </div>

        <button
          onClick={handleCopy}
          disabled={!hasResult}
          className="flex items-center gap-2 px-3 py-2 text-sm bg-stone-100 dark:bg-slate-700 
                   hover:bg-stone-200 dark:hover:bg-slate-600 text-stone-700 dark:text-slate-200 rounded-md transition-colors
                   disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {copied ? (
            <>
              <Check className="w-4 h-4 text-green-400" />
              <span className="text-green-400">Copied!</span>
            </>
          ) : (
            <>
              <Copy className="w-4 h-4" />
              Copy
            </>
          )}
        </button>
      </div>

      {/* Right side - Apply button */}
      <button
        onClick={onApply}
        disabled={!hasResult}
        className="flex items-center gap-2 px-4 py-2 text-sm bg-accent-600 
                 hover:bg-accent-500 text-white rounded-md transition-colors
                 disabled:opacity-50 disabled:cursor-not-allowed"
      >
        <Rocket className="w-4 h-4" />
        Apply to Target
      </button>
    </motion.div>
  );
}
