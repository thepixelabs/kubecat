import { useState, useRef, useEffect, useCallback } from "react";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";
import {
  Sparkles,
  Send,
  X,
  Check,
  Trash2,
  Pin,
  MessageSquare,
  Zap,
  ChevronDown,
  Wrench,
  Square,
  Brain,
  ChevronRight,
  Bot,
  Clipboard,
  ClipboardCheck,
} from "lucide-react";
import Markdown from "react-markdown";
import rehypeSanitize, { defaultSchema } from "rehype-sanitize";
import { useAIStore } from "../stores/aiStore";
import { useToastStore } from "../stores/toastStore";
import { useTelemetry } from "../hooks/useTelemetry";
import {
  AIQueryWithContext,
  AIAgentQuery,
  ApproveAgentAction,
  RejectAgentAction,
  StopAgentSession,
  ExecuteCommand,
  GetAvailableProviders,
  GetAISettings,
  SaveAISettings,
} from "../../wailsjs/go/main/App";
import {
  shouldAutoExecute,
  getCommandSafetyIcon,
} from "../utils/commandClassifier";
import {
  CloudAIConsentDialog,
  type CloudAIProvider,
} from "./CloudAIConsentDialog";

// ── Cloud provider metadata for the consent dialog ──────────────────────────
// Keep in sync with backend `GetAvailableProviders`. Ollama is intentionally
// absent — local providers never trigger the consent gate.
const CLOUD_PROVIDER_META: Record<string, CloudAIProvider> = {
  openai: {
    id: "openai",
    name: "OpenAI",
    dpaUrl: "https://openai.com/policies/data-processing-addendum",
    dataResidency: "United States",
  },
  anthropic: {
    id: "anthropic",
    name: "Anthropic",
    dpaUrl: "https://www.anthropic.com/legal/dpa",
    dataResidency: "United States",
  },
  google: {
    id: "google",
    name: "Google Gemini",
    dpaUrl: "https://cloud.google.com/terms/data-processing-addendum",
    dataResidency: "United States",
  },
  gemini: {
    id: "gemini",
    name: "Google Gemini",
    dpaUrl: "https://cloud.google.com/terms/data-processing-addendum",
    dataResidency: "United States",
  },
};

function isLocalProvider(providerId: string): boolean {
  return providerId === "ollama" || providerId === "local";
}

// Strict sanitization schema for AI-generated markdown.
// Extends the rehype-sanitize default (which already blocks <script>, <style>,
// and all on* event handlers) by restricting href/src to safe protocols only,
// which eliminates javascript: and vbscript: injection vectors.
const markdownSanitizeSchema = {
  ...defaultSchema,
  protocols: {
    href: ["http", "https", "mailto"],
    src: ["http", "https"],
  },
};

interface CommandBlockLocal {
  id: string;
  command: string;
  status: "pending" | "running" | "completed" | "rejected" | "error";
  output?: string;
  error?: string;
}

// Agent event types emitted by the backend agent loop.
interface AgentThinkingEvent { iteration: number; }
interface AgentToolCallEvent { tool: string; parameters: Record<string,string>; iteration: number; }
interface AgentToolResultEvent { tool: string; result: string; allowed: boolean; reason?: string; iteration: number; }
interface AgentApprovalNeededEvent { sessionId: string; toolCallId: string; tool: string; parameters: Record<string,string>; category: string; iteration: number; }
interface AgentCompleteEvent { response: string; iterations: number; }

// AgentStep represents a visible step in the agent UI timeline.
type AgentStep =
  | { kind: "thinking"; iteration: number }
  | { kind: "tool_call"; tool: string; parameters: Record<string,string>; iteration: number }
  | { kind: "tool_result"; tool: string; result: string; allowed: boolean; reason?: string; iteration: number }
  | { kind: "approval_needed"; sessionId: string; toolCallId: string; tool: string; parameters: Record<string,string>; category: string; iteration: number }
  | { kind: "complete"; response: string };

