/// <reference types="vitest" />
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.{test,spec}.{js,mjs,cjs,ts,mts,cts,jsx,tsx}'],
    exclude: ['node_modules', 'dist', 'wailsjs'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json', 'html'],
      // Only report on files actually executed during tests (avoids source-map
      // crash in @ampproject/remapping when scanning untested Wails-generated files)
      all: false,
      exclude: [
        'node_modules/',
        'src/test/',
        'wailsjs/',
        '.vite/',
        '**/*.d.ts',
        '**/*.config.*',
      ],
    },
    // Ensure async operations complete
    testTimeout: 10000,
    hookTimeout: 10000,
  },
});

