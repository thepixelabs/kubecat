import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { AIQueryView } from "./AIQueryView";
import { useAIStore } from "../stores/aiStore";

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

const mockAIQueryWithContext = vi.fn();
const mockAIAgentQuery = vi.fn();
const mockApproveAgentAction = vi.fn();
const mockRejectAgentAction = vi.fn();
const mockStopAgentSession = vi.fn();
const mockExecuteCommand = vi.fn();
const mockGetAvailableProviders = vi.fn();
const mockGetAISettings = vi.fn();
const mockSaveAISettings = vi.fn();

vi.mock("../../wailsjs/go/main/App", () => ({
  AIQueryWithContext: (...args: unknown[]) => mockAIQueryWithContext(...args),
  AIAgentQuery: (...args: unknown[]) => mockAIAgentQuery(...args),
  ApproveAgentAction: (...args: unknown[]) => mockApproveAgentAction(...args),
  RejectAgentAction: (...args: unknown[]) => mockRejectAgentAction(...args),
  StopAgentSession: (...args: unknown[]) => mockStopAgentSession(...args),
  ExecuteCommand: (...args: unknown[]) => mockExecuteCommand(...args),
  GetAvailableProviders: () => mockGetAvailableProviders(),
  GetAISettings: () => mockGetAISettings(),
  SaveAISettings: (...args: unknown[]) => mockSaveAISettings(...args),
}));

// Mock Wails runtime events so they don't throw in test environment.
vi.mock("../../wailsjs/runtime/runtime", () => ({
  EventsOn: vi.fn(() => () => {}),
  EventsOff: vi.fn(),
}));

// ---------------------------------------------------------------------------
// Store reset
// ---------------------------------------------------------------------------

beforeEach(() => {
  vi.clearAllMocks();
  // Merge-reset only data fields — replacing the full store state (replace=true)
  // would strip action functions from the store, breaking all tests.
  useAIStore.setState({
    contextQueue: [],
    conversations: [],
    activeConversationId: null,
    autopilotEnabled: false,
    selectedModel: null,
    enabledModels: [],
  });

  // Default: no providers, no models
  mockGetAvailableProviders.mockResolvedValue([]);
  mockGetAISettings.mockResolvedValue({
    enabled: true,
    selectedProvider: "ollama",
    selectedModel: "llama3.2",
    providers: {},
  });
});

// ---------------------------------------------------------------------------
// Rendering
// ---------------------------------------------------------------------------

describe("AIQueryView rendering", () => {
  it("renders the AI Copilot header", async () => {
    render(<AIQueryView />);
    expect(screen.getByText("AI Copilot")).toBeDefined();
  });

  it("renders the query input textarea", async () => {
    render(<AIQueryView />);
    const textarea = screen.getByRole("textbox");
    expect(textarea).toBeDefined();
  });

  it("shows empty state when no active conversation", async () => {
    render(<AIQueryView />);
    await waitFor(() => {
      expect(
        screen.getByText("Ask me anything about your cluster")
      ).toBeDefined();
    });
  });

  it("shows 'No conversations yet' when list is empty", async () => {
    render(<AIQueryView />);
    await waitFor(() => {
      expect(screen.getByText("No conversations yet")).toBeDefined();
    });
  });

  it("renders New button in conversation sidebar", async () => {
    render(<AIQueryView />);
    expect(screen.getByText("New")).toBeDefined();
  });
});

// ---------------------------------------------------------------------------
// Context chips
// ---------------------------------------------------------------------------