export function AIQueryView() {
  const [query, setQuery] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [agentMode, setAgentMode] = useState(false);
  const [agentSessionId, setAgentSessionId] = useState<string | null>(null);
  const [agentSteps, setAgentSteps] = useState<AgentStep[]>([]);
  const [agentRunning, setAgentRunning] = useState(false);
  const [availableModels, setAvailableModels] = useState<
    { providerId: string; providerName: string; models: string[] }[]
  >([]);
  // Cloud AI consent dialog state. Holds the pending (provider, model) pair
  // while we wait for the user to acknowledge data transmission terms.
  const [consentDialog, setConsentDialog] = useState<{
    provider: CloudAIProvider;
    pendingProviderId: string;
    pendingModel: string;
  } | null>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);

  const contextQueue = useAIStore((state) => state.contextQueue);
  const removeFromContext = useAIStore((state) => state.removeFromContext);
  const clearContext = useAIStore((state) => state.clearContext);
  const conversations = useAIStore((state) => state.conversations);
  const activeConversationId = useAIStore(
    (state) => state.activeConversationId
  );
  const createConversation = useAIStore((state) => state.createConversation);
  const setActiveConversation = useAIStore(
    (state) => state.setActiveConversation
  );
  const addMessage = useAIStore((state) => state.addMessage);
  const updateMessage = useAIStore((state) => state.updateMessage);
  const deleteConversation = useAIStore((state) => state.deleteConversation);
  const pinConversation = useAIStore((state) => state.pinConversation);
  const unpinConversation = useAIStore((state) => state.unpinConversation);
  const autopilotEnabled = useAIStore((state) => state.autopilotEnabled);
  const setAutopilotEnabled = useAIStore((state) => state.setAutopilotEnabled);
  const selectedModel = useAIStore((state) => state.selectedModel);
  const setSelectedModel = useAIStore((state) => state.setSelectedModel);
  // enabledModels is no longer used here, we use availableModels derived from settings
  const addToast = useToastStore((state) => state.addToast);
  const { track } = useTelemetry();

  const activeConversation = conversations.find(
    (c) => c.id === activeConversationId
  );

  // Load AI settings and available models on mount
  useEffect(() => {
    loadAIConfig();
  }, []);

  // Wire up Wails event listeners for the agent loop.
  // We subscribe once on mount and clean up on unmount.
  useEffect(() => {
    const handleThinking = (data: AgentThinkingEvent) => {
      setAgentSteps((prev) => [...prev, { kind: "thinking", iteration: data.iteration }]);
    };

    const handleToolCall = (data: AgentToolCallEvent) => {
      setAgentSteps((prev) => [...prev, {
        kind: "tool_call",
        tool: data.tool,
        parameters: data.parameters,
        iteration: data.iteration,
      }]);
    };

    const handleToolResult = (data: AgentToolResultEvent) => {
      setAgentSteps((prev) => [...prev, {
        kind: "tool_result",
        tool: data.tool,
        result: data.result,
        allowed: data.allowed,
        reason: data.reason,
        iteration: data.iteration,
      }]);
    };

    const handleApprovalNeeded = (data: AgentApprovalNeededEvent) => {
      setAgentSteps((prev) => [...prev, {
        kind: "approval_needed",
        sessionId: data.sessionId,
        toolCallId: data.toolCallId,
        tool: data.tool,
        parameters: data.parameters,
        category: data.category,
        iteration: data.iteration,
      }]);
    };

    const handleComplete = (data: AgentCompleteEvent) => {
      setAgentRunning(false);
      setAgentSteps((prev) => [...prev, { kind: "complete", response: data.response }]);
      // Update the loading assistant message with the final response.
      // We read state directly from the Zustand store (outside render) since
      // this runs inside an event callback.
      const storeState = useAIStore.getState();
      const activeConv = storeState.getActiveConversation();
      if (activeConv) {
        // Find the most recent loading message (last one in the array).
        const msgs = activeConv.messages;
        let loadingMsg = null;
        for (let i = msgs.length - 1; i >= 0; i--) {
          if (msgs[i].isLoading) {
            loadingMsg = msgs[i];
            break;
          }
        }
        if (loadingMsg) {
          const { rawText, commands } = parseResponse(data.response);
          storeState.updateMessage(activeConv.id, loadingMsg.id, {
            content: rawText,
            isLoading: false,
            response: {
              insights: [],
              commands,
              visualizations: [],
              followUpSuggestions: [],
              rawText,
            },
          });
        }
      }
      setIsLoading(false);
    };

    const handleError = () => {
      setAgentRunning(false);
      setIsLoading(false);
    };

    EventsOn("ai:agent:thinking", handleThinking);
    EventsOn("ai:agent:tool-call", handleToolCall);
    EventsOn("ai:agent:tool-result", handleToolResult);
    EventsOn("ai:agent:approval-needed", handleApprovalNeeded);
    EventsOn("ai:agent:complete", handleComplete);
    EventsOn("ai:agent:error", handleError);

    return () => {
      EventsOff("ai:agent:thinking");
      EventsOff("ai:agent:tool-call");
      EventsOff("ai:agent:tool-result");
      EventsOff("ai:agent:approval-needed");
      EventsOff("ai:agent:complete");
      EventsOff("ai:agent:error");
    };
  }, []);  

  const handleStopAgent = useCallback(async () => {
    if (agentSessionId) {
      try {
        await StopAgentSession(agentSessionId);
      } catch (err) {
        console.error("Failed to stop agent session:", err);
      }
    }
    setAgentRunning(false);
    setIsLoading(false);
  }, [agentSessionId]);

  const handleApproveAgent = useCallback(async (sessionId: string, toolCallId: string) => {
    try {
      await ApproveAgentAction(sessionId, toolCallId);
      // Replace the approval_needed step with a tool_call step (approved indicator)
      setAgentSteps((prev) =>
        prev.map((s): AgentStep => {
          if (s.kind === "approval_needed" && s.toolCallId === toolCallId) {
            return { kind: "tool_call", tool: s.tool, parameters: s.parameters, iteration: s.iteration };
          }
          return s;
        })
      );
    } catch (err) {
      console.error("Failed to approve agent action:", err);
    }
  }, []);

  const handleRejectAgent = useCallback(async (sessionId: string, toolCallId: string) => {
    try {
      await RejectAgentAction(sessionId, toolCallId);
      // Remove the approval_needed step
      setAgentSteps((prev) =>
        prev.filter((s) => {
          if (s.kind === "approval_needed" && s.toolCallId === toolCallId) {
            return false;
          }
          return true;
        })
      );
    } catch (err) {
      console.error("Failed to reject agent action:", err);
    }
  }, []);

  const loadAIConfig = async () => {
    try {
      const [settings, providersInfo] = await Promise.all([
        GetAISettings(),
        GetAvailableProviders(),
      ]);

      const groupedModels: {
        providerId: string;
        providerName: string;
        models: string[];
      }[] = [];

      // Iterate through providers in settings
      // We cast settings to any because interface in wailsjs might be outdated until rebuild
      // but at runtime it comes from Go.
      const settingsObj = settings as any;

      if (settingsObj.providers) {
        for (const [providerId, config] of Object.entries(
          settingsObj.providers
        )) {
          if (
            (config as any).enabled &&
            (config as any).models &&
            (config as any).models.length > 0
          ) {
            const info = (providersInfo as any[]).find(
              (p) => p.id === providerId
            );
            groupedModels.push({
              providerId,
              providerName: info ? info.name : providerId,
              models: (config as any).models,
            });
          }
        }
      }

      setAvailableModels(groupedModels);

      // Set default selected model if not set or invalid
      if (settingsObj.selectedModel) {
        setSelectedModel(settingsObj.selectedModel);
      } else if (
        groupedModels.length > 0 &&
        groupedModels[0].models.length > 0
      ) {
        handleModelChange(groupedModels[0].models[0], groupedModels);
      }
    } catch (err) {
      console.error("Failed to load AI configuration:", err);
    }
  };

  // Persist a (provider, model) selection. Optionally stamps consent fields
  // when the user has just accepted the cloud-AI disclosure.
  const persistSelection = async (
    providerId: string,
    model: string,
    consent?: { given: boolean; provider: string }
  ) => {
    const settings = await GetAISettings();
    const settingsObj = settings as any;
    settingsObj.selectedProvider = providerId;
    settingsObj.selectedModel = model;
    if (consent) {
      settingsObj.aiConsentGiven = consent.given;
      settingsObj.aiConsentDate = new Date().toISOString().slice(0, 10);
      settingsObj.aiConsentProvider = consent.provider;
    }
    await SaveAISettings(settingsObj);
    return settingsObj;
  };

  const handleModelChange = async (
    model: string,
    modelsList = availableModels
  ) => {
    // Find provider for this model
    let providerId = "";
    for (const group of modelsList) {
      if (group.models.includes(model)) {
        providerId = group.providerId;
        break;
      }
    }

    if (!providerId) {
      setSelectedModel(model);
      return;
    }

    // Cloud provider gating: if the user is switching to a non-local provider,
    // require explicit consent before persisting and before any query can fire.
    // Re-prompt on provider change (per phase 2 acceptance criteria).
    if (!isLocalProvider(providerId)) {
      try {
        const current = (await GetAISettings()) as any;
        const alreadyConsented =
          current?.aiConsentGiven === true &&
          current?.aiConsentProvider === providerId;

        if (!alreadyConsented) {
          const meta =
            CLOUD_PROVIDER_META[providerId] ?? {
              id: providerId,
              name: providerId,
              dpaUrl: "https://example.com",
              dataResidency: "United States",
            };
          // Defer the model swap and the save until the dialog resolves.
          setConsentDialog({
            provider: meta,
            pendingProviderId: providerId,
            pendingModel: model,
          });
          return;
        }
      } catch (err) {
        console.error("Failed to read AI settings for consent check:", err);
        return;
      }
    }

    setSelectedModel(model);
    try {
      await persistSelection(providerId, model);
    } catch (err) {
      console.error("Failed to update settings:", err);
    }
  };

  // Consent dialog handlers ----------------------------------------------------
  const handleConsentAccept = async () => {
    if (!consentDialog) return;
    const { pendingProviderId, pendingModel } = consentDialog;
    try {
      await persistSelection(pendingProviderId, pendingModel, {
        given: true,
        provider: pendingProviderId,
      });
      setSelectedModel(pendingModel);
    } catch (err) {
      console.error("Failed to persist consent:", err);
    } finally {
      setConsentDialog(null);
    }
  };

  const handleConsentDecline = () => {
    // User declined: do not change selection or persist anything.
    setConsentDialog(null);
  };

  const handleConsentUseLocal = async () => {
    // Escape hatch — try to switch to the first local provider available.
    setConsentDialog(null);
    const localGroup = availableModels.find((g) => isLocalProvider(g.providerId));
    if (localGroup && localGroup.models.length > 0) {
      const localModel = localGroup.models[0];
      setSelectedModel(localModel);
      try {
        await persistSelection(localGroup.providerId, localModel);
      } catch (err) {
        console.error("Failed to switch to local provider:", err);
      }
    } else {
      addToast({
        type: "info",
        message:
          "No local Ollama provider configured. Enable Ollama in AI Provider settings to use local-only mode.",
      });
    }
  };

  // Parse AI response to extract commands
  const parseResponse = (
    response: string
  ): { rawText: string; commands: CommandBlockLocal[] } => {
    const commands: CommandBlockLocal[] = [];
    let rawText = response;

    // Extract code blocks with ```bash or ```sh
    const codeBlockRegex = /```(?:bash|sh|shell)?\n([\s\S]*?)```/g;
    let match;
    while ((match = codeBlockRegex.exec(response)) !== null) {
      const cmd = match[1].trim();
      if (cmd) {
        commands.push({
          id: `cmd_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`,
          command: cmd,
          status: "pending",
        });
      }
    }

    // Remove code blocks from text
    rawText = response.replace(codeBlockRegex, "").trim();

    // Look for inline commands like `kubectl ...`
    if (commands.length === 0) {
      const inlineRegex = /`(kubectl[^`]+)`/g;
      while ((match = inlineRegex.exec(response)) !== null) {
        commands.push({
          id: `cmd_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`,
          command: match[1].trim(),
          status: "pending",
        });
      }
    }

    return { rawText, commands };
  };

  const handleSubmit = async () => {
    if (!query.trim() || isLoading) return;

    track({ name: "ai_query_submitted" });

    // Get or create active conversation
    let convId = activeConversationId;
    if (!convId) {
      convId = createConversation("default"); // TODO: Get actual cluster name
    }

    // Snapshot the existing messages BEFORE adding the new user message so we
    // can pass them as conversation history to the backend.
    const existingConv = conversations.find((c) => c.id === convId);
    const previousMessages = (existingConv?.messages ?? [])
      .filter((m) => !m.isLoading && m.content)
      .map((m) => ({ role: m.role, content: m.content }));

    // Add user message
    addMessage(convId, {
      role: "user",
      content: query,
      contextItems: [...contextQueue], // Attach current context
    });

    // Clear input
    const currentQuery = query;
    setQuery("");

    // Add loading assistant message
    const assistantMessageId = addMessage(convId, {
      role: "assistant",
      content: "",
      isLoading: true,
    });

    setIsLoading(true);

    // For agent mode we return early from the entire handleSubmit function
    // before setting up the try/finally that would clear isLoading.
    // The ai:agent:complete / ai:agent:error event handlers own isLoading
    // from this point forward.
    const contextItems = contextQueue.map((item) => ({
      id: item.id,
      type: item.type,
      namespace: item.namespace || "",
      name: item.name,
      cluster: item.cluster,
    }));

    let providerId = "";
    if (selectedModel) {
      for (const group of availableModels) {
        if (group.models.includes(selectedModel)) {
          providerId = group.providerId;
          break;
        }
      }
    }

    if (agentMode) {
      const sessionId = `agent_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
      setAgentSessionId(sessionId);
      setAgentSteps([]);
      setAgentRunning(true);

      updateMessage(convId!, assistantMessageId, {
        content: "Agent is investigating...",
        isLoading: true,
      });

      try {
        await AIAgentQuery(currentQuery, convId!, "", providerId, selectedModel || "");
        clearContext();
      } catch (err) {
        console.error("[AIAgent] Launch error:", err);
        updateMessage(convId!, assistantMessageId, {
          content: `Agent error: ${err instanceof Error ? err.message : String(err)}`,
          isLoading: false,
        });
        setAgentRunning(false);
        setIsLoading(false);
      }
      // isLoading stays true until the agent completion event fires.
      return;
    }

    try {
      console.log("[AIQuery] Sending query:", currentQuery);
      console.log("[AIQuery] Context items:", contextItems.length);
      console.log("[AIQuery] History messages:", previousMessages.length);
      console.log(
        `[AIQuery] Using provider: ${providerId}, model: ${selectedModel}`
      );

      // Standard (non-agent) query path with conversation history.
      // Call backend with context with timeout
      const timeoutPromise = new Promise<never>((_, reject) => {
        setTimeout(
          () => reject(new Error("AI query timeout after 60 seconds")),
          60000
        );
      });

      const response = await Promise.race([
        AIQueryWithContext(
          currentQuery,
          contextItems,
          providerId,
          selectedModel || "",
          previousMessages
        ),
        timeoutPromise,
      ]);

      console.log(
        "[AIQuery] Received response:",
        response ? "Success" : "Empty"
      );

      // Ensure we have a response
      if (!response || typeof response !== "string") {
        throw new Error("Invalid response from AI");
      }

      // Parse response
      const { rawText, commands } = parseResponse(response);

      console.log("[AIQuery] Parsed commands:", commands.length);

      // Update assistant message with response
      updateMessage(convId!, assistantMessageId, {
        content: rawText,
        isLoading: false,
        response: {
          insights: [], // TODO: Parse structured insights
          commands: commands,
          visualizations: [],
          followUpSuggestions: [],
          rawText,
        },
      });

      // Auto-execute safe commands if autopilot is enabled
      if (autopilotEnabled && commands.length > 0) {
        for (const cmd of commands) {
          if (shouldAutoExecute(cmd.command, autopilotEnabled)) {
            // Delay slightly to avoid overwhelming the system
            setTimeout(() => {
              handleExecuteCommand(assistantMessageId, cmd.id);
            }, 200);
          }
        }
      }

      // Clear context queue after successful query
      clearContext();

      addToast({
        type: "success",
        message: "Response received",
        duration: 1500,
      });
    } catch (err) {
      console.error("[AIQuery] Error:", err);

      // Ensure message exists before updating
      const conv = conversations.find((c) => c.id === convId);
      if (conv) {
        // Update message with error - clear isLoading and response
        updateMessage(convId!, assistantMessageId, {
          content: `Error: ${err instanceof Error ? err.message : String(err)}`,
          isLoading: false,
          response: undefined, // Clear any partial response
        });
      }

      addToast({
        type: "error",
        message:
          err instanceof Error ? err.message : "Failed to get response from AI",
        duration: 4000,
      });
    } finally {
      // Always clear loading state
      console.log("[AIQuery] Cleaning up, setting isLoading to false");
      setIsLoading(false);
    }
  };

  const handleExecuteCommand = async (messageId: string, commandId: string) => {
    if (!activeConversationId) return;

    const message = activeConversation?.messages.find(
      (m) => m.id === messageId
    );
    const command = message?.response?.commands.find((c) => c.id === commandId);
    if (!command) return;

    // Update status to running
    updateMessage(activeConversationId, messageId, {
      response: {
        ...message!.response!,
        commands: message!.response!.commands.map((c) =>
          c.id === commandId ? { ...c, status: "running" } : c
        ),
      },
    });

    try {
      const output = await ExecuteCommand(command.command);

      // Update with output
      updateMessage(activeConversationId, messageId, {
        response: {
          ...message!.response!,
          commands: message!.response!.commands.map((c) =>
            c.id === commandId ? { ...c, status: "completed", output } : c
          ),
        },
      });
    } catch (err) {
      // Update with error
      updateMessage(activeConversationId, messageId, {
        response: {
          ...message!.response!,
          commands: message!.response!.commands.map((c) =>
            c.id === commandId
              ? {
                  ...c,
                  status: "error",
                  error: err instanceof Error ? err.message : String(err),
                }
              : c
          ),
        },
      });
    }
  };

  const handleRejectCommand = (messageId: string, commandId: string) => {
    if (!activeConversationId) return;

    const message = activeConversation?.messages.find(
      (m) => m.id === messageId
    );
    if (!message?.response) return;

    updateMessage(activeConversationId, messageId, {
      response: {
        ...message.response,
        commands: message.response.commands.map((c) =>
          c.id === commandId ? { ...c, status: "rejected" } : c
        ),
      },
    });
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSubmit();
    }
  };

  // Auto-resize textarea
  useEffect(() => {
    if (inputRef.current) {
      inputRef.current.style.height = "auto";
      inputRef.current.style.height = `${inputRef.current.scrollHeight}px`;
    }
  }, [query]);

  return (
    <div className="flex h-full gap-4">
      {/* Cloud AI consent gate (compliance phase 2) */}
      {consentDialog && (
        <CloudAIConsentDialog
          isOpen={true}
          provider={consentDialog.provider}
          onAccept={handleConsentAccept}
          onDecline={handleConsentDecline}
          onUseLocal={handleConsentUseLocal}
        />
      )}
      {/* History Sidebar */}
      <div className="w-64 flex-shrink-0 bg-white/80 dark:bg-slate-800/50 rounded-xl border border-stone-200 dark:border-slate-700/50 backdrop-blur-sm overflow-hidden flex flex-col">
        <div className="p-4 border-b border-stone-200 dark:border-slate-700/50">
          <div className="flex items-center justify-between mb-3">
            <h3 className="text-sm font-medium text-stone-700 dark:text-slate-300">
              Conversations
            </h3>
            <button
              onClick={() => createConversation("default")}
              className="px-2 py-1 text-xs bg-accent-500 hover:bg-accent-600 text-white rounded transition-colors"
            >
              New
            </button>
          </div>
        </div>

        <div className="flex-1 overflow-y-auto p-2 space-y-1">
          {conversations.length === 0 ? (
            <div className="p-4 text-center text-sm text-stone-500 dark:text-slate-500">
              No conversations yet
            </div>
          ) : (
            <>
              {/* Pinned conversations */}
              {conversations
                .filter((c) => c.pinned)
                .map((conv) => (
                  <ConversationItem
                    key={conv.id}
                    conversation={conv}
                    isActive={conv.id === activeConversationId}
                    onSelect={() => setActiveConversation(conv.id)}
                    onDelete={() => deleteConversation(conv.id)}
                    onTogglePin={() =>
                      conv.pinned
                        ? unpinConversation(conv.id)
                        : pinConversation(conv.id)
                    }
                  />
                ))}

              {/* Unpinned conversations */}
              {conversations
                .filter((c) => !c.pinned)
                .map((conv) => (
                  <ConversationItem
                    key={conv.id}
                    conversation={conv}
                    isActive={conv.id === activeConversationId}
                    onSelect={() => setActiveConversation(conv.id)}
                    onDelete={() => deleteConversation(conv.id)}
                    onTogglePin={() =>
                      conv.pinned
                        ? unpinConversation(conv.id)
                        : pinConversation(conv.id)
                    }
                  />
                ))}
            </>
          )}
        </div>
      </div>

      {/* Main Conversation Area */}
      <div className="flex-1 flex flex-col bg-white/80 dark:bg-slate-800/50 rounded-xl border border-stone-200 dark:border-slate-700/50 backdrop-blur-sm overflow-hidden">
        {/* Header */}
        <div className="p-4 border-b border-stone-200 dark:border-slate-700/50 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Sparkles className="w-5 h-5 text-purple-400" />
            <h2 className="text-lg font-medium">AI Copilot</h2>
          </div>

          <div className="flex items-center gap-3">
            {/* Model Selector */}
            {availableModels.length > 0 && (
              <div className="flex items-center gap-2">
                <div className="relative">
                  <select
                    value={selectedModel || ""}
                    onChange={(e) => handleModelChange(e.target.value)}
                    className="appearance-none bg-stone-100 dark:bg-slate-700 border border-stone-200 dark:border-slate-600 rounded-lg pl-3 pr-8 py-1.5 text-sm font-medium text-stone-700 dark:text-slate-200 hover:bg-stone-200 dark:hover:bg-slate-600 transition-colors cursor-pointer focus:outline-none focus:ring-2 focus:ring-purple-500/50 max-w-[200px]"
                  >
                    {availableModels.map((group) => (
                      <optgroup
                        key={group.providerId}
                        label={group.providerName}
                      >
                        {group.models.map((model) => (
                          <option
                            key={`${group.providerId}-${model}`}
                            value={model}
                          >
                            {model}
                          </option>
                        ))}
                      </optgroup>
                    ))}
                  </select>
                  <ChevronDown className="absolute right-2 top-1/2 -translate-y-1/2 w-4 h-4 text-stone-500 dark:text-slate-400 pointer-events-none" />
                </div>
              </div>
            )}

            {/* Agent Mode Toggle */}
            <button
              onClick={() => setAgentMode(!agentMode)}
              title="Agent mode: AI investigates autonomously using tools"
              className={`
                flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm font-medium transition-colors
                ${
                  agentMode
                    ? "bg-blue-500/20 text-blue-400 hover:bg-blue-500/30"
                    : "bg-stone-200 dark:bg-slate-700 text-stone-600 dark:text-slate-400 hover:bg-stone-300 dark:hover:bg-slate-600"
                }
              `}
            >
              <Bot className="w-4 h-4" />
              <span>Agent {agentMode ? "ON" : "OFF"}</span>
            </button>

            {/* Stop Agent (only visible while agent is running) */}
            {agentRunning && (
              <button
                onClick={handleStopAgent}
                className="flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm font-medium bg-red-500/20 text-red-400 hover:bg-red-500/30 transition-colors"
              >
                <Square className="w-4 h-4" />
                <span>Stop Agent</span>
              </button>
            )}

            {/* Autopilot Toggle */}
            <button
              onClick={() => setAutopilotEnabled(!autopilotEnabled)}
              className={`
                flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm font-medium transition-colors
                ${
                  autopilotEnabled
                    ? "bg-purple-500/20 text-purple-400 hover:bg-purple-500/30"
                    : "bg-stone-200 dark:bg-slate-700 text-stone-600 dark:text-slate-400 hover:bg-stone-300 dark:hover:bg-slate-600"
                }
              `}
            >
              <Zap className="w-4 h-4" />
              <span>Autopilot {autopilotEnabled ? "ON" : "OFF"}</span>
            </button>
          </div>
        </div>

        {/* Messages */}
        <div className="flex-1 overflow-y-auto p-6 space-y-6">
          {!activeConversation || activeConversation.messages.length === 0 ? (
            <div className="flex items-center justify-center h-full">
              <div className="text-center max-w-md">
                <Sparkles className="w-16 h-16 text-purple-400 mx-auto mb-4 opacity-50" />
                <h3 className="text-xl font-medium mb-2">
                  Ask me anything about your cluster
                </h3>
                <p className="text-stone-600 dark:text-slate-400 text-sm">
                  I can help you troubleshoot issues, analyze resources, and run
                  kubectl commands.
                </p>
              </div>
            </div>
          ) : (
            <>
              {activeConversation.messages.map((message) => (
                <MessageBubble
                  key={message.id}
                  message={message}
                  onExecuteCommand={(cmdId) =>
                    handleExecuteCommand(message.id, cmdId)
                  }
                  onRejectCommand={(cmdId) =>
                    handleRejectCommand(message.id, cmdId)
                  }
                />
              ))}
              {/* Agent step timeline shown inline at the bottom while running */}
              {agentSteps.length > 0 && (
                <AgentTimeline
                  steps={agentSteps}
                  running={agentRunning}
                  onApprove={handleApproveAgent}
                  onReject={handleRejectAgent}
                />
              )}
            </>
          )}
        </div>

        {/* Input Area */}
        <div className="p-4 border-t border-stone-200 dark:border-slate-700/50">
          {/* Collapsible Context Panel */}
          {contextQueue.length > 0 && (
            <ContextPanel
              items={contextQueue}
              onRemove={removeFromContext}
              onClearAll={clearContext}
            />
          )}

          <div className="flex gap-2">
            <textarea
              ref={inputRef}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder={
                contextQueue.length > 0
                  ? "Ask about the selected resources..."
                  : "Ask about your cluster..."
              }
              className="flex-1 bg-white dark:bg-slate-900 border border-stone-200 dark:border-slate-700 rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-purple-500/50 text-stone-900 dark:text-slate-100 resize-none max-h-32"
              rows={1}
              disabled={isLoading}
            />
            <button
              onClick={handleSubmit}
              disabled={!query.trim() || isLoading}
              className="px-4 py-3 bg-purple-500 hover:bg-purple-600 text-white rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center"
            >
              {isLoading ? (
                <div className="w-5 h-5 border-2 border-white border-t-transparent rounded-full animate-spin" />
              ) : (
                <Send className="w-5 h-5" />
              )}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

// ── Copy button ───────────────────────────────────────────────────────────
function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  const handleCopy = () => {
    navigator.clipboard.writeText(text).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  };
  return (
    <button
      onClick={handleCopy}
      className="absolute top-2 right-2 opacity-0 group-hover:opacity-100 transition-opacity p-1.5 rounded text-stone-400 hover:text-stone-600 dark:text-slate-500 dark:hover:text-slate-300 hover:bg-stone-200 dark:hover:bg-slate-700"
      title="Copy response"
    >
      {copied ? <ClipboardCheck className="w-3.5 h-3.5 text-green-500" /> : <Clipboard className="w-3.5 h-3.5" />}
    </button>
  );
}

