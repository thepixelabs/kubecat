/** @type {import('tailwindcss').Config} */
export default {
  darkMode: "class",
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  theme: {
    screens: {
      xs: "480px",
      sm: "640px",
      md: "768px",
      lg: "1024px",
      xl: "1280px",
      "2xl": "1536px",
    },
    extend: {
      colors: {
        // Custom Nexus colors - mapped to CSS variables for theming
        nexus: {
          50: "rgb(var(--bg-50) / <alpha-value>)",
          100: "rgb(var(--bg-100) / <alpha-value>)",
          200: "rgb(var(--bg-200) / <alpha-value>)",
          300: "rgb(var(--bg-300) / <alpha-value>)",
          400: "rgb(var(--bg-400) / <alpha-value>)",
          500: "rgb(var(--bg-500) / <alpha-value>)",
          600: "rgb(var(--bg-600) / <alpha-value>)",
          700: "rgb(var(--bg-700) / <alpha-value>)",
          800: "rgb(var(--bg-800) / <alpha-value>)",
          900: "rgb(var(--bg-900) / <alpha-value>)",
        },
        // Theme accent colors using CSS variables
        accent: {
          400: "rgb(var(--accent-400) / <alpha-value>)",
          500: "rgb(var(--accent-500) / <alpha-value>)",
          600: "rgb(var(--accent-600) / <alpha-value>)",
        },
        // Dynamic selection color
        selection: "rgb(var(--color-selection) / <alpha-value>)",
      },
      fontFamily: {
        sans: ["Inter", "system-ui", "sans-serif"],
        mono: ["JetBrains Mono", "Menlo", "Monaco", "monospace"],
      },
    },
  },
  plugins: [],
};
