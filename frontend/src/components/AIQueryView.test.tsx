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

// ---------------------------------------------------------------------------
// CurrentModelChip (provider/model selector popover in AIQueryView header)
// ---------------------------------------------------------------------------

describe("CurrentModelChip", () => {
  it("renders the 'Configure AI' CTA when no providers have enabled models", async () => {
    // No providers returned, no enabled models.
    mockGetAvailableProviders.mockResolvedValue([]);
    mockGetAISettings.mockResolvedValue({
      enabled: false,
      selectedProvider: "",
      selectedModel: "",
      providers: {},
    });
    const onOpenSettings = vi.fn();
    render(<AIQueryView onOpenSettings={onOpenSettings} />);
    const cta = await screen.findByRole("button", { name: /Configure AI/i });
    expect(cta).toBeDefined();
    fireEvent.click(cta);
    expect(onOpenSettings).toHaveBeenCalledTimes(1);
  });

  it("renders the current model truncated with title=<full-model-id>", async () => {
    mockGetAvailableProviders.mockResolvedValue([
      { id: "openai", name: "OpenAI", requiresApiKey: true, defaultEndpoint: "", defaultModel: "gpt-4o", models: [] },
    ]);
    const longModel = "openai/gpt-4o-2024-11-20-with-some-very-long-suffix";
    mockGetAISettings.mockResolvedValue({
      enabled: true,
      selectedProvider: "openai",
      selectedModel: longModel,
      providers: {
        openai: {
          enabled: true,
          apiKey: "sk-x",
          endpoint: "https://api.openai.com/v1",
          models: [longModel],
        },
      },
    });
    render(<AIQueryView />);
    // The chip's button has title containing "OpenAI — <model>".
    const chipBtn = await screen.findByTitle(new RegExp(longModel));
    expect(chipBtn).toBeDefined();
    // The displayed label uses truncate CSS; the raw text is still there.
    expect(chipBtn.textContent).toContain(longModel);
  });

  it("opens the popover and lists providers grouped by name", async () => {
    mockGetAvailableProviders.mockResolvedValue([
      { id: "openai", name: "OpenAI", requiresApiKey: true, defaultEndpoint: "", defaultModel: "gpt-4o", models: [] },
      { id: "ollama", name: "Ollama", requiresApiKey: false, defaultEndpoint: "", defaultModel: "llama3.2", models: [] },
    ]);
    mockGetAISettings.mockResolvedValue({
      enabled: true,
      selectedProvider: "ollama",
      selectedModel: "llama3.2",
      providers: {
        openai: {
          enabled: true,
          apiKey: "sk-x",
          endpoint: "",
          models: ["gpt-4o-mini"],
        },
        ollama: {
          enabled: true,
          apiKey: "",
          endpoint: "",
          models: ["llama3.2"],
        },
      },
    });
    render(<AIQueryView />);
    const chipBtn = await screen.findByTitle(/Ollama — llama3.2/);
    fireEvent.click(chipBtn);

    // Two provider group headings
    expect(await screen.findByText("OpenAI")).toBeDefined();
    expect(screen.getByText("Ollama")).toBeDefined();
    // Two menuitemradios for the two models
    const items = screen.getAllByRole("menuitemradio");
    expect(items.length).toBe(2);
  });

  it("invokes onOpenSettings when 'Manage providers & keys…' is clicked", async () => {
    mockGetAvailableProviders.mockResolvedValue([
      { id: "ollama", name: "Ollama", requiresApiKey: false, defaultEndpoint: "", defaultModel: "llama3.2", models: [] },
    ]);
    mockGetAISettings.mockResolvedValue({
      enabled: true,
      selectedProvider: "ollama",
      selectedModel: "llama3.2",
      providers: {
        ollama: {
          enabled: true,
          apiKey: "",
          endpoint: "",
          models: ["llama3.2"],
        },
      },
    });
    const onOpenSettings = vi.fn();
    render(<AIQueryView onOpenSettings={onOpenSettings} />);
    const chipBtn = await screen.findByTitle(/Ollama — llama3.2/);
    fireEvent.click(chipBtn);
    const manage = await screen.findByRole("menuitem", {
      name: /Manage providers & keys/i,
    });
    fireEvent.click(manage);
    expect(onOpenSettings).toHaveBeenCalledTimes(1);
  });

  it("closes the popover when Escape is pressed", async () => {
    mockGetAvailableProviders.mockResolvedValue([
      { id: "ollama", name: "Ollama", requiresApiKey: false, defaultEndpoint: "", defaultModel: "llama3.2", models: [] },
    ]);
    mockGetAISettings.mockResolvedValue({
      enabled: true,
      selectedProvider: "ollama",
      selectedModel: "llama3.2",
      providers: {
        ollama: {
          enabled: true,
          apiKey: "",
          endpoint: "",
          models: ["llama3.2"],
        },
      },
    });
    render(<AIQueryView />);
    const chipBtn = await screen.findByTitle(/Ollama — llama3.2/);
    fireEvent.click(chipBtn);
    await screen.findByRole("menu");
    fireEvent.keyDown(document, { key: "Escape" });
    await waitFor(() => {
      expect(screen.queryByRole("menu")).toBeNull();
    });
  });

  it("closes the popover on outside click", async () => {
    mockGetAvailableProviders.mockResolvedValue([
      { id: "ollama", name: "Ollama", requiresApiKey: false, defaultEndpoint: "", defaultModel: "llama3.2", models: [] },
    ]);
    mockGetAISettings.mockResolvedValue({
      enabled: true,
      selectedProvider: "ollama",
      selectedModel: "llama3.2",
      providers: {
        ollama: {
          enabled: true,
          apiKey: "",
          endpoint: "",
          models: ["llama3.2"],
        },
      },
    });
    render(<AIQueryView />);
    const chipBtn = await screen.findByTitle(/Ollama — llama3.2/);
    fireEvent.click(chipBtn);
    await screen.findByRole("menu");
    // Simulate mousedown on document.body (outside the popover).
    fireEvent.mouseDown(document.body);
    await waitFor(() => {
      expect(screen.queryByRole("menu")).toBeNull();
    });
  });

  it("clicking a model updates selectedModel and closes the popover (local provider, no consent gate)", async () => {
    mockGetAvailableProviders.mockResolvedValue([
      { id: "ollama", name: "Ollama", requiresApiKey: false, defaultEndpoint: "", defaultModel: "llama3.2", models: [] },
    ]);
    mockGetAISettings.mockResolvedValue({
      enabled: true,
      selectedProvider: "ollama",
      selectedModel: "llama3.2",
      providers: {
        ollama: {
          enabled: true,
          apiKey: "",
          endpoint: "",
          models: ["llama3.2", "mistral"],
        },
      },
    });
    mockSaveAISettings.mockResolvedValue(undefined);
    render(<AIQueryView />);
    const chipBtn = await screen.findByTitle(/Ollama — llama3.2/);
    fireEvent.click(chipBtn);
    const mistral = await screen.findByRole("menuitemradio", { name: /mistral/ });
    fireEvent.click(mistral);
    // Popover should close.
    await waitFor(() => {
      expect(screen.queryByRole("menu")).toBeNull();
    });
    // SaveAISettings should have been called with the new model.
    await waitFor(() => {
      expect(mockSaveAISettings).toHaveBeenCalled();
    });
    const last = mockSaveAISettings.mock.calls.pop()![0];
    expect(last.selectedModel).toBe("mistral");
    expect(last.selectedProvider).toBe("ollama");
  });
});
