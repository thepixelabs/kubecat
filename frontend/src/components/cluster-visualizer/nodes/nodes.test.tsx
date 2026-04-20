import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { PodNode } from "./PodNode";
import { ServiceNode } from "./ServiceNode";
import { IngressNode } from "./IngressNode";
import { ControllerNode } from "./ControllerNode";
import { ServerNode } from "./ServerNode";
import { NamespaceNode } from "./NamespaceNode";

// `@xyflow/react` Handle requires a NodeId from React Flow context. Mocking it
// out keeps these tests a pure React render exercise — matches the pattern
// used by ClusterVisualizer.test.tsx.
vi.mock("@xyflow/react", () => ({
  Handle: (props: any) => <div data-testid={`handle-${props.type}`} />,
  Position: { Top: "top", Bottom: "bottom", Left: "left", Right: "right" },
}));

// ── PodNode ──────────────────────────────────────────────────────────────────

describe("PodNode", () => {
  it("renders the pod label", () => {
    render(<PodNode data={{ label: "web-api", status: "Running" }} />);
    expect(screen.getByText("web-api")).toBeInTheDocument();
  });

  // Regression pin for the "truncated label has no title" bug. The truncated
  // span must carry the full label on title= so hover reveals it.
  it("sets title={label} on the truncated span so hover reveals the full name", () => {
    const longName = "prod-api-7d4b8c9f6-xk2mq"; // 24 chars — will be truncated
    const { container } = render(
      <PodNode data={{ label: longName, status: "Running" }} />
    );
    const span = container.querySelector("span[title]");
    expect(span).toBeTruthy();
    expect(span?.getAttribute("title")).toBe(longName);
  });

  // Regression pin for PodNode's middle-truncate behaviour — preserves both
  // the deployment prefix AND the last 5+ chars of the pod hash.
  it("middle-truncates long pod names, preserving the last 5+ chars of the hash", () => {
    const longName = "prod-api-7d4b8c9f6-xk2mq";
    render(<PodNode data={{ label: longName, status: "Running" }} />);
    // Last 5 chars of the pod hash should be visible in the displayed text
    const rendered = screen.getByTitle(longName).textContent ?? "";
    expect(rendered).toContain("xk2mq");
    // And the prefix should also be present
    expect(rendered.startsWith("prod")).toBe(true);
    // And it must actually be truncated (contain the ellipsis)
    expect(rendered).toContain("\u2026");
  });

  it("shows restart badge when restarts > 0", () => {
    render(<PodNode data={{ label: "p", status: "Running", restarts: 3 }} />);
    expect(screen.getByText("3")).toBeInTheDocument();
  });

  it("omits restart badge when restarts is 0", () => {
    render(<PodNode data={{ label: "p", status: "Running", restarts: 0 }} />);
    expect(screen.queryByText("0")).not.toBeInTheDocument();
  });

  it.each([
    ["running", "bg-emerald-500"],
    ["pending", "bg-amber-500"],
    ["failed", "bg-red-500"],
    ["error", "bg-red-500"],
    ["succeeded", "bg-blue-500"],
    ["unknown-state", "bg-slate-500"],
  ])("maps status %s → indicator %s", (status, expectedClass) => {
    const { container } = render(<PodNode data={{ label: "p", status }} />);
    // Status dot is the first w-2 h-2 rounded circle inside the flex row
    const dot = container.querySelector(`.${expectedClass}`);
    expect(dot).toBeTruthy();
  });

  it.each([
    ["isUpstream", "ring-cyan-400"],
    ["isDownstream", "ring-orange-400"],
    ["isDimmed", "opacity-30"],
  ])("applies %s highlight class", (key, className) => {
    const { container } = render(
      <PodNode data={{ label: "p", status: "Running", [key]: true } as any} />
    );
    expect(container.innerHTML).toContain(className);
  });
});

// ── ServiceNode ──────────────────────────────────────────────────────────────

describe("ServiceNode", () => {
  it("renders label and service type", () => {
    render(<ServiceNode data={{ label: "my-svc", serviceType: "ClusterIP" }} />);
    expect(screen.getByText("my-svc")).toBeInTheDocument();
    expect(screen.getByText("ClusterIP")).toBeInTheDocument();
  });

  it("sets title={label} on the truncated span (regression pin)", () => {
    const label = "extremely-long-service-name-that-overflows";
    const { container } = render(<ServiceNode data={{ label }} />);
    const span = container.querySelector("span[title]");
    expect(span?.getAttribute("title")).toBe(label);
  });

  it.each([
    ["loadbalancer", "border-purple-500"],
    ["nodeport", "border-blue-500"],
    ["clusterip", "border-cyan-500"],
    [undefined, "border-cyan-500"],
  ])("maps serviceType %s → border color %s", (serviceType, expectedClass) => {
    const { container } = render(
      <ServiceNode data={{ label: "s", serviceType }} />
    );
    expect(container.innerHTML).toContain(expectedClass);
  });
});