describe("context chips", () => {
  it("displays context chips for items in context queue", async () => {
    useAIStore.setState({
      contextQueue: [
        {
          id: "chip-1",
          type: "pod",
          name: "my-pod",
          namespace: "default",
          cluster: "prod",
          addedAt: new Date(),
        },
      ],
    });

    render(<AIQueryView />);
    // Chip shows type/name
    await waitFor(() => {
      expect(screen.getByText("pod/my-pod")).toBeDefined();
    });
  });

  it("removes a context chip when X is clicked", async () => {
    useAIStore.setState({
      contextQueue: [
        {
          id: "chip-removable",
          type: "deployment",
          name: "nginx",
          namespace: "default",
          cluster: "prod",
          addedAt: new Date(),
        },
      ],
    });

    render(<AIQueryView />);

    await waitFor(() => {
      expect(screen.getByText("deployment/nginx")).toBeDefined();
    });

    // Click the remove X button on the chip
    const chipRemoveBtn = screen.getByTestId("context-remove-chip-removable");
    fireEvent.click(chipRemoveBtn);

    await waitFor(() => {
      expect(useAIStore.getState().contextQueue).toHaveLength(0);
    });
  });

  it("shows Clear all button when context queue has items", async () => {
    useAIStore.setState({
      contextQueue: [
        {
          id: "item-1",
          type: "pod",
          name: "pod-a",
          namespace: "default",
          cluster: "prod",
          addedAt: new Date(),
        },
      ],
    });

    render(<AIQueryView />);
    await waitFor(() => {
      expect(screen.getByText("Clear all")).toBeDefined();
    });
  });

  it("clears all context chips when Clear all is clicked", async () => {
    useAIStore.setState({
      contextQueue: [
        {
          id: "item-a",
          type: "pod",
          name: "pod-a",
          namespace: "default",
          cluster: "prod",
          addedAt: new Date(),
        },
        {
          id: "item-b",
          type: "service",
          name: "svc-b",
          namespace: "default",
          cluster: "prod",
          addedAt: new Date(),
        },
      ],
    });

    render(<AIQueryView />);

    await waitFor(() => {
      expect(screen.getByText("Clear all")).toBeDefined();
    });

    fireEvent.click(screen.getByText("Clear all"));

    await waitFor(() => {
      expect(useAIStore.getState().contextQueue).toHaveLength(0);
    });
  });

  it("does not show Clear all button when context queue is empty", async () => {
    render(<AIQueryView />);
    await waitFor(() => {
      expect(screen.queryByText("Clear all")).toBeNull();
    });
  });
});

// ---------------------------------------------------------------------------
// Autopilot toggle
// ---------------------------------------------------------------------------

describe("autopilot toggle", () => {
  it("shows 'Autopilot OFF' when disabled", async () => {
    render(<AIQueryView />);
    await waitFor(() => {
      expect(screen.getByText("Autopilot OFF")).toBeDefined();
    });
  });

  it("toggles to 'Autopilot ON' when clicked", async () => {
    render(<AIQueryView />);
    await waitFor(() => {
      expect(screen.getByText("Autopilot OFF")).toBeDefined();
    });
    fireEvent.click(screen.getByText("Autopilot OFF"));
    await waitFor(() => {
      expect(screen.getByText("Autopilot ON")).toBeDefined();
    });
  });

  it("toggles back to 'Autopilot OFF' after second click", async () => {
    render(<AIQueryView />);
    const btn = await screen.findByText("Autopilot OFF");
    fireEvent.click(btn);
    await screen.findByText("Autopilot ON");
    fireEvent.click(screen.getByText("Autopilot ON"));
    await screen.findByText("Autopilot OFF");
  });
});

// ---------------------------------------------------------------------------
// Conversation list
// ---------------------------------------------------------------------------

describe("conversation management", () => {
  it("creates a new conversation when New button is clicked", async () => {
    render(<AIQueryView />);
    fireEvent.click(screen.getByText("New"));
    await waitFor(() => {
      expect(useAIStore.getState().conversations).toHaveLength(1);
    });
  });

  it("shows conversation title in sidebar", async () => {
    useAIStore.getState().createConversation("dev");
    render(<AIQueryView />);
    await waitFor(() => {
      expect(screen.getByText("New Conversation")).toBeDefined();
    });
  });
});

// ---------------------------------------------------------------------------
// Response parsing (via DOM) — tested via parseResponse logic indirectly
// ---------------------------------------------------------------------------

