import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, cleanup } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TerminalDrawer } from "./TerminalDrawer";

// ── Shared xterm mock ────────────────────────────────────────────────────────
// Hoisted so the vi.mock factory can read them. We use a real class expression
// (not vi.fn) because the source constructs the Terminal with `new`, and
// vi.fn() spies cannot be used as constructors.
const { terminalInstances, onDataCallbacks, ctorSpy } = vi.hoisted(() => {
  const terminalInstances: any[] = [];
  const onDataCallbacks: ((data: string) => void)[] = [];
  const ctorSpy = vi.fn();
  return { terminalInstances, onDataCallbacks, ctorSpy };
});

vi.mock("@xterm/xterm", () => {
  class MockTerminal {
    loadAddon = vi.fn();
    open = vi.fn();
    write = vi.fn();
    writeln = vi.fn();
    focus = vi.fn();
    dispose = vi.fn();
    rows = 24;
    cols = 80;
    onData = vi.fn((cb: (d: string) => void) => {
      onDataCallbacks.push(cb);
      return { dispose: vi.fn() };
    });
    constructor(..._args: unknown[]) {
      ctorSpy(..._args);
      terminalInstances.push(this);
    }
  }
  return { Terminal: MockTerminal };
});
vi.mock("@xterm/addon-fit", () => {
  class FitAddon {
    fit = vi.fn();
  }
  return { FitAddon };
});
vi.mock("@xterm/xterm/css/xterm.css", () => ({}));

// ── Wails backend mock ───────────────────────────────────────────────────────

const { mockWrite, mockResize, mockEventsOn, mockEventsOff } = vi.hoisted(() => ({
  mockWrite: vi.fn(),
  mockResize: vi.fn(),
  mockEventsOn: vi.fn(),
  mockEventsOff: vi.fn(),
}));

vi.mock("../../../wailsjs/go/main/App", () => ({
  ResizeTerminal: (...a: unknown[]) => mockResize(...a),
  WriteTerminal: (...a: unknown[]) => mockWrite(...a),
}));

vi.mock("../../../wailsjs/runtime/runtime", () => ({
  EventsOn: (...a: unknown[]) => mockEventsOn(...a),
  EventsOff: (...a: unknown[]) => mockEventsOff(...a),
}));

// ── Tests ────────────────────────────────────────────────────────────────────

beforeEach(() => {
  vi.clearAllMocks();
  terminalInstances.length = 0;
  onDataCallbacks.length = 0;
  cleanup();
});

