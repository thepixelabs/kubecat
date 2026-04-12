// Utility functions for command classification and execution

export interface CommandClassification {
  isSafe: boolean;
  category: "read" | "write" | "destructive" | "unknown";
  reasoning: string;
}

const SAFE_READ_PATTERNS = [
  /^kubectl\s+get\s/i,
  /^kubectl\s+describe\s/i,
  /^kubectl\s+logs\s/i,
  /^kubectl\s+top\s/i,
  /^kubectl\s+explain\s/i,
  /^kubectl\s+api-resources/i,
  /^kubectl\s+api-versions/i,
  /^kubectl\s+config\s+view/i,
  /^kubectl\s+version/i,
  /^kubectl\s+cluster-info/i,
];

const WRITE_PATTERNS = [
  /^kubectl\s+apply\s/i,
  /^kubectl\s+create\s/i,
  /^kubectl\s+patch\s/i,
  /^kubectl\s+edit\s/i,
  /^kubectl\s+set\s/i,
  /^kubectl\s+label\s/i,
  /^kubectl\s+annotate\s/i,
  /^kubectl\s+scale\s/i,
  /^kubectl\s+autoscale\s/i,
  /^kubectl\s+rollout\s/i,
  /^kubectl\s+expose\s/i,
  /^kubectl\s+run\s/i,
];

const DESTRUCTIVE_PATTERNS = [
  /^kubectl\s+delete\s/i,
  /^kubectl\s+drain\s/i,
  /^kubectl\s+cordon\s/i,
  /^kubectl\s+uncordon\s/i,
  /^kubectl\s+taint\s/i,
  /--force/i,
  /--grace-period=0/i,
];

// Shell metacharacters that allow command chaining, substitution, or redirection.
// Any command containing these is rejected outright — the prefix-pattern allow-list
// below is meaningless if `kubectl get pods; rm -rf ~` can slip through.
// Shell metacharacters that enable injection, chaining, or substitution.
// Intentionally excludes `{}`/`[]` — those appear in legitimate kubectl
// arguments (`-p '{}'`, label selectors). `$` already catches `${VAR}` and
// `$(cmd)`. Backtick is its own substitution form and is included explicitly.
const SHELL_META_RE = /[;&|`$><\\\n\r*?!]/;

function containsShellMeta(command: string): boolean {
  return SHELL_META_RE.test(command);
}

/**
 * Classifies a kubectl command as safe, write, or destructive
 */
export function classifyCommand(command: string): CommandClassification {
  const trimmedCommand = command.trim();

  // Reject anything with shell metacharacters before pattern matching — otherwise
  // a command like `kubectl get pods; rm -rf ~` would match SAFE_READ_PATTERNS[0]
  // and become eligible for autopilot auto-execution.
  if (containsShellMeta(trimmedCommand)) {
    return {
      isSafe: false,
      category: "unknown",
      reasoning:
        "Command contains shell metacharacters (chaining, substitution, or redirection) — rejected",
    };
  }

  // Check for destructive patterns first
  for (const pattern of DESTRUCTIVE_PATTERNS) {
    if (pattern.test(trimmedCommand)) {
      return {
        isSafe: false,
        category: "destructive",
        reasoning:
          "Command performs destructive operations that modify or delete resources",
      };
    }
  }

  // Check for write patterns
  for (const pattern of WRITE_PATTERNS) {
    if (pattern.test(trimmedCommand)) {
      return {
        isSafe: false,
        category: "write",
        reasoning: "Command modifies cluster state",
      };
    }
  }

  // Check for safe read patterns
  for (const pattern of SAFE_READ_PATTERNS) {
    if (pattern.test(trimmedCommand)) {
      return {
        isSafe: true,
        category: "read",
        reasoning: "Read-only command that does not modify cluster state",
      };
    }
  }

  // Unknown command - treat as unsafe
  return {
    isSafe: false,
    category: "unknown",
    reasoning: "Command pattern not recognized - treating as unsafe",
  };
}

/**
 * Checks if a command should be auto-executed in autopilot mode
 */
export function shouldAutoExecute(
  command: string,
  autopilotEnabled: boolean
): boolean {
  if (!autopilotEnabled) {
    return false;
  }

  const classification = classifyCommand(command);
  return classification.isSafe && classification.category === "read";
}

/**
 * Get a visual indicator for command safety
 */
export function getCommandSafetyIcon(command: string): {
  icon: string;
  color: string;
  label: string;
} {
  const classification = classifyCommand(command);

  switch (classification.category) {
    case "read":
      return {
        icon: "✓",
        color: "text-green-400",
        label: "Safe to auto-execute",
      };
    case "write":
      return {
        icon: "⚠",
        color: "text-yellow-400",
        label: "Modifies cluster - requires approval",
      };
    case "destructive":
      return {
        icon: "⚠",
        color: "text-red-400",
        label: "Destructive - requires approval",
      };
    default:
      return {
        icon: "?",
        color: "text-gray-400",
        label: "Unknown - requires approval",
      };
  }
}
