import { describe, it, expect, vi } from "vitest";
import { render, screen, cleanup } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HelpModal } from "./HelpModal";

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

describe("HelpModal", () => {
  it("renders nothing when closed", () => {
    render(<HelpModal isOpen={false} onClose={() => {}} />);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("renders the title and at least one shortcut group when open", () => {
    render(<HelpModal isOpen onClose={() => {}} />);
    expect(screen.getByRole("dialog", { name: /Keyboard Shortcuts/i })).toBeInTheDocument();
    expect(screen.getByText("Navigation")).toBeInTheDocument();
    expect(screen.getByText("Global")).toBeInTheDocument();
  });

  it("closes when the user presses Escape", async () => {
    const onClose = vi.fn();
    const user = userEvent.setup();
    render(<HelpModal isOpen onClose={onClose} />);
    await user.keyboard("{Escape}");
    expect(onClose).toHaveBeenCalled();
  });

  it("closes via the X (close) button", async () => {
    const onClose = vi.fn();
    const user = userEvent.setup();
    render(<HelpModal isOpen onClose={onClose} />);
    await user.click(
      screen.getByRole("button", { name: /Close keyboard shortcuts/i })
    );
    expect(onClose).toHaveBeenCalled();
  });

  it("closes when the backdrop is clicked", async () => {
    const onClose = vi.fn();
    const user = userEvent.setup();
    const { container } = render(<HelpModal isOpen onClose={onClose} />);
    // The backdrop is the first motion.div child (aria-hidden="true").
    const backdrop = container.querySelector('[aria-hidden="true"]');
    await user.click(backdrop!);
    expect(onClose).toHaveBeenCalled();
    cleanup();
  });

  it("highlights the Explorer group when activeView='explorer'", () => {
    render(<HelpModal isOpen onClose={() => {}} activeView="explorer" />);
    // "Explorer" appears both as a group header <p> AND as a shortcut
    // description <span>. The regression signal is on the header <p>, which
    // carries the accent class only when highlighted.
    const all = screen.getAllByText("Explorer");
    const header = all.find((el) => el.tagName === "P");
    expect(header).toBeTruthy();
    expect(header!.className).toContain("accent");
  });
});
