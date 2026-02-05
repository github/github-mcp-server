import { useApp as useExtApp } from "@modelcontextprotocol/ext-apps/react";
import type { App } from "@modelcontextprotocol/ext-apps";
import type { CallToolResult } from "@modelcontextprotocol/sdk/types.js";
import { useState, useCallback } from "react";

interface UseMcpAppOptions {
  appName: string;
  appVersion?: string;
  onToolResult?: (result: CallToolResult) => void;
  onToolInput?: (input: Record<string, unknown>) => void;
}

interface UseMcpAppReturn {
  app: App | null;
  error: Error | null;
  toolResult: CallToolResult | null;
  toolInput: Record<string, unknown> | null;
  callTool: (name: string, args: Record<string, unknown>) => Promise<CallToolResult>;
}

export function useMcpApp({
  appName,
  appVersion = "1.0.0",
  onToolResult,
  onToolInput,
}: UseMcpAppOptions): UseMcpAppReturn {
  const [toolResult, setToolResult] = useState<CallToolResult | null>(null);
  const [toolInput, setToolInput] = useState<Record<string, unknown> | null>(null);

  const { app, error } = useExtApp({
    appInfo: { name: appName, version: appVersion },
    capabilities: {},
    onAppCreated: (app) => {
      app.ontoolresult = async (result) => {
        setToolResult(result);
        onToolResult?.(result);
      };
      app.ontoolinput = async (input) => {
        const args = (input.arguments ?? {}) as Record<string, unknown>;
        setToolInput(args);
        onToolInput?.(args);
      };
      app.onerror = console.error;
    },
  });

  const callTool = useCallback(
    async (name: string, args: Record<string, unknown>) => {
      if (!app) throw new Error("App not connected");
      return app.callServerTool({ name, arguments: args });
    },
    [app]
  );

  return { app, error, toolResult, toolInput, callTool };
}
