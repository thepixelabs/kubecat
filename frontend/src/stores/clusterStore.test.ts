import { describe, it, expect, beforeEach } from "vitest";
import { useClusterStore } from "./clusterStore";

// ---------------------------------------------------------------------------
// The cluster store is a Zustand store wrapped in `persist`. Tests reset
// state through `resetAll` — the store's own action — to avoid clobbering
// the action closures (which `setState(_, true)` would do).
// ---------------------------------------------------------------------------

const PERSIST_KEY = "kubecat-cluster-store";

beforeEach(() => {
  window.localStorage.clear();
  useClusterStore.getState().resetAll();
});

// ---------------------------------------------------------------------------
// Initial state
// ---------------------------------------------------------------------------

describe("initial state", () => {
  it("matches the documented defaults", () => {
    const s = useClusterStore.getState();
    expect(s.isConnected).toBe(false);
    expect(s.connecting).toBe(false);
    expect(s.activeContext).toBe("");
    expect(s.contexts).toEqual([]);
    expect(s.namespaces).toEqual([]);
    expect(s.appVersion).toBe("v0.1.0");
    expect(s.isTimelineAvailable).toBe(false);
    expect(s.showContextMenu).toBe(false);
    expect(s.contextMenuIndex).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// Simple setters
// ---------------------------------------------------------------------------

describe("simple setters mutate the expected field", () => {
  it("setIsConnected flips the flag", () => {
    useClusterStore.getState().setIsConnected(true);
    expect(useClusterStore.getState().isConnected).toBe(true);
  });

  it("setConnecting flips the flag", () => {
    useClusterStore.getState().setConnecting(true);
    expect(useClusterStore.getState().connecting).toBe(true);
  });

  it("setActiveContext stores the raw context string", () => {
    useClusterStore.getState().setActiveContext("gke_p_z_prod");
    expect(useClusterStore.getState().activeContext).toBe("gke_p_z_prod");
  });

  it("setContexts replaces the context list", () => {
    useClusterStore.getState().setContexts(["a", "b"]);
    useClusterStore.getState().setContexts(["c"]);
    expect(useClusterStore.getState().contexts).toEqual(["c"]);
  });

  it("setNamespaces replaces the namespace list", () => {
    useClusterStore.getState().setNamespaces(["default", "kube-system"]);
    expect(useClusterStore.getState().namespaces).toEqual([
      "default",
      "kube-system",
    ]);
  });

  it("setAppVersion overrides the default version", () => {
    useClusterStore.getState().setAppVersion("v1.2.3");
    expect(useClusterStore.getState().appVersion).toBe("v1.2.3");
  });

  it("setIsTimelineAvailable flips the flag", () => {
    useClusterStore.getState().setIsTimelineAvailable(true);
    expect(useClusterStore.getState().isTimelineAvailable).toBe(true);
  });

  it("setShowContextMenu flips the flag", () => {
    useClusterStore.getState().setShowContextMenu(true);
    expect(useClusterStore.getState().showContextMenu).toBe(true);
  });

  it("setContextMenuIndex stores the numeric index", () => {
    useClusterStore.getState().setContextMenuIndex(3);
    expect(useClusterStore.getState().contextMenuIndex).toBe(3);
  });

  it("setters only mutate the intended field", () => {
    const before = useClusterStore.getState();
    useClusterStore.getState().setActiveContext("prod");
    const after = useClusterStore.getState();
    expect(after.activeContext).toBe("prod");
    // Unrelated fields untouched
    expect(after.isConnected).toBe(before.isConnected);
    expect(after.namespaces).toEqual(before.namespaces);
    expect(after.appVersion).toBe(before.appVersion);
  });
});

// ---------------------------------------------------------------------------
// Compound actions
// ---------------------------------------------------------------------------

describe("connectSuccess", () => {
  it("sets connection + context + namespaces and closes menu in one call", () => {
    // Seed some distracting state first
    useClusterStore.getState().setConnecting(true);
    useClusterStore.getState().setShowContextMenu(true);
    useClusterStore.getState().setContextMenuIndex(5);

    useClusterStore.getState().connectSuccess("prod-ctx", ["default", "kube-system"]);

    const s = useClusterStore.getState();
    expect(s.isConnected).toBe(true);
    expect(s.connecting).toBe(false);
    expect(s.activeContext).toBe("prod-ctx");
    expect(s.namespaces).toEqual(["default", "kube-system"]);
    expect(s.showContextMenu).toBe(false);
    // contextMenuIndex is NOT reset by connectSuccess — verify our
    // assumption matches the code so accidental changes get caught.
    expect(s.contextMenuIndex).toBe(5);
  });
});

describe("disconnectCluster", () => {
  it("clears connection + context + namespaces + timeline flag", () => {
    useClusterStore.getState().connectSuccess("prod-ctx", ["default"]);
    useClusterStore.getState().setIsTimelineAvailable(true);

    useClusterStore.getState().disconnectCluster();

    const s = useClusterStore.getState();
    expect(s.isConnected).toBe(false);
    expect(s.activeContext).toBe("");
    expect(s.namespaces).toEqual([]);
    expect(s.isTimelineAvailable).toBe(false);
  });

  it("leaves contexts (available list) untouched on disconnect", () => {
    useClusterStore.getState().setContexts(["a", "b"]);
    useClusterStore.getState().connectSuccess("a", ["default"]);
    useClusterStore.getState().disconnectCluster();
    expect(useClusterStore.getState().contexts).toEqual(["a", "b"]);
  });
});

describe("resetAll", () => {
  it("restores every field to its initial value", () => {
    // Mutate everything
    const s = useClusterStore.getState();
    s.setIsConnected(true);
    s.setConnecting(true);
    s.setActiveContext("x");
    s.setContexts(["a"]);
    s.setNamespaces(["n"]);
    s.setAppVersion("v9.9.9");
    s.setIsTimelineAvailable(true);
    s.setShowContextMenu(true);
    s.setContextMenuIndex(7);

    useClusterStore.getState().resetAll();

    const after = useClusterStore.getState();
    expect(after.isConnected).toBe(false);
    expect(after.connecting).toBe(false);
    expect(after.activeContext).toBe("");
    expect(after.contexts).toEqual([]);
    expect(after.namespaces).toEqual([]);
    expect(after.appVersion).toBe("v0.1.0");
    expect(after.isTimelineAvailable).toBe(false);
    expect(after.showContextMenu).toBe(false);
    expect(after.contextMenuIndex).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// Sequential setState — pin race-free behaviour
// ---------------------------------------------------------------------------

describe("sequential mutations", () => {
  it("two consecutive setIsConnected calls reflect the last value", () => {
    useClusterStore.getState().setIsConnected(true);
    useClusterStore.getState().setIsConnected(false);
    expect(useClusterStore.getState().isConnected).toBe(false);
  });

  it("interleaved setters compose — each mutates only its field", () => {
    useClusterStore.getState().setActiveContext("a");
    useClusterStore.getState().setNamespaces(["x"]);
    useClusterStore.getState().setActiveContext("b");
    useClusterStore.getState().setNamespaces(["y", "z"]);
    const s = useClusterStore.getState();
    expect(s.activeContext).toBe("b");
    expect(s.namespaces).toEqual(["y", "z"]);
  });
});

// ---------------------------------------------------------------------------
// Persistence contract — partialize
// ---------------------------------------------------------------------------

describe("persistence (partialize)", () => {
  it("persists ONLY activeContext and appVersion", () => {
    const s = useClusterStore.getState();
    // Mutate persisted fields
    s.setActiveContext("prod");
    s.setAppVersion("v2.0.0");
    // Mutate non-persisted fields
    s.setIsConnected(true);
    s.setContexts(["a", "b"]);
    s.setNamespaces(["default"]);
    s.setShowContextMenu(true);
    s.setContextMenuIndex(3);
    s.setIsTimelineAvailable(true);

    const raw = window.localStorage.getItem(PERSIST_KEY);
    expect(raw).not.toBeNull();

    const parsed = JSON.parse(raw as string);
    // Zustand's persist wraps in { state, version }
    expect(parsed.state).toEqual({
      activeContext: "prod",
      appVersion: "v2.0.0",
    });
    // Ensure non-persisted keys are NOT in the serialized blob
    expect(parsed.state).not.toHaveProperty("isConnected");
    expect(parsed.state).not.toHaveProperty("contexts");
    expect(parsed.state).not.toHaveProperty("namespaces");
    expect(parsed.state).not.toHaveProperty("showContextMenu");
    expect(parsed.state).not.toHaveProperty("contextMenuIndex");
    expect(parsed.state).not.toHaveProperty("isTimelineAvailable");
    expect(parsed.state).not.toHaveProperty("connecting");
  });

  it("rehydration from a seeded localStorage key restores activeContext", async () => {
    // Seed storage with a pre-existing persisted snapshot
    window.localStorage.setItem(
      PERSIST_KEY,
      JSON.stringify({
        state: { activeContext: "seeded-ctx", appVersion: "v9.9.9" },
        version: 0,
      })
    );

    // Trigger rehydration manually
    await useClusterStore.persist.rehydrate();

    const s = useClusterStore.getState();
    expect(s.activeContext).toBe("seeded-ctx");
    expect(s.appVersion).toBe("v9.9.9");
    // Non-persisted fields remain at their initial defaults
    expect(s.isConnected).toBe(false);
    expect(s.contexts).toEqual([]);
  });

  it("corrupt localStorage JSON does not crash the app", async () => {
    window.localStorage.setItem(PERSIST_KEY, "{not valid json");
    await expect(useClusterStore.persist.rehydrate()).resolves.not.toThrow();
    // After a failed rehydrate the store keeps its initial state
    expect(useClusterStore.getState().activeContext).toBe("");
  });
});
