import { useCallback } from "react";
import { motion } from "framer-motion";
import type { DiffViewerProps } from "./types";

interface DiffLine {
  lineNumber: number;
  content: string;
  type: "unchanged" | "added" | "removed" | "modified";
  path?: string;
}

export function DiffViewer({
  leftYaml,
  rightYaml,
  leftLabel,
  rightLabel,
  differences,
}: DiffViewerProps) {
  // Parse YAML lines and identify differences
  const parseLines = useCallback(
    (yaml: string, isLeft: boolean): DiffLine[] => {
      const lines = yaml.split("\n");

      return lines.map((content, idx) => {
        const lineNumber = idx + 1;

        // Simple heuristic: check if line contains any changed path
        const pathMatch = differences.find((d) => {
          const pathParts = d.path.split(".");
          return pathParts.some((part) => content.includes(part + ":"));
        });

        let type: DiffLine["type"] = "unchanged";
        if (pathMatch) {
          if (isLeft && pathMatch.changeType === "removed") {
            type = "removed";
          } else if (!isLeft && pathMatch.changeType === "added") {
            type = "added";
          } else if (pathMatch.changeType === "modified") {
            type = "modified";
          }
        }

        return { lineNumber, content, type, path: pathMatch?.path };
      });
    },
    [differences]
  );

  const leftLines = parseLines(leftYaml, true);
  const rightLines = parseLines(rightYaml, false);

  // Get line style based on type
  const getLineStyle = (type: DiffLine["type"]) => {
    switch (type) {
      case "added":
        return "bg-green-500/20 border-l-2 border-green-500";
      case "removed":
        return "bg-red-500/20 border-l-2 border-red-500";
      case "modified":
        return "bg-yellow-500/20 border-l-2 border-yellow-500";
      default:
        return "";
    }
  };

  // Get prefix symbol
  const getPrefix = (type: DiffLine["type"]) => {
    switch (type) {
      case "added":
        return "+";
      case "removed":
        return "-";
      case "modified":
        return "~";
      default:
        return " ";
    }
  };

  const renderPane = (lines: DiffLine[], label: string) => (
    <div className="flex-1 flex flex-col min-w-0 border-r border-stone-200 dark:border-slate-700 last:border-r-0">
      {/* Header */}
      <div className="px-3 py-2 bg-stone-100 dark:bg-slate-800/90 backdrop-blur-sm border-b border-stone-200 dark:border-slate-700 flex items-center justify-between sticky top-0 z-10">
        <span className="text-sm font-medium text-stone-700 dark:text-slate-300 truncate">
          {label}
        </span>
        <span className="text-xs text-stone-500 dark:text-slate-500">
          {lines.length} lines
        </span>
      </div>

      {/* Content */}
      <div className="font-mono text-xs bg-white dark:bg-slate-900">
        {lines.length === 0 ? (
          <div className="p-4 text-stone-400 dark:text-slate-500 italic">
            No content available
          </div>
        ) : (
          <table className="w-full border-collapse table-fixed">
            <colgroup>
              <col className="w-10" />
              <col className="w-4" />
              <col />
            </colgroup>
            <tbody>
              {lines.map((line, idx) => (
                <tr
                  key={idx}
                  className={`${getLineStyle(
                    line.type
                  )} hover:bg-stone-100 dark:hover:bg-slate-800 transition-colors`}
                  data-path={line.path}
                >
                  {/* Line number */}
                  <td className="px-2 py-0.5 text-right text-stone-400 dark:text-slate-500 select-none border-r border-stone-100 dark:border-slate-800 align-top h-5">
                    {line.lineNumber}
                  </td>
                  {/* Prefix */}
                  <td
                    className={`px-1 py-0.5 select-none text-center align-top h-5 ${
                      line.type !== "unchanged"
                        ? line.type === "added"
                          ? "text-emerald-600 dark:text-emerald-400"
                          : line.type === "removed"
                          ? "text-red-500 dark:text-red-400"
                          : "text-amber-500 dark:text-amber-400"
                        : "text-stone-300 dark:text-slate-600"
                    }`}
                  >
                    {getPrefix(line.type)}
                  </td>
                  {/* Content */}
                  <td className="px-2 py-0.5 whitespace-pre-wrap break-all text-stone-800 dark:text-slate-300 align-top h-5">
                    {highlightYamlLine(line.content)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      className="flex flex-col bg-white dark:bg-slate-900 rounded-lg border border-stone-200 dark:border-slate-700"
    >
      {/* Split panes */}
      <div className="flex-1 flex min-h-0">
        {renderPane(leftLines, leftLabel)}
        {renderPane(rightLines, rightLabel)}
      </div>
    </motion.div>
  );
}

// Simple YAML syntax highlighting
function highlightYamlLine(line: string): React.ReactNode {
  // Key: value pattern
  const keyValueMatch = line.match(/^(\s*)([a-zA-Z0-9_-]+)(:)(.*)$/);
  if (keyValueMatch) {
    const [, indent, key, colon, value] = keyValueMatch;
    return (
      <>
        {indent}
        <span className="text-emerald-700 dark:text-emerald-400">{key}</span>
        <span className="text-stone-400 dark:text-slate-500">{colon}</span>
        {highlightValue(value)}
      </>
    );
  }

  // List item pattern
  const listMatch = line.match(/^(\s*)(-)(.*)$/);
  if (listMatch) {
    const [, indent, dash, rest] = listMatch;
    return (
      <>
        {indent}
        <span className="text-purple-600 dark:text-purple-400">{dash}</span>
        {rest}
      </>
    );
  }

  // Comment
  if (line.trim().startsWith("#")) {
    return (
      <span className="text-stone-400 dark:text-slate-500 italic">{line}</span>
    );
  }

  return line;
}

function highlightValue(value: string): React.ReactNode {
  const trimmed = value.trim();

  // String values
  if (trimmed.startsWith('"') || trimmed.startsWith("'")) {
    return <span className="text-amber-600 dark:text-amber-400">{value}</span>;
  }

  // Numbers
  if (/^\s*\d+(\.\d+)?$/.test(trimmed)) {
    return <span className="text-blue-600 dark:text-blue-400">{value}</span>;
  }

  // Booleans
  if (/^\s*(true|false)$/i.test(trimmed)) {
    return (
      <span className="text-purple-600 dark:text-purple-400">{value}</span>
    );
  }

  // Null
  if (/^\s*(null|~)$/i.test(trimmed)) {
    return <span className="text-stone-400 dark:text-slate-500">{value}</span>;
  }

  return <span className="text-stone-600 dark:text-slate-300">{value}</span>;
}
