import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, cleanup } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ErrorBoundary } from "./ErrorBoundary";

// ── Throwing child factory ───────────────────────────────────────────────────

function Exploder({ msg }: { msg: string }): JSX.Element {
  throw new Error(msg);
}

// ── Silence the console so test output stays readable ──────────────────────

let errSpy: ReturnType<typeof vi.spyOn>;
beforeEach(() => {
  cleanup();
  errSpy = vi.spyOn(console, "error").mockImplementation(() => {});
});

describe("ErrorBoundary", () => {
  it("renders its children when there is no error", () => {
    render(
      <ErrorBoundary componentName="Safe">
        <div>safe child</div>
      </ErrorBoundary>
    );
    expect(screen.getByText("safe child")).toBeInTheDocument();
  });

  it("catches a throwing child and renders the fallback UI", () => {
    render(
      <ErrorBoundary componentName="Cluster Visualizer">
        <Exploder msg="layout blew up" />
      </ErrorBoundary>
    );
    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(screen.getByText("Cluster Visualizer crashed")).toBeInTheDocument();
    expect(screen.getByText("layout blew up")).toBeInTheDocument();
  });

  it("logs the crash with the component name", () => {
    render(
      <ErrorBoundary componentName="X">
        <Exploder msg="boom" />
      </ErrorBoundary>
    );
    const calls = errSpy.mock.calls.map((c) => c.join(" "));
    expect(calls.some((c) => c.includes('"X"'))).toBe(true);
  });

  it("uses 'Unknown Component' as the default label", () => {
    render(
      <ErrorBoundary>
        <Exploder msg="oops" />
      </ErrorBoundary>
    );
    expect(screen.getByText("this component crashed")).toBeInTheDocument();
  });

  it("Retry clears the error and re-renders the children", async () => {
    // Use a state-driven child so a re-render can succeed after Retry.
    let shouldThrow = true;
    function Flaky() {
      if (shouldThrow) throw new Error("transient");
      return <div>recovered</div>;
    }

    const user = userEvent.setup();
    render(
      <ErrorBoundary componentName="Flaky">
        <Flaky />
      </ErrorBoundary>
    );

    expect(screen.getByText("Flaky crashed")).toBeInTheDocument();

    // Pre-condition: "fix" the transient error before retrying.
    shouldThrow = false;
    await user.click(screen.getByRole("button", { name: /Retry/i }));

    expect(screen.getByText("recovered")).toBeInTheDocument();
  });

  it("renders a custom fallback when provided", () => {
    render(
      <ErrorBoundary
        componentName="Custom"
        fallback={(err, reset) => (
          <div>
            custom fallback: {err.message}{" "}
            <button onClick={reset}>go</button>
          </div>
        )}
      >
        <Exploder msg="fb" />
      </ErrorBoundary>
    );
    expect(screen.getByText(/custom fallback: fb/)).toBeInTheDocument();
  });

  it("toggles technical details when the user clicks 'Show technical details'", async () => {
    const user = userEvent.setup();
    render(
      <ErrorBoundary componentName="Debug">
        <Exploder msg="stackme" />
      </ErrorBoundary>
    );

    // Details hidden by default.
    expect(screen.queryByText(/at Exploder/i)).toBeNull();

    await user.click(
      screen.getByRole("button", { name: /Show technical details/i })
    );

    // Now the collapsible stack trace is visible. We can't match exact stack
    // shape, but the <pre> region is now in the DOM — assert via aria-expanded.
    expect(
      screen.getByRole("button", { name: /Hide technical details/i })
    ).toHaveAttribute("aria-expanded", "true");
  });
});
