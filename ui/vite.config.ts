import { defineConfig, Plugin } from "vite";
import react from "@vitejs/plugin-react";
import { viteSingleFile } from "vite-plugin-singlefile";
import { resolve } from "path";

// Get the app to build from environment variable
const app = process.env.APP;

if (!app) {
  throw new Error("APP environment variable must be set");
}

// Plugin to rename the output file and remove the nested directory structure
function renameOutput(): Plugin {
  return {
    name: "rename-output",
    enforce: "post",
    generateBundle(_, bundle) {
      // Find the HTML file and rename it
      for (const fileName of Object.keys(bundle)) {
        if (fileName.endsWith("index.html")) {
          const chunk = bundle[fileName];
          chunk.fileName = `${app}.html`;
          delete bundle[fileName];
          bundle[`${app}.html`] = chunk;
          break;
        }
      }
    },
  };
}

export default defineConfig({
  plugins: [react(), viteSingleFile(), renameOutput()],
  build: {
    outDir: resolve(__dirname, "../pkg/github/ui_dist"),
    emptyOutDir: false,
    rollupOptions: {
      input: resolve(__dirname, `src/apps/${app}/index.html`),
    },
  },
});
