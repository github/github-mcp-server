import { defineConfig, Plugin } from "vite";
import react from "@vitejs/plugin-react";
import { viteSingleFile } from "vite-plugin-singlefile";
import { existsSync, renameSync, rmSync } from "fs";
import { resolve } from "path";

const app = process.env.APP;

if (!app) {
  throw new Error("APP environment variable must be set");
}

const outDir = resolve(__dirname, "../pkg/github/ui_dist");

// vite-plugin-singlefile inlines all JS/CSS into the HTML, but Vite preserves
// the input file's relative path in the output (src/apps/<app>/index.html).
// After the bundle is written, hoist that file to <outDir>/<app>.html and
// remove the now-empty nested directories. Done in closeBundle (post-write)
// because Rolldown disallows mutating the in-memory bundle in generateBundle.
function flattenOutput(): Plugin {
  return {
    name: "flatten-output",
    enforce: "post",
    closeBundle() {
      const nested = resolve(outDir, `src/apps/${app}/index.html`);
      const flat = resolve(outDir, `${app}.html`);
      if (!existsSync(nested)) {
        throw new Error(
          `flatten-output: expected built HTML at ${nested} for app "${app}" but it was not emitted`,
        );
      }
      renameSync(nested, flat);
      rmSync(resolve(outDir, "src"), { recursive: true, force: true });
    },
  };
}

export default defineConfig({
  plugins: [react(), viteSingleFile(), flattenOutput()],
  build: {
    outDir,
    emptyOutDir: false,
    rollupOptions: {
      input: resolve(__dirname, `src/apps/${app}/index.html`),
    },
  },
});
