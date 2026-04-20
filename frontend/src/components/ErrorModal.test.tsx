import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ErrorModal } from "./ErrorModal";

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

describe("ErrorModal", () => {
  const baseProps = {
    isOpen: true,
    onClose: vi.fn(),
    title: "Bad things",
    message: "The cluster caught fire",
  };

  it("renders nothing when closed", () => {
    render(<ErrorModal {...baseProps} isOpen={false} />);
    expect(screen.queryByText("Bad things")).toBeNull();
  });

  it("renders title + message when open", () => {
    render(<ErrorModal {...baseProps} />);
    expect(screen.getByText("Bad things")).toBeInTheDocument();
    expect(screen.getByText("The cluster caught fire")).toBeInTheDocument();
  });

  it("invokes onClose when the footer Close button is clicked", async () => {
    const onClose = vi.fn();
    const user = userEvent.setup();
    render(<ErrorModal {...baseProps} onClose={onClose} />);

    // There are multiple close affordances — the footer one is named "Close".
    await user.click(screen.getByRole("button", { name: /^Close$/i }));
    expect(onClose).toHaveBeenCalled();
  });

  it("invokes onClose when the backdrop is clicked", async () => {
    const onClose = vi.fn();
    const user = userEvent.setup();
    const { container } = render(<ErrorModal {...baseProps} onClose={onClose} />);
    // The outermost motion.div serves as the backdrop.
    const backdrop = container.firstChild as HTMLElement;
    await user.click(backdrop);
    expect(onClose).toHaveBeenCalled();
  });

  it("does not close when clicking the modal body (stopPropagation)", async () => {
    const onClose = vi.fn();
    const user = userEvent.setup();
    render(<ErrorModal {...baseProps} onClose={onClose} />);
    // Clicking the message text should NOT bubble to the backdrop handler.
    await user.click(screen.getByText("The cluster caught fire"));
    expect(onClose).not.toHaveBeenCalled();
  });

  it("preserves whitespace in the message (whitespace-pre-wrap)", () => {
    render(
      <ErrorModal {...baseProps} message={"line1\nline2\n  indented"} />
    );
    // The <p> holding the message has the class — assert it's present.
    const p = screen.getByText(/line1/).closest("p");
    expect(p?.className).toContain("whitespace-pre-wrap");
  });
});
