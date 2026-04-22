import { defineConfig } from "vitest/config";
import tsconfigPaths from "vite-tsconfig-paths";

export default defineConfig({
  plugins: [tsconfigPaths()],
  test: {
    environment: "happy-dom",
    globals: true,
    setupFiles: ["./app/test-setup.ts"],
    include: ["app/**/*.{test,spec}.{ts,tsx}"],
    css: false,
  },
});
