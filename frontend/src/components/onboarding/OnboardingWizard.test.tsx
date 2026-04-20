import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  OnboardingWizard,
  getOnboardingState,
  markOnboardingComplete,
  resetOnboarding,
} from "./OnboardingWizard";

// framer-motion adds animations we don't want in unit tests. Stub it down.
vi.mock("framer-motion", () => ({
  motion: new Proxy(
    {},
    {
      get: () => {
        return ({ children, ...props }: any) => <div {...props}>{children}</div>;
      },
    }
  ),
  AnimatePresence: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
}));

beforeEach(() => {
  resetOnboarding();
});

// ── Storage helpers ──────────────────────────────────────────────────────────

describe("OnboardingWizard storage helpers", () => {
  it("returns {completed: false} when nothing is stored", () => {
    expect(getOnboardingState()).toEqual({ completed: false });
  });

  it("markOnboardingComplete persists completed=true", () => {
    markOnboardingComplete();
    const state = getOnboardingState();
    expect(state.completed).toBe(true);
    expect(state.skipped).toBe(false);
    expect(typeof state.completedAt).toBe("string");
  });

  it("markOnboardingComplete(true) records skipped=true", () => {
    markOnboardingComplete(true);
    expect(getOnboardingState().skipped).toBe(true);
  });

  it("resetOnboarding clears state", () => {
    markOnboardingComplete();
    resetOnboarding();
    expect(getOnboardingState()).toEqual({ completed: false });
  });

  it("returns the default on parse errors", () => {
    localStorage.setItem("kubecat-onboarding-v1", "<not json>");
    expect(getOnboardingState()).toEqual({ completed: false });
  });
});

// ── Component rendering ─────────────────────────────────────────────────────

const defaultProps = {
  contexts: ["minikube", "prod-eks"],
  onConnect: vi.fn().mockResolvedValue(undefined),
  onFinish: vi.fn(),
  connecting: false,
  activeContext: "",
  isConnected: false,
  onRefreshContexts: vi.fn().mockResolvedValue(undefined),
};

describe("OnboardingWizard step 0 (Welcome)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the welcome heading and start button", () => {
    render(<OnboardingWizard {...defaultProps} />);
    expect(screen.getByText("Welcome to Kubecat")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Let's get started/i })
    ).toBeInTheDocument();
  });

  it("hides the Skip button on step 0 (first impression)", () => {
    render(<OnboardingWizard {...defaultProps} />);
    expect(screen.queryByRole("button", { name: /Skip/i })).toBeNull();
  });

  it("advances to step 1 when the user clicks 'Let's get started'", async () => {
    const user = userEvent.setup();
    render(<OnboardingWizard {...defaultProps} />);
    await user.click(screen.getByRole("button", { name: /Let's get started/i }));
    expect(screen.getByText("Connect to a cluster")).toBeInTheDocument();
  });
});

describe("OnboardingWizard step 1 (Cluster)", () => {
  it("shows every context from props", async () => {
    const user = userEvent.setup();
    render(<OnboardingWizard {...defaultProps} />);
    await user.click(screen.getByRole("button", { name: /Let's get started/i }));
    expect(screen.getByText("minikube")).toBeInTheDocument();
    expect(screen.getByText("prod-eks")).toBeInTheDocument();
  });

  // Regression pin: long context names must carry a title= on the card so the
  // user can hover to see the full name.
  it("sets title={ctx} on the context button and its label span", async () => {
    const longCtx =
      "arn:aws:eks:us-east-1:123456789012:cluster/super-long-name-here";
    const user = userEvent.setup();
    render(<OnboardingWizard {...defaultProps} contexts={[longCtx]} />);
    await user.click(screen.getByRole("button", { name: /Let's get started/i }));

    // The <motion.button> becomes a <div> via our mock, but the inner span
    // still carries a title= attribute — this is the regression pin.
    const labelSpan = screen
      .getByText(longCtx)
      .closest("span");
    expect(labelSpan?.getAttribute("title")).toBe(longCtx);
  });

  it("calls onConnect when a context button is clicked", async () => {
    const onConnect = vi.fn().mockResolvedValue(undefined);
    const user = userEvent.setup();
    render(<OnboardingWizard {...defaultProps} onConnect={onConnect} />);
    await user.click(screen.getByRole("button", { name: /Let's get started/i }));
    // Use getAllByText to get the button — the motion mock renders it as a div.
    // We use the truncate-target span inside.
    const ctxBtn = screen.getByText("minikube");
    await user.click(ctxBtn);
    expect(onConnect).toHaveBeenCalledWith("minikube");
  });

  it('shows an empty-state when contexts=[] with a "Refresh" affordance', async () => {
    const user = userEvent.setup();
    render(<OnboardingWizard {...defaultProps} contexts={[]} />);
    await user.click(screen.getByRole("button", { name: /Let's get started/i }));
    expect(screen.getByText(/No kubeconfig contexts found/i)).toBeInTheDocument();
  });
});

describe("OnboardingWizard skip flow", () => {
  it("clicking Skip on step 1 marks complete+skipped and fires onFinish", async () => {
    const onFinish = vi.fn();
    const user = userEvent.setup();
    render(<OnboardingWizard {...defaultProps} onFinish={onFinish} />);
    await user.click(screen.getByRole("button", { name: /Let's get started/i }));
    await user.click(screen.getByRole("button", { name: /Skip/i }));

    expect(onFinish).toHaveBeenCalled();
    const state = getOnboardingState();
    expect(state.completed).toBe(true);
    expect(state.skipped).toBe(true);
  });
});

describe("OnboardingWizard full finish", () => {
  it("marks onboarding complete (not skipped) when the tour's final button is clicked", async () => {
    const onFinish = vi.fn();
    const user = userEvent.setup();
    render(
      <OnboardingWizard
        {...defaultProps}
        isConnected
        activeContext="minikube"
        onFinish={onFinish}
      />
    );

    // Advance to step 1, 2, 3 — the "Continue" button each time.
    await user.click(screen.getByRole("button", { name: /Let's get started/i }));

    // Step 1 Continue requires isConnected.
    const continueStep1 = screen.getAllByRole("button", {
      name: /Continue/i,
    })[0];
    await user.click(continueStep1);

    // Step 2 AI setup — one Continue button.
    await user.click(screen.getByRole("button", { name: /Continue/i }));

    // Step 3 Tour — walk to the last slide and click the "Let's go!" button.
    // The default opens on slide 0, and "Next" advances.
    for (let i = 0; i < 3; i++) {
      await user.click(screen.getByRole("button", { name: /^Next$/i }));
    }
    await user.click(screen.getByRole("button", { name: /Let's go!/i }));

    expect(onFinish).toHaveBeenCalled();
    expect(getOnboardingState().skipped).toBe(false);
  });
});
