import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";
import { fileURLToPath } from "node:url";

export default defineConfig({
  plugins: [react({ jsxRuntime: "automatic" })],
  esbuild: {
    jsx: "automatic",
    tsconfigRaw: {
      compilerOptions: {
        jsx: "react-jsx",
      },
    },
  },
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./apps/web/src", import.meta.url)),
    },
  },
  test: {
    environment: "node",
    tsconfig: "./vitest.tsconfig.json",
    environmentMatchGlobs: [
      ["apps/web/src/modules/filesystem/**/*.test.{ts,tsx}", "jsdom"],
    ],
  },
});
