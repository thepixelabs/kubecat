import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CloudAIConsentDialog } from "./CloudAIConsentDialog";

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

const { mockOpenUrl } = vi.hoisted(() => ({ mockOpenUrl: vi.fn() }));
vi.mock("../../wailsjs/runtime/runtime", () => ({
  BrowserOpenURL: (...a: unknown[]) => mockOpenUrl(...a),
}));

const provider = {
  id: "openai",
  name: "OpenAI",
  dpaUrl: "https://openai.com/dpa",
  dataResidency: "US",
};

const baseProps = () => ({
  isOpen: true,
  provider,
  onAccept: vi.fn(),
  onDecline: vi.fn(),
  onUseLocal: vi.fn(),
});

describe("CloudAIConsentDialog", () => {
  it("renders nothing when closed", () => {
    render(<CloudAIConsentDialog {...baseProps()} isOpen={false} />);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("renders the provider name in the title when open", () => {
    render(<CloudAIConsentDialog {...baseProps()} />);
    expect(
      screen.getByRole("dialog", { name: /Send data to OpenAI/i })
    ).toBeInTheDocument();
  });

  // Privacy-important regression: the Accept button must be disabled until
  // the user explicitly checks "I understand". We cannot bypass this silently.
  it("disables the Send button until the user checks 'I understand'", async () => {
    const onAccept = vi.fn();
    const user = userEvent.setup();
    render(<CloudAIConsentDialog {...baseProps()} onAccept={onAccept} />);

    const send = screen.getByRole("button", { name: /Send to OpenAI/i });
    expect(send).toBeDisabled();

    // Click the Send button anyway — MUST NOT fire onAccept.
    await user.click(send);
    expect(onAccept).not.toHaveBeenCalled();

    // Tick the checkbox (it is a <button role="checkbox">).
    await user.click(screen.getByRole("checkbox"));

    expect(send).not.toBeDisabled();

    await user.click(send);
    expect(onAccept).toHaveBeenCalled();
  });

  it("default consent state is 'not consented' (checkbox starts unchecked)", () => {
    render(<CloudAIConsentDialog {...baseProps()} />);
    expect(screen.getByRole("checkbox")).toHaveAttribute("aria-checked", "false");
  });

  it("invokes onDecline when Escape is pressed", async () => {
    const onDecline = vi.fn();
    const user = userEvent.setup();
    render(<CloudAIConsentDialog {...baseProps()} onDecline={onDecline} />);
    await user.keyboard("{Escape}");
    expect(onDecline).toHaveBeenCalled();
  });

  it("invokes onDecline when Cancel button is clicked", async () => {
    const onDecline = vi.fn();
    const user = userEvent.setup();
    render(<CloudAIConsentDialog {...baseProps()} onDecline={onDecline} />);
    await user.click(screen.getByRole("button", { name: /Cancel/i }));
    expect(onDecline).toHaveBeenCalled();
  });

  it("invokes onUseLocal when the local escape hatch is clicked", async () => {
    const onUseLocal = vi.fn();
    const user = userEvent.setup();
    render(<CloudAIConsentDialog {...baseProps()} onUseLocal={onUseLocal} />);
    await user.click(
      screen.getByRole("button", { name: /Use local Ollama instead/i })
    );
    expect(onUseLocal).toHaveBeenCalled();
  });

  it("opens the DPA URL in the system browser on click", async () => {
    const user = userEvent.setup();
    render(<CloudAIConsentDialog {...baseProps()} />);
    await user.click(
      screen.getByRole("button", { name: /data processing agreement/i })
    );
    expect(mockOpenUrl).toHaveBeenCalledWith(provider.dpaUrl);
  });

  it("resets the checkbox when the dialog re-opens (can't stay pre-checked)", async () => {
    const { rerender } = render(<CloudAIConsentDialog {...baseProps()} />);
    const user = userEvent.setup();
    await user.click(screen.getByRole("checkbox"));
    expect(screen.getByRole("checkbox")).toHaveAttribute("aria-checked", "true");

    // Close, then re-open.
    rerender(<CloudAIConsentDialog {...baseProps()} isOpen={false} />);
    rerender(<CloudAIConsentDialog {...baseProps()} isOpen={true} />);

    expect(screen.getByRole("checkbox")).toHaveAttribute(
      "aria-checked",
      "false"
    );
  });
});
