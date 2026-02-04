import { test as base } from "@playwright/test";
import {
  mockUser,
  mockBranches,
  mockLabels,
  mockAssignees,
  mockMilestones,
  mockRepositories,
  mockCreatedPR,
  mockCreatedIssue,
  MockUser,
} from "./mcp-mocks";

/**
 * Extended test fixture that provides MCP mocking capabilities.
 *
 * The MCP ext-apps library uses PostMessageTransport which communicates via
 * window.postMessage. We intercept outgoing messages and respond with mock data.
 */

interface McpFixtures {
  /**
   * Navigate to an app with mocked MCP communication
   */
  gotoApp: (appName: "get-me" | "issue-write" | "pr-write", options?: GotoAppOptions) => Promise<void>;
}

interface MockOverrides {
  user?: MockUser;
  simulateError?: boolean;
  errorMessage?: string;
  toolInput?: Record<string, unknown>;
}

interface GotoAppOptions {
  mocks?: MockOverrides;
}

export const test = base.extend<McpFixtures>({
  gotoApp: async ({ page }, use) => {
    const gotoApp = async (
      appName: "get-me" | "issue-write" | "pr-write",
      options: GotoAppOptions = {}
    ) => {
      const mocks = options.mocks || {};

      // Prepare mock data to inject
      const mockData = {
        user: mocks.user || mockUser,
        branches: mockBranches,
        labels: mockLabels,
        assignees: mockAssignees,
        milestones: mockMilestones,
        repositories: mockRepositories,
        createdPR: mockCreatedPR,
        createdIssue: mockCreatedIssue,
        simulateError: mocks.simulateError || false,
        errorMessage: mocks.errorMessage || "Simulated error",
        toolInput: mocks.toolInput || {},
        appName,
      };

      // Inject script that intercepts postMessage before the app loads
      await page.addInitScript((data) => {
        // Store mock data globally for debugging
        (window as unknown as Record<string, unknown>).__mcpMockData = data;

        // Create mock tool result helper
        const createToolResult = (content: unknown, isError = false) => ({
          content: [{ type: "text", text: JSON.stringify(content) }],
          isError,
        });

        // Mock tool responses
        const getMockToolResponse = (toolName: string, args?: Record<string, unknown>) => {
          if (data.simulateError) {
            return createToolResult({ error: data.errorMessage }, true);
          }

          switch (toolName) {
            case "get_me":
              return createToolResult(data.user);
            case "search_repositories":
              return createToolResult({ repositories: data.repositories });
            case "list_branches":
              return createToolResult({ branches: data.branches });
            case "list_label":
              return createToolResult({ labels: data.labels });
            case "list_assignees":
              return createToolResult({ assignees: data.assignees });
            case "list_milestones":
              return createToolResult({ milestones: data.milestones });
            case "list_issue_types":
              return createToolResult({ issueTypes: [] });
            case "create_pull_request":
              return createToolResult({ ...data.createdPR, title: args?.title || data.createdPR.title });
            case "create_issue":
            case "update_issue":
              return createToolResult({ ...data.createdIssue, title: args?.title || data.createdIssue.title });
            default:
              return createToolResult({ message: `Mock response for ${toolName}` });
          }
        };

        // Handle JSON-RPC requests
        const handleRequest = (message: { id?: number | string; method?: string; params?: unknown }) => {
          const { id, method, params } = message;

          // Handle ui/initialize request
          if (method === "ui/initialize") {
            return {
              jsonrpc: "2.0",
              id,
              result: {
                protocolVersion: "2026-01-26",
                hostInfo: { name: "mock-host", version: "1.0.0" },
                hostCapabilities: {
                  serverTools: { listChanged: false },
                  logging: {},
                },
                hostContext: {
                  theme: "light",
                },
              },
            };
          }

          // Handle ping
          if (method === "ping") {
            return { jsonrpc: "2.0", id, result: {} };
          }

          // Handle ui/notifications/initialized (this is a notification, no response needed)
          if (method === "ui/notifications/initialized") {
            return null;
          }

          // Handle tools/call
          if (method === "tools/call") {
            const toolParams = params as { name?: string; arguments?: Record<string, unknown> };
            const response = getMockToolResponse(toolParams.name || "", toolParams.arguments);
            return {
              jsonrpc: "2.0",
              id,
              result: response,
            };
          }

          // Default response for unknown methods
          console.warn("[MCP Mock] Unknown method:", method);
          return {
            jsonrpc: "2.0",
            id,
            error: { code: -32601, message: `Method not found: ${method}` },
          };
        };

        // Store the real window.parent
        const realParent = window.parent;

        // Create a proxy for window.parent.postMessage
        const originalPostMessage = realParent.postMessage.bind(realParent);

        // We need to mock the parent's postMessage
        // The transport sends to window.parent and listens on window for responses
        // We intercept the send and dispatch a response back to window

        // Create our mock postMessage function
        const mockPostMessage = function(message: unknown, targetOrigin: string) {
          const msg = message as { jsonrpc?: string; id?: number | string; method?: string; params?: unknown };

          // Only intercept JSON-RPC messages
          if (msg.jsonrpc === "2.0" && msg.method) {
            console.debug("[MCP Mock] Received request:", msg.method, msg);

            // Handle the request
            const response = handleRequest(msg);

            if (response) {
              console.debug("[MCP Mock] Sending response:", response);

              // Dispatch response back to the app's window
              // The transport listens on `window` for message events
              setTimeout(() => {
                // Create a MessageEvent that the transport will accept
                // The source check is against window.parent, so we need to trick it
                const event = new MessageEvent("message", {
                  data: response,
                  origin: window.location.origin,
                });
                // Override the source getter to return realParent
                Object.defineProperty(event, "source", {
                  get: () => realParent,
                });
                window.dispatchEvent(event);

                // After initialization, send tool input if provided
                if (msg.method === "ui/initialize" && Object.keys(data.toolInput).length > 0) {
                  setTimeout(() => {
                    const inputEvent = new MessageEvent("message", {
                      data: {
                        jsonrpc: "2.0",
                        method: "ui/notifications/tool-input",
                        params: { arguments: data.toolInput },
                      },
                      origin: window.location.origin,
                    });
                    Object.defineProperty(inputEvent, "source", {
                      get: () => realParent,
                    });
                    window.dispatchEvent(inputEvent);
                  }, 50);
                }

                // For get-me app, send tool result after initialization
                if (msg.method === "ui/initialize" && data.appName === "get-me") {
                  setTimeout(() => {
                    const resultEvent = new MessageEvent("message", {
                      data: {
                        jsonrpc: "2.0",
                        method: "ui/notifications/tool-result",
                        params: getMockToolResponse("get_me"),
                      },
                      origin: window.location.origin,
                    });
                    Object.defineProperty(resultEvent, "source", {
                      get: () => realParent,
                    });
                    window.dispatchEvent(resultEvent);
                  }, 100);
                }
              }, 10);
              return;
            }
          }

          // Forward non-MCP messages (shouldn't happen in practice)
          originalPostMessage(message, targetOrigin);
        };

        // Replace postMessage on the real parent
        realParent.postMessage = mockPostMessage as typeof realParent.postMessage;
      }, mockData);

      // Navigate to the app
      await page.goto(`/${appName}/index.html`);

      // Wait for React to render
      await page.waitForLoadState("domcontentloaded");

      // Give React time to render with mock data
      await page.waitForTimeout(500);
    };

    await use(gotoApp);
  },
});

export { expect } from "@playwright/test";
