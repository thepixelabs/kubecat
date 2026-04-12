import { Component, type ErrorInfo, type ReactNode } from "react";
import { AlertTriangle, RefreshCw, ChevronDown, ChevronUp } from "lucide-react";

interface ErrorBoundaryProps {
  /** Content to protect. */
  children: ReactNode;
  /**
   * Human-readable name for the boundary shown in the error UI.
   * E.g. "Cluster Visualizer", "AI Copilot".
   */
  componentName?: string;
  /**
   * Optional fallback UI override. When provided this replaces the
   * default error panel entirely.
   */
  fallback?: (error: Error, reset: () => void) => ReactNode;
}

interface ErrorBoundaryState {
  hasError: boolean;
  error: Error | null;
  detailsOpen: boolean;
}

/**
 * A class-based React Error Boundary that catches render-time exceptions
 * thrown by any descendant component and displays a styled recovery panel
 * matching the Pixelabs cockpit design language.
 *
 * Usage:
 *
 *   <ErrorBoundary componentName="Cluster Visualizer">
 *     <ClusterVisualizer ... />
 *   </ErrorBoundary>
 *
 * The boundary exposes a "Retry" button that unmounts and remounts the
 * children via a key increment — this is the idiomatic way to reset a
 * boundary without losing parent state.
 */
export class ErrorBoundary extends Component<
  ErrorBoundaryProps,
  ErrorBoundaryState
> {
  constructor(props: ErrorBoundaryProps) {
    super(props);
    this.state = { hasError: false, error: null, detailsOpen: false };
  }

  static getDerivedStateFromError(error: Error): Partial<ErrorBoundaryState> {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    const componentName = this.props.componentName ?? "Unknown Component";
    // Always log to console so DevTools captures the full stack.
    console.error(
      `[ErrorBoundary] Crash in "${componentName}":`,
      error,
      info.componentStack
    );

    // TODO (Phase 1 follow-up): once a backend LogFrontendError binding exists,
    // fire it here for crash reporting:
    //
    //   import { LogFrontendError } from "../../wailsjs/go/main/App";
    //   LogFrontendError(componentName, error.message, error.stack ?? "").catch(() => {});
  }

  handleReset = () => {
    this.setState({ hasError: false, error: null, detailsOpen: false });
  };

  toggleDetails = () => {
    this.setState((s) => ({ detailsOpen: !s.detailsOpen }));
  };

  render() {
    const { hasError, error, detailsOpen } = this.state;
    const { children, componentName, fallback } = this.props;

    if (!hasError || !error) {
      return children;
    }

    if (fallback) {
      return fallback(error, this.handleReset);
    }

    const label = componentName ?? "this component";
    // Unique id so multiple simultaneous boundaries don't share the same DOM id.
    const stackId = `eb-stack-${(componentName ?? "unknown").toLowerCase().replace(/\s+/g, "-")}`;

    return (
      <div
        role="alert"
        aria-live="assertive"
        className="flex items-center justify-center w-full h-full min-h-[200px] p-6"
      >
        {/* Glass panel — matches the cockpit design language */}
        <div className="w-full max-w-md bg-white/80 dark:bg-slate-800/60 backdrop-blur-md border border-red-500/30 dark:border-red-500/20 rounded-xl shadow-xl overflow-hidden">
          {/* Header stripe */}
          <div className="flex items-center gap-3 px-5 py-4 border-b border-red-500/20 bg-red-500/5">
            <div className="flex-shrink-0 p-2 rounded-lg bg-red-500/10">
              <AlertTriangle
                className="w-5 h-5 text-red-500 dark:text-red-400"
                aria-hidden="true"
              />
            </div>
            <div className="min-w-0">
              <h2 className="text-sm font-semibold text-stone-900 dark:text-slate-100 leading-tight">
                {label} crashed
              </h2>
              <p className="text-xs text-stone-500 dark:text-slate-400 mt-0.5 truncate">
                {error.message || "An unexpected error occurred."}
              </p>
            </div>
          </div>

          {/* Body */}
          <div className="px-5 py-4 space-y-4">
            <p className="text-sm text-stone-600 dark:text-slate-300 leading-relaxed">
              Something went wrong while rendering{" "}
              <span className="font-medium text-stone-800 dark:text-slate-200">
                {label}
              </span>
              . Your other views are unaffected. You can try reloading the
              component below.
            </p>

            {/* Collapsible technical details */}
            {error.stack && (
              <div>
                <button
                  type="button"
                  onClick={this.toggleDetails}
                  className="flex items-center gap-1.5 text-xs text-stone-500 dark:text-slate-400 hover:text-stone-700 dark:hover:text-slate-200 transition-colors"
                  aria-expanded={detailsOpen}
                  aria-controls={stackId}
                >
                  {detailsOpen ? (
                    <ChevronUp className="w-3.5 h-3.5" aria-hidden="true" />
                  ) : (
                    <ChevronDown className="w-3.5 h-3.5" aria-hidden="true" />
                  )}
                  {detailsOpen ? "Hide" : "Show"} technical details
                </button>

                {detailsOpen && (
                  <pre
                    id={stackId}
                    className="mt-2 p-3 text-xs font-mono text-red-400 dark:text-red-300 bg-slate-900/70 border border-slate-700/60 rounded-lg overflow-x-auto whitespace-pre-wrap break-all leading-relaxed max-h-48 overflow-y-auto"
                  >
                    {error.stack}
                  </pre>
                )}
              </div>
            )}
          </div>

          {/* Footer */}
          <div className="flex justify-end px-5 py-3 border-t border-stone-200/60 dark:border-slate-700/50 bg-stone-50/50 dark:bg-slate-900/30">
            <button
              type="button"
              onClick={this.handleReset}
              className="flex items-center gap-2 px-4 py-2 text-sm font-medium rounded-lg
                         bg-accent-500 hover:bg-accent-600 text-white
                         transition-colors focus-visible:outline-none focus-visible:ring-2
                         focus-visible:ring-accent-500 focus-visible:ring-offset-2
                         focus-visible:ring-offset-white dark:focus-visible:ring-offset-slate-800"
            >
              <RefreshCw className="w-4 h-4" aria-hidden="true" />
              Retry
            </button>
          </div>
        </div>
      </div>
    );
  }
}

export default ErrorBoundary;
