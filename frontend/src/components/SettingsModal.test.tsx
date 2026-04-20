/**
 * SettingsModal — tab routing, close interactions, and initialTab deep-link.
 *
 * The modal wraps ThemeSettings / TelemetrySettings / CostSettings /
 * AIProviderSettings. We mock the heavy subtrees so each test isolates tab
 * behavior; individual section tests live in their own files.
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { SettingsModal } from "./SettingsModal";

// ---------------------------------------------------------------------------
// Isolate the modal: replace child tab panels with cheap stubs so we test
// routing, not their internals.
// ---------------------------------------------------------------------------

vi.mock("./ThemeSettings", () => ({
  ThemeSettings: () => <div data-testid="panel-appearance">Theme panel</div>,
}));
vi.mock("./TelemetrySettings", () => ({
  TelemetrySettings: () => <div data-testid="panel-telemetry">Telemetry panel</div>,
}));
vi.mock("./CostSettings", () => ({
  CostSettings: () => <div data-testid="panel-cost">Cost panel</div>,
}));
vi.mock("./AIProviderSettings", () => ({
  AIProviderSettings: () => <div data-testid="panel-ai">AI provider panel</div>,
}));

// Framer-motion's AnimatePresence exit animation can leave nodes in the DOM
// briefly; default behavior is fine for our assertions using findBy / waitFor.

// ---------------------------------------------------------------------------
// Defaults + factory
// ---------------------------------------------------------------------------

const defaultProps = {
  isOpen: true,
  onClose: vi.fn(),
  colorTheme: "cyan" as const,
  setColorTheme: vi.fn(),
  selectionColor: "accent-500",
  setSelectionColor: vi.fn(),
};

beforeEach(() => {
  vi.clearAllMocks();
});

// ---------------------------------------------------------------------------
// Visibility
// ---------------------------------------------------------------------------

describe("SettingsModal — visibility", () => {
  it("does not render dialog content when isOpen is false", () => {
    render(<SettingsModal {...defaultProps} isOpen={false} />);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("renders dialog with a11y attributes when isOpen is true", () => {
    render(<SettingsModal {...defaultProps} />);
    const dialog = screen.getByRole("dialog");
    expect(dialog.getAttribute("aria-modal")).toBe("true");
    expect(dialog.getAttribute("aria-labelledby")).toBe("settings-title");
  });
});

// ---------------------------------------------------------------------------
// initialTab routing
// ---------------------------------------------------------------------------

describe("SettingsModal — initialTab routing", () => {
  it.each([
    ["appearance", "panel-appearance"],
    ["ai", "panel-ai"],
    ["cost", "panel-cost"],
    ["telemetry", "panel-telemetry"],
  ] as const)("opens on the %s tab when initialTab is %s", async (tab, panelId) => {
    render(<SettingsModal {...defaultProps} initialTab={tab} />);
    expect(await screen.findByTestId(panelId)).toBeDefined();
  });

  it("defaults to the appearance tab when no initialTab is given", async () => {
    render(<SettingsModal {...defaultProps} />);
    expect(await screen.findByTestId("panel-appearance")).toBeDefined();
  });

  it("syncs the active tab when initialTab changes while open", async () => {
    const { rerender } = render(
      <SettingsModal {...defaultProps} initialTab="appearance" />
    );
    await screen.findByTestId("panel-appearance");
    rerender(<SettingsModal {...defaultProps} initialTab="ai" />);
    await waitFor(() => {
      expect(screen.queryByTestId("panel-ai")).toBeDefined();
    });
  });
});

// ---------------------------------------------------------------------------
// Tab switching via clicks
// ---------------------------------------------------------------------------

describe("SettingsModal — tab switching", () => {
  it("clicking the AI Provider tab shows the AI panel", async () => {
    render(<SettingsModal {...defaultProps} />);
    fireEvent.click(screen.getByRole("tab", { name: /AI Provider/i }));
    expect(await screen.findByTestId("panel-ai")).toBeDefined();
  });

  it("clicking the Cost tab shows the Cost panel", async () => {
    render(<SettingsModal {...defaultProps} />);
    fireEvent.click(screen.getByRole("tab", { name: /^Cost$/i }));
    expect(await screen.findByTestId("panel-cost")).toBeDefined();
  });

  it("clicking the Analytics tab shows the Telemetry panel", async () => {
    render(<SettingsModal {...defaultProps} />);
    fireEvent.click(screen.getByRole("tab", { name: /Analytics/i }));
    expect(await screen.findByTestId("panel-telemetry")).toBeDefined();
  });

  it("sets aria-selected on the active tab only", async () => {
    render(<SettingsModal {...defaultProps} initialTab="ai" />);
    const aiTab = screen.getByRole("tab", { name: /AI Provider/i });
    const apTab = screen.getByRole("tab", { name: /Appearance/i });
    expect(aiTab.getAttribute("aria-selected")).toBe("true");
    expect(apTab.getAttribute("aria-selected")).toBe("false");
  });
});

// ---------------------------------------------------------------------------
// Close interactions
// ---------------------------------------------------------------------------

describe("SettingsModal — closing", () => {
  it("calls onClose when the X button is clicked", () => {
    const onClose = vi.fn();
    render(<SettingsModal {...defaultProps} onClose={onClose} />);
    fireEvent.click(screen.getByRole("button", { name: /Close settings/i }));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("calls onClose when Escape is pressed", () => {
    const onClose = vi.fn();
    render(<SettingsModal {...defaultProps} onClose={onClose} />);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("does NOT attach the Escape handler when closed", () => {
    const onClose = vi.fn();
    render(<SettingsModal {...defaultProps} isOpen={false} onClose={onClose} />);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// Focus trap — Tab / Shift+Tab cycle within the dialog
// ---------------------------------------------------------------------------

describe("SettingsModal — focus trap", () => {
  it("focuses the Close button on open", async () => {
    render(<SettingsModal {...defaultProps} />);
    await waitFor(() => {
      expect(document.activeElement).toBe(
        screen.getByRole("button", { name: /Close settings/i })
      );
    });
  });

  it("wraps Tab from the last focusable element back to the first", () => {
    render(<SettingsModal {...defaultProps} />);
    const dialog = screen.getByRole("dialog");
    const focusable = dialog.querySelectorAll<HTMLElement>(
      'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
    );
    const first = focusable[0];
    const last = focusable[focusable.length - 1];

    // Park focus on the last element, then press Tab.
    last.focus();
    expect(document.activeElement).toBe(last);

    const ev = new KeyboardEvent("keydown", {
      key: "Tab",
      bubbles: true,
      cancelable: true,
    });
    document.dispatchEvent(ev);

    expect(document.activeElement).toBe(first);
  });

  it("wraps Shift+Tab from the first focusable element back to the last", () => {
    render(<SettingsModal {...defaultProps} />);
    const dialog = screen.getByRole("dialog");
    const focusable = dialog.querySelectorAll<HTMLElement>(
      'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
    );
    const first = focusable[0];
    const last = focusable[focusable.length - 1];

    first.focus();
    expect(document.activeElement).toBe(first);

    const ev = new KeyboardEvent("keydown", {
      key: "Tab",
      shiftKey: true,
      bubbles: true,
      cancelable: true,
    });
    document.dispatchEvent(ev);

    expect(document.activeElement).toBe(last);
  });

  it("restores focus to the previously focused element on close", async () => {
    // Stage a trigger button that owns focus before the dialog opens.
    const trigger = document.createElement("button");
    trigger.textContent = "Open settings";
    document.body.appendChild(trigger);
    trigger.focus();

    const { rerender } = render(
      <SettingsModal {...defaultProps} isOpen={false} />
    );
    // Open.
    rerender(<SettingsModal {...defaultProps} isOpen />);
    await waitFor(() => {
      expect(document.activeElement).not.toBe(trigger);
    });
    // Close.
    rerender(<SettingsModal {...defaultProps} isOpen={false} />);
    await waitFor(() => {
      expect(document.activeElement).toBe(trigger);
    });
    trigger.remove();
  });
});
