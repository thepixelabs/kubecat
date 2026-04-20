import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { useToastStore } from "./toastStore";
import type { ToastType } from "./toastStore";

// ---------------------------------------------------------------------------
// toastStore is NOT wrapped in `persist` and schedules timers via setTimeout.
// We fake timers around auto-removal tests so they stay deterministic.
// ---------------------------------------------------------------------------

beforeEach(() => {
  // Reset state via merge so the store's actions remain intact.
  useToastStore.setState({ toasts: [] });
});

afterEach(() => {
  // Defensive — always return to real timers between tests
  vi.useRealTimers();
});

// ---------------------------------------------------------------------------
// Initial state
// ---------------------------------------------------------------------------

describe("initial state", () => {
  it("starts with an empty toasts array", () => {
    expect(useToastStore.getState().toasts).toEqual([]);
  });
});

// ---------------------------------------------------------------------------
// addToast
// ---------------------------------------------------------------------------

describe("addToast", () => {
  it("appends a toast with an assigned id", () => {
    useToastStore.getState().addToast({ type: "info", message: "Hello" });
    const toasts = useToastStore.getState().toasts;
    expect(toasts).toHaveLength(1);
    expect(toasts[0].type).toBe("info");
    expect(toasts[0].message).toBe("Hello");
    expect(typeof toasts[0].id).toBe("string");
    expect(toasts[0].id.length).toBeGreaterThan(0);
  });

  it("generates unique ids across many rapid calls", () => {
    for (let i = 0; i < 20; i++) {
      useToastStore.getState().addToast({ type: "info", message: `m${i}` });
    }
    const ids = useToastStore.getState().toasts.map((t) => t.id);
    expect(new Set(ids).size).toBe(20);
  });

  it("preserves insertion order", () => {
    useToastStore.getState().addToast({ type: "info", message: "one" });
    useToastStore.getState().addToast({ type: "success", message: "two" });
    useToastStore.getState().addToast({ type: "error", message: "three" });

    const messages = useToastStore.getState().toasts.map((t) => t.message);
    expect(messages).toEqual(["one", "two", "three"]);
  });

  it.each<ToastType>(["success", "error", "info", "warning"])(
    "supports %s type",
    (type) => {
      useToastStore.getState().addToast({ type, message: "m" });
      expect(useToastStore.getState().toasts[0].type).toBe(type);
    }
  );

  it("auto-removes the toast after the default 3s duration", () => {
    vi.useFakeTimers();
    useToastStore.getState().addToast({ type: "info", message: "ephemeral" });
    expect(useToastStore.getState().toasts).toHaveLength(1);

    vi.advanceTimersByTime(2999);
    expect(useToastStore.getState().toasts).toHaveLength(1);

    vi.advanceTimersByTime(1);
    expect(useToastStore.getState().toasts).toHaveLength(0);
  });

  it("auto-removes after the explicit custom duration", () => {
    vi.useFakeTimers();
    useToastStore.getState().addToast({ type: "info", message: "m", duration: 500 });
    vi.advanceTimersByTime(500);
    expect(useToastStore.getState().toasts).toHaveLength(0);
  });

  it("does NOT schedule auto-removal when duration is 0 (sticky)", () => {
    vi.useFakeTimers();
    useToastStore.getState().addToast({ type: "info", message: "sticky", duration: 0 });
    vi.advanceTimersByTime(60_000);
    expect(useToastStore.getState().toasts).toHaveLength(1);
  });

  it("only removes the matching toast when multiple are queued", () => {
    vi.useFakeTimers();
    useToastStore.getState().addToast({ type: "info", message: "first", duration: 1000 });
    useToastStore.getState().addToast({ type: "info", message: "second", duration: 5000 });

    vi.advanceTimersByTime(1000);
    const remaining = useToastStore.getState().toasts;
    expect(remaining).toHaveLength(1);
    expect(remaining[0].message).toBe("second");
  });
});

// ---------------------------------------------------------------------------
// removeToast
// ---------------------------------------------------------------------------

describe("removeToast", () => {
  it("removes the toast with the matching id", () => {
    useToastStore.getState().addToast({ type: "info", message: "m", duration: 0 });
    const id = useToastStore.getState().toasts[0].id;

    useToastStore.getState().removeToast(id);
    expect(useToastStore.getState().toasts).toHaveLength(0);
  });

  it("is a no-op for an unknown id", () => {
    useToastStore.getState().addToast({ type: "info", message: "m", duration: 0 });
    useToastStore.getState().removeToast("does-not-exist");
    expect(useToastStore.getState().toasts).toHaveLength(1);
  });

  it("leaves sibling toasts intact", () => {
    useToastStore.getState().addToast({ type: "info", message: "a", duration: 0 });
    useToastStore.getState().addToast({ type: "info", message: "b", duration: 0 });

    const firstId = useToastStore.getState().toasts[0].id;
    useToastStore.getState().removeToast(firstId);

    const after = useToastStore.getState().toasts;
    expect(after).toHaveLength(1);
    expect(after[0].message).toBe("b");
  });
});

// ---------------------------------------------------------------------------
// Interaction: manual removal before timer fires
// ---------------------------------------------------------------------------

describe("manual remove + scheduled auto-remove", () => {
  it("does not double-remove when auto-timer fires after manual remove", () => {
    vi.useFakeTimers();
    useToastStore.getState().addToast({ type: "info", message: "m" });
    const id = useToastStore.getState().toasts[0].id;

    useToastStore.getState().removeToast(id);
    expect(useToastStore.getState().toasts).toHaveLength(0);

    // Advancing past the default duration must not re-add or crash
    expect(() => vi.advanceTimersByTime(5000)).not.toThrow();
    expect(useToastStore.getState().toasts).toHaveLength(0);
  });
});
