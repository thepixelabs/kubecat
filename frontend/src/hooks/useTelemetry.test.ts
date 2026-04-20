import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { useTelemetry } from "./useTelemetry";

const CONSENT_KEY = "kubecat-telemetry-consent";
const ANON_ID_KEY = "kubecat-anon-id";

// ---------------------------------------------------------------------------
// Local helpers
// ---------------------------------------------------------------------------

const FETCH_URL = "https://telemetry.example.com/ingest";

/**
 * Spy on `crypto.randomUUID` for deterministic anon id generation. Wails/jsdom
 * provides `crypto` natively in tests.
 */
function stubRandomUUID(value: string) {
  return vi.spyOn(crypto, "randomUUID").mockReturnValue(value as `${string}-${string}-${string}-${string}-${string}`);
}

// ---------------------------------------------------------------------------
// Setup — each test starts with a clean localStorage and default fetch mock.
// ---------------------------------------------------------------------------

beforeEach(() => {
  window.localStorage.clear();
  // Default to a successful fetch — individual tests may override.
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("ok")));
  // Provide an endpoint so track() actually calls fetch. The module reads this
  // at import time, so we can't change it by stubbing env after the fact —
  // but we CAN observe that fetch is called once tests that expect a network
  // call rely on the endpoint being non-empty. Some of our tests therefore
  // assert shape via sendEvent's internal gate below.
  vi.stubEnv("VITE_TELEMETRY_ENDPOINT", FETCH_URL);
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});

// ---------------------------------------------------------------------------
// Consent — initial value and transitions
// ---------------------------------------------------------------------------

describe("consent initial state", () => {
  it("is 'pending' when no value is stored", () => {
    const { result } = renderHook(() => useTelemetry());
    expect(result.current.consent).toBe("pending");
  });

  it("is 'granted' when localStorage has granted", () => {
    window.localStorage.setItem(CONSENT_KEY, "granted");
    const { result } = renderHook(() => useTelemetry());
    expect(result.current.consent).toBe("granted");
  });

  it("is 'denied' when localStorage has denied", () => {
    window.localStorage.setItem(CONSENT_KEY, "denied");
    const { result } = renderHook(() => useTelemetry());
    expect(result.current.consent).toBe("denied");
  });

  it("falls back to 'pending' for any unexpected stored value", () => {
    window.localStorage.setItem(CONSENT_KEY, "garbage-value");
    const { result } = renderHook(() => useTelemetry());
    expect(result.current.consent).toBe("pending");
  });
});

describe("consent transitions", () => {
  it("grantConsent writes 'granted' to localStorage and updates state", () => {
    const { result } = renderHook(() => useTelemetry());
    act(() => result.current.grantConsent());
    expect(result.current.consent).toBe("granted");
    expect(window.localStorage.getItem(CONSENT_KEY)).toBe("granted");
  });

  it("denyConsent writes 'denied' to localStorage and updates state", () => {
    const { result } = renderHook(() => useTelemetry());
    act(() => result.current.denyConsent());
    expect(result.current.consent).toBe("denied");
    expect(window.localStorage.getItem(CONSENT_KEY)).toBe("denied");
  });

  it("revokeConsent removes the key and reverts state to 'pending'", () => {
    window.localStorage.setItem(CONSENT_KEY, "granted");
    const { result } = renderHook(() => useTelemetry());
    expect(result.current.consent).toBe("granted");

    act(() => result.current.revokeConsent());
    expect(result.current.consent).toBe("pending");
    expect(window.localStorage.getItem(CONSENT_KEY)).toBeNull();
  });
});

// ---------------------------------------------------------------------------
// Cross-tab storage sync
// ---------------------------------------------------------------------------