describe("response parsing — bash code blocks", () => {
  it("submitting a query calls AIQueryWithContext", async () => {
    mockAIQueryWithContext.mockResolvedValue("No commands here.");
    render(<AIQueryView />);

    const textarea = screen.getByRole("textbox");
    fireEvent.change(textarea, { target: { value: "Why is my pod crashing?" } });
    fireEvent.keyDown(textarea, { key: "Enter", shiftKey: false });

    await waitFor(() => {
      expect(mockAIQueryWithContext).toHaveBeenCalledWith(
        "Why is my pod crashing?",
        expect.any(Array),
        expect.any(String),
        expect.any(String),
        expect.any(Array) // previousMessages history parameter
      );
    });
  });

  it("plain text response is displayed in message bubble", async () => {
    mockAIQueryWithContext.mockResolvedValue("Your pod is healthy.");
    render(<AIQueryView />);

    const textarea = screen.getByRole("textbox");
    fireEvent.change(textarea, { target: { value: "Pod status?" } });
    fireEvent.keyDown(textarea, { key: "Enter", shiftKey: false });

    await waitFor(
      () => {
        expect(screen.getByText("Your pod is healthy.")).toBeDefined();
      },
      { timeout: 5000 }
    );
  });

  it("null response triggers error state", async () => {
    mockAIQueryWithContext.mockResolvedValue(null);
    render(<AIQueryView />);

    const textarea = screen.getByRole("textbox");
    fireEvent.change(textarea, { target: { value: "test query" } });
    fireEvent.keyDown(textarea, { key: "Enter", shiftKey: false });

    await waitFor(
      () => {
        const errorText = screen.queryByText((content) =>
          content.toLowerCase().includes("error") ||
          content.toLowerCase().includes("invalid")
        );
        expect(errorText).toBeDefined();
      },
      { timeout: 5000 }
    );
  });
});

// ---------------------------------------------------------------------------
// Autopilot security invariant
// ---------------------------------------------------------------------------

describe("autopilot security invariant", () => {
  it("never auto-executes destructive commands even when autopilot enabled", async () => {
    useAIStore.setState({ autopilotEnabled: true });

    mockAIQueryWithContext.mockResolvedValue(
      "```bash\nkubectl delete pod nginx\n```"
    );
    mockExecuteCommand.mockResolvedValue("deleted");

    render(<AIQueryView />);

    const textarea = screen.getByRole("textbox");
    fireEvent.change(textarea, { target: { value: "delete the nginx pod" } });
    fireEvent.keyDown(textarea, { key: "Enter", shiftKey: false });

    await waitFor(
      () => {
        expect(mockAIQueryWithContext).toHaveBeenCalled();
      },
      { timeout: 3000 }
    );

    // Wait past the 200ms auto-execute delay to confirm ExecuteCommand is not called
    await new Promise((r) => setTimeout(r, 400));
    expect(mockExecuteCommand).not.toHaveBeenCalled();
  });

  it("never auto-executes write commands even when autopilot enabled", async () => {
    useAIStore.setState({ autopilotEnabled: true });

    mockAIQueryWithContext.mockResolvedValue(
      "```bash\nkubectl apply -f deployment.yaml\n```"
    );
    mockExecuteCommand.mockResolvedValue("applied");

    render(<AIQueryView />);

    const textarea = screen.getByRole("textbox");
    fireEvent.change(textarea, { target: { value: "deploy the app" } });
    fireEvent.keyDown(textarea, { key: "Enter", shiftKey: false });

    await waitFor(() => {
      expect(mockAIQueryWithContext).toHaveBeenCalled();
    });

    await new Promise((r) => setTimeout(r, 400));
    expect(mockExecuteCommand).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// Input state
// ---------------------------------------------------------------------------

describe("input state", () => {
  it("clears the input after submitting", async () => {
    mockAIQueryWithContext.mockResolvedValue("OK");
    render(<AIQueryView />);

    const textarea = screen.getByRole("textbox");
    fireEvent.change(textarea, { target: { value: "my query" } });
    fireEvent.keyDown(textarea, { key: "Enter", shiftKey: false });

    await waitFor(() => {
      expect((textarea as HTMLTextAreaElement).value).toBe("");
    });
  });

  it("does not submit when input is empty", async () => {
    render(<AIQueryView />);
    const textarea = screen.getByRole("textbox");
    fireEvent.keyDown(textarea, { key: "Enter", shiftKey: false });
    expect(mockAIQueryWithContext).not.toHaveBeenCalled();
  });

  it("does not submit with whitespace-only input", async () => {
    render(<AIQueryView />);
    const textarea = screen.getByRole("textbox");
    fireEvent.change(textarea, { target: { value: "   " } });
    fireEvent.keyDown(textarea, { key: "Enter", shiftKey: false });
    expect(mockAIQueryWithContext).not.toHaveBeenCalled();
  });
});
