import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { viteSingleFile } from "vite-plugin-singlefile";
import { resolve } from "path";

// Get the app to build from environment variable
const app = process.env.APP;

if (!app) {
  throw new Error("APP environment variable must be set (get-me or create-issue)");
}

export default defineConfig({
  plugins: [react(), viteSingleFile()],
  build: {
    outDir: "dist",
    emptyOutDir: false,
    rollupOptions: {
      input: resolve(__dirname, `src/apps/${app}/index.html`),
    },
  },
});
