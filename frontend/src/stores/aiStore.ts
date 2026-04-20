import { create } from "zustand";
import { persist } from "zustand/middleware";

// ============================================================================
// Types & Interfaces
// ============================================================================

export type ResourceType =
  | "pod"
  | "deployment"
  | "service"
  | "configmap"
  | "secret"
  | "ingress"
  | "statefulset"
  | "daemonset"
  | "job"
  | "cronjob"
  | "node"
  | "pvc"
  | "pv"
  | "namespace"
  | "event";

export interface AIContextItem {
  id: string; // unique identifier for the item
  type: ResourceType;
  namespace?: string;
  name: string;
  cluster: string;
  addedAt: Date;
}

export interface CommandBlock {
  id: string;
  command: string;
  status: "pending" | "running" | "completed" | "rejected" | "error";
  output?: string;
  error?: string;
  executedAt?: Date;
}

export interface InsightBlock {
  type: "info" | "warning" | "error" | "success";
  title: string;
  content: string;
  details?: string[];
}

export interface VisualizationBlock {
  type: "table" | "timeline" | "chart" | "list";
  title: string;
  data: any; // Will be typed more specifically per visualization type
}

export interface StructuredResponse {
  insights: InsightBlock[];
  commands: CommandBlock[];
  visualizations: VisualizationBlock[];
  followUpSuggestions: string[];
  rawText?: string; // Fallback for unstructured responses
}

export interface Message {
  id: string;
  role: "user" | "assistant";
  content: string;
  timestamp: Date;
  contextItems?: AIContextItem[]; // Resources attached to this query
  response?: StructuredResponse; // Structured response from AI
  isLoading?: boolean;
}

export interface Conversation {
  id: string;
  title: string; // Auto-generated from first query or user-renamed
  messages: Message[];
  createdAt: Date;
  updatedAt: Date;
  cluster: string;
  pinned?: boolean;
}

// ============================================================================
// Store Interface
// ============================================================================

interface AIState {
  // Context Queue
  contextQueue: AIContextItem[];
  addToContext: (item: AIContextItem) => void;
  removeFromContext: (itemId: string) => void;
  clearContext: () => void;

  // Conversations
  conversations: Conversation[];
  activeConversationId: string | null;
  createConversation: (cluster: string) => string; // Returns new conversation ID
  deleteConversation: (conversationId: string) => void;
  setActiveConversation: (conversationId: string | null) => void;
  getActiveConversation: () => Conversation | null;
  pinConversation: (conversationId: string) => void;
  unpinConversation: (conversationId: string) => void;

  // Messages
  addMessage: (
    conversationId: string,
    message: Omit<Message, "id" | "timestamp">
  ) => string;
  updateMessage: (
    conversationId: string,
    messageId: string,
    updates: Partial<Message>
  ) => void;

  // Command execution
  updateCommandStatus: (
    conversationId: string,
    messageId: string,
    commandId: string,
    status: CommandBlock["status"],
    output?: string,
    error?: string
  ) => void;

  // Settings
  autopilotEnabled: boolean;
  setAutopilotEnabled: (enabled: boolean) => void;

  // Model selection
  selectedModel: string | null;
  setSelectedModel: (model: string) => void;
}

// ============================================================================
// Store Implementation
// ============================================================================