describe("TerminalDrawer", () => {
  const defaultProps = {
    isOpen: false,
    onClose: vi.fn(),
    sessionId: null as string | null,
    nodeName: "",
    namespace: "",
  };

  it("renders the drawer hidden when not open (translated off-screen)", () => {
    const { container } = render(<TerminalDrawer {...defaultProps} />);
    // The drawer root is always rendered but translated off-screen when closed.
    const drawer = container.firstChild as HTMLElement;
    expect(drawer.className).toContain("translate-y-full");
  });

  it("slides in when isOpen=true and shows the resource header", () => {
    const { container } = render(
      <TerminalDrawer
        {...defaultProps}
        isOpen
        sessionId="s-1"
        nodeName="web-pod-1"
        namespace="default"
      />
    );
    expect(screen.getByText("default/web-pod-1")).toBeInTheDocument();
    const drawer = container.firstChild as HTMLElement;
    expect(drawer.className).toContain("translate-y-0");
  });

  it("invokes onClose when the Close button is clicked", async () => {
    const onClose = vi.fn();
    const user = userEvent.setup();
    render(
      <TerminalDrawer
        {...defaultProps}
        onClose={onClose}
        isOpen
        sessionId="s-1"
      />
    );
    await user.click(screen.getByTitle("Close terminal"));
    expect(onClose).toHaveBeenCalled();
  });

  it("instantiates xterm.Terminal and wires it to the container when session opens", () => {
    render(
      <TerminalDrawer
        {...defaultProps}
        isOpen
        sessionId="sess-123"
        nodeName="web-pod-1"
        namespace="default"
      />
    );
    // One Terminal constructed.
    expect(ctorSpy).toHaveBeenCalledTimes(1);
    const inst = terminalInstances[0];
    expect(inst.open).toHaveBeenCalled();
    expect(inst.focus).toHaveBeenCalled();
    expect(inst.onData).toHaveBeenCalled();
  });

  it("sends typed input to the Wails backend via WriteTerminal", () => {
    render(
      <TerminalDrawer
        {...defaultProps}
        isOpen
        sessionId="sess-123"
        nodeName="p"
        namespace="ns"
      />
    );
    // Simulate user typing — onData was registered during mount.
    onDataCallbacks[0]?.("ls -la\n");
    expect(mockWrite).toHaveBeenCalledWith("sess-123", "ls -la\n");
  });

  it("subscribes to backend data events via EventsOn(terminal:data:<id>)", () => {
    render(
      <TerminalDrawer
        {...defaultProps}
        isOpen
        sessionId="sess-abc"
        nodeName="p"
        namespace="ns"
      />
    );
    expect(mockEventsOn).toHaveBeenCalledWith(
      "terminal:data:sess-abc",
      expect.any(Function)
    );
    expect(mockEventsOn).toHaveBeenCalledWith(
      "terminal:closed:sess-abc",
      expect.any(Function)
    );
  });

  it("decodes base64 backend data and writes it to the terminal", () => {
    render(
      <TerminalDrawer
        {...defaultProps}
        isOpen
        sessionId="sess-b64"
        nodeName="p"
        namespace="ns"
      />
    );
    // The first EventsOn call registered the data callback.
    const dataCall = mockEventsOn.mock.calls.find(
      (c) => c[0] === "terminal:data:sess-b64"
    );
    expect(dataCall).toBeTruthy();
    const cb = dataCall![1] as (s: string) => void;

    const payload = "hello";
    const b64 = btoa(payload);
    cb(b64);

    expect(terminalInstances[0].write).toHaveBeenCalledWith(payload);
  });

  it("writes '[Process completed]' when the backend signals terminal:closed", () => {
    render(
      <TerminalDrawer
        {...defaultProps}
        isOpen
        sessionId="sess-closed"
        nodeName="p"
        namespace="ns"
      />
    );
    const closedCall = mockEventsOn.mock.calls.find(
      (c) => c[0] === "terminal:closed:sess-closed"
    );
    const cb = closedCall![1] as () => void;
    cb();
    expect(terminalInstances[0].writeln).toHaveBeenCalledWith(
      "\r\n[Process completed]\r\n"
    );
  });

  it("reports initial size back via ResizeTerminal on mount", () => {
    render(
      <TerminalDrawer
        {...defaultProps}
        isOpen
        sessionId="sess-resize"
        nodeName="p"
        namespace="ns"
      />
    );
    expect(mockResize).toHaveBeenCalledWith("sess-resize", 24, 80);
  });

  it("cleans up xterm + EventsOff when the session closes", () => {
    const { rerender } = render(
      <TerminalDrawer
        {...defaultProps}
        isOpen
        sessionId="sess-cleanup"
        nodeName="p"
        namespace="ns"
      />
    );
    expect(ctorSpy).toHaveBeenCalledTimes(1);

    // Simulate "session closed" by re-rendering with sessionId=null.
    rerender(
      <TerminalDrawer
        {...defaultProps}
        isOpen
        sessionId={null}
        nodeName="p"
        namespace="ns"
      />
    );

    // The xterm instance from the previous mount must be disposed.
    expect(terminalInstances[0].dispose).toHaveBeenCalled();
    // And the event listeners detached.
    expect(mockEventsOff).toHaveBeenCalledWith("terminal:data:sess-cleanup");
    expect(mockEventsOff).toHaveBeenCalledWith("terminal:closed:sess-cleanup");
  });
});
