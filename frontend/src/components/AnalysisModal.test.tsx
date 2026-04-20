import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AnalysisModal } from "./AnalysisModal";

// ── Wails mock ───────────────────────────────────────────────────────────────

const { mockAnalyze } = vi.hoisted(() => ({ mockAnalyze: vi.fn() }));
vi.mock("../../wailsjs/go/main/App", () => ({
  AIAnalyzeResource: (...a: unknown[]) => mockAnalyze(...a),
}));

// ── framer-motion mock ───────────────────────────────────────────────────────

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

// react-markdown renders arbitrary markdown into real DOM. For unit tests we
// just want to see the raw text passthrough — stub to a div.
vi.mock("react-markdown", () => ({
  default: ({ children }: { children: string }) => (
    <div data-testid="md">{children}</div>
  ),
}));
vi.mock("rehype-sanitize", () => ({
  default: () => ({}),
  defaultSchema: { tagNames: [], attributes: {} },
}));

// ── Factories ────────────────────────────────────────────────────────────────

const resource = { kind: "Pod", namespace: "default", name: "web-1" };

const baseProps = () => ({
  isOpen: true,
  onClose: vi.fn(),
  resource,
});

beforeEach(() => {
  mockAnalyze.mockReset();
});

// ── Tests ────────────────────────────────────────────────────────────────────

describe("AnalysisModal", () => {
  describe("open/close", () => {
    it("renders nothing when closed", () => {
      mockAnalyze.mockResolvedValue("hello");
      render(<AnalysisModal {...baseProps()} isOpen={false} />);
      expect(screen.queryByText("AI Resource Analysis")).toBeNull();
    });

    it("renders the modal header with the resource breadcrumb when open", () => {
      mockAnalyze.mockResolvedValue("hello");
      render(<AnalysisModal {...baseProps()} />);
      expect(screen.getByText("AI Resource Analysis")).toBeInTheDocument();
      expect(screen.getByText("Pod / default / web-1")).toBeInTheDocument();
    });

    it("invokes onClose when the header X is clicked", async () => {
      mockAnalyze.mockResolvedValue("hello");
      const onClose = vi.fn();
      const user = userEvent.setup();
      render(<AnalysisModal {...baseProps()} onClose={onClose} />);
      // The header X button carries the `lucide-x` svg and sits next to the h2.
      // Find it by picking the button whose only child is the lucide-x svg.
      const buttons = screen.getAllByRole("button");
      const xBtn = buttons.find(
        (b) => b.querySelector(".lucide-x") && !b.textContent?.trim()
      );
      await user.click(xBtn!);
      expect(onClose).toHaveBeenCalled();
    });

    it("invokes onClose when the backdrop is clicked", async () => {
      mockAnalyze.mockResolvedValue("hello");
      const onClose = vi.fn();
      const user = userEvent.setup();
      const { container } = render(
        <AnalysisModal {...baseProps()} onClose={onClose} />
      );
      // The backdrop is the first motion.div — a sibling of the modal.
      const backdrop = container.querySelector(".fixed.inset-0.z-\\[60\\]");
      await user.click(backdrop!);
      expect(onClose).toHaveBeenCalled();
    });
  });

  describe("analysis lifecycle", () => {
    it("auto-calls AIAnalyzeResource with kind/namespace/name on open", async () => {
      mockAnalyze.mockResolvedValue("result markdown");
      render(<AnalysisModal {...baseProps()} />);
      await waitFor(() => {
        expect(mockAnalyze).toHaveBeenCalledWith("Pod", "default", "web-1");
      });
    });

    it("renders the sanitized markdown result once resolved", async () => {
      mockAnalyze.mockResolvedValue("## Root cause\nSomething broke.");
      render(<AnalysisModal {...baseProps()} />);
      await waitFor(() => {
        expect(screen.getByTestId("md")).toHaveTextContent(/Root cause/);
      });
    });

    it("shows the loading state while the backend is still working", () => {
      mockAnalyze.mockImplementation(
        () => new Promise(() => {}) // never resolves
      );
      render(<AnalysisModal {...baseProps()} />);
      expect(
        screen.getByText(/Analyzing resource telemetry/i)
      ).toBeInTheDocument();
    });

    it("shows the error branch when the backend rejects", async () => {
      mockAnalyze.mockRejectedValue(new Error("LLM rate limited"));
      render(<AnalysisModal {...baseProps()} />);
      await waitFor(() => {
        expect(screen.getByText("Analysis Failed")).toBeInTheDocument();
        expect(screen.getByText("LLM rate limited")).toBeInTheDocument();
      });
    });

    it("normalizes a string rejection into the error message", async () => {
      mockAnalyze.mockRejectedValue("Plain string error");
      render(<AnalysisModal {...baseProps()} />);
      await waitFor(() => {
        expect(screen.getByText("Plain string error")).toBeInTheDocument();
      });
    });

    it('the error branch "Try Again" button re-invokes the backend', async () => {
      mockAnalyze.mockRejectedValueOnce(new Error("first-fail"));
      const user = userEvent.setup();
      render(<AnalysisModal {...baseProps()} />);
      await waitFor(() => {
        expect(screen.getByText("Analysis Failed")).toBeInTheDocument();
      });

      mockAnalyze.mockResolvedValueOnce("works now");
      await user.click(screen.getByRole("button", { name: /Try Again/i }));
      await waitFor(() => {
        expect(screen.getByTestId("md")).toHaveTextContent("works now");
      });
    });

    it("does not call the backend when resource is null", () => {
      render(<AnalysisModal {...baseProps()} resource={null} />);
      expect(mockAnalyze).not.toHaveBeenCalled();
    });
  });

  describe("footer actions", () => {
    it('shows Re-analyze and Copy after a successful result', async () => {
      mockAnalyze.mockResolvedValue("markdown");
      render(<AnalysisModal {...baseProps()} />);
      await waitFor(() => {
        expect(
          screen.getByRole("button", { name: /Re-analyze/i })
        ).toBeInTheDocument();
        expect(
          screen.getByRole("button", { name: /Copy Result/i })
        ).toBeInTheDocument();
      });
    });

    it("renders the Copy Result button with the correct handler wired", async () => {
      // The clipboard API itself is owned by the browser — mocking it across
      // vitest+userEvent is brittle. We assert on the surface the component
      // owns: that Copy Result is wired as a button and clicking it does not
      // crash or clear the analysis state.
      mockAnalyze.mockResolvedValue("markdown body");
      const user = userEvent.setup();
      render(<AnalysisModal {...baseProps()} />);
      await waitFor(() => {
        expect(
          screen.getByRole("button", { name: /Copy Result/i })
        ).toBeInTheDocument();
      });
      // Click should not throw and the analysis is still visible.
      await user.click(screen.getByRole("button", { name: /Copy Result/i }));
      expect(screen.getByTestId("md")).toHaveTextContent("markdown body");
    });
  });
});
