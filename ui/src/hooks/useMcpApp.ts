import { useApp as useExtApp } from "@modelcontextprotocol/ext-apps/react";
import type {
  App,
  McpUiDisplayMode,
  McpUiHostContext,
  McpUiUpdateModelContextRequest,
} from "@modelcontextprotocol/ext-apps";
import type { CallToolResult } from "@modelcontextprotocol/sdk/types.js";
import { useState, useCallback, useEffect } from "react";

interface UseMcpAppOptions {
  appName: string;
  appVersion?: string;
  /**
   * Display modes this view supports. Per the MCP Apps 2026-01-26 spec, a
   * view MUST declare every display mode it supports during initialization.
   * Defaults to ["inline"] which is the only mode the bundled github-mcp-server
   * views currently render.
   */
  availableDisplayModes?: McpUiDisplayMode[];
  onToolResult?: (result: CallToolResult) => void;
  onToolInput?: (input: Record<string, unknown>) => void;
}

interface UseMcpAppReturn {
  app: App | null;
  error: Error | null;
  toolResult: CallToolResult | null;
  toolInput: Record<string, unknown> | null;
  hostContext: McpUiHostContext | undefined;
  callTool: (name: string, args: Record<string, unknown>) => Promise<CallToolResult>;
  /**
   * Sends `ui/update-model-context` so the agent's next turn sees the
   * supplied structured content / blocks. No-op when the app isn't connected.
   */
  setModelContext: (
    params: McpUiUpdateModelContextRequest["params"]
  ) => Promise<void>;
  /**
   * Sends `ui/open-link` so the host opens an external URL in the user's
   * browser. Falls back to `window.open` when the app isn't connected.
   */
  openLink: (url: string) => Promise<void>;
}

export function useMcpApp({
  appName,
  appVersion = "1.0.0",
  availableDisplayModes = ["inline"],
  onToolResult,
  onToolInput,
}: UseMcpAppOptions): UseMcpAppReturn {
  const [toolResult, setToolResult] = useState<CallToolResult | null>(null);
  const [toolInput, setToolInput] = useState<Record<string, unknown> | null>(null);
  const [hostContext, setHostContext] = useState<McpUiHostContext | undefined>(undefined);

  // The SDK's autoResize=true installs a ResizeObserver that emits
  // `ui/notifications/size-changed` automatically; no manual wiring needed.
  const { app, error } = useExtApp({
    appInfo: { name: appName, version: appVersion },
    capabilities: { availableDisplayModes },
    autoResize: true,
    strict: import.meta.env.DEV,
    onAppCreated: (app) => {
      app.ontoolresult = async (result) => {
        setToolResult(result);
        onToolResult?.(result);
      };
      app.ontoolinput = async (input) => {
        const args = (input.arguments ?? {}) as Record<string, unknown>;
        setToolInput(args);
        // A tool-input notification marks a new invocation, and the spec
        // guarantees it is delivered before that invocation's tool-result.
        // Drop any prior result so a completed result from a previous
        // invocation can't leak into the new render (e.g. a stale success card
        // showing over a fresh, still-deferred form). The current invocation's
        // result, if any, arrives next via ontoolresult.
        setToolResult(null);
        onToolInput?.(args);
      };
      app.onhostcontextchanged = (params) => {
        setHostContext((prev) => ({ ...(prev ?? {}), ...params }));
      };
      app.onerror = console.error;
    },
  });

  useEffect(() => {
    if (!app) return;
    const initial = app.getHostContext();
    if (initial) setHostContext(initial);
  }, [app]);

  const callTool = useCallback(
    async (name: string, args: Record<string, unknown>) => {
      if (!app) throw new Error("App not connected");
      return app.callServerTool({ name, arguments: args });
    },
    [app]
  );

  const setModelContext = useCallback<UseMcpAppReturn["setModelContext"]>(
    async (params) => {
      if (!app) return;
      await app.updateModelContext(params);
    },
    [app]
  );

  const openLink = useCallback<UseMcpAppReturn["openLink"]>(
    async (url) => {
      if (!app) {
        window.open(url, "_blank", "noopener,noreferrer");
        return;
      }
      const result = await app.openLink({ url });
      // The host may deny the request (e.g. blocked domain or user cancelled).
      // Fall back to a direct window.open so the link still works.
      if (result?.isError) {
        window.open(url, "_blank", "noopener,noreferrer");
      }
    },
    [app]
  );

  return {
    app,
    error,
    toolResult,
    toolInput,
    hostContext,
    callTool,
    setModelContext,
    openLink,
  };
}
