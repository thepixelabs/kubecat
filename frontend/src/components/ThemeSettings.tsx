import { useTheme } from "next-themes";
import {
  Check,
  Moon,
  Sun,
  Monitor,
  Palette,
  MousePointer2,
} from "lucide-react";
import { useEffect, useState } from "react";

export type ColorTheme =
  | "rain"
  | "purple"
  | "green"
  | "orange"
  | "rose"
  | "amber"
  | "nord"
  | "catppuccin";

export const THEMES: { id: ColorTheme; name: string; color: string }[] = [
  { id: "rain", name: "Rain", color: "bg-cyan-500" },
  { id: "purple", name: "Purple", color: "bg-purple-500" },
  { id: "green", name: "Green", color: "bg-green-500" },
  { id: "orange", name: "Orange", color: "bg-orange-500" },
  { id: "rose", name: "Rose", color: "bg-rose-500" },
  { id: "amber", name: "Amber", color: "bg-amber-500" },
  { id: "nord", name: "Nord", color: "bg-[#88c0d0]" },
  { id: "catppuccin", name: "Catppuccin", color: "bg-[#cba6f7]" },
];

interface ThemeSettingsProps {
  colorTheme: string;
  setColorTheme: (theme: ColorTheme) => void;
  selectionColor: string;
  setSelectionColor: (color: string) => void;
}

export function ThemeSettings({
  colorTheme,
  setColorTheme,
  selectionColor,
  setSelectionColor,
}: ThemeSettingsProps) {
  const { theme, setTheme } = useTheme();
  const [mounted, setMounted] = useState(false);

  // Avoid hydration mismatch by waiting for mount
  useEffect(() => {
    setMounted(true);
  }, []);

  if (!mounted) return null;

  return (
    <div className="space-y-8">
      {/* Mode Section */}
      <div className="space-y-4">
        <h3 className="text-sm font-medium text-slate-400 dark:text-slate-400 flex items-center gap-2">
          <Monitor size={16} />
          Appearance Mode
        </h3>
        <div className="grid grid-cols-3 gap-3 bg-slate-100 dark:bg-slate-900/50 p-1.5 rounded-xl border border-slate-200 dark:border-slate-700/50">
          {[
            { id: "light", label: "Light", icon: Sun },
            { id: "dark", label: "Dark", icon: Moon },
            { id: "system", label: "System", icon: Monitor },
          ].map((mode) => {
            const Icon = mode.icon;
            const isActive = theme === mode.id;
            return (
              <button
                key={mode.id}
                onClick={() => setTheme(mode.id)}
                className={`
                  flex items-center justify-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-all
                  ${
                    isActive
                      ? "bg-white dark:bg-slate-800 text-slate-900 dark:text-slate-100 shadow-sm ring-1 ring-slate-200 dark:ring-slate-700"
                      : "text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-300 hover:bg-slate-200/50 dark:hover:bg-slate-800/50"
                  }
                `}
              >
                <Icon size={16} />
                {mode.label}
              </button>
            );
          })}
        </div>
      </div>

      {/* Color Theme Section */}
      <div className="space-y-4">
        <h3 className="text-sm font-medium text-slate-400 dark:text-slate-400 flex items-center gap-2">
          <Palette size={16} />
          Color Theme
        </h3>
        <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
          {THEMES.map((t) => {
            const isActive = colorTheme === t.id;
            return (
              <button
                key={t.id}
                onClick={() => setColorTheme(t.id)}
                className={`
                  relative group flex items-center gap-3 px-3 py-3 rounded-xl border text-sm transition-all text-left
                  ${
                    isActive
                      ? "bg-accent-500/10 border-accent-500/50 text-slate-900 dark:text-slate-100 ring-1 ring-accent-500/20"
                      : "bg-white dark:bg-slate-900/50 border-slate-200 dark:border-slate-700 text-slate-500 dark:text-slate-400 hover:border-accent-500/30 hover:bg-slate-50 dark:hover:bg-slate-800"
                  }
                `}
              >
                <div
                  className={`w-4 h-4 rounded-full ${t.color} shadow-sm group-hover:scale-110 transition-transform`}
                />
                <span className="flex-1 font-medium">{t.name}</span>
                {isActive && (
                  <div className="absolute top-0 right-0 p-1.5">
                    <div className="bg-accent-500 text-white rounded-full p-0.5">
                      <Check size={10} strokeWidth={3} />
                    </div>
                  </div>
                )}
              </button>
            );
          })}
        </div>
      </div>

      {/* Selection Color Section */}
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-medium text-slate-400 dark:text-slate-400 flex items-center gap-2">
            <MousePointer2 size={16} />
            Selection Color
          </h3>
          <button
            onClick={() => setSelectionColor("")}
            className="text-xs text-slate-500 hover:text-accent-500 transition-colors"
            title="Reset to theme default"
          >
            Reset to Default
          </button>
        </div>

        <div className="flex items-center gap-4 bg-white dark:bg-slate-900/50 p-4 rounded-xl border border-slate-200 dark:border-slate-700/50">
          <div className="relative group">
            <div
              className="w-10 h-10 rounded-lg border-2 border-slate-200 dark:border-slate-600 shadow-sm overflow-hidden cursor-pointer"
              style={{ backgroundColor: selectionColor || "var(--accent-500)" }}
            >
              <input
                type="color"
                value={selectionColor || "#06b6d4"} // Fallback to cyan if empty
                onChange={(e) => setSelectionColor(e.target.value)}
                className="opacity-0 w-full h-full cursor-pointer absolute inset-0"
              />
            </div>
            <div className="absolute -bottom-8 left-1/2 -translate-x-1/2 bg-slate-800 text-white text-xs px-2 py-1 rounded opacity-0 group-hover:opacity-100 transition-opacity whitespace-nowrap pointer-events-none z-10">
              Pick Color
            </div>
          </div>

          <div className="flex-1 space-y-1">
            <div className="text-sm font-medium text-slate-700 dark:text-slate-200">
              Preview Selection
            </div>
            <p className="text-xs text-slate-500 leading-relaxed dark:text-slate-400">
              Try selecting this text to see your custom color in action.
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
