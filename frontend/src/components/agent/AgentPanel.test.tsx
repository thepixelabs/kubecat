import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, act } from "@testing-library/react";
import { AgentPanel } from "./AgentPanel";

// ── Wails EventsOn capture ───────────────────────────────────────────────────
// The panel wires itself to the runtime via EventsOn and expects it to return
// an unsubscribe function. We capture registered handlers so the test can fire
// them synthetically.

const { handlers, mockEventsOn, unsubMock } = vi.hoisted(() => {
  const handlers = new Map<string, (data: unknown) => void>();
  const unsubMock = vi.fn();
  const mockEventsOn = vi.fn((event: string, cb: (d: unknown) => void) => {
    handlers.set(event, cb);
    return unsubMock;
  });
  return { handlers, mockEventsOn, unsubMock };
});

vi.mock("../../../wailsjs/runtime/runtime", () => ({
  EventsOn: (...args: unknown[]) => mockEventsOn(...args),
  EventsOff: vi.fn(),
}));

// ── Strip framer + markdown ──────────────────────────────────────────────────

vi.mock("framer-motion", () => ({
  motion: {
    div: ({ children, ...props }: React.HTMLAttributes<HTMLDivElement>) => (
      <div {...props}>{children}</div>
    ),
  },
  AnimatePresence: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
}));
vi.mock("react-markdown", () => ({
  default: ({ children }: { children: string }) => (
    <div data-testid="md">{children}</div>
  ),
}));
vi.mock("rehype-sanitize", () => ({
  default: () => ({}),
  defaultSchema: { tagNames: [], attributes: {} },
}));

// ── Helpers ──────────────────────────────────────────────────────────────────

function fire(event: string, data: unknown) {
  const handler = handlers.get(event);
  if (!handler) throw new Error(`No handler registered for ${event}`);
  act(() => {
    handler(data);
  });
}

beforeEach(() => {
  handlers.clear();
  mockEventsOn.mockClear();
  unsubMock.mockClear();
});

// ── Tests ────────────────────────────────────────────────────────────────────

describe("AgentPanel", () => {
  it("subscribes to all agent event channels on mount", () => {
    render(<AgentPanel sessionId="s1" />);
    expect(mockEventsOn).toHaveBeenCalledWith(
      "ai:agent:thinking",
      expect.any(Function)
    );
    expect(mockEventsOn).toHaveBeenCalledWith(
      "ai:agent:tool-call",
      expect.any(Function)
    );
    expect(mockEventsOn).toHaveBeenCalledWith(
      "ai:agent:tool-result",
      expect.any(Function)
    );
    expect(mockEventsOn).toHaveBeenCalledWith(
      "ai:agent:complete",
      expect.any(Function)
    );
    expect(mockEventsOn).toHaveBeenCalledWith(
      "ai:agent:error",
      expect.any(Function)
    );
  });

  it("renders idle empty-state when no events have fired", () => {
    render(<AgentPanel sessionId="s1" />);
    expect(screen.getByText("Agent is ready")).toBeInTheDocument();
    // Status bar label defaults to "Agent"
    expect(screen.getByText("Agent")).toBeInTheDocument();
  });

  it('renders the "Thinking..." state when a thinking event arrives', () => {
    render(<AgentPanel sessionId="s1" />);
    fire("ai:agent:thinking", { sessionId: "s1", message: "Pondering..." });
    expect(screen.getByText("Pondering...")).toBeInTheDocument();
    // Status bar updates.
    expect(screen.getAllByText(/Thinking/i).length).toBeGreaterThanOrEqual(1);
  });

  it("ignores events whose sessionId does not match", () => {
    render(<AgentPanel sessionId="mine" />);
    fire("ai:agent:thinking", {
      sessionId: "other",
      message: "Should not appear",
    });
    expect(screen.queryByText("Should not appear")).toBeNull();
    // Status stays idle
    expect(screen.getByText("Agent is ready")).toBeInTheDocument();
  });

  it("renders the final answer panel when the agent completes", () => {
    render(<AgentPanel sessionId="s1" />);
    fire("ai:agent:complete", {
      sessionId: "s1",
      answer: "the root cause is X",
      totalTokens: 1234,
    });
    expect(screen.getByText("Final Answer")).toBeInTheDocument();
    expect(screen.getByTestId("md")).toHaveTextContent("the root cause is X");
    // Token meter updates.
    expect(screen.getByText(/1,234 tok/)).toBeInTheDocument();
  });

  it("renders the error in both the feed and the dedicated error footer", () => {
    render(<AgentPanel sessionId="s1" />);
    fire("ai:agent:error", {
      sessionId: "s1",
      message: "API key invalid",
    });
    // Both the event-row (span) and the footer (p) render the message.
    const matches = screen.getAllByText("API key invalid");
    expect(matches.length).toBe(2);
    // One of them should be a <p> — the dedicated footer region.
    const footer = matches.find((el) => el.tagName === "P");
    expect(footer).toBeTruthy();
  });

  it("counts tool calls and increments iterations per thinking event", () => {
    render(<AgentPanel sessionId="s1" />);
    fire("ai:agent:thinking", { sessionId: "s1", message: "plan" });
    fire("ai:agent:tool-call", {
      sessionId: "s1",
      tool: {
        id: "t1",
        name: "get_pods",
        description: "list pods",
        risk: "read",
        parameters: {},
      },
    });
    // "1 tool" badge
    expect(screen.getByText(/1 tool/)).toBeInTheDocument();
    // "1 iter" badge
    expect(screen.getByText(/1 iter/)).toBeInTheDocument();
  });

  it("shows the Stop button only while active and fires onStop", async () => {
    const onStop = vi.fn();
    const { rerender: _r } = render(
      <AgentPanel sessionId="s1" onStop={onStop} />
    );
    // Idle → no Stop button.
    expect(screen.queryByRole("button", { name: /Stop agent/i })).toBeNull();

    fire("ai:agent:thinking", { sessionId: "s1" });
    const stopBtn = screen.getByRole("button", { name: /Stop agent/i });
    // userEvent is overkill here — fire a native click.
    stopBtn.click();
    expect(onStop).toHaveBeenCalled();
  });
});
