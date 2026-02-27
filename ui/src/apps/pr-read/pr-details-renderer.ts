interface PRDetails {
  number: number;
  title: string;
  body?: string;
  state: string;
  draft: boolean;
  merged: boolean;
  mergeable_state?: string;
  html_url: string;
  user?: { login: string; html_url?: string };
  labels?: string[];
  assignees?: string[];
  requested_reviewers?: string[];
  merged_by?: string;
  head?: { ref: string; sha: string; repo?: { full_name: string } };
  base?: { ref: string; sha: string; repo?: { full_name: string } };
  additions?: number;
  deletions?: number;
  changed_files?: number;
  commits?: number;
  comments?: number;
  created_at?: string;
  updated_at?: string;
  closed_at?: string;
  merged_at?: string;
  milestone?: string;
}

export function renderPRDetails(data: PRDetails): void {
  const container = document.getElementById("content-area");
  const stats = document.getElementById("stats");

  if (!container) return;
  if (stats) stats.innerHTML = "";

  const state = getStateDisplay(data);

  container.innerHTML = `
    <div class="pr-details">
      <div class="pr-details-header">
        <span class="pr-state ${state.className}">${escapeHtml(state.label)}</span>
        <h2 class="pr-title">${escapeHtml(data.title)} <span class="pr-number">#${data.number}</span></h2>
      </div>

      <div class="pr-meta">
        ${data.user ? `<div class="pr-meta-item"><span class="pr-meta-label">Author</span><span class="pr-meta-value">${escapeHtml(data.user.login)}</span></div>` : ""}
        ${data.head && data.base ? `<div class="pr-meta-item"><span class="pr-meta-label">Branches</span><span class="pr-meta-value pr-branches"><code>${escapeHtml(data.head.ref)}</code> → <code>${escapeHtml(data.base.ref)}</code></span></div>` : ""}
        ${data.created_at ? `<div class="pr-meta-item"><span class="pr-meta-label">Created</span><span class="pr-meta-value">${formatDate(data.created_at)}</span></div>` : ""}
        ${data.updated_at ? `<div class="pr-meta-item"><span class="pr-meta-label">Updated</span><span class="pr-meta-value">${formatDate(data.updated_at)}</span></div>` : ""}
        ${data.merged_at ? `<div class="pr-meta-item"><span class="pr-meta-label">Merged</span><span class="pr-meta-value">${formatDate(data.merged_at)}${data.merged_by ? ` by ${escapeHtml(data.merged_by)}` : ""}</span></div>` : ""}
        ${data.closed_at && !data.merged ? `<div class="pr-meta-item"><span class="pr-meta-label">Closed</span><span class="pr-meta-value">${formatDate(data.closed_at)}</span></div>` : ""}
        ${data.milestone ? `<div class="pr-meta-item"><span class="pr-meta-label">Milestone</span><span class="pr-meta-value">${escapeHtml(data.milestone)}</span></div>` : ""}
      </div>

      ${renderStatsBadges(data)}

      ${data.labels && data.labels.length > 0 ? `
        <div class="pr-labels">
          <span class="pr-meta-label">Labels</span>
          <div class="pr-label-list">${data.labels.map((l) => `<span class="pr-label">${escapeHtml(l)}</span>`).join("")}</div>
        </div>
      ` : ""}

      ${data.assignees && data.assignees.length > 0 ? `
        <div class="pr-people">
          <span class="pr-meta-label">Assignees</span>
          <span class="pr-meta-value">${data.assignees.map((a) => escapeHtml(a)).join(", ")}</span>
        </div>
      ` : ""}

      ${data.requested_reviewers && data.requested_reviewers.length > 0 ? `
        <div class="pr-people">
          <span class="pr-meta-label">Reviewers</span>
          <span class="pr-meta-value">${data.requested_reviewers.map((r) => escapeHtml(r)).join(", ")}</span>
        </div>
      ` : ""}

      ${data.body ? `
        <div class="pr-body">
          <div class="pr-body-content">${escapeHtml(data.body)}</div>
        </div>
      ` : ""}
    </div>
  `;
}

function getStateDisplay(data: PRDetails): { label: string; className: string } {
  if (data.merged) return { label: "Merged", className: "merged" };
  if (data.draft) return { label: "Draft", className: "draft" };
  if (data.state === "closed") return { label: "Closed", className: "closed" };
  return { label: "Open", className: "open" };
}

function renderStatsBadges(data: PRDetails): string {
  const badges: string[] = [];

  if (data.commits !== undefined) {
    badges.push(`<span class="pr-stat-badge">${data.commits} commit${data.commits !== 1 ? "s" : ""}</span>`);
  }
  if (data.changed_files !== undefined) {
    badges.push(`<span class="pr-stat-badge">${data.changed_files} file${data.changed_files !== 1 ? "s" : ""} changed</span>`);
  }
  if (data.additions !== undefined) {
    badges.push(`<span class="pr-stat-badge additions">+${data.additions}</span>`);
  }
  if (data.deletions !== undefined) {
    badges.push(`<span class="pr-stat-badge deletions">-${data.deletions}</span>`);
  }
  if (data.comments !== undefined && data.comments > 0) {
    badges.push(`<span class="pr-stat-badge">${data.comments} comment${data.comments !== 1 ? "s" : ""}</span>`);
  }

  if (badges.length === 0) return "";
  return `<div class="pr-stats-row">${badges.join("")}</div>`;
}

function formatDate(isoDate: string): string {
  try {
    const date = new Date(isoDate);
    return date.toLocaleDateString(undefined, {
      year: "numeric",
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  } catch {
    return isoDate;
  }
}

function escapeHtml(text: string): string {
  const div = document.createElement("div");
  div.textContent = text;
  return div.innerHTML;
}