// ── Collapsible context panel ──────────────────────────────────────────────
function ContextPanel({
  items,
  onRemove,
  onClearAll,
}: {
  items: any[];
  onRemove: (id: string) => void;
  onClearAll: () => void;
}) {
  const [open, setOpen] = useState(false);
  return (
    <div className="mb-3 border border-purple-500/30 rounded-lg overflow-hidden">
      <button
        onClick={() => setOpen((v) => !v)}
        className="w-full flex items-center justify-between px-3 py-2 bg-purple-500/10 text-sm hover:bg-purple-500/15 transition-colors"
      >
        <span className="text-purple-400 font-medium flex items-center gap-1.5">
          <Sparkles className="w-3.5 h-3.5" />
          Context
          <span className="bg-purple-500/30 text-purple-300 text-xs px-1.5 py-0.5 rounded-full">
            {items.length}
          </span>
        </span>
        <ChevronDown
          className={`w-4 h-4 text-purple-400 transition-transform duration-200 ${open ? "rotate-180" : ""}`}
        />
      </button>
      <div
        className="transition-all duration-200 overflow-hidden"
        style={{ maxHeight: open ? "200px" : "0px", opacity: open ? 1 : 0 }}
      >
        <div className="p-2 flex flex-wrap gap-2">
          {items.map((item) => (
            <div
              key={item.id}
              className="inline-flex items-center gap-1.5 px-2.5 py-1 bg-purple-500/10 border border-purple-500/30 rounded-lg text-sm"
            >
              <span className="text-purple-400 font-mono">
                {item.type}/{item.name}
              </span>
              <button
                data-testid={`context-remove-${item.id}`}
                onClick={() => onRemove(item.id)}
                className="text-purple-400 hover:text-purple-300"
              >
                <X className="w-3 h-3" />
              </button>
            </div>
          ))}
          <button
            onClick={onClearAll}
            className="text-xs text-stone-500 hover:text-stone-700 dark:text-slate-500 dark:hover:text-slate-300 self-center"
          >
            Clear all
          </button>
        </div>
      </div>
    </div>
  );
}

