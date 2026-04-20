import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  AgentToolApprovalCard,
  type AgentTool,
} from "./AgentToolApprovalCard";

// framer-motion adds layout effects irrelevant to logic. Stub down.
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

// ── Factory ──────────────────────────────────────────────────────────────────

const makeTool = (overrides: Partial<AgentTool> = {}): AgentTool => ({
  id: "t1",
  name: "get_pod_logs",
  description: "Read pod logs",
  risk: "read",
  parameters: { pod: "web", ns: "default" },
  ...overrides,
});

const render_ = (props: Partial<React.ComponentProps<typeof AgentToolApprovalCard>> = {}) =>
  render(
    <AgentToolApprovalCard
      tool={makeTool()}
      state="pending"
      onApprove={vi.fn()}
      onReject={vi.fn()}
      {...props}
    />
  );

// ── Tests ────────────────────────────────────────────────────────────────────

describe("AgentToolApprovalCard", () => {
  describe("header", () => {
    it("renders tool name, description, and risk badge", () => {
      render_({
        tool: makeTool({
          name: "patch_deployment",
          description: "Patch deployment spec",
          risk: "write",
        }),
      });
      expect(screen.getByText("patch_deployment")).toBeInTheDocument();
      expect(screen.getByText("Patch deployment spec")).toBeInTheDocument();
      expect(screen.getByText("Write")).toBeInTheDocument();
    });

    it.each([
      ["read", "Read-only"],
      ["write", "Write"],
      ["destructive", "Destructive"],
    ] as const)("maps risk %s → badge label %s", (risk, label) => {
      render_({ tool: makeTool({ risk }) });
      expect(screen.getByText(label)).toBeInTheDocument();
    });
  });

  describe("read-only tools", () => {
    it("shows the 'auto' badge and no approval buttons", () => {
      render_();
      expect(screen.getByText("auto")).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: /Approve tool/i })).toBeNull();
      expect(screen.queryByRole("button", { name: /Reject tool/i })).toBeNull();
    });
  });

  describe("write tools — single confirm flow", () => {
    it("single approve click transitions to 'awaiting-confirm' without firing onApprove", async () => {
      const onApprove = vi.fn();
      const user = userEvent.setup();
      render_({ tool: makeTool({ risk: "write" }), onApprove });

      await user.click(screen.getByRole("button", { name: /Approve tool/i }));

      expect(onApprove).not.toHaveBeenCalled();
      // UI now shows Confirm button.
      expect(screen.getByRole("button", { name: /^Confirm$/ })).toBeInTheDocument();
    });

    it("second Confirm click fires onApprove with the tool id", async () => {
      const onApprove = vi.fn();
      const user = userEvent.setup();
      render_({
        tool: makeTool({ id: "confirm-me", risk: "write" }),
        onApprove,
      });

      await user.click(screen.getByRole("button", { name: /Approve tool/i }));
      await user.click(screen.getByRole("button", { name: /^Confirm$/ }));

      expect(onApprove).toHaveBeenCalledWith("confirm-me");
    });
  });

  describe("destructive tools — double confirm flow", () => {
    it("first two clicks require the explicit 'I understand the risk' confirmation", async () => {
      const onApprove = vi.fn();
      const user = userEvent.setup();
      render_({
        tool: makeTool({ id: "nuke-it", risk: "destructive" }),
        onApprove,
      });

      // Click 1 → awaiting-confirm
      await user.click(screen.getByRole("button", { name: /Approve tool/i }));
      expect(onApprove).not.toHaveBeenCalled();

      // Click 2 → awaiting-double-confirm
      await user.click(screen.getByRole("button", { name: /^Confirm$/ }));
      expect(onApprove).not.toHaveBeenCalled();
      expect(
        screen.getByText(/This action is destructive and may be irreversible/i)
      ).toBeInTheDocument();

      // Click 3 → approved
      await user.click(
        screen.getByRole("button", { name: /I understand the risk/i })
      );
      expect(onApprove).toHaveBeenCalledWith("nuke-it");
    });
  });

  describe("reject", () => {
    it("fires onReject with the tool id and stops further approval", async () => {
      const onReject = vi.fn();
      const user = userEvent.setup();
      render_({
        tool: makeTool({ id: "rej-1", risk: "write" }),
        onReject,
      });
      await user.click(screen.getByRole("button", { name: /Reject tool/i }));
      expect(onReject).toHaveBeenCalledWith("rej-1");
      // After rejection, approval actions are hidden.
      expect(screen.queryByRole("button", { name: /Approve tool/i })).toBeNull();
    });
  });

  describe("parameters panel", () => {
    it("is collapsed by default and expands on click", async () => {
      const user = userEvent.setup();
      render_({
        tool: makeTool({ parameters: { a: 1, b: "two" } }),
      });
      // Param count is visible; JSON body only appears after expand.
      expect(screen.getByText(/Parameters \(2\)/)).toBeInTheDocument();
      expect(screen.queryByText(/"b": "two"/)).toBeNull();

      await user.click(screen.getByRole("button", { name: /Parameters/i }));
      expect(screen.getByText(/"b": "two"/)).toBeInTheDocument();
    });
  });

  describe("result panel", () => {
    it("renders the result body when tool.result is present", () => {
      render_({
        tool: makeTool({ result: "pod is running\nlogs ok" }),
      });
      expect(screen.getByText(/pod is running/)).toBeInTheDocument();
    });

    it('renders the error body (not result) when tool.error is present', () => {
      render_({
        tool: makeTool({ error: "permission denied" }),
      });
      expect(screen.getByText("permission denied")).toBeInTheDocument();
    });
  });
});
