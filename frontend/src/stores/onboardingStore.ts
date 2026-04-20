/**
 * onboardingStore — Persisted first-use flags for the Getting Started card.
 *
 * Each flag is a one-way latch: set to `true` when the user performs the
 * corresponding action for the first time. The card reads these to render
 * real progress instead of decorative checkmarks.
 *
 * Setters are intentionally thin and side-effect free so they can be called
 * from natural event sites (snapshot success, scan success, diff run, etc.)
 * without coupling to the card.
 *
 * Persistence: localStorage, under the `kubecat-onboarding` key.
 */

import { create } from "zustand";
import { persist } from "zustand/middleware";

export interface OnboardingState {
  // First-use flags (one-way latches)
  everConnected: boolean;
  aiQuerySent: boolean;
  snapshotTaken: boolean;
  securityScanRun: boolean;

  // Card dismissal — hides the card regardless of progress
  dismissed: boolean;

  // Setters (idempotent)
  markEverConnected: () => void;
  markAIQuerySent: () => void;
  markSnapshotTaken: () => void;
  markSecurityScanRun: () => void;
  dismiss: () => void;

  // Test / dev helper — reset all flags
  resetOnboarding: () => void;
}

const initialState = {
  everConnected: false,
  aiQuerySent: false,
  snapshotTaken: false,
  securityScanRun: false,
  dismissed: false,
};

export const useOnboardingStore = create<OnboardingState>()(
  persist(
    (set) => ({
      ...initialState,

      markEverConnected: () =>
        set((state) =>
          state.everConnected ? state : { everConnected: true }
        ),
      markAIQuerySent: () =>
        set((state) => (state.aiQuerySent ? state : { aiQuerySent: true })),
      markSnapshotTaken: () =>
        set((state) => (state.snapshotTaken ? state : { snapshotTaken: true })),
      markSecurityScanRun: () =>
        set((state) =>
          state.securityScanRun ? state : { securityScanRun: true }
        ),

      dismiss: () => set({ dismissed: true }),

      resetOnboarding: () => set(initialState),
    }),
    {
      name: "kubecat-onboarding",
      // Persist all flags — none are volatile.
    }
  )
);
