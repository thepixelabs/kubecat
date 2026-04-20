import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, act } from "@testing-library/react";
import { CostSettings } from "./CostSettings";

// ---------------------------------------------------------------------------
// Mocks — Wails bindings
// ---------------------------------------------------------------------------

const mockGetCostSettings = vi.fn();
const mockSaveCostSettings = vi.fn();
const mockDetectCostBackend = vi.fn();

vi.mock("../../wailsjs/go/main/App", () => ({
  GetCostSettings: () => mockGetCostSettings(),
  SaveCostSettings: (...args: unknown[]) => mockSaveCostSettings(...args),
  DetectCostBackend: () => mockDetectCostBackend(),
}));

// The generated models file exports a namespace object; the class's
// createFrom() is a thin wrapper that just returns a new instance. Stub it
// with a pass-through so the test doesn't pull in the real generated file
// (which is excluded from test coverage, and importing it pollutes jsdom
// with Wails-runtime globals).
vi.mock("../../wailsjs/go/models", () => ({
  config: {
    CostConfig: {
      createFrom: (src: unknown) => src,
    },
  },
}));

// ---------------------------------------------------------------------------
// Fetch mock for the "Test connection" button
// ---------------------------------------------------------------------------

const originalFetch = globalThis.fetch;
let mockFetch: ReturnType<typeof vi.fn>;

beforeEach(() => {
  vi.clearAllMocks();

  mockGetCostSettings.mockResolvedValue({
    CPUCostPerCoreHour: 0.048,
    MemCostPerGBHour: 0.006,
    Currency: "USD",
    OpenCostEndpoint: "",
  });
  mockSaveCostSettings.mockResolvedValue(undefined);
  mockDetectCostBackend.mockResolvedValue("none");

  mockFetch = vi.fn();
  globalThis.fetch = mockFetch as unknown as typeof fetch;
});

afterEach(() => {
  globalThis.fetch = originalFetch;
});

// ---------------------------------------------------------------------------
// Rendering
// ---------------------------------------------------------------------------

describe("CostSettings rendering", () => {
  it("shows a loading indicator while settings load", () => {
    // Block the promise so we can observe the loading state.
    mockGetCostSettings.mockReturnValue(new Promise(() => {}));
    render(<CostSettings />);
    expect(screen.getByText(/loading cost settings/i)).toBeDefined();
  });

  it("renders the endpoint input and test button once loaded", async () => {
    render(<CostSettings />);
    await waitFor(() => {
      expect(
        screen.getByLabelText(/opencost endpoint url/i)
      ).toBeDefined();
    });
    expect(screen.getByRole("button", { name: /test connection/i })).toBeDefined();
  });

  it("calls DetectCostBackend on mount and reflects the result", async () => {
    mockDetectCostBackend.mockResolvedValue("opencost");
    render(<CostSettings />);
    await waitFor(() => {
      expect(mockDetectCostBackend).toHaveBeenCalledTimes(1);
    });
    await waitFor(() => {
      expect(
        screen.getByText(/opencost detected in the active cluster/i)
      ).toBeDefined();
    });
  });

  it("falls back gracefully when DetectCostBackend throws", async () => {
    mockDetectCostBackend.mockRejectedValue(new Error("no active cluster"));
    render(<CostSettings />);
    await waitFor(() => {
      expect(
        screen.getByText(/backend detection unavailable/i)
      ).toBeDefined();
    });
  });

  it("shows 'heuristic' as the current source when no endpoint is set", async () => {
    render(<CostSettings />);
    await waitFor(() => {
      // Use a predicate matcher because the text is split across spans.
      expect(
        screen.getAllByText((_, node) =>
          /current source/i.test(node?.textContent ?? "")
        ).length
      ).toBeGreaterThan(0);
    });
    expect(screen.getByText("heuristic")).toBeDefined();
  });
});

// ---------------------------------------------------------------------------
// Persistence
// ---------------------------------------------------------------------------

