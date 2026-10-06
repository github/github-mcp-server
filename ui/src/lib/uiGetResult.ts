interface UIUser {
  login: string;
  avatar_url?: string;
}

interface UIIssueType {
  id: number;
  name: string;
  description?: string;
}

interface UIGetPayloads {
  labels: { labels: Array<{ id: string; name: string; color: string }> };
  assignees: { assignees: UIUser[] };
  milestones: { milestones: Array<{ number: number; title: string; description: string }> };
  issue_types: Array<UIIssueType | string | null>;
  branches: { branches: Array<{ name: string; protected?: boolean }> };
  issue_fields: { fields: Array<{
    id?: string;
    name?: string;
    data_type?: string;
    description?: string;
    options?: Array<{ id?: string; name?: string; description?: string; color?: string }>;
  }> };
  reviewers: {
    users: UIUser[];
    teams: Array<{ slug: string; name?: string; org: string }>;
  };
}

export function parseUIGetResult<M extends keyof UIGetPayloads>(
  method: M,
  text: string,
): UIGetPayloads[M] {
  let payload: unknown = JSON.parse(text);
  if (payload !== null && typeof payload === "object" && "method" in payload) {
    if (payload.method !== method || !(method in payload)) {
      throw new Error(`Invalid ui_get ${method} response envelope`);
    }
    payload = (payload as Record<string, unknown>)[method];
  }
  if (method === "issue_types") {
    if (payload !== null && typeof payload === "object" && !Array.isArray(payload)) {
      const wrapped = payload as Record<string, unknown>;
      payload = wrapped.issue_types ?? wrapped.types ?? [];
    }
    payload ??= [];
  } else if (method === "branches" && Array.isArray(payload)) {
    payload = { branches: payload };
  }
  return payload as UIGetPayloads[M];
}
