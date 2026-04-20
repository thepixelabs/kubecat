/**
 * Display name helpers for cluster/context/ARN identifiers.
 *
 * Philosophy: the truncated label shown in the UI should be *useful* at a glance
 * (human-readable cluster name, not the raw ARN/URI), while the full value is
 * always preserved on hover via `title={fullValue}`.
 */

/**
 * Extract the human-meaningful short name from a raw kubeconfig context or
 * cloud-provider cluster identifier.
 *
 * Handles the common shapes we see in the wild:
 * - EKS ARN:   `arn:aws:eks:us-east-1:123456789012:cluster/prod-api` → `prod-api`
 * - GKE ID:    `gke_myproj_us-central1-a_prod-cluster`               → `prod-cluster (myproj/us-central1-a)`
 * - AKS FQDN:  `prod-cluster.hcp.eastus.azmk8s.io`                   → `prod-cluster`
 * - anything else returns the raw value verbatim
 */
export function shortClusterName(raw: string): string {
  if (!raw) return raw;

  // EKS ARN — `arn:aws:eks:<region>:<account>:cluster/<name>`
  if (raw.startsWith("arn:")) {
    const slash = raw.lastIndexOf("/");
    if (slash !== -1 && slash < raw.length - 1) {
      return raw.slice(slash + 1);
    }
    return raw;
  }

  // GKE canonical context — `gke_<project>_<zone>_<name>`
  if (raw.startsWith("gke_")) {
    const parts = raw.split("_");
    if (parts.length >= 4) {
      const project = parts[1];
      const zone = parts[2];
      const name = parts.slice(3).join("_");
      return `${name} (${project}/${zone})`;
    }
    return raw;
  }

  // AKS FQDN — `<name>.hcp.<region>.azmk8s.io`
  if (raw.includes(".azmk8s.io") || raw.includes(".hcp.")) {
    const first = raw.split(".")[0];
    if (first) return first;
  }

  return raw;
}

/**
 * Middle-truncate a string, preserving the first and last segments.
 * Useful for pod hashes and other IDs where both the prefix *and* suffix are
 * informative (e.g. `prod-api-7d4b8c9f6-xk2mq`).
 *
 * If `s` is already <= maxLen, returns it unchanged.
 */
export function middleTruncate(s: string, maxLen: number): string {
  if (!s) return s;
  if (s.length <= maxLen) return s;
  if (maxLen <= 3) return s.slice(0, maxLen);

  const ellipsis = "\u2026"; // single-char ellipsis
  const keep = maxLen - 1; // room for the ellipsis
  const head = Math.ceil(keep / 2);
  const tail = Math.floor(keep / 2);
  return `${s.slice(0, head)}${ellipsis}${s.slice(s.length - tail)}`;
}

/**
 * True if the identifier looks like something worth a copy-to-clipboard
 * affordance (ARN / GKE / etc).
 */
export function isCopyableIdentifier(s: string): boolean {
  if (!s) return false;
  return s.startsWith("arn:") || s.startsWith("gke_");
}
