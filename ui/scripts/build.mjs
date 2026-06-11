// Build all UI apps in a single Node process.
//
// Replaces serial `cross-env APP=<app> vite build` invocations: doing it
// in one process avoids paying Vite/plugin startup cost for each app and is
// portable without `cross-env`.

import { build } from "vite";

const apps = ["get-me", "issue-write", "pr-write", "pr-edit"];

for (const app of apps) {
  process.env.APP = app;
  await build();
}
