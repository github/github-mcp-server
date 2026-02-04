import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { viteSingleFile } from "vite-plugin-singlefile";
import { resolve } from "path";

// Get the app to build from environment variable
const app = process.env.APP;

// In dev mode (no APP specified), serve all apps
const isDev = !app;

export default defineConfig({
  plugins: isDev ? [react()] : [react(), viteSingleFile()],
  root: isDev ? resolve(__dirname, "src/apps") : undefined,
  build: isDev
    ? {}
    : {
        outDir: "dist",
        emptyOutDir: false,
        rollupOptions: {
          input: resolve(__dirname, `src/apps/${app}/index.html`),
        },
      },
});
