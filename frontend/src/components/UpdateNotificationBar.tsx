/**
 * UpdateNotificationBar — Slim update banner between Navbar and content.
 *
 * - AnimatePresence slide-in from top
 * - "Download" opens the GitHub release in the system browser via Wails
 * - Accent-color adaptive (respects current CSS theme vars)
 * - Fully dismissable with keyboard support
 */

import { motion, AnimatePresence } from "framer-motion";
import { X, Download, Sparkles } from "lucide-react";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";

interface UpdateNotificationBarProps {
  /** New version string, e.g. "v1.2.3" */
  version: string;
  /** Full GitHub release URL */
  releaseUrl: string;
  /** Whether the bar is visible */
  show: boolean;
  /** Called when the user dismisses the bar */
  onDismiss: () => void;
}

export function UpdateNotificationBar({
  version,
  releaseUrl,
  show,
  onDismiss,
}: UpdateNotificationBarProps) {
  const handleDownload = () => {
    BrowserOpenURL(releaseUrl);
  };

  return (
    <AnimatePresence>
      {show && (
        <motion.div
          initial={{ height: 0, opacity: 0 }}
          animate={{ height: "auto", opacity: 1 }}
          exit={{ height: 0, opacity: 0 }}
          transition={{ duration: 0.22, ease: "easeInOut" }}
          className="overflow-hidden flex-shrink-0"
          role="status"
          aria-live="polite"
          aria-label={`Update available: Kubecat ${version}`}
        >
          <div
            className="
              flex items-center justify-between
              px-4 py-2
              bg-accent-500/10 dark:bg-accent-400/10
              border-b border-accent-500/20 dark:border-accent-400/20
              backdrop-blur-sm
            "
          >
            {/* Left — icon + text */}
            <div className="flex items-center gap-2 min-w-0">
              <Sparkles
                size={14}
                className="text-accent-500 dark:text-accent-400 flex-shrink-0"
                aria-hidden="true"
              />
              <p className="text-xs text-stone-700 dark:text-slate-300 truncate">
                <span className="font-semibold text-accent-600 dark:text-accent-400">
                  Kubecat {version}
                </span>{" "}
                is available — get the latest features and fixes.
              </p>
            </div>

            {/* Right — actions */}
            <div className="flex items-center gap-2 flex-shrink-0 ml-4">
              <button
                onClick={handleDownload}
                className="
                  flex items-center gap-1.5
                  px-2.5 py-1 rounded-md
                  text-xs font-medium
                  bg-accent-500 dark:bg-accent-400
                  text-white dark:text-slate-900
                  hover:bg-accent-600 dark:hover:bg-accent-500
                  transition-colors duration-150
                  focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500/50
                "
                aria-label={`Download Kubecat ${version}`}
              >
                <Download size={12} aria-hidden="true" />
                Download
              </button>

              <button
                onClick={onDismiss}
                className="
                  p-1 rounded-md
                  text-stone-400 hover:text-stone-700
                  dark:text-slate-500 dark:hover:text-slate-300
                  hover:bg-stone-200/60 dark:hover:bg-slate-700/60
                  transition-colors duration-150
                  focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500/50
                "
                aria-label="Dismiss update notification"
              >
                <X size={14} aria-hidden="true" />
              </button>
            </div>
          </div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
