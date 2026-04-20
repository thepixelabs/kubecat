import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AskAIButton } from "./AskAIButton";
import { useAIStore } from "../stores/aiStore";
import { useToastStore } from "../stores/toastStore";

// ── Wails mock ───────────────────────────────────────────────────────────────

const { mockGetAISettings } = vi.hoisted(() => ({
  mockGetAISettings: vi.fn(),
}));
vi.mock("../../wailsjs/go/main/App", () => ({
  GetAISettings: (...args: unknown[]) => mockGetAISettings(...args),
}));

// ── Factories ────────────────────────────────────────────────────────────────

const resource = {
  type: "pod",
  namespace: "default",
  name: "web-1",
  cluster: "minikube",
};

const validAISettings = {
  enabled: true,
  selectedProvider: "openai",
  providers: {
    openai: { apiKey: "sk-123" },
  },
};

// ── Setup ────────────────────────────────────────────────────────────────────

beforeEach(() => {
  useAIStore.setState({
    contextQueue: [],
    conversations: [],
    activeConversationId: null,
    autopilotEnabled: false,
    selectedModel: null,
  });
  useToastStore.setState({ toasts: [] });
  mockGetAISettings.mockReset();
});

// ── Tests ────────────────────────────────────────────────────────────────────

describe("AskAIButton", () => {
  describe("icon variant (default)", () => {
    it("renders as an icon button with the sparkles glyph", () => {
      render(<AskAIButton resource={resource} />);
      expect(
        screen.getByRole("button", { name: /Ask AI about this resource/i })
      ).toBeInTheDocument();
    });

    it("disables the button when the resource has no cluster", () => {
      render(<AskAIButton resource={{ ...resource, cluster: "" }} />);
      expect(
        screen.getByRole("button", { name: /Connect to a cluster first/i })
      ).toBeDisabled();
    });
  });

  describe("button variant", () => {
    it('renders with "Ask AI" label when not in context', () => {
      render(<AskAIButton resource={resource} variant="button" />);
      expect(screen.getByRole("button", { name: /Ask AI/i })).toBeInTheDocument();
    });

    it('shows "In Context" and is disabled when the resource is already queued', () => {
      useAIStore.setState({
        contextQueue: [
          {
            id: "pre-existing",
            type: "pod",
            namespace: "default",
            name: "web-1",
            cluster: "minikube",
            addedAt: new Date(),
          },
        ],
      });
      render(<AskAIButton resource={resource} variant="button" />);
      const btn = screen.getByTitle("Already in AI context");
      expect(btn).toBeDisabled();
      expect(btn).toHaveTextContent(/In Context/i);
    });
  });

  describe("click handler — AI settings gating", () => {
    it("adds to the AI context and fires a success toast when settings are valid", async () => {
      mockGetAISettings.mockResolvedValue(validAISettings);
      const onAdded = vi.fn();
      const user = userEvent.setup();
      render(<AskAIButton resource={resource} onAdded={onAdded} />);

      await user.click(screen.getByRole("button"));

      // Item queued
      expect(useAIStore.getState().contextQueue).toHaveLength(1);
      expect(useAIStore.getState().contextQueue[0]).toMatchObject({
        type: "pod",
        name: "web-1",
        namespace: "default",
        cluster: "minikube",
      });

      // Success toast emitted
      const toasts = useToastStore.getState().toasts;
      expect(toasts).toHaveLength(1);
      expect(toasts[0]).toMatchObject({
        type: "success",
        message: expect.stringMatching(/Added pod\/web-1/i),
      });

      // onAdded callback fired
      expect(onAdded).toHaveBeenCalledTimes(1);
    });

    it("shows a warning toast and does NOT queue when AI is disabled", async () => {
      mockGetAISettings.mockResolvedValue({ ...validAISettings, enabled: false });
      const user = userEvent.setup();
      render(<AskAIButton resource={resource} />);

      await user.click(screen.getByRole("button"));

      expect(useAIStore.getState().contextQueue).toHaveLength(0);
      const toast = useToastStore.getState().toasts[0];
      expect(toast.type).toBe("warning");
      expect(toast.message).toMatch(/AI features are not enabled/i);
    });

    it("warns and does not queue when no provider is selected", async () => {
      mockGetAISettings.mockResolvedValue({
        ...validAISettings,
        selectedProvider: "",
      });
      const user = userEvent.setup();
      render(<AskAIButton resource={resource} />);
      await user.click(screen.getByRole("button"));

      expect(useAIStore.getState().contextQueue).toHaveLength(0);
      expect(useToastStore.getState().toasts[0].message).toMatch(
        /provider not selected/i
      );
    });

    it("warns and does not queue when the selected provider has no API key", async () => {
      mockGetAISettings.mockResolvedValue({
        ...validAISettings,
        providers: { openai: { apiKey: "" } },
      });
      const user = userEvent.setup();
      render(<AskAIButton resource={resource} />);
      await user.click(screen.getByRole("button"));

      expect(useAIStore.getState().contextQueue).toHaveLength(0);
      expect(useToastStore.getState().toasts[0].message).toMatch(
        /provider not configured/i
      );
    });

    it("fires an error toast when GetAISettings throws", async () => {
      mockGetAISettings.mockRejectedValue(new Error("backend down"));
      const user = userEvent.setup();
      render(<AskAIButton resource={resource} />);
      await user.click(screen.getByRole("button"));

      expect(useAIStore.getState().contextQueue).toHaveLength(0);
      expect(useToastStore.getState().toasts[0]).toMatchObject({
        type: "error",
        message: expect.stringMatching(/Failed to check AI settings/i),
      });
    });

    it("early-returns when the resource is already in the queue (no AI settings call)", () => {
      useAIStore.setState({
        contextQueue: [
          {
            id: "already-there",
            type: "pod",
            namespace: "default",
            name: "web-1",
            cluster: "minikube",
            addedAt: new Date(),
          },
        ],
      });
      render(<AskAIButton resource={resource} />);
      // The button is disabled in this state. Assert presentation pins the
      // guard: disabled + "already in AI context" title, and that no
      // AI-settings fetch has been issued on render alone.
      const btn = screen.getByRole("button");
      expect(btn).toBeDisabled();
      expect(btn.getAttribute("title")).toBe("Already in AI context");
      expect(mockGetAISettings).not.toHaveBeenCalled();
    });
  });

  describe("event propagation", () => {
    it("stops click propagation so it doesn't trigger parent onClick", async () => {
      mockGetAISettings.mockResolvedValue(validAISettings);
      const parentClick = vi.fn();
      const user = userEvent.setup();
      render(
        <div onClick={parentClick}>
          <AskAIButton resource={resource} />
        </div>
      );
      await user.click(screen.getByRole("button"));
      expect(parentClick).not.toHaveBeenCalled();
    });
  });
});