// ── Animated text (character-reveal for fresh AI responses) ────────────────
function AnimatedText({
  text,
  messageId,
}: {
  text: string;
  messageId: string;
}) {
  const revealedRef = useRef<Set<string>>(new Set());
  const [displayed, setDisplayed] = useState(() =>
    revealedRef.current.has(messageId) ? text : ""
  );
  const [done, setDone] = useState(() => revealedRef.current.has(messageId));

  useEffect(() => {
    if (revealedRef.current.has(messageId)) return;
    let i = 0;
    const interval = setInterval(() => {
      i += 30;
      if (i >= text.length) {
        setDisplayed(text);
        setDone(true);
        revealedRef.current.add(messageId);
        clearInterval(interval);
      } else {
        setDisplayed(text.slice(0, i));
      }
    }, 16);
    return () => clearInterval(interval);
  }, [text, messageId]);

  return (
    <div className="relative">
      <Markdown rehypePlugins={[[rehypeSanitize, markdownSanitizeSchema]]}>
        {displayed}
      </Markdown>
      {!done && (
        <button
          onClick={() => {
            setDisplayed(text);
            setDone(true);
            revealedRef.current.add(messageId);
          }}
          className="absolute top-0 right-0 text-xs text-stone-400 hover:text-stone-600 dark:text-slate-500 dark:hover:text-slate-300 px-1"
        >
          Skip
        </button>
      )}
    </div>
  );
}

