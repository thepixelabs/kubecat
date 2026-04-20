import { describe, it, expect, beforeEach, vi } from "vitest";
import { render, screen, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ToastContainer } from "./ToastContainer";
import { useToastStore } from "../stores/toastStore";

beforeEach(() => {
  // Reset the store between tests.
  useToastStore.setState({ toasts: [] });
});

describe("ToastContainer", () => {
  it("renders nothing when there are no toasts", () => {
    const { container } = render(<ToastContainer />);
    expect(container.firstChild).toBeNull();
  });

  it("renders a success toast with its message", () => {
    act(() => {
      useToastStore.getState().addToast({
        type: "success",
        message: "Applied!",
        duration: 0,
      });
    });
    render(<ToastContainer />);
    expect(screen.getByText("Applied!")).toBeInTheDocument();
  });

  it.each(["success", "error", "info", "warning"] as const)(
    "renders a %s-typed toast",
    (type) => {
      act(() => {
        useToastStore.getState().addToast({
          type,
          message: `msg-${type}`,
          duration: 0,
        });
      });
      render(<ToastContainer />);
      expect(screen.getByText(`msg-${type}`)).toBeInTheDocument();
    }
  );

  it("dismissing via the X removes the toast from the store", async () => {
    act(() => {
      useToastStore.getState().addToast({
        type: "info",
        message: "ephemeral",
        duration: 0,
      });
    });
    const user = userEvent.setup();
    render(<ToastContainer />);

    // Click the only close button in the container.
    const { container } = render(<ToastContainer />);
    const closeBtn = container.querySelector("button");
    await user.click(closeBtn!);

    expect(useToastStore.getState().toasts).toHaveLength(0);
  });

  it("auto-dismisses after the specified duration", async () => {
    vi.useFakeTimers();
    try {
      act(() => {
        useToastStore.getState().addToast({
          type: "info",
          message: "gone in a flash",
          duration: 100,
        });
      });
      expect(useToastStore.getState().toasts).toHaveLength(1);

      await act(async () => {
        vi.advanceTimersByTime(150);
      });

      expect(useToastStore.getState().toasts).toHaveLength(0);
    } finally {
      vi.useRealTimers();
    }
  });

  it("renders multiple toasts simultaneously in the order they were added", () => {
    act(() => {
      useToastStore.getState().addToast({
        type: "success",
        message: "first",
        duration: 0,
      });
      useToastStore.getState().addToast({
        type: "error",
        message: "second",
        duration: 0,
      });
    });
    render(<ToastContainer />);
    const first = screen.getByText("first");
    const second = screen.getByText("second");
    // Both visible
    expect(first).toBeInTheDocument();
    expect(second).toBeInTheDocument();
    // First should appear before second in DOM order
    expect(
      first.compareDocumentPosition(second) &
        Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy();
  });
});
