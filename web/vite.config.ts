import path from "node:path";

import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

export default defineConfig({
  base: "/ui/",
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(import.meta.dirname, "./src"),
    },
  },
  test: {
    environment: "jsdom",
    // The e2e/*.spec.ts files are Playwright suites, not Vitest: they need a
    // real browser and the preview server (see playwright.config.ts).
    exclude: ["**/node_modules/**", "**/dist/**", "e2e/**"],
    environmentOptions: {
      jsdom: {
        url: "http://localhost/",
      },
    },
    setupFiles: ["./src/test/setup.ts"],
    coverage: {
      enabled: true,
      include: [
        "src/App.tsx",
        "src/api.ts",
        "src/lib/render-rich-content.ts",
        "src/lib/runtime-loader.ts",
        "src/model.ts",
        "src/use-preview.ts",
        "src/use-preview-events.ts",
        "src/use-toc-spy.ts",
        "src/components/document-tree.tsx",
        "src/components/table-of-contents.tsx",
      ],
      provider: "v8",
      reporter: ["text", "json-summary"],
      thresholds: {
        branches: 70,
        functions: 70,
        lines: 70,
        statements: 70,
      },
    },
  },
});