// Conversation Item Component
interface ConversationItemProps {
  conversation: any;
  isActive: boolean;
  onSelect: () => void;
  onDelete: () => void;
  onTogglePin: () => void;
}

function ConversationItem({
  conversation,
  isActive,
  onSelect,
  onDelete,
  onTogglePin,
}: ConversationItemProps) {
  return (
    <div
      className={`
        group relative p-3 rounded-lg cursor-pointer transition-colors
        ${
          isActive
            ? "bg-purple-500/10 border border-purple-500/30"
            : "hover:bg-stone-100 dark:hover:bg-slate-700/50"
        }
      `}
      onClick={onSelect}
    >
      <div className="flex items-start gap-2">
        <MessageSquare className="w-4 h-4 mt-0.5 flex-shrink-0 text-stone-500 dark:text-slate-400" />
        <div className="flex-1 min-w-0">
          <p className="text-sm font-medium truncate">{conversation.title}</p>
          <p className="text-xs text-stone-500 dark:text-slate-500">
            {new Date(conversation.updatedAt).toLocaleDateString()}
          </p>
        </div>
      </div>

      {/* Actions */}
      <div className="absolute top-2 right-2 flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
        <button
          onClick={(e) => {
            e.stopPropagation();
            onTogglePin();
          }}
          className={`p-1 rounded hover:bg-stone-200 dark:hover:bg-slate-600 ${
            conversation.pinned ? "text-purple-400" : "text-stone-400"
          }`}
        >
          <Pin className="w-3 h-3" />
        </button>
        <button
          onClick={(e) => {
            e.stopPropagation();
            onDelete();
          }}
          className="p-1 rounded hover:bg-red-500/10 text-stone-400 hover:text-red-400"
        >
          <Trash2 className="w-3 h-3" />
        </button>
      </div>
    </div>
  );
}

