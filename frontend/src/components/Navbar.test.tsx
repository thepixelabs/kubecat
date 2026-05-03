/**
 * Navbar — label map, short cluster name rendering, context dropdown behavior.
 *
 * Regression targets:
 * - VIEW_LABELS must render "GitOps"/"RBAC"/"Port Forwards" — plain CSS
 *   `capitalize` would produce "Gitops"/"Rbac" which is wrong product naming.
 * - Active cluster pill must use shortClusterName() for display + carry the
 *   full ARN/URI on its `title` attribute for hover.
 * - Context dropdown items must similarly carry `title=` for full visibility.
 * - Dropdown widens to w-80 / sm:w-96 (regression for cramped-layout bug).
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { createRef } from "react";
import { Navbar } from "./Navbar";

const makeProps = (overrides: Partial<React.ComponentProps<typeof Navbar>> = {}) => ({
  activeView: "dashboard",
  isConnected: false,
  connecting: false,
  activeContext: "",
  contexts: [],
  contextMenuIndex: -1,
  showContextMenu: false,
  appVersion: "0.1.0",
  contextMenuContainerRef: createRef<HTMLDivElement>() as React.RefObject<HTMLDivElement | null>,
  onToggleContextMenu: vi.fn(),
  onConnect: vi.fn(),
  onDisconnect: vi.fn(),
  onRefreshContexts: vi.fn(),
  onSetContextMenuIndex: vi.fn(),
  onShowHelp: vi.fn(),
  onShowSettings: vi.fn(),
  ...overrides,
});

beforeEach(() => {
  vi.clearAllMocks();
});

// ---------------------------------------------------------------------------
// View label map
// ---------------------------------------------------------------------------

describe("Navbar — view labels", () => {
  it.each([
    ["gitops", "GitOps"],
    ["rbac", "RBAC"],
    ["port-forwards", "Port Forwards"],
    ["cluster-diff", "Cluster Diff"],
    ["ai-settings", "AI Settings"],
    ["visualizer", "Cluster Visualizer"],
    ["query", "AI Query"],
    ["dashboard", "Dashboard"],
    ["security", "Security"],
  ])("renders '%s' as '%s'", (view, expected) => {
    render(<Navbar {...makeProps({ activeView: view })} />);
    expect(screen.getByRole("heading", { level: 1 }).textContent).toBe(expected);
  });

  it("falls back to title-case for unknown views", () => {
    render(<Navbar {...makeProps({ activeView: "custom-unknown-view" })} />);
    expect(screen.getByRole("heading", { level: 1 }).textContent).toBe(
      "Custom Unknown View"
    );
  });
});

// ---------------------------------------------------------------------------
// Active cluster pill
// ---------------------------------------------------------------------------

describe("Navbar — active cluster pill", () => {
  it("shows shortClusterName for an EKS ARN and carries the full ARN on title", () => {
    const fullArn =
      "arn:aws:eks:us-east-1:123456789012:cluster/prod-api";
    render(
      <Navbar
        {...makeProps({
          isConnected: true,
          activeContext: fullArn,
          contexts: [fullArn],
        })}
      />
    );
    // The active cluster button carries title={full ARN}.
    const pill = screen.getByTitle(fullArn);
    expect(pill).toBeDefined();
    expect(pill.textContent).toContain("prod-api");
    expect(pill.textContent).not.toContain("arn:aws:eks");
  });

  it("shows 'Select cluster' when disconnected", () => {
    render(<Navbar {...makeProps({ isConnected: false, activeContext: "" })} />);
    expect(screen.getByText("Select cluster")).toBeDefined();
  });

  it("shows 'Connecting…' state while connecting", () => {
    render(
      <Navbar
        {...makeProps({
          connecting: true,
          activeContext: "arn:aws:eks:...:cluster/x",
          contexts: ["arn:aws:eks:...:cluster/x"],
        })}
      />
    );
    expect(screen.getByText("Connecting…")).toBeDefined();
  });
});

// ---------------------------------------------------------------------------
// Context dropdown
// ---------------------------------------------------------------------------

describe("Navbar — context dropdown", () => {
  it("renders each context option with title=<full> and shortClusterName as label", () => {
    const ctxs = [
      "arn:aws:eks:us-east-1:111:cluster/alpha",
      "gke_myproj_us-central1-a_beta",
      "kind-local",
    ];
    render(
      <Navbar
        {...makeProps({
          isConnected: false,
          activeContext: "",
          contexts: ctxs,
          showContextMenu: true,
        })}
      />
    );
    for (const ctx of ctxs) {
      expect(screen.getAllByTitle(ctx).length).toBeGreaterThan(0);
    }
    // The first two are ARN/GKE — their short labels should appear.
    expect(screen.getByText("alpha")).toBeDefined();
    // GKE short form adds project/zone in parens.
    expect(screen.getByText(/beta \(myproj\/us-central1-a\)/)).toBeDefined();
    expect(screen.getByText("kind-local")).toBeDefined();
  });

  it("uses a wide dropdown panel (w-80 sm:w-96) so long ARNs don't crowd", () => {
    render(
      <Navbar
        {...makeProps({
          contexts: ["arn:aws:eks:us-east-1:111:cluster/foo"],
          showContextMenu: true,
        })}
      />
    );
    const panel = screen.getByRole("listbox");
    // The class list contains w-80 (mobile) and sm:w-96 (≥sm). This is a
    // regression guard for the pre-truncation cramped dropdown.
    expect(panel.className).toMatch(/w-80/);
    expect(panel.className).toMatch(/sm:w-96/);
  });

  it("renders 'No contexts found in kubeconfig' when list is empty", () => {
    render(<Navbar {...makeProps({ showContextMenu: true, contexts: [] })} />);
    expect(screen.getByText(/No contexts found in kubeconfig/)).toBeDefined();
  });

  it("clicking a context calls onConnect with the full identifier, not the short name", () => {
    const onConnect = vi.fn();
    const fullArn = "arn:aws:eks:us-east-1:111:cluster/prod";
    render(
      <Navbar
        {...makeProps({
          onConnect,
          contexts: [fullArn],
          showContextMenu: true,
        })}
      />
    );
    // The option button — match via role=option which is unique to list items.
    const option = screen.getByRole("option");
    fireEvent.click(option);
    expect(onConnect).toHaveBeenCalledWith(fullArn);
  });

  it("shows the Disconnect affordance when connected", () => {
    render(
      <Navbar
        {...makeProps({
          isConnected: true,
          activeContext: "arn:aws:eks:...:cluster/x",
          contexts: ["arn:aws:eks:...:cluster/x"],
          showContextMenu: true,
        })}
      />
    );
    expect(screen.getByText(/Disconnect from cluster/)).toBeDefined();
  });
});

// ---------------------------------------------------------------------------
// Action buttons
// ---------------------------------------------------------------------------

describe("Navbar — action buttons", () => {
  it("clicking settings calls onShowSettings", () => {
    const onShowSettings = vi.fn();
    render(<Navbar {...makeProps({ onShowSettings })} />);
    fireEvent.click(screen.getByLabelText(/Settings \(,\)/));
    expect(onShowSettings).toHaveBeenCalled();
  });

  it("clicking help calls onShowHelp", () => {
    const onShowHelp = vi.fn();
    render(<Navbar {...makeProps({ onShowHelp })} />);
    fireEvent.click(screen.getByLabelText(/Keyboard shortcuts/));
    expect(onShowHelp).toHaveBeenCalled();
  });

  it("renders optional Terminal and Epics buttons only when handlers are passed", () => {
    const onToggleTerminal = vi.fn();
    const onShowEpics = vi.fn();
    render(
      <Navbar
        {...makeProps({
          onToggleTerminal,
          onShowEpics,
        })}
      />
    );
    expect(screen.getByLabelText(/Open terminal/)).toBeDefined();
    expect(screen.getByLabelText(/Agent epics/)).toBeDefined();
  });

  it("shows 'Close terminal' label when terminal is open", () => {
    render(
      <Navbar
        {...makeProps({
          onToggleTerminal: vi.fn(),
          terminalOpen: true,
        })}
      />
    );
    expect(screen.getByLabelText(/Close terminal/)).toBeDefined();
  });
});
