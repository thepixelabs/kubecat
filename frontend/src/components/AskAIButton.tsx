import { Sparkles } from "lucide-react";
import { useAIStore } from "../stores/aiStore";
import { useToastStore } from "../stores/toastStore";
import { GetAISettings } from "../../wailsjs/go/main/App";
import type { AIContextItem } from "../stores/aiStore";

interface AskAIButtonProps {
  resource: {
    type: string;
    namespace?: string;
    name: string;
    cluster: string;
  };
  size?: "sm" | "md";
  variant?: "icon" | "button";
  onAdded?: () => void;
}

export function AskAIButton({
  resource,
  size = "sm",
  variant = "icon",
  onAdded,
}: AskAIButtonProps) {
  const addToContext = useAIStore((state) => state.addToContext);
  const contextQueue = useAIStore((state) => state.contextQueue);
  const addToast = useToastStore((state) => state.addToast);

  // Check if already in queue
  const isInQueue = contextQueue.some(
    (item) =>
      item.type === resource.type &&
      item.name === resource.name &&
      item.namespace === resource.namespace &&
      item.cluster === resource.cluster
  );

  const handleClick = async (e: React.MouseEvent) => {
    e.stopPropagation(); // Prevent triggering parent click events

    if (isInQueue) {
      return; // Already added
    }

    // Check if AI is enabled
    try {
      const aiSettings = await GetAISettings();
      if (!aiSettings.enabled) {
        addToast({
          type: "warning",
          message:
            "AI features are not enabled. Please enable them in Settings.",
          duration: 4000,
        });
        return;
      }

      if (!aiSettings.selectedProvider) {
        addToast({
          type: "warning",
          message: "AI provider not selected. Please configure AI in Settings.",
          duration: 4000,
        });
        return;
      }

      const providerConfig =
        aiSettings.providers?.[aiSettings.selectedProvider];
      if (!providerConfig || !providerConfig.apiKey) {
        addToast({
          type: "warning",
          message:
            "AI provider not configured. Please configure AI in Settings.",
          duration: 4000,
        });
        return;
      }
    } catch {
      addToast({
        type: "error",
        message: "Failed to check AI settings",
        duration: 3000,
      });
      return;
    }

    const contextItem: AIContextItem = {
      id: `${resource.cluster}_${resource.type}_${
        resource.namespace || "default"
      }_${resource.name}_${Date.now()}`,
      type: resource.type as any,
      namespace: resource.namespace,
      name: resource.name,
      cluster: resource.cluster,
      addedAt: new Date(),
    };

    addToContext(contextItem);

    // Show success toast
    addToast({
      type: "success",
      message: `Added ${resource.type}/${resource.name} to AI context`,
      duration: 2000,
    });

    onAdded?.();
  };

  if (variant === "button") {
    return (
      <button
        onClick={handleClick}
        disabled={isInQueue}
        className={`
          inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md
          ${
            isInQueue
              ? "bg-purple-500/20 text-purple-400 cursor-not-allowed"
              : "bg-purple-500/10 text-purple-400 hover:bg-purple-500/20"
          }
          transition-colors text-sm font-medium
        `}
        title={
          isInQueue ? "Already in AI context" : "Ask AI about this resource"
        }
      >
        <Sparkles className={size === "sm" ? "w-3.5 h-3.5" : "w-4 h-4"} />
        <span>{isInQueue ? "In Context" : "Ask AI"}</span>
      </button>
    );
  }

  // Icon variant (default)
  return (
    <button
      onClick={handleClick}
      disabled={isInQueue || !resource.cluster}
      className={`
        inline-flex items-center justify-center rounded-md
        ${size === "sm" ? "w-6 h-6" : "w-8 h-8"}
        ${
          isInQueue
            ? "text-purple-400 bg-purple-500/20 cursor-not-allowed"
            : !resource.cluster
            ? "text-gray-600 dark:text-slate-600 cursor-not-allowed opacity-50"
            : "text-purple-400 hover:text-purple-300 hover:bg-purple-500/20 bg-purple-500/10"
        }
        transition-colors
      `}
      title={
        !resource.cluster
          ? "Connect to a cluster first"
          : isInQueue
          ? "Already in AI context"
          : "Ask AI about this resource"
      }
    >
      <Sparkles className={size === "sm" ? "w-3.5 h-3.5" : "w-4 h-4"} />
    </button>
  );
}
