import { describe, it, expect, beforeEach, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import { useAppKeyboard } from "./useAppKeyboard";
import type { View } from "../types/resources";

// ---------------------------------------------------------------------------
// Factory — builds a full options bundle with vi.fn() handlers. Individual
// tests override only the fields they care about so each test body stays
// focused on a single behaviour.
// ---------------------------------------------------------------------------

type Options = Parameters<typeof useAppKeyboard>[0];

function buildOptions(overrides: Partial<Options> = {}): Options {
  return {
    showHelp: false,
    showSettings: false,
    showEpics: false,
    showContextMenu: false,
    sidebarCollapsed: false,
    activeView: "dashboard" as View,
    contexts: [],
    contextMenuIndex: 0,
    navItems: [
      { id: "dashboard" },
      { id: "explorer" },
      { id: "logs" },
      { id: "timeline" },
      { id: "security" },
    ],
    onCloseHelp: vi.fn(),
    onCloseSettings: vi.fn(),
    onCloseEpics: vi.fn(),
    onCloseContextMenu: vi.fn(),
    onToggleSidebar: vi.fn(),
    onOpenSettings: vi.fn(),
    onOpenHelp: vi.fn(),
    onToggleContextMenu: vi.fn(),
    onNavigate: vi.fn(),
    onConnect: vi.fn(),
    onSetContextMenuIndex: vi.fn(),
    onZoomIn: vi.fn(),
    onZoomOut: vi.fn(),
    onZoomReset: vi.fn(),
    ...overrides,
  };
}

function press(
  key: string,
  init: KeyboardEventInit = {},
  target: EventTarget = window
) {
  const event = new KeyboardEvent("keydown", {
    key,
    bubbles: true,
    cancelable: true,
    ...init,
  });
  target.dispatchEvent(event);
  return event;
}

// Each test mounts a fresh hook instance
let cleanup: (() => void) | null = null;
beforeEach(() => {
  cleanup?.();
  cleanup = null;
});

function mount(opts: Options) {
  const { unmount } = renderHook((props: Options) => useAppKeyboard(props), {
    initialProps: opts,
  });
  cleanup = unmount;
  return unmount;
}

// ---------------------------------------------------------------------------
// Suppression inside text inputs / textareas / contenteditable
// ---------------------------------------------------------------------------

describe("input suppression", () => {
  it("ignores shortcuts while focus is inside an INPUT", () => {
    const opts = buildOptions();
    mount(opts);

    const input = document.createElement("input");
    document.body.appendChild(input);
    try {
      press("c", {}, input);
      expect(opts.onToggleContextMenu).not.toHaveBeenCalled();
      press("[", {}, input);
      expect(opts.onToggleSidebar).not.toHaveBeenCalled();
    } finally {
      input.remove();
    }
  });

  it("ignores shortcuts while focus is inside a TEXTAREA", () => {
    const opts = buildOptions();
    mount(opts);

    const ta = document.createElement("textarea");
    document.body.appendChild(ta);
    try {
      press(",", {}, ta);
      expect(opts.onOpenSettings).not.toHaveBeenCalled();
    } finally {
      ta.remove();
    }
  });

  it("ignores shortcuts inside a contentEditable element", () => {
    const opts = buildOptions();
    mount(opts);

    const div = document.createElement("div");
    div.contentEditable = "true";
    // jsdom does not fully wire `isContentEditable` from the attribute, so we
    // force the getter to return true to exercise the suppression path the
    // hook actually checks.
    Object.defineProperty(div, "isContentEditable", {
      configurable: true,
      get: () => true,
    });
    document.body.appendChild(div);
    try {
      press("?", {}, div);
      expect(opts.onOpenHelp).not.toHaveBeenCalled();
    } finally {
      div.remove();
    }
  });

  it("Escape inside an input blurs the input but does NOT close overlays", () => {
    const opts = buildOptions({ showHelp: true });
    mount(opts);

    const input = document.createElement("input");
    document.body.appendChild(input);
    input.focus();
    const blurSpy = vi.spyOn(input, "blur");
    try {
      press("Escape", {}, input);
      expect(blurSpy).toHaveBeenCalled();
      expect(opts.onCloseHelp).not.toHaveBeenCalled();
    } finally {
      input.remove();
    }
  });
});

// ---------------------------------------------------------------------------
// Escape cascade — closes the top-most overlay in documented priority
// ---------------------------------------------------------------------------

describe("Escape cascade", () => {
  it("closes help first when help is open", () => {
    const opts = buildOptions({
      showHelp: true,
      showSettings: true,
      showEpics: true,
      showContextMenu: true,
    });
    mount(opts);

    press("Escape");
    expect(opts.onCloseHelp).toHaveBeenCalledTimes(1);
    expect(opts.onCloseSettings).not.toHaveBeenCalled();
    expect(opts.onCloseEpics).not.toHaveBeenCalled();
    expect(opts.onCloseContextMenu).not.toHaveBeenCalled();
  });

  it("closes settings when only settings is open", () => {
    const opts = buildOptions({ showSettings: true });
    mount(opts);
    press("Escape");
    expect(opts.onCloseSettings).toHaveBeenCalledTimes(1);
  });

  it("closes epics when only epics is open", () => {
    const opts = buildOptions({ showEpics: true });
    mount(opts);
    press("Escape");
    expect(opts.onCloseEpics).toHaveBeenCalledTimes(1);
  });

  it("closes context menu when only it is open", () => {
    const opts = buildOptions({ showContextMenu: true });
    mount(opts);
    press("Escape");
    expect(opts.onCloseContextMenu).toHaveBeenCalledTimes(1);
  });

  it("does nothing on Escape with no overlays open", () => {
    const opts = buildOptions();
    mount(opts);
    press("Escape");
    expect(opts.onCloseHelp).not.toHaveBeenCalled();
    expect(opts.onCloseSettings).not.toHaveBeenCalled();
    expect(opts.onCloseEpics).not.toHaveBeenCalled();
    expect(opts.onCloseContextMenu).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// Context menu navigation
// ---------------------------------------------------------------------------

describe("context menu navigation", () => {
  it("ArrowDown increments index with wrap-around", () => {
    const opts = buildOptions({
      showContextMenu: true,
      contexts: ["a", "b", "c"],
      contextMenuIndex: 2,
    });
    mount(opts);

    press("ArrowDown");
    expect(opts.onSetContextMenuIndex).toHaveBeenCalledWith(0);
  });

  it("ArrowUp decrements index with wrap-around", () => {
    const opts = buildOptions({
      showContextMenu: true,
      contexts: ["a", "b", "c"],
      contextMenuIndex: 0,
    });
    mount(opts);

    press("ArrowUp");
    expect(opts.onSetContextMenuIndex).toHaveBeenCalledWith(2);
  });

  it("Enter connects to the highlighted context", () => {
    const opts = buildOptions({
      showContextMenu: true,
      contexts: ["prod", "staging", "dev"],
      contextMenuIndex: 1,
    });
    mount(opts);

    press("Enter");
    expect(opts.onConnect).toHaveBeenCalledWith("staging");
  });

  it("ArrowDown is a no-op when menu is closed", () => {
    const opts = buildOptions({
      showContextMenu: false,
      contexts: ["a", "b"],
    });
    mount(opts);

    press("ArrowDown");
    expect(opts.onSetContextMenuIndex).not.toHaveBeenCalled();
  });

  it("ArrowDown is a no-op when contexts list is empty", () => {
    const opts = buildOptions({
      showContextMenu: true,
      contexts: [],
    });
    mount(opts);

    press("ArrowDown");
    expect(opts.onSetContextMenuIndex).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// Zoom shortcuts — Cmd/Ctrl + =, -, 0
// ---------------------------------------------------------------------------

describe("zoom shortcuts", () => {
  it("metaKey + '=' triggers zoom in", () => {
    const opts = buildOptions();
    mount(opts);
    press("=", { metaKey: true });
    expect(opts.onZoomIn).toHaveBeenCalledTimes(1);
  });

  it("metaKey + '+' also triggers zoom in (Shift-equals)", () => {
    const opts = buildOptions();
    mount(opts);
    press("+", { metaKey: true });
    expect(opts.onZoomIn).toHaveBeenCalledTimes(1);
  });

  it("ctrlKey + '-' triggers zoom out", () => {
    const opts = buildOptions();
    mount(opts);
    press("-", { ctrlKey: true });
    expect(opts.onZoomOut).toHaveBeenCalledTimes(1);
  });

  it("metaKey + '0' triggers zoom reset", () => {
    const opts = buildOptions();
    mount(opts);
    press("0", { metaKey: true });
    expect(opts.onZoomReset).toHaveBeenCalledTimes(1);
  });

  it("bare '0' WITHOUT modifier does NOT trigger zoom (navigates instead)", () => {
    const opts = buildOptions();
    mount(opts);
    press("0");
    expect(opts.onZoomReset).not.toHaveBeenCalled();
    // navItems has 5 items, so key "0" -> index 9 is out of range
    expect(opts.onNavigate).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// Numeric navigation — 1..9, 0 maps to index 9
// ---------------------------------------------------------------------------

describe("numeric navigation", () => {
  it("'1' navigates to the first nav item", () => {
    const opts = buildOptions();
    mount(opts);
    press("1");
    expect(opts.onNavigate).toHaveBeenCalledWith("dashboard");
  });

  it("'5' navigates to the fifth nav item", () => {
    const opts = buildOptions();
    mount(opts);
    press("5");
    expect(opts.onNavigate).toHaveBeenCalledWith("security");
  });

  it("'0' would navigate to the 10th item (index 9) — ignored when nav is shorter", () => {
    const opts = buildOptions(); // only 5 nav items
    mount(opts);
    press("0");
    expect(opts.onNavigate).not.toHaveBeenCalled();
  });

  it("'0' navigates to the 10th nav item when provided", () => {
    const navItems = Array.from({ length: 10 }, (_, i) => ({
      id: (i === 9 ? "rbac" : "dashboard") as View,
    }));
    const opts = buildOptions({ navItems });
    mount(opts);
    press("0");
    expect(opts.onNavigate).toHaveBeenCalledWith("rbac");
  });

  it("numeric keys are ignored when help is open", () => {
    const opts = buildOptions({ showHelp: true });
    mount(opts);
    press("1");
    expect(opts.onNavigate).not.toHaveBeenCalled();
  });

  it("numeric keys are ignored when settings is open", () => {
    const opts = buildOptions({ showSettings: true });
    mount(opts);
    press("1");
    expect(opts.onNavigate).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// Single-char shortcuts
// ---------------------------------------------------------------------------

describe("single-char shortcuts", () => {
  it("'[' toggles the sidebar", () => {
    const opts = buildOptions();
    mount(opts);
    press("[");
    expect(opts.onToggleSidebar).toHaveBeenCalledTimes(1);
  });

  it("',' opens settings", () => {
    const opts = buildOptions();
    mount(opts);
    press(",");
    expect(opts.onOpenSettings).toHaveBeenCalledTimes(1);
  });

  it("'?' opens help", () => {
    const opts = buildOptions();
    mount(opts);
    press("?");
    expect(opts.onOpenHelp).toHaveBeenCalledTimes(1);
  });

  it("'c' toggles the context menu and resets index when opening", () => {
    const opts = buildOptions({ showContextMenu: false });
    mount(opts);
    press("c");
    expect(opts.onToggleContextMenu).toHaveBeenCalledTimes(1);
    expect(opts.onSetContextMenuIndex).toHaveBeenCalledWith(0);
  });

  it("'c' toggles the context menu and does NOT reset index when closing", () => {
    const opts = buildOptions({ showContextMenu: true });
    mount(opts);
    press("c");
    expect(opts.onToggleContextMenu).toHaveBeenCalledTimes(1);
    // showContextMenu is true -> closing -> index should not be reset
    expect(opts.onSetContextMenuIndex).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// '/' focus search on explorer only
// ---------------------------------------------------------------------------

describe("'/' search shortcut", () => {
  it("focuses the explorer search input when on explorer", () => {
    const opts = buildOptions({ activeView: "explorer" });
    mount(opts);

    const searchInput = document.createElement("input");
    searchInput.setAttribute("placeholder", "Search resources");
    const focusSpy = vi.spyOn(searchInput, "focus");
    document.body.appendChild(searchInput);

    try {
      press("/");
      expect(focusSpy).toHaveBeenCalled();
    } finally {
      searchInput.remove();
    }
  });

  it("does not attempt focus on non-explorer views", () => {
    const opts = buildOptions({ activeView: "dashboard" });
    mount(opts);

    const searchInput = document.createElement("input");
    searchInput.setAttribute("placeholder", "Search resources");
    const focusSpy = vi.spyOn(searchInput, "focus");
    document.body.appendChild(searchInput);

    try {
      press("/");
      expect(focusSpy).not.toHaveBeenCalled();
    } finally {
      searchInput.remove();
    }
  });
});

// ---------------------------------------------------------------------------
// Cleanup — listener must be removed on unmount
// ---------------------------------------------------------------------------

describe("lifecycle", () => {
  it("removes the keydown listener on unmount", () => {
    const opts = buildOptions();
    const removeSpy = vi.spyOn(window, "removeEventListener");
    const { unmount } = renderHook(() => useAppKeyboard(opts));
    unmount();

    const keydownCalls = removeSpy.mock.calls.filter(
      (args) => args[0] === "keydown"
    );
    expect(keydownCalls.length).toBeGreaterThan(0);
  });

  it("after unmount, subsequent keypresses do not invoke handlers", () => {
    const opts = buildOptions();
    const { unmount } = renderHook(() => useAppKeyboard(opts));
    unmount();

    press("?");
    press("[");
    press(",");
    expect(opts.onOpenHelp).not.toHaveBeenCalled();
    expect(opts.onToggleSidebar).not.toHaveBeenCalled();
    expect(opts.onOpenSettings).not.toHaveBeenCalled();
  });
});