// ── IngressNode ──────────────────────────────────────────────────────────────

describe("IngressNode", () => {
  it("renders the ingress label", () => {
    render(<IngressNode data={{ label: "my-ingress" }} />);
    expect(screen.getByText("my-ingress")).toBeInTheDocument();
  });

  it("sets title={label} on truncated span (regression pin)", () => {
    const label = "very-long-ingress-name-with-many-hyphens";
    const { container } = render(<IngressNode data={{ label }} />);
    const span = container.querySelector("span[title]");
    expect(span?.getAttribute("title")).toBe(label);
  });

  it("applies selected ring when `selected`", () => {
    const { container } = render(
      <IngressNode data={{ label: "i" }} selected />
    );
    expect(container.innerHTML).toContain("ring-accent-400");
  });
});

// ── ControllerNode ───────────────────────────────────────────────────────────

describe("ControllerNode", () => {
  it.each([
    ["Deployment", "border-green-500"],
    ["StatefulSet", "border-amber-500"],
    ["DaemonSet", "border-rose-500"],
    ["ReplicaSet", "border-slate-500"],
    ["Job", "border-blue-500"],
    ["CronJob", "border-blue-500"],
    ["Operator", "border-indigo-500"],
  ])("renders %s with its border color", (resourceType, expected) => {
    const { container } = render(
      <ControllerNode
        data={{ label: "ctrl", resourceType: resourceType as any }}
      />
    );
    expect(container.innerHTML).toContain(expected);
    expect(screen.getByText("ctrl")).toBeInTheDocument();
    expect(screen.getByText(resourceType)).toBeInTheDocument();
  });

  it("sets title={label} on truncated span (regression pin)", () => {
    const label = "overly-long-controller-name-that-must-truncate";
    const { container } = render(
      <ControllerNode data={{ label, resourceType: "Deployment" }} />
    );
    const span = container.querySelector("span[title]");
    expect(span?.getAttribute("title")).toBe(label);
  });
});

// ── ServerNode ───────────────────────────────────────────────────────────────

describe("ServerNode", () => {
  it("renders label and capacity stats", () => {
    render(
      <ServerNode
        data={{
          label: "node-01",
          status: "Ready",
          cpuCapacity: "4",
          memCapacity: "16Gi",
        }}
      />
    );
    expect(screen.getByText("node-01")).toBeInTheDocument();
    expect(screen.getByText("4")).toBeInTheDocument();
    expect(screen.getByText("16Gi")).toBeInTheDocument();
  });

  it("falls back to N/A when capacity fields are missing", () => {
    render(<ServerNode data={{ label: "n", status: "Ready" }} />);
    expect(screen.getAllByText("N/A").length).toBeGreaterThanOrEqual(2);
  });

  it("applies the ready border when status contains 'ready'", () => {
    const { container } = render(
      <ServerNode data={{ label: "n", status: "Ready" }} />
    );
    expect(container.innerHTML).toContain("border-indigo-500");
  });

  it("applies the not-ready border when status is 'Unknown'", () => {
    const { container } = render(
      <ServerNode data={{ label: "n", status: "Unknown" }} />
    );
    expect(container.innerHTML).toContain("border-red-500");
  });

  it("applies the not-ready border when status is 'NotReady' (no false-positive from substring match)", () => {
    const { container } = render(
      <ServerNode data={{ label: "n", status: "NotReady" }} />
    );
    expect(container.innerHTML).toContain("border-red-500");
  });

  it("applies the not-ready border when status is 'Not Ready' (with space)", () => {
    const { container } = render(
      <ServerNode data={{ label: "n", status: "Not Ready" }} />
    );
    expect(container.innerHTML).toContain("border-red-500");
  });

  it("sets title={label} on truncated span (regression pin)", () => {
    const label = "extremely-long-k8s-node-hostname-something";
    const { container } = render(
      <ServerNode data={{ label, status: "Ready" }} />
    );
    const span = container.querySelector("span[title]");
    expect(span?.getAttribute("title")).toBe(label);
  });
});

// ── NamespaceNode ────────────────────────────────────────────────────────────

describe("NamespaceNode", () => {
  it("renders the namespace label", () => {
    render(<NamespaceNode data={{ label: "kube-system" }} />);
    expect(screen.getByText("kube-system")).toBeInTheDocument();
  });

  it("sets title={label} on truncated span (regression pin)", () => {
    const label = "istio-system-extended-with-suffix-name";
    const { container } = render(<NamespaceNode data={{ label }} />);
    const span = container.querySelector("span[title]");
    expect(span?.getAttribute("title")).toBe(label);
  });
});