// Message Bubble Component
interface MessageBubbleProps {
  message: any;
  onExecuteCommand: (commandId: string) => void;
  onRejectCommand: (commandId: string) => void;
}

function MessageBubble({
  message,
  onExecuteCommand,
  onRejectCommand,
}: MessageBubbleProps) {
  if (message.role === "user") {
    return (
      <div className="flex justify-end">
        <div className="max-w-[70%] bg-purple-500/10 border border-purple-500/30 rounded-lg p-4 select-text">
          <p className="text-sm whitespace-pre-wrap">{message.content}</p>
          {message.contextItems && message.contextItems.length > 0 && (
            <div className="mt-2 pt-2 border-t border-purple-500/20 flex flex-wrap gap-1">
              {message.contextItems.map((item: any) => (
                <span
                  key={item.id}
                  className="text-xs px-2 py-0.5 bg-purple-500/20 rounded font-mono"
                >
                  {item.type}/{item.name}
                </span>
              ))}
            </div>
          )}
        </div>
      </div>
    );
  }

  // Assistant message
  return (
    <div className="flex justify-start">
      <div className="max-w-[85%] space-y-3">
        {/* Loading state */}
        {message.isLoading && (
          <div className="flex items-center gap-2 text-sm text-stone-500 dark:text-slate-400">
            <div className="w-4 h-4 border-2 border-purple-400 border-t-transparent rounded-full animate-spin" />
            <span>Thinking...</span>
          </div>
        )}

        {/* Text response */}
        {message.content && !message.isLoading && (
          <div className="relative group bg-stone-50 dark:bg-slate-900/50 border border-stone-200 dark:border-slate-700 rounded-lg p-4 select-text">
            <CopyButton text={message.content} />
            <div className="text-sm prose prose-sm max-w-none dark:prose-invert">
              <AnimatedText text={message.content} messageId={message.id} />
            </div>
          </div>
        )}

        {/* Commands */}
        {message.response?.commands && message.response.commands.length > 0 && (
          <div className="space-y-2">
            <h4 className="text-xs font-medium text-stone-500 dark:text-slate-400 uppercase">
              Suggested Commands
            </h4>
            {message.response.commands.map(
              (cmd: CommandBlockLocal, index: number) => {
                const safetyInfo = getCommandSafetyIcon(cmd.command);

                // Determine if the current command should be locked
                const prevCmd =
                  index > 0 ? message.response.commands[index - 1] : null;
                const isLocked =
                  prevCmd &&
                  prevCmd.status !== "completed" &&
                  prevCmd.status !== "rejected";

                return (
                  <div
                    key={cmd.id}
                    className="bg-stone-50 dark:bg-slate-900/50 border border-stone-200 dark:border-slate-700 rounded-lg overflow-hidden"
                  >
                    <div className="p-3 flex items-start gap-3">
                      {/* Safety indicator */}
                      <span
                        className={`text-sm ${safetyInfo.color} mt-0.5`}
                        title={safetyInfo.label}
                      >
                        {safetyInfo.icon}
                      </span>
                      <code className="flex-1 text-sm text-cyan-400 font-mono whitespace-pre-wrap break-all select-text">
                        {cmd.command}
                      </code>
                      {cmd.status === "pending" && (
                        <div className="flex gap-2 flex-shrink-0">
                          <button
                            onClick={() =>
                              !isLocked && onExecuteCommand(cmd.id)
                            }
                            disabled={isLocked}
                            title={
                              isLocked
                                ? "Complete previous command first"
                                : "Run command"
                            }
                            className={`px-3 py-1.5 text-sm ${
                              isLocked
                                ? "bg-emerald-600/50 cursor-not-allowed opacity-50"
                                : "bg-emerald-600 hover:bg-emerald-700"
                            } text-white rounded-lg transition-colors flex items-center gap-1`}
                          >
                            <Check size={14} />
                            Run
                          </button>
                          <button
                            onClick={() => !isLocked && onRejectCommand(cmd.id)}
                            disabled={isLocked}
                            title={
                              isLocked
                                ? "Complete previous command first"
                                : "Skip command"
                            }
                            className={`px-3 py-1.5 text-sm ${
                              isLocked
                                ? "bg-stone-200/50 dark:bg-slate-700/50 cursor-not-allowed opacity-50"
                                : "bg-stone-200 hover:bg-stone-300 dark:bg-slate-700 dark:hover:bg-slate-600"
                            } text-stone-600 dark:text-slate-300 rounded-lg transition-colors flex items-center gap-1`}
                          >
                            <X size={14} />
                            Skip
                          </button>
                        </div>
                      )}
                      {cmd.status === "running" && (
                        <div className="flex items-center gap-2 text-sm text-stone-400 dark:text-slate-400">
                          <div className="w-4 h-4 border-2 border-cyan-400 border-t-transparent rounded-full animate-spin" />
                          Running...
                        </div>
                      )}
                      {cmd.status === "completed" && (
                        <span className="text-sm text-emerald-400 flex items-center gap-1">
                          <Check size={14} />
                          Done
                        </span>
                      )}
                      {cmd.status === "rejected" && (
                        <span className="text-sm text-stone-500 dark:text-slate-500">
                          Skipped
                        </span>
                      )}
                      {cmd.status === "error" && (
                        <span className="text-sm text-red-400 flex items-center gap-1">
                          <X size={14} />
                          Failed
                        </span>
                      )}
                    </div>
                    {(cmd.output || cmd.error) && (
                      <div className="border-t border-slate-700 p-3 bg-slate-950">
                        <pre className="text-xs text-slate-400 font-mono whitespace-pre-wrap overflow-x-auto max-h-60 overflow-y-auto">
                          {cmd.output || cmd.error}
                        </pre>
                      </div>
                    )}
                  </div>
                );
              }
            )}
          </div>
        )}
      </div>
    </div>
  );
}

