import { useState, useEffect } from "react";
import { X, Check, Settings2 } from "lucide-react";

interface ModelConfigModalProps {
  isOpen: boolean;
  onClose: () => void;
  availableModels: string[];
  enabledModels: string[];
  onSave: (enabledModels: string[]) => void;
}

export function ModelConfigModal({
  isOpen,
  onClose,
  availableModels,
  enabledModels,
  onSave,
}: ModelConfigModalProps) {
  const [localEnabledModels, setLocalEnabledModels] =
    useState<string[]>(enabledModels);

  useEffect(() => {
    setLocalEnabledModels(enabledModels);
  }, [enabledModels, isOpen]);

  if (!isOpen) return null;

  const handleToggle = (model: string) => {
    setLocalEnabledModels((prev) =>
      prev.includes(model)
        ? prev.filter((m) => m !== model)
        : [...prev, model]
    );
  };

  const handleSelectAll = () => {
    setLocalEnabledModels([...availableModels]);
  };

  const handleDeselectAll = () => {
    setLocalEnabledModels([]);
  };

  const handleSave = () => {
    onSave(localEnabledModels);
    onClose();
  };

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-gray-800 rounded-lg shadow-xl w-full max-w-md border border-gray-700">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-700">
          <div className="flex items-center gap-2">
            <Settings2 className="w-5 h-5 text-blue-400" />
            <h2 className="text-lg font-semibold text-gray-100">
              Configure Models
            </h2>
          </div>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-gray-200 transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Content */}
        <div className="p-6">
          <p className="text-sm text-gray-400 mb-4">
            Select which models should appear in the model dropdown. You need at
            least one model enabled.
          </p>

          {/* Quick Actions */}
          <div className="flex gap-2 mb-4">
            <button
              onClick={handleSelectAll}
              className="px-3 py-1.5 text-xs bg-gray-700 text-gray-200 rounded hover:bg-gray-600 transition-colors"
            >
              Select All
            </button>
            <button
              onClick={handleDeselectAll}
              className="px-3 py-1.5 text-xs bg-gray-700 text-gray-200 rounded hover:bg-gray-600 transition-colors"
            >
              Deselect All
            </button>
          </div>

          {/* Model List */}
          <div className="space-y-2 max-h-96 overflow-y-auto">
            {availableModels.length === 0 ? (
              <div className="text-center py-8 text-gray-500">
                No models available. Please configure an AI provider in Settings.
              </div>
            ) : (
              availableModels.map((model) => (
                <label
                  key={model}
                  className="flex items-center gap-3 p-3 bg-gray-750 rounded-lg hover:bg-gray-700 cursor-pointer transition-colors"
                >
                  <div className="relative flex items-center">
                    <input
                      type="checkbox"
                      checked={localEnabledModels.includes(model)}
                      onChange={() => handleToggle(model)}
                      className="w-4 h-4 rounded border-gray-600 bg-gray-800 text-blue-500 focus:ring-2 focus:ring-blue-500 focus:ring-offset-0 cursor-pointer"
                    />
                    {localEnabledModels.includes(model) && (
                      <Check className="w-3 h-3 text-white absolute left-0.5 top-0.5 pointer-events-none" />
                    )}
                  </div>
                  <span className="text-sm text-gray-200 font-medium">
                    {model}
                  </span>
                </label>
              ))
            )}
          </div>

          {/* Warning if none selected */}
          {localEnabledModels.length === 0 && availableModels.length > 0 && (
            <div className="mt-4 p-3 bg-yellow-900/20 border border-yellow-700/50 rounded-lg">
              <p className="text-xs text-yellow-400">
                ⚠️ You must enable at least one model
              </p>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-end gap-3 px-6 py-4 border-t border-gray-700">
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm text-gray-300 hover:text-gray-100 transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={handleSave}
            disabled={localEnabledModels.length === 0}
            className="px-4 py-2 text-sm bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            Save Changes
          </button>
        </div>
      </div>
    </div>
  );
}