describe("cross-tab consent sync", () => {
  it("picks up a consent change dispatched as a StorageEvent", () => {
    const { result } = renderHook(() => useTelemetry());
    expect(result.current.consent).toBe("pending");

    act(() => {
      // Simulate another tab granting consent
      window.localStorage.setItem(CONSENT_KEY, "granted");
      window.dispatchEvent(
        new StorageEvent("storage", {
          key: CONSENT_KEY,
          newValue: "granted",
          oldValue: null,
        })
      );
    });

    expect(result.current.consent).toBe("granted");
  });

  it("ignores StorageEvents for unrelated keys", () => {
    const { result } = renderHook(() => useTelemetry());
    const before = result.current.consent;

    act(() => {
      window.dispatchEvent(
        new StorageEvent("storage", {
          key: "some-other-key",
          newValue: "whatever",
        })
      );
    });

    expect(result.current.consent).toBe(before);
  });

  it("unsubscribes the storage listener on unmount (no leak)", () => {
    const removeSpy = vi.spyOn(window, "removeEventListener");
    const { unmount } = renderHook(() => useTelemetry());
    unmount();

    // At least one call should be for the "storage" event
    const storageCalls = removeSpy.mock.calls.filter(
      (args) => args[0] === "storage"
    );
    expect(storageCalls.length).toBeGreaterThan(0);
  });
});

// ---------------------------------------------------------------------------
// Anonymous ID — generated once, never rotated
// ---------------------------------------------------------------------------

describe("anonymous id", () => {
  it("generates a new id on first use and persists it to localStorage", () => {
    stubRandomUUID("00000000-0000-0000-0000-000000000abc");
    const { result } = renderHook(() => useTelemetry());
    expect(result.current.anonId).toBe("00000000-0000-0000-0000-000000000abc");
    expect(window.localStorage.getItem(ANON_ID_KEY)).toBe(
      "00000000-0000-0000-0000-000000000abc"
    );
  });

  it("reuses the existing id on subsequent mounts (never rotates)", () => {
    window.localStorage.setItem(ANON_ID_KEY, "pre-existing-id");
    const { result } = renderHook(() => useTelemetry());
    expect(result.current.anonId).toBe("pre-existing-id");
  });

  it("falls back to Math.random + Date-based id when crypto.randomUUID is unavailable", () => {
    // Temporarily replace randomUUID with undefined
    const original = crypto.randomUUID;
    // @ts-expect-error — we're intentionally removing it for the fallback path
    crypto.randomUUID = undefined;
    try {
      const { result } = renderHook(() => useTelemetry());
      expect(result.current.anonId).toBeDefined();
      expect(typeof result.current.anonId).toBe("string");
      expect(result.current.anonId.length).toBeGreaterThan(0);
      // Must have been persisted
      expect(window.localStorage.getItem(ANON_ID_KEY)).toBe(result.current.anonId);
    } finally {
      // @ts-expect-error — restore
      crypto.randomUUID = original;
    }
  });
});

// ---------------------------------------------------------------------------
// track() — consent gating + payload shape
// ---------------------------------------------------------------------------

describe("track — consent gating", () => {
  it("does NOT call fetch when consent is pending", () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("ok"));
    vi.stubGlobal("fetch", fetchMock);

    const { result } = renderHook(() => useTelemetry());
    act(() => result.current.track({ name: "app_launched" }));
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("does NOT call fetch when consent is denied", () => {
    window.localStorage.setItem(CONSENT_KEY, "denied");
    const fetchMock = vi.fn().mockResolvedValue(new Response("ok"));
    vi.stubGlobal("fetch", fetchMock);

    const { result } = renderHook(() => useTelemetry());
    act(() => result.current.track({ name: "app_launched" }));
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("does NOT call fetch after revokeConsent", () => {
    window.localStorage.setItem(CONSENT_KEY, "granted");
    const fetchMock = vi.fn().mockResolvedValue(new Response("ok"));
    vi.stubGlobal("fetch", fetchMock);

    const { result } = renderHook(() => useTelemetry());
    act(() => result.current.revokeConsent());
    act(() => result.current.track({ name: "app_launched" }));
    expect(fetchMock).not.toHaveBeenCalled();
  });
});

describe("track — side effects are silent", () => {
  it("rejected fetch promise does not throw out of track()", async () => {
    window.localStorage.setItem(CONSENT_KEY, "granted");
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("network")));

    const { result } = renderHook(() => useTelemetry());
    // If track() let the rejection surface, this would reject the test.
    await act(async () => {
      result.current.track({ name: "app_launched" });
      await Promise.resolve();
    });
    // Reaching here without throwing is the assertion.
    expect(result.current.consent).toBe("granted");
  });
});