// ── AgentTimeline ──────────────────────────────────────────────────────────────
// Renders the observe-think-act loop steps inline in the conversation.

interface AgentTimelineProps {
  steps: AgentStep[];
  running: boolean;
  onApprove: (sessionId: string, toolCallId: string) => void;
  onReject: (sessionId: string, toolCallId: string) => void;
}

function categoryBadge(category: string) {
  switch (category) {
    case "read":
      return (
        <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-xs font-medium bg-green-500/20 text-green-400">
          read
        </span>
      );
    case "write":
      return (
        <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-xs font-medium bg-yellow-500/20 text-yellow-400">
          write
        </span>
      );
    case "destructive":
      return (
        <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-xs font-medium bg-red-500/20 text-red-400">
          destructive
        </span>
      );
    default:
      return null;
  }
}

function AgentTimeline({ steps, running, onApprove, onReject }: AgentTimelineProps) {
  const [expandedResults, setExpandedResults] = useState<Set<number>>(new Set());

  const toggleResult = (index: number) => {
    setExpandedResults((prev) => {
      const next = new Set(prev);
      if (next.has(index)) {
        next.delete(index);
      } else {
        next.add(index);
      }
      return next;
    });
  };

  return (
    <div className="border border-blue-500/20 rounded-xl bg-blue-500/5 p-4 space-y-2">
      <div className="flex items-center gap-2 mb-3">
        <Bot className="w-4 h-4 text-blue-400" />
        <span className="text-sm font-medium text-blue-400">Agent Investigation</span>
        {running && (
          <div className="w-3 h-3 rounded-full border-2 border-blue-400 border-t-transparent animate-spin ml-auto" />
        )}
      </div>

      {steps.map((step, index) => {
        if (step.kind === "thinking") {
          return (
            <div key={index} className="flex items-center gap-2 text-sm text-stone-500 dark:text-slate-400">
              <Brain className="w-3 h-3 text-blue-400 animate-pulse flex-shrink-0" />
              <span>Investigating... (iteration {step.iteration})</span>
            </div>
          );
        }

        if (step.kind === "tool_call") {
          const params = Object.entries(step.parameters)
            .map(([k, v]) => `${k}: "${v}"`)
            .join(", ");
          return (
            <div key={index} className="flex items-start gap-2 text-sm">
              <Wrench className="w-3 h-3 text-cyan-400 mt-0.5 flex-shrink-0" />
              <div className="flex-1 min-w-0">
                <span className="font-mono text-cyan-400">{step.tool}</span>
                {params && (
                  <span className="text-stone-500 dark:text-slate-500 ml-1">
                    {"{"}
                    {params}
                    {"}"}
                  </span>
                )}
              </div>
            </div>
          );
        }

        if (step.kind === "tool_result") {
          const isExpanded = expandedResults.has(index);
          return (
            <div key={index} className="ml-5 space-y-1">
              {!step.allowed && (
                <div className="text-xs text-red-400 flex items-center gap-1">
                  <X className="w-3 h-3" />
                  {step.reason || "Blocked"}
                </div>
              )}
              {step.allowed && (
                <button
                  onClick={() => toggleResult(index)}
                  className="flex items-center gap-1 text-xs text-stone-500 dark:text-slate-400 hover:text-stone-700 dark:hover:text-slate-300 transition-colors"
                >
                  <ChevronRight
                    className={`w-3 h-3 transition-transform ${isExpanded ? "rotate-90" : ""}`}
                  />
                  Result ({step.result.length} chars)
                </button>
              )}
              {step.allowed && isExpanded && (
                <pre className="text-xs font-mono bg-slate-950 text-slate-300 p-2 rounded overflow-x-auto max-h-40 overflow-y-auto whitespace-pre-wrap">
                  {step.result.slice(0, 2000)}
                  {step.result.length > 2000 && "\n... (truncated)"}
                </pre>
              )}
            </div>
          );
        }

        if (step.kind === "approval_needed") {
          return (
            <ApprovalCard
              key={index}
              sessionId={step.sessionId}
              toolCallId={step.toolCallId}
              tool={step.tool}
              parameters={step.parameters}
              category={step.category}
              onApprove={() => onApprove(step.sessionId, step.toolCallId)}
              onReject={() => onReject(step.sessionId, step.toolCallId)}
            />
          );
        }

        if (step.kind === "complete") {
          return (
            <div key={index} className="flex items-center gap-2 text-sm text-emerald-400 pt-1 border-t border-blue-500/20">
              <Check className="w-3 h-3 flex-shrink-0" />
              <span>Investigation complete</span>
            </div>
          );
        }

        return null;
      })}
    </div>
  );
}

