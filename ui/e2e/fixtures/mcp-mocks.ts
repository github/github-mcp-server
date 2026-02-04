/**
 * Mock MCP responses for E2E testing.
 * These mocks simulate responses from the MCP server.
 */

export interface MockUser {
  login: string;
  avatar_url?: string;
  details?: {
    name?: string;
    company?: string;
    location?: string;
    blog?: string;
    email?: string;
    public_repos?: number;
    followers?: number;
    following?: number;
  };
}

export interface MockRepository {
  id: number;
  owner: { login: string };
  name: string;
  full_name: string;
  private: boolean;
}

export interface MockBranch {
  name: string;
  protected: boolean;
}

export interface MockLabel {
  id: string;
  name: string;
  color: string;
}

export interface MockPullRequest {
  id: number;
  number: number;
  title: string;
  html_url: string;
}

export interface MockIssue {
  id: number;
  number: number;
  title: string;
  html_url: string;
}

// Sample mock data

export const mockUser: MockUser = {
  login: "octocat",
  avatar_url: "https://avatars.githubusercontent.com/u/583231?v=4",
  details: {
    name: "The Octocat",
    company: "@github",
    location: "San Francisco",
    blog: "https://github.blog",
    email: "octocat@github.com",
    public_repos: 8,
    followers: 1000,
    following: 9,
  },
};

export const mockRepositories: MockRepository[] = [
  {
    id: 1,
    owner: { login: "octocat" },
    name: "hello-world",
    full_name: "octocat/hello-world",
    private: false,
  },
  {
    id: 2,
    owner: { login: "octocat" },
    name: "private-repo",
    full_name: "octocat/private-repo",
    private: true,
  },
];

export const mockBranches: MockBranch[] = [
  { name: "main", protected: true },
  { name: "develop", protected: false },
  { name: "feature/test", protected: false },
];

export const mockLabels: MockLabel[] = [
  { id: "1", name: "bug", color: "d73a4a" },
  { id: "2", name: "enhancement", color: "a2eeef" },
  { id: "3", name: "documentation", color: "0075ca" },
];

export const mockAssignees = [
  { login: "octocat" },
  { login: "hubot" },
];

export const mockMilestones = [
  { number: 1, title: "v1.0", description: "First release" },
  { number: 2, title: "v2.0", description: "Second release" },
];

export const mockCreatedPR: MockPullRequest = {
  id: 1,
  number: 42,
  title: "Test PR",
  html_url: "https://github.com/octocat/hello-world/pull/42",
};

export const mockCreatedIssue: MockIssue = {
  id: 1,
  number: 123,
  title: "Test Issue",
  html_url: "https://github.com/octocat/hello-world/issues/123",
};

/**
 * Create a mock MCP tool result response
 */
export function createToolResult(data: unknown, isError = false) {
  return {
    content: [{ type: "text", text: JSON.stringify(data) }],
    isError,
  };
}

/**
 * Map of tool names to their mock responses
 */
export function getMockResponse(toolName: string, args?: Record<string, unknown>): unknown {
  switch (toolName) {
    case "get_me":
      return createToolResult(mockUser);

    case "search_repositories":
      return createToolResult({ repositories: mockRepositories });

    case "list_branches":
      return createToolResult({ branches: mockBranches });

    case "list_label":
      return createToolResult({ labels: mockLabels });

    case "list_assignees":
      return createToolResult({ assignees: mockAssignees });

    case "list_milestones":
      return createToolResult({ milestones: mockMilestones });

    case "create_pull_request":
      return createToolResult({
        ...mockCreatedPR,
        title: args?.title || mockCreatedPR.title,
      });

    case "create_issue":
      return createToolResult({
        ...mockCreatedIssue,
        title: args?.title || mockCreatedIssue.title,
      });

    default:
      return createToolResult({ error: `Unknown tool: ${toolName}` }, true);
  }
}
