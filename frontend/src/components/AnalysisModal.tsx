import { useState, useEffect } from "react";
import { motion, AnimatePresence } from "framer-motion";
import {
  X,
  Bot,
  RefreshCw,
  AlertCircle,
  Sparkles,
  Copy,
  Check,
} from "lucide-react";
import Markdown from "react-markdown";
import rehypeSanitize, { defaultSchema } from "rehype-sanitize";
import { AIAnalyzeResource } from "../../wailsjs/go/main/App";

// Strict sanitization schema for AI-generated markdown.
// Extends the rehype-sanitize default (which already blocks <script>, <style>,
// and all on* event handlers) by restricting href/src to safe protocols only.
const markdownSanitizeSchema = {
  ...defaultSchema,
  protocols: {
    href: ["http", "https", "mailto"],
    src: ["http", "https"],
  },
};

interface AnalysisModalProps {
  isOpen: boolean;
  onClose: () => void;
  resource: {
    kind: string;
    namespace: string;
    name: string;
  } | null;
}

export function AnalysisModal({
  isOpen,
  onClose,
  resource,
}: AnalysisModalProps) {
  const [analysis, setAnalysis] = useState<string>("");
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (isOpen && resource) {
      handleAnalyze();
    } else {
      setAnalysis("");
      setError(null);
    }
  }, [isOpen, resource]);

  const handleAnalyze = async () => {
    if (!resource) return;
    setIsLoading(true);
    setError(null);
    setAnalysis("");

    try {
      const result = await AIAnalyzeResource(
        resource.kind,
        resource.namespace,
        resource.name
      );
      setAnalysis(result);
    } catch (err: any) {
      console.error("Analysis failed:", err);
      setError(
        typeof err === "string"
          ? err
          : err.message || "Failed to analyze resource"
      );
    } finally {
      setIsLoading(false);
    }
  };

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(analysis);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      console.error("Failed to copy:", err);
    }
  };

  return (
    <AnimatePresence>
      {isOpen && (
        <>
          {/* Backdrop */}
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={onClose}
            className="fixed inset-0 z-[60] bg-slate-500/20 dark:bg-black/60 backdrop-blur-sm"
          />

          {/* Modal */}
          <motion.div
            initial={{ opacity: 0, scale: 0.95, y: 20 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.95, y: 20 }}
            className="fixed inset-0 z-[70] flex items-center justify-center p-4 pointer-events-none"
          >
            <div className="bg-white/80 dark:bg-slate-900/80 backdrop-blur-xl border border-cyan-800/50 rounded-xl shadow-2xl w-full max-w-2xl max-h-[85vh] flex flex-col pointer-events-auto overflow-hidden">
              {/* Header */}
              <div className="flex items-center justify-between p-4 border-b border-cyan-900/30 bg-white/50 dark:bg-slate-900/50">
                <div className="flex items-center gap-2">
                  <div className="p-2 bg-cyan-950/20 dark:bg-cyan-950/50 rounded-lg">
                    <Sparkles className="w-5 h-5 text-cyan-600 dark:text-cyan-400" />
                  </div>
                  <div>
                    <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">
                      AI Resource Analysis
                    </h2>
                    {resource && (
                      <p className="text-xs text-slate-500 dark:text-slate-400 font-mono">
                        {resource.kind} / {resource.namespace} / {resource.name}
                      </p>
                    )}
                  </div>
                </div>
                <button
                  onClick={onClose}
                  className="p-1 hover:bg-slate-100 dark:hover:bg-slate-800 rounded-lg transition-colors text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
                >
                  <X className="w-5 h-5" />
                </button>
              </div>

              {/* Content */}
              <div className="flex-1 overflow-y-auto p-6 scrollbar-thin scrollbar-thumb-cyan-900/20 scrollbar-track-transparent">
                {isLoading ? (
                  <div className="flex flex-col items-center justify-center py-12 gap-4">
                    <RefreshCw className="w-8 h-8 text-cyan-500 animate-spin" />
                    <p className="text-slate-500 dark:text-slate-400 animate-pulse">
                      Analyzing resource telemetry...
                    </p>
                    <div className="flex gap-2 text-xs text-slate-500 font-mono mt-2">
                      <span className="px-2 py-1 bg-slate-100 dark:bg-slate-800 rounded">
                        Events
                      </span>
                      <span className="px-2 py-1 bg-slate-100 dark:bg-slate-800 rounded">
                        Logs
                      </span>
                      <span className="px-2 py-1 bg-slate-100 dark:bg-slate-800 rounded">
                        YAML
                      </span>
                    </div>
                  </div>
                ) : error ? (
                  <div className="flex flex-col items-center justify-center py-8 text-red-500 dark:text-red-400 gap-2">
                    <AlertCircle className="w-10 h-10 mb-2 opacity-80" />
                    <p className="font-medium">Analysis Failed</p>
                    <p className="text-sm text-red-500/70 dark:text-red-400/70 text-center max-w-sm">
                      {error}
                    </p>
                    <button
                      onClick={handleAnalyze}
                      className="mt-4 px-4 py-2 bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 rounded-lg text-sm text-slate-600 dark:text-slate-300 transition-colors flex items-center gap-2"
                    >
                      <RefreshCw className="w-4 h-4" /> Try Again
                    </button>
                  </div>
                ) : (
                  <div className="prose prose-slate dark:prose-invert prose-cyan max-w-none select-text">
                    {analysis ? (
                      <Markdown rehypePlugins={[[rehypeSanitize, markdownSanitizeSchema]]}>
                        {analysis}
                      </Markdown>
                    ) : (
                      <div className="text-center text-slate-500 py-8">
                        No analysis generated.
                      </div>
                    )}
                  </div>
                )}
              </div>

              {/* Footer */}
              <div className="p-4 border-t border-cyan-900/30 bg-white/50 dark:bg-slate-900/50 flex justify-between items-center">
                <div className="flex items-center gap-2 text-xs text-slate-500">
                  <Bot className="w-4 h-4" />
                  <span>Generated by AI • Verify critical findings</span>
                </div>
                {!isLoading && !error && (
                  <div className="flex gap-2">
                    <button
                      onClick={handleCopy}
                      className="px-3 py-1.5 hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-500 hover:text-cyan-600 dark:text-slate-400 dark:hover:text-cyan-400 text-xs rounded-lg transition-colors flex items-center gap-2"
                    >
                      {copied ? (
                        <>
                          <Check className="w-3 h-3" /> Copied
                        </>
                      ) : (
                        <>
                          <Copy className="w-3 h-3" /> Copy Result
                        </>
                      )}
                    </button>
                    <button
                      onClick={handleAnalyze}
                      className="px-3 py-1.5 hover:bg-slate-100 dark:hover:bg-slate-800 text-cyan-600 dark:text-cyan-400 text-xs rounded-lg transition-colors flex items-center gap-2"
                    >
                      <RefreshCw className="w-3 h-3" /> Re-analyze
                    </button>
                  </div>
                )}
              </div>
            </div>
          </motion.div>
        </>
      )}
    </AnimatePresence>
  );
}
