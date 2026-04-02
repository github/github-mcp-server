import type { ParsedDiff, DiffFile, DiffHunk, DiffLine } from "./diff-parser";

export type DiffViewMode = "unified" | "split";

let currentMode: DiffViewMode = "unified";
let currentParsedDiff: ParsedDiff | null = null;

export function setViewMode(mode: DiffViewMode): void {
  currentMode = mode;
  if (currentParsedDiff) {
    renderDiff(currentParsedDiff);
  }
}

export function getViewMode(): DiffViewMode {
  return currentMode;
}

export function renderDiff(parsed: ParsedDiff): void {
  currentParsedDiff = parsed;
  const container = document.getElementById("diff-container");
  const stats = document.getElementById("stats");

  if (!container || !stats) return;

  // Render stats
  stats.innerHTML = `
    <span class="stat-additions">+${parsed.totalAdditions}</span>
    <span class="stat-deletions">-${parsed.totalDeletions}</span>
    <span> across ${parsed.files.length} file${parsed.files.length !== 1 ? "s" : ""}</span>
  `;

  // Render files
  if (parsed.files.length === 0) {
    container.innerHTML = '<div class="empty-state">No changes in this diff</div>';
    return;
  }

  // Update container class for view mode
  container.className = `diff-container ${currentMode}`;
  container.innerHTML = parsed.files.map((file) => renderFile(file, currentMode)).join("");

  // Add toggle handlers
  container.querySelectorAll(".diff-file-header").forEach((header) => {
    header.addEventListener("click", () => {
      const file = header.closest(".diff-file");
      if (file) {
        file.classList.toggle("collapsed");
        const content = file.querySelector(".diff-file-content");
        if (content) {
          content.classList.toggle("collapsed");
        }
      }
    });
  });
}

function renderFile(file: DiffFile, mode: DiffViewMode): string {
  const displayPath = file.newPath || file.oldPath;

  return `
    <div class="diff-file">
      <div class="diff-file-header">
        <span class="diff-file-path">${escapeHtml(displayPath)}</span>
        <div class="diff-file-stats">
          <span class="diff-file-additions">+${file.additions}</span>
          <span class="diff-file-deletions">-${file.deletions}</span>
          <span class="collapse-indicator">▼</span>
        </div>
      </div>
      <div class="diff-file-content">
        ${file.hunks.map((hunk) => renderHunk(hunk, mode)).join("")}
      </div>
    </div>
  `;
}

function renderHunk(hunk: DiffHunk, mode: DiffViewMode): string {
  if (mode === "split") {
    return renderHunkSplit(hunk);
  }
  return renderHunkUnified(hunk);
}

function renderHunkUnified(hunk: DiffHunk): string {
  return `
    <div class="diff-hunk">
      <div class="diff-hunk-header">${escapeHtml(hunk.header)}</div>
      ${hunk.lines.map(renderLineUnified).join("")}
    </div>
  `;
}

function renderLineUnified(line: DiffLine): string {
  const prefix =
    line.type === "addition" ? "+" : line.type === "deletion" ? "-" : " ";
  const oldNum = line.oldLineNumber !== null ? line.oldLineNumber : "";
  const newNum = line.newLineNumber !== null ? line.newLineNumber : "";

  return `
    <div class="diff-line ${line.type}">
      <div class="diff-line-numbers">
        <span class="diff-line-number">${oldNum}</span>
        <span class="diff-line-number">${newNum}</span>
      </div>
      <div class="diff-line-content"><span class="diff-line-prefix">${prefix}</span>${escapeHtml(line.content)}</div>
    </div>
  `;
}

function renderHunkSplit(hunk: DiffHunk): string {
  // Pair up deletions and additions for side-by-side view
  const rows = buildSplitRows(hunk.lines);

  return `
    <div class="diff-hunk">
      <div class="diff-hunk-header">${escapeHtml(hunk.header)}</div>
      <div class="diff-split-container">
        ${rows.map(renderSplitRow).join("")}
      </div>
    </div>
  `;
}

interface SplitRow {
  left: DiffLine | null;
  right: DiffLine | null;
}

function buildSplitRows(lines: DiffLine[]): SplitRow[] {
  const rows: SplitRow[] = [];
  let i = 0;

  while (i < lines.length) {
    const line = lines[i];

    if (line.type === "context") {
      rows.push({ left: line, right: line });
      i++;
    } else if (line.type === "deletion") {
      // Collect consecutive deletions
      const deletions: DiffLine[] = [];
      while (i < lines.length && lines[i].type === "deletion") {
        deletions.push(lines[i]);
        i++;
      }
      // Collect consecutive additions
      const additions: DiffLine[] = [];
      while (i < lines.length && lines[i].type === "addition") {
        additions.push(lines[i]);
        i++;
      }
      // Pair them up
      const maxLen = Math.max(deletions.length, additions.length);
      for (let j = 0; j < maxLen; j++) {
        rows.push({
          left: deletions[j] || null,
          right: additions[j] || null,
        });
      }
    } else if (line.type === "addition") {
      // Addition without preceding deletion
      rows.push({ left: null, right: line });
      i++;
    } else {
      i++;
    }
  }

  return rows;
}

function renderSplitRow(row: SplitRow): string {
  return `
    <div class="diff-split-row">
      <div class="diff-split-side left ${row.left?.type || "empty"}">
        <span class="diff-line-number">${row.left?.oldLineNumber ?? ""}</span>
        <div class="diff-line-content">${row.left ? escapeHtml(row.left.content) : ""}</div>
      </div>
      <div class="diff-split-side right ${row.right?.type || "empty"}">
        <span class="diff-line-number">${row.right?.newLineNumber ?? ""}</span>
        <div class="diff-line-content">${row.right ? escapeHtml(row.right.content) : ""}</div>
      </div>
    </div>
  `;
}

function escapeHtml(text: string): string {
  const div = document.createElement("div");
  div.textContent = text;
  return div.innerHTML;
}