export const useAIStore = create<AIState>()(
  persist(
    (set, get) => ({
      // ========================================================================
      // Context Queue
      // ========================================================================
      contextQueue: [],

      addToContext: (item: AIContextItem) => {
        set((state) => {
          // Prevent duplicates
          const exists = state.contextQueue.some(
            (existing) =>
              existing.type === item.type &&
              existing.name === item.name &&
              existing.namespace === item.namespace &&
              existing.cluster === item.cluster
          );

          if (exists) {
            return state;
          }

          return {
            contextQueue: [
              ...state.contextQueue,
              { ...item, addedAt: new Date() },
            ],
          };
        });
      },

      removeFromContext: (itemId: string) => {
        set((state) => ({
          contextQueue: state.contextQueue.filter((item) => item.id !== itemId),
        }));
      },

      clearContext: () => {
        set({ contextQueue: [] });
      },

      // ========================================================================
      // Conversations
      // ========================================================================
      conversations: [],
      activeConversationId: null,

      createConversation: (cluster: string) => {
        const newConversation: Conversation = {
          id: `conv_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`,
          title: "New Conversation",
          messages: [],
          createdAt: new Date(),
          updatedAt: new Date(),
          cluster,
          pinned: false,
        };

        set((state) => ({
          conversations: [newConversation, ...state.conversations],
          activeConversationId: newConversation.id,
        }));

        return newConversation.id;
      },

      deleteConversation: (conversationId: string) => {
        set((state) => ({
          conversations: state.conversations.filter(
            (c) => c.id !== conversationId
          ),
          activeConversationId:
            state.activeConversationId === conversationId
              ? null
              : state.activeConversationId,
        }));
      },

      setActiveConversation: (conversationId: string | null) => {
        set({ activeConversationId: conversationId });
      },

      getActiveConversation: () => {
        const state = get();
        return (
          state.conversations.find(
            (c) => c.id === state.activeConversationId
          ) || null
        );
      },

      pinConversation: (conversationId: string) => {
        set((state) => ({
          conversations: state.conversations.map((c) =>
            c.id === conversationId ? { ...c, pinned: true } : c
          ),
        }));
      },

      unpinConversation: (conversationId: string) => {
        set((state) => ({
          conversations: state.conversations.map((c) =>
            c.id === conversationId ? { ...c, pinned: false } : c
          ),
        }));
      },

      // ========================================================================
      // Messages
      // ========================================================================
      addMessage: (
        conversationId: string,
        message: Omit<Message, "id" | "timestamp">
      ) => {
        const newMessage: Message = {
          ...message,
          id: `msg_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`,
          timestamp: new Date(),
        };

        set((state) => ({
          conversations: state.conversations.map((conv) => {
            if (conv.id !== conversationId) return conv;

            const updatedConv = {
              ...conv,
              messages: [...conv.messages, newMessage],
              updatedAt: new Date(),
            };

            // Auto-generate title from first user message
            if (updatedConv.messages.length === 1 && message.role === "user") {
              updatedConv.title =
                message.content.slice(0, 50) +
                (message.content.length > 50 ? "..." : "");
            }

            return updatedConv;
          }),
        }));

        return newMessage.id;
      },

      updateMessage: (
        conversationId: string,
        messageId: string,
        updates: Partial<Message>
      ) => {
        set((state) => ({
          conversations: state.conversations.map((conv) => {
            if (conv.id !== conversationId) return conv;

            return {
              ...conv,
              messages: conv.messages.map((msg) =>
                msg.id === messageId ? { ...msg, ...updates } : msg
              ),
              updatedAt: new Date(),
            };
          }),
        }));
      },

      updateCommandStatus: (
        conversationId: string,
        messageId: string,
        commandId: string,
        status: CommandBlock["status"],
        output?: string,
        error?: string
      ) => {
        set((state) => ({
          conversations: state.conversations.map((conv) => {
            if (conv.id !== conversationId) return conv;

            return {
              ...conv,
              messages: conv.messages.map((msg) => {
                if (msg.id !== messageId || !msg.response) return msg;

                return {
                  ...msg,
                  response: {
                    ...msg.response,
                    commands: msg.response.commands.map((cmd) =>
                      cmd.id === commandId
                        ? {
                            ...cmd,
                            status,
                            output,
                            error,
                            executedAt: new Date(),
                          }
                        : cmd
                    ),
                  },
                };
              }),
              updatedAt: new Date(),
            };
          }),
        }));
      },

      // ========================================================================
      // Settings
      // ========================================================================
      autopilotEnabled: false,

      setAutopilotEnabled: (enabled: boolean) => {
        set({ autopilotEnabled: enabled });
      },

      // ========================================================================
      // Model Selection
      // ========================================================================
      selectedModel: null,

      setSelectedModel: (model: string) => {
        set({ selectedModel: model });
      },
    }),
    {
      name: "nexus-ai-storage",
      // Only persist conversations and settings, not context queue
      partialize: (state) => ({
        conversations: state.conversations,
        autopilotEnabled: state.autopilotEnabled,
        selectedModel: state.selectedModel,
      }),
    }
  )
);
