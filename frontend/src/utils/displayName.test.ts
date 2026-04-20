import { describe, it, expect } from "vitest";
import {
  shortClusterName,
  middleTruncate,
  isCopyableIdentifier,
} from "./displayName";

describe("shortClusterName", () => {
  it("extracts the cluster name from an EKS ARN", () => {
    expect(
      shortClusterName(
        "arn:aws:eks:us-east-1:123456789012:cluster/prod-api"
      )
    ).toBe("prod-api");
  });

  it("formats a GKE context with project + zone", () => {
    expect(
      shortClusterName("gke_my-project_us-central1-a_prod-cluster")
    ).toBe("prod-cluster (my-project/us-central1-a)");
  });

  it("preserves underscores in GKE cluster names", () => {
    expect(
      shortClusterName("gke_proj_zone_name_with_extra")
    ).toBe("name_with_extra (proj/zone)");
  });

  it("extracts the cluster name from an AKS FQDN", () => {
    expect(
      shortClusterName("prod-cluster.hcp.eastus.azmk8s.io")
    ).toBe("prod-cluster");
  });

  it("returns a plain context verbatim", () => {
    expect(shortClusterName("minikube")).toBe("minikube");
    expect(shortClusterName("docker-desktop")).toBe("docker-desktop");
  });

  it("returns empty strings unchanged", () => {
    expect(shortClusterName("")).toBe("");
  });

  it("does not blow up on a malformed ARN", () => {
    expect(shortClusterName("arn:aws:eks:")).toBe("arn:aws:eks:");
  });

  it("returns ARN verbatim when slash is the last char (no cluster name after)", () => {
    expect(shortClusterName("arn:aws:eks:us-east-1:123:cluster/")).toBe(
      "arn:aws:eks:us-east-1:123:cluster/"
    );
  });

  it("returns the gke_ prefix verbatim when fewer than 4 underscore parts", () => {
    // Hits the fallback branch at line 40 — malformed GKE context
    expect(shortClusterName("gke_foo_bar")).toBe("gke_foo_bar");
    expect(shortClusterName("gke_only")).toBe("gke_only");
  });

  it("handles the .hcp. AKS variant", () => {
    expect(shortClusterName("dev-cluster.hcp.westeurope.azmk8s.io")).toBe(
      "dev-cluster"
    );
  });

  it("handles unicode in plain cluster names", () => {
    // Non-ASCII chars are a common real-world edge case
    expect(shortClusterName("клас\u0442ер-prod")).toBe("клас\u0442ер-prod");
  });
});

describe("middleTruncate", () => {
  it("returns short strings unchanged", () => {
    expect(middleTruncate("short", 10)).toBe("short");
  });

  it("truncates with a single-char ellipsis in the middle", () => {
    const out = middleTruncate("prod-api-7d4b8c9f6-xk2mq", 10);
    expect(out.length).toBe(10);
    expect(out).toContain("\u2026");
    // Keeps informative prefix *and* suffix
    expect(out.startsWith("prod")).toBe(true);
    expect(out.endsWith("k2mq")).toBe(true);
  });

  it("handles very small maxLen without crashing", () => {
    expect(middleTruncate("abcdef", 2)).toHaveLength(2);
  });

  it("returns empty string unchanged", () => {
    expect(middleTruncate("", 10)).toBe("");
  });

  it("returns s verbatim when maxLen === len(s) (boundary)", () => {
    expect(middleTruncate("hello", 5)).toBe("hello");
  });

  it("clips with head slice when maxLen <= 3", () => {
    // The "<= 3" branch uses a plain head-slice with no ellipsis
    expect(middleTruncate("abcdef", 3)).toBe("abc");
    expect(middleTruncate("abcdef", 1)).toBe("a");
  });

  it("handles multi-byte characters without crashing", () => {
    // Emoji count as 2 UTF-16 code units — spec does not require
    // grapheme awareness, only that it does not throw.
    expect(() => middleTruncate("🎉🎊-prod-pod-xyz", 8)).not.toThrow();
  });

  it("is symmetric for odd keep counts — head gets the extra char", () => {
    // keep = maxLen - 1 = 9, head = ceil(9/2) = 5, tail = 4
    const out = middleTruncate("abcdefghijklmnop", 10);
    expect(out.length).toBe(10);
    expect(out.slice(0, 5)).toBe("abcde");
    expect(out.slice(-4)).toBe("mnop");
    expect(out[5]).toBe("\u2026");
  });
});

describe("isCopyableIdentifier", () => {
  it("flags ARNs", () => {
    expect(isCopyableIdentifier("arn:aws:eks:…")).toBe(true);
  });
  it("flags GKE canonical names", () => {
    expect(isCopyableIdentifier("gke_p_z_n")).toBe(true);
  });
  it("does not flag plain names", () => {
    expect(isCopyableIdentifier("minikube")).toBe(false);
    expect(isCopyableIdentifier("")).toBe(false);
  });
});
