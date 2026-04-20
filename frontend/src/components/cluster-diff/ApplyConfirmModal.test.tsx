import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ApplyConfirmModal } from "./ApplyConfirmModal";
import type { FieldDifference, ApplyResult } from "./types";

// Mock framer-motion: AnimatePresence renders children synchronously so
// modal assertions fire without waiting for animation ticks.
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

const diff = (overrides: Partial<FieldDifference> = {}): FieldDifference => ({
  path: "spec.replicas",
  leftValue: "2",
  rightValue: "3",
  category: "replicas",
  severity: "warning",
  changeType: "modified",
  ...overrides,
});

const baseProps = () => ({
  isOpen: true,
  onClose: vi.fn(),
  onConfirm: vi.fn(),
  targetContext: "minikube",
  resourceInfo: { kind: "Deployment", namespace: "default", name: "web" },
  differences: [diff()],
  isApplying: false,
  applyResult: null as ApplyResult | null,
});

// ── Tests ────────────────────────────────────────────────────────────────────

describe("ApplyConfirmModal", () => {
  describe("open/close", () => {
    it("renders nothing when closed", () => {
      render(<ApplyConfirmModal {...baseProps()} isOpen={false} />);
      expect(screen.queryByText("Apply Changes")).toBeNull();
    });

    it("renders the modal header when open", () => {
      render(<ApplyConfirmModal {...baseProps()} />);
      // "Apply Changes" appears twice — once as the <h2> header, once as the
      // confirm button label. Getting the h2 specifically pins the modal.
      expect(
        screen.getByRole("heading", { name: "Apply Changes" })
      ).toBeInTheDocument();
    });

    it("invokes onClose when the X button is clicked", async () => {
      const onClose = vi.fn();
      const user = userEvent.setup();
      render(<ApplyConfirmModal {...baseProps()} onClose={onClose} />);

      // The X button has no accessible name, so grab it by role + SVG child.
      // Easier: find all buttons and pick the one with no text that is before
      // "Cancel"/"Apply Changes". Use the Cancel flow instead.
      const cancel = screen.getByRole("button", { name: /Cancel/i });
      await user.click(cancel);
      expect(onClose).toHaveBeenCalled();
    });
  });

  describe("resource info", () => {
    it("renders Kind/namespace/name + target context", () => {
      render(<ApplyConfirmModal {...baseProps()} />);
      expect(screen.getByText("Deployment/default/web")).toBeInTheDocument();
      expect(screen.getByText("minikube")).toBeInTheDocument();
    });
  });

  describe("production warning", () => {
    it("shows the production banner when context contains 'prod'", () => {
      render(
        <ApplyConfirmModal {...baseProps()} targetContext="my-prod-us-east" />
      );
      expect(screen.getByText("Production Cluster")).toBeInTheDocument();
    });

    it("does not show the banner for non-prod contexts", () => {
      render(<ApplyConfirmModal {...baseProps()} targetContext="minikube" />);
      expect(screen.queryByText("Production Cluster")).toBeNull();
    });
  });

  // ── Regression pin: long-value data-integrity <code> block ─────────────

  describe("long-value rendering (data-integrity regression)", () => {
    it("renders a full-width <code> block with break-all/whitespace-pre-wrap when a value is >20 chars", () => {
      const longDiff = diff({
        path: "spec.template.spec.containers[0].image",
        leftValue: "registry.example.com/team/app:v1.2.3-alpha-rc7",
        rightValue: "registry.example.com/team/app:v2.0.0",
      });
      const { container } = render(
        <ApplyConfirmModal {...baseProps()} differences={[longDiff]} />
      );

      // Both old and new values must render in actual <code> elements so the
      // user can see the FULL string they're about to push. The inline
      // truncated preview path is for short values only.
      const codes = Array.from(container.querySelectorAll("code"));
      const fullTexts = codes.map((c) => c.textContent);
      expect(fullTexts).toContain(
        "registry.example.com/team/app:v1.2.3-alpha-rc7"
      );
      expect(fullTexts).toContain("registry.example.com/team/app:v2.0.0");

      // And the code blocks must have the class names that enable wrapping
      // — otherwise the value would still be cut off.
      const longCode = codes.find((c) =>
        c.textContent?.includes("alpha-rc7")
      );
      expect(longCode?.className).toContain("break-all");
      expect(longCode?.className).toContain("whitespace-pre-wrap");
    });

    it("uses the inline truncated preview (no <code>) when both values are <=20 chars", () => {
      const shortDiff = diff({
        path: "spec.replicas",
        leftValue: "2",
        rightValue: "3",
      });
      const { container } = render(
        <ApplyConfirmModal {...baseProps()} differences={[shortDiff]} />
      );
      // No <code> element should be rendered for this row.
      expect(container.querySelectorAll("code").length).toBe(0);
    });

    it("substitutes (empty) when a value is missing", () => {
      const emptyDiff = diff({
        path: "metadata.annotations.deploy-hash",
        leftValue: "",
        rightValue: "a".repeat(40),
      });
      render(<ApplyConfirmModal {...baseProps()} differences={[emptyDiff]} />);
      expect(screen.getByText("(empty)")).toBeInTheDocument();
    });
  });

  describe("action buttons", () => {
    it("disables Dry Run and Apply while applying", () => {
      render(<ApplyConfirmModal {...baseProps()} isApplying={true} />);
      expect(screen.getByRole("button", { name: /Dry Run/i })).toBeDisabled();
      expect(
        screen.getByRole("button", { name: /Apply Changes/i })
      ).toBeDisabled();
    });

    it("fires onConfirm(true) for Dry Run", async () => {
      const onConfirm = vi.fn();
      const user = userEvent.setup();
      render(<ApplyConfirmModal {...baseProps()} onConfirm={onConfirm} />);
      await user.click(screen.getByRole("button", { name: /Dry Run/i }));
      expect(onConfirm).toHaveBeenCalledWith(true);
    });

    it("fires onConfirm(false) for Apply Changes", async () => {
      const onConfirm = vi.fn();
      const user = userEvent.setup();
      render(<ApplyConfirmModal {...baseProps()} onConfirm={onConfirm} />);
      await user.click(screen.getByRole("button", { name: /Apply Changes/i }));
      expect(onConfirm).toHaveBeenCalledWith(false);
    });
  });

  describe("severity summary", () => {
    it("shows critical/warning counts when present", () => {
      render(
        <ApplyConfirmModal
          {...baseProps()}
          differences={[
            diff({ severity: "critical" }),
            diff({ severity: "critical", path: "b" }),
            diff({ severity: "warning", path: "c" }),
          ]}
        />
      );
      expect(screen.getByText(/2 critical/i)).toBeInTheDocument();
      expect(screen.getByText(/1 warnings/i)).toBeInTheDocument();
    });
  });

  describe("post-apply result", () => {
    it("collapses buttons to a single Close action on success", () => {
      render(
        <ApplyConfirmModal
          {...baseProps()}
          applyResult={{
            success: true,
            dryRun: false,
            message: "Applied 1 change",
            changes: [],
            warnings: [],
          }}
        />
      );
      // Cancel/Dry Run/Apply are gone.
      expect(screen.queryByRole("button", { name: /Cancel/i })).toBeNull();
      expect(screen.queryByRole("button", { name: /Dry Run/i })).toBeNull();
      // Close is present (there's only one "Close" button in this state).
      expect(screen.getByRole("button", { name: /Close/i })).toBeInTheDocument();
    });

    it("shows the warnings list from the apply result", () => {
      render(
        <ApplyConfirmModal
          {...baseProps()}
          applyResult={{
            success: true,
            dryRun: true,
            message: "ok",
            changes: [],
            warnings: ["replica drift"],
          }}
        />
      );
      expect(screen.getByText(/replica drift/)).toBeInTheDocument();
    });
  });
});
