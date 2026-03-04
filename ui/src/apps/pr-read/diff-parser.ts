export interface DiffLine {
  type: "addition" | "deletion" | "context";
  content: string;
  oldLineNumber: number | null;
  newLineNumber: number | null;
}

export interface DiffHunk {
  header: string;
  oldStart: number;
  oldCount: number;
  newStart: number;
  newCount: number;
  lines: DiffLine[];
}

export interface DiffFile {
  oldPath: string;
  newPath: string;
  hunks: DiffHunk[];
  additions: number;
  deletions: number;
}

export interface ParsedDiff {
  files: DiffFile[];
  totalAdditions: number;
  totalDeletions: number;
}

export function parseDiff(diffText: string): ParsedDiff {
  const files: DiffFile[] = [];
  let totalAdditions = 0;
  let totalDeletions = 0;

  const lines = diffText.split("\n");
  let i = 0;

  while (i < lines.length) {
    const line = lines[i];

    // Look for file header
    if (line.startsWith("diff --git")) {
      const file = parseFile(lines, i);
      if (file) {
        files.push(file.file);
        totalAdditions += file.file.additions;
        totalDeletions += file.file.deletions;
        i = file.nextIndex;
        continue;
      }
    }

    i++;
  }

  return { files, totalAdditions, totalDeletions };
}

function parseFile(
  lines: string[],
  startIndex: number
): { file: DiffFile; nextIndex: number } | null {
  let i = startIndex;
  const diffLine = lines[i];

  // Parse file paths from diff --git line
  const gitMatch = diffLine.match(/^diff --git a\/(.+) b\/(.+)$/);
  if (!gitMatch) return null;

  let oldPath = gitMatch[1];
  let newPath = gitMatch[2];
  i++;

  // Skip extended headers until we find --- or the next diff
  while (i < lines.length) {
    const line = lines[i];

    if (line.startsWith("--- ")) {
      const oldMatch = line.match(/^--- (?:a\/)?(.+)$/);
      if (oldMatch && oldMatch[1] !== "/dev/null") {
        oldPath = oldMatch[1];
      }
      i++;
      continue;
    }

    if (line.startsWith("+++ ")) {
      const newMatch = line.match(/^\+\+\+ (?:b\/)?(.+)$/);
      if (newMatch && newMatch[1] !== "/dev/null") {
        newPath = newMatch[1];
      }
      i++;
      continue;
    }

    if (line.startsWith("@@") || line.startsWith("diff --git")) {
      break;
    }

    i++;
  }

  // Parse hunks
  const hunks: DiffHunk[] = [];
  let additions = 0;
  let deletions = 0;

  while (i < lines.length && !lines[i].startsWith("diff --git")) {
    if (lines[i].startsWith("@@")) {
      const hunkResult = parseHunk(lines, i);
      if (hunkResult) {
        hunks.push(hunkResult.hunk);
        additions += hunkResult.hunk.lines.filter(
          (l) => l.type === "addition"
        ).length;
        deletions += hunkResult.hunk.lines.filter(
          (l) => l.type === "deletion"
        ).length;
        i = hunkResult.nextIndex;
        continue;
      }
    }
    i++;
  }

  return {
    file: { oldPath, newPath, hunks, additions, deletions },
    nextIndex: i,
  };
}

function parseHunk(
  lines: string[],
  startIndex: number
): { hunk: DiffHunk; nextIndex: number } | null {
  const headerLine = lines[startIndex];
  const headerMatch = headerLine.match(
    /^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@(.*)$/
  );
  if (!headerMatch) return null;

  const oldStart = parseInt(headerMatch[1], 10);
  const oldCount = headerMatch[2] ? parseInt(headerMatch[2], 10) : 1;
  const newStart = parseInt(headerMatch[3], 10);
  const newCount = headerMatch[4] ? parseInt(headerMatch[4], 10) : 1;
  const headerContext = headerMatch[5] || "";

  const hunkLines: DiffLine[] = [];
  let oldLine = oldStart;
  let newLine = newStart;
  let i = startIndex + 1;

  while (i < lines.length) {
    const line = lines[i];

    // Stop at next hunk or file
    if (line.startsWith("@@") || line.startsWith("diff --git")) {
      break;
    }

    // Handle "\ No newline at end of file"
    if (line.startsWith("\\ ")) {
      i++;
      continue;
    }

    if (line.startsWith("+")) {
      hunkLines.push({
        type: "addition",
        content: line.slice(1),
        oldLineNumber: null,
        newLineNumber: newLine++,
      });
    } else if (line.startsWith("-")) {
      hunkLines.push({
        type: "deletion",
        content: line.slice(1),
        oldLineNumber: oldLine++,
        newLineNumber: null,
      });
    } else if (line.startsWith(" ") || line === "") {
      hunkLines.push({
        type: "context",
        content: line.slice(1),
        oldLineNumber: oldLine++,
        newLineNumber: newLine++,
      });
    }

    i++;
  }

  return {
    hunk: {
      header: `@@ -${oldStart},${oldCount} +${newStart},${newCount} @@${headerContext}`,
      oldStart,
      oldCount,
      newStart,
      newCount,
      lines: hunkLines,
    },
    nextIndex: i,
  };
}
