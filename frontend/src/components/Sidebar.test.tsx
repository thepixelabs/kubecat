/**
 * Sidebar — navigation rail and footer connection state.
 *
 * Regression targets:
 * - Connection status shows shortClusterName with full value on title=.
 * - Nav items fire onNavigate with the correct id.
 * - Collapsed mode hides the status label but keeps the dot.
 * - contextQueueCount badge shows on the AI Query item.
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { Sidebar } from "./Sidebar";
import { Home, Sparkles, Server } from "lucide-react";

const navItems = [
  { id: "dashboard", label: "Dashboard", icon: Home, shortcut: "g d" },
  { id: "query", label: "AI Query", icon: Sparkles, shortcut: "g q" },
  { id: "resources", label: "Resources", icon: Server, shortcut: "g r" },
];

const makeProps = (overrides: Partial<React.ComponentProps<typeof Sidebar>> = {}) => ({
  navItems,
  activeView: "dashboard",
  sidebarCollapsed: false,
  isConnected: false,
  appVersion: "1.0.0",
  contextQueueCount: 0,
  onNavigate: vi.fn(),
  onToggleCollapse: vi.fn(),
  ...overrides,
});

beforeEach(() => {
  vi.clearAllMocks();
});

// ---------------------------------------------------------------------------
// Connection status
// ---------------------------------------------------------------------------

describe("Sidebar — connection status", () => {
  it("shows shortClusterName with title=<full ARN> when connected", () => {
    const fullArn = "arn:aws:eks:us-east-1:999:cluster/production";
    render(<Sidebar {...makeProps({ isConnected: true, activeContext: fullArn })} />);
    const label = screen.getByTitle(fullArn);
    expect(label.textContent).toContain("production");
    expect(label.textContent).not.toContain("arn:aws:eks");
  });

  it("shows 'Connected' as fallback when no activeContext is provided", () => {
    render(<Sidebar {...makeProps({ isConnected: true })} />);
    expect(screen.getByText("Connected")).toBeDefined();
  });

  it("shows 'No cluster' when disconnected", () => {
    render(<Sidebar {...makeProps({ isConnected: false })} />);
    expect(screen.getByText("No cluster")).toBeDefined();
  });

  it("hides the status label text when collapsed", () => {
    render(
      <Sidebar
        {...makeProps({
          isConnected: true,
          activeContext: "arn:aws:eks:...:cluster/foo",
          sidebarCollapsed: true,
        })}
      />
    );
    // The "foo" label wraps only the expanded footer — collapsed state removes it.
    expect(screen.queryByText(/foo/)).toBeNull();
  });
});

// ---------------------------------------------------------------------------
// Nav item behavior
// ---------------------------------------------------------------------------

describe("Sidebar — nav items", () => {
  it("renders each nav item with its label and shortcut", () => {
    render(<Sidebar {...makeProps()} />);
    expect(screen.getByText("Dashboard")).toBeDefined();
    expect(screen.getByText("AI Query")).toBeDefined();
    expect(screen.getByText("Resources")).toBeDefined();
    expect(screen.getByText("g d")).toBeDefined();
    expect(screen.getByText("g q")).toBeDefined();
  });

  it("calls onNavigate with the item id on click", () => {
    const onNavigate = vi.fn();
    render(<Sidebar {...makeProps({ onNavigate })} />);
    // aria-label contains the full label; use that to find the button.
    fireEvent.click(screen.getByRole("button", { name: /^AI Query,/ }));
    expect(onNavigate).toHaveBeenCalledWith("query");
  });

  it("marks the active view with aria-current", () => {
    render(<Sidebar {...makeProps({ activeView: "query" })} />);
    const active = screen.getByRole("button", { name: /^AI Query,/ });
    expect(active.getAttribute("aria-current")).toBe("page");
    const inactive = screen.getByRole("button", { name: /^Dashboard,/ });
    expect(inactive.getAttribute("aria-current")).toBeNull();
  });
});

// ---------------------------------------------------------------------------
// Context queue badge
// ---------------------------------------------------------------------------

describe("Sidebar — context queue badge", () => {
  it("shows badge count on the AI Query item when contextQueueCount > 0", () => {
    render(<Sidebar {...makeProps({ contextQueueCount: 3 })} />);
    // Find the badge via its numeric content inside the AI Query button area.
    const badges = screen.getAllByText("3");
    expect(badges.length).toBeGreaterThan(0);
  });

  it("caps the badge at '9+' for counts above 9", () => {
    render(<Sidebar {...makeProps({ contextQueueCount: 42 })} />);
    expect(screen.getAllByText("9+").length).toBeGreaterThan(0);
  });

  it("does not show a badge when contextQueueCount is 0", () => {
    render(<Sidebar {...makeProps({ contextQueueCount: 0 })} />);
    expect(screen.queryByText(/^9\+$/)).toBeNull();
  });
});

// ---------------------------------------------------------------------------
// Collapse toggle
// ---------------------------------------------------------------------------

describe("Sidebar — collapse toggle", () => {
  it("calls onToggleCollapse when the toggle button is clicked", () => {
    const onToggleCollapse = vi.fn();
    render(<Sidebar {...makeProps({ onToggleCollapse })} />);
    fireEvent.click(screen.getByLabelText(/Collapse sidebar/));
    expect(onToggleCollapse).toHaveBeenCalled();
  });

  it("uses 'Expand sidebar' label when collapsed", () => {
    render(<Sidebar {...makeProps({ sidebarCollapsed: true })} />);
    expect(screen.getByLabelText(/Expand sidebar/)).toBeDefined();
  });
});
