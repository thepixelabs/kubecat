import js from "@eslint/js";
import globals from "globals";
import reactHooks from "eslint-plugin-react-hooks";
import tseslint from "typescript-eslint";

export default tseslint.config(
  // Global ignores
  {
    ignores: [
      "dist/**",
      "coverage/**",
      "node_modules/**",
      "wailsjs/**",
      ".vite/**",
      "*.config.js",
      "*.config.ts",
    ],
  },

  // Base JS recommended
  js.configs.recommended,

  // TypeScript rules (non-type-checked to keep CI fast and baseline clean)
  ...tseslint.configs.recommended,

  // Language options
  {
    languageOptions: {
      globals: {
        ...globals.browser,
        ...globals.es2022,
      },
    },
  },

  // React Hooks enforcement
  {
    plugins: {
      "react-hooks": reactHooks,
    },
    rules: {
      // react-hooks/rules-of-hooks enforced; exhaustive-deps disabled until hooks
      // are audited for stale closure safety
      "react-hooks/rules-of-hooks": "error",
      "react-hooks/exhaustive-deps": "off",
    },
  },

  // Project-wide rules
  {
    rules: {
      // TypeScript — errors we can enforce today
      "@typescript-eslint/no-unused-vars": [
        "error",
        { argsIgnorePattern: "^_", varsIgnorePattern: "^_" },
      ],
      "@typescript-eslint/consistent-type-imports": [
        "error",
        { prefer: "type-imports" },
      ],

      // Permissive until codebase is fully typed
      "@typescript-eslint/no-explicit-any": "off",

      // General quality — warn disabled since --max-warnings=0 is strict
      "no-console": "off",
      eqeqeq: ["error", "always"],
      "prefer-const": "error",
    },
  },

  // Test files — relaxed rules
  {
    files: ["**/*.test.ts", "**/*.test.tsx", "**/*.spec.ts", "**/*.spec.tsx"],
    rules: {
      "@typescript-eslint/no-explicit-any": "off",
      "@typescript-eslint/no-unused-vars": "off",
      "no-console": "off",
    },
  },
);