// ── ApprovalCard ───────────────────────────────────────────────────────────────
// Shown when the agent emits ai:agent:approval-needed for a write/destructive tool.

interface ApprovalCardProps {
  sessionId: string;
  toolCallId: string;
  tool: string;
  parameters: Record<string, string>;
  category: string;
  onApprove: () => void;
  onReject: () => void;
}

function ApprovalCard({
  tool,
  parameters,
  category,
  onApprove,
  onReject,
}: ApprovalCardProps) {
  const params = Object.entries(parameters)
    .map(([k, v]) => `${k}: ${v}`)
    .join("\n");

  return (
    <div className="border border-yellow-500/40 rounded-xl bg-yellow-500/5 p-4 space-y-3">
      <div className="flex items-center gap-2">
        <Wrench className="w-4 h-4 text-yellow-400" />
        <span className="text-sm font-medium text-yellow-400">Approval Required</span>
        {categoryBadge(category)}
      </div>

      <div>
        <p className="text-sm font-mono text-stone-200">{tool}</p>
        {params && (
          <pre className="mt-1 text-xs font-mono text-stone-400 whitespace-pre-wrap">
            {params}
          </pre>
        )}
      </div>

      <div className="flex gap-2">
        <button
          onClick={onApprove}
          className="flex items-center gap-1.5 px-3 py-1.5 text-sm bg-emerald-600 hover:bg-emerald-700 text-white rounded-lg transition-colors"
        >
          <Check className="w-3.5 h-3.5" />
          Approve
        </button>
        <button
          onClick={onReject}
          className="flex items-center gap-1.5 px-3 py-1.5 text-sm bg-stone-200 hover:bg-stone-300 dark:bg-slate-700 dark:hover:bg-slate-600 text-stone-600 dark:text-slate-300 rounded-lg transition-colors"
        >
          <X className="w-3.5 h-3.5" />
          Reject
        </button>
      </div>
    </div>
  );
}

// Re-export ApprovalCard so tests can import it independently.
export { ApprovalCard };