describe("CostSettings persistence", () => {
  it("persists the endpoint on blur when the value changes", async () => {
    render(<CostSettings />);
    const input = (await screen.findByLabelText(
      /opencost endpoint url/i
    )) as HTMLInputElement;

    fireEvent.change(input, {
      target: { value: "http://localhost:9003" },
    });
    fireEvent.blur(input);

    await waitFor(() => {
      expect(mockSaveCostSettings).toHaveBeenCalledTimes(1);
    });
    // First (and only) arg should carry the new endpoint.
    expect(mockSaveCostSettings.mock.calls[0][0].OpenCostEndpoint).toBe(
      "http://localhost:9003"
    );
    // Preserves the other fields we loaded.
    expect(mockSaveCostSettings.mock.calls[0][0].CPUCostPerCoreHour).toBe(0.048);
    expect(mockSaveCostSettings.mock.calls[0][0].Currency).toBe("USD");
  });

  it("does NOT persist on blur if the endpoint value is unchanged", async () => {
    mockGetCostSettings.mockResolvedValue({
      CPUCostPerCoreHour: 0.048,
      MemCostPerGBHour: 0.006,
      Currency: "USD",
      OpenCostEndpoint: "http://localhost:9003",
    });

    render(<CostSettings />);
    const input = (await screen.findByLabelText(
      /opencost endpoint url/i
    )) as HTMLInputElement;

    // Focus and blur without editing.
    fireEvent.focus(input);
    fireEvent.blur(input);

    // Flush any queued microtasks.
    await act(async () => {});

    expect(mockSaveCostSettings).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// Test connection
// ---------------------------------------------------------------------------

describe("CostSettings test-connection", () => {
  it("shows an error when the endpoint field is empty", async () => {
    render(<CostSettings />);
    const btn = await screen.findByRole("button", { name: /test connection/i });
    fireEvent.click(btn);
    await waitFor(() => {
      expect(
        screen.getByText(/enter an opencost endpoint first/i)
      ).toBeDefined();
    });
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("shows a success state on 2xx response", async () => {
    mockFetch.mockResolvedValue(
      new Response(null, { status: 200, statusText: "OK" })
    );

    render(<CostSettings />);
    const input = (await screen.findByLabelText(
      /opencost endpoint url/i
    )) as HTMLInputElement;
    fireEvent.change(input, { target: { value: "http://localhost:9003" } });
    fireEvent.blur(input);

    // Wait for the persist-on-blur to settle.
    await waitFor(() => {
      expect(mockSaveCostSettings).toHaveBeenCalled();
    });

    fireEvent.click(screen.getByRole("button", { name: /test connection/i }));

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalled();
    });
    expect(mockFetch.mock.calls[0][0]).toBe("http://localhost:9003/healthz");

    await waitFor(() => {
      expect(screen.getByRole("status").textContent).toMatch(/connected/i);
    });
  });

  it("shows an unreachable hint when the endpoint is an in-cluster svc URL and fetch fails", async () => {
    // Both the /healthz and the fallback probe fail — simulates unreachable
    // in-cluster DNS.
    mockFetch.mockRejectedValue(new TypeError("Failed to fetch"));

    render(<CostSettings />);
    const input = (await screen.findByLabelText(
      /opencost endpoint url/i
    )) as HTMLInputElement;
    fireEvent.change(input, {
      target: {
        value: "http://opencost.opencost.svc.cluster.local:9003",
      },
    });
    fireEvent.blur(input);

    await waitFor(() => {
      expect(mockSaveCostSettings).toHaveBeenCalled();
    });

    fireEvent.click(screen.getByRole("button", { name: /test connection/i }));

    await waitFor(() => {
      expect(screen.getByRole("alert").textContent).toMatch(
        /endpoint not reachable/i
      );
    });
    // Hint should mention port-forward.
    expect(
      screen.getByText(/kubectl port-forward/i)
    ).toBeDefined();
  });

  it("rejects non-http(s) URLs without touching the network", async () => {
    render(<CostSettings />);
    const input = (await screen.findByLabelText(
      /opencost endpoint url/i
    )) as HTMLInputElement;
    fireEvent.change(input, { target: { value: "ftp://example.com" } });
    fireEvent.blur(input);

    await waitFor(() => {
      expect(mockSaveCostSettings).toHaveBeenCalled();
    });

    fireEvent.click(screen.getByRole("button", { name: /test connection/i }));

    await waitFor(() => {
      expect(screen.getByRole("alert").textContent).toMatch(
        /only http\/https/i
      );
    });
    expect(mockFetch).not.toHaveBeenCalled();
  });
});
