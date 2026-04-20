import { describe, it, expect, beforeEach } from "vitest";
import { useOnboardingStore } from "./onboardingStore";

beforeEach(() => {
  // Hard reset — use the store's own action so we don't wipe out setters.
  useOnboardingStore.getState().resetOnboarding();
});

describe("onboardingStore", () => {
  it("starts with every flag false", () => {
    const s = useOnboardingStore.getState();
    expect(s.everConnected).toBe(false);
    expect(s.aiQuerySent).toBe(false);
    expect(s.snapshotTaken).toBe(false);
    expect(s.securityScanRun).toBe(false);
    expect(s.dismissed).toBe(false);
  });

  it("latches everConnected via markEverConnected", () => {
    useOnboardingStore.getState().markEverConnected();
    expect(useOnboardingStore.getState().everConnected).toBe(true);
  });

  it("latches aiQuerySent via markAIQuerySent", () => {
    useOnboardingStore.getState().markAIQuerySent();
    expect(useOnboardingStore.getState().aiQuerySent).toBe(true);
  });

  it("latches snapshotTaken via markSnapshotTaken", () => {
    useOnboardingStore.getState().markSnapshotTaken();
    expect(useOnboardingStore.getState().snapshotTaken).toBe(true);
  });

  it("latches securityScanRun via markSecurityScanRun", () => {
    useOnboardingStore.getState().markSecurityScanRun();
    expect(useOnboardingStore.getState().securityScanRun).toBe(true);
  });

  it("setters are idempotent — calling twice does not re-trigger state changes needlessly", () => {
    const { markEverConnected } = useOnboardingStore.getState();
    markEverConnected();
    const snap1 = useOnboardingStore.getState();
    markEverConnected();
    const snap2 = useOnboardingStore.getState();
    // Since the setter returns `state` unchanged on second call, reference
    // equality should hold for the entire store object.
    expect(snap2).toBe(snap1);
  });

  it("dismiss flips the dismissed flag", () => {
    useOnboardingStore.getState().dismiss();
    expect(useOnboardingStore.getState().dismissed).toBe(true);
  });

  it("resetOnboarding clears all flags", () => {
    const s = useOnboardingStore.getState();
    s.markEverConnected();
    s.markAIQuerySent();
    s.markSnapshotTaken();
    s.markSecurityScanRun();
    s.dismiss();

    s.resetOnboarding();

    const after = useOnboardingStore.getState();
    expect(after.everConnected).toBe(false);
    expect(after.aiQuerySent).toBe(false);
    expect(after.snapshotTaken).toBe(false);
    expect(after.securityScanRun).toBe(false);
    expect(after.dismissed).toBe(false);
  });
});
