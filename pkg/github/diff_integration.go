package github

import (
	"fmt"
	"strings"
)

// applySemanticDiffToUnifiedPatch takes a unified diff patch for a single file
// and attempts to produce a semantic diff. It reconstructs the base and head
// content from the patch hunks, then runs them through SemanticDiff.
// Returns the original patch unchanged if the file type doesn't benefit from
// semantic diffing or if reconstruction fails.
func applySemanticDiffToUnifiedPatch(filename, patch string) string {
	if patch == "" {
		return patch
	}

	format := DetectDiffFormat(filename)
	if format == DiffFormatUnified {
		// Not a structured data or code file — keep the original patch
		return patch
	}

	base, head, ok := reconstructFromPatch(patch)
	if !ok {
		return patch
	}

	result := SemanticDiff(filename, base, head)
	if result.Format == DiffFormatFallback {
		return patch
	}

	return result.Diff
}

// reconstructFromPatch extracts the base and head file content from a unified
// diff patch. Returns the reconstructed contents and true if successful.
// This only works well for complete file diffs — partial context diffs will
// produce incomplete content, which is fine for semantic comparison of
// structured data where the full structure is usually in the diff.
func reconstructFromPatch(patch string) (base, head []byte, ok bool) {
	lines := strings.Split(patch, "\n")

	var baseLines, headLines []string
	inHunk := false

	for _, line := range lines {
		if strings.HasPrefix(line, "@@") {
			inHunk = true
			continue
		}
		if !inHunk {
			continue
		}

		switch {
		case strings.HasPrefix(line, "-"):
			baseLines = append(baseLines, line[1:])
		case strings.HasPrefix(line, "+"):
			headLines = append(headLines, line[1:])
		case strings.HasPrefix(line, " "):
			baseLines = append(baseLines, line[1:])
			headLines = append(headLines, line[1:])
		case line == "":
			// Could be end of patch or an empty context line
			baseLines = append(baseLines, "")
			headLines = append(headLines, "")
		}
	}

	if len(baseLines) == 0 && len(headLines) == 0 {
		return nil, nil, false
	}

	return []byte(strings.Join(baseLines, "\n")),
		[]byte(strings.Join(headLines, "\n")),
		true
}

// processMultiFileDiff splits a multi-file unified diff into per-file sections
// and applies semantic diffing to each file where applicable. Returns a
// combined result with structural diffs for supported formats and original
// patches for unsupported ones.
func processMultiFileDiff(rawDiff string) string {
	sections := splitDiffByFile(rawDiff)
	if len(sections) == 0 {
		return rawDiff
	}

	var result strings.Builder
	for i, section := range sections {
		if i > 0 {
			result.WriteString("\n")
		}

		semanticPatch := applySemanticDiffToUnifiedPatch(section.filename, section.patch)
		if semanticPatch != section.patch {
			result.WriteString(fmt.Sprintf("--- %s (semantic diff) ---\n", section.filename))
			result.WriteString(semanticPatch)
		} else {
			result.WriteString(section.header)
			if section.patch != "" {
				result.WriteString("\n")
				result.WriteString(section.patch)
			}
		}
	}

	return result.String()
}

type diffSection struct {
	filename string
	header   string
	patch    string
}

// splitDiffByFile splits a raw multi-file unified diff into per-file sections.
func splitDiffByFile(rawDiff string) []diffSection {
	lines := strings.Split(rawDiff, "\n")
	var sections []diffSection
	var current *diffSection

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			if current != nil {
				sections = append(sections, *current)
			}
			// Extract filename from "diff --git a/path b/path"
			parts := strings.SplitN(line, " b/", 2)
			filename := ""
			if len(parts) == 2 {
				filename = parts[1]
			}
			current = &diffSection{
				filename: filename,
				header:   line,
			}
			continue
		}

		if current == nil {
			continue
		}

		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") ||
			strings.HasPrefix(line, "index ") || strings.HasPrefix(line, "new file") ||
			strings.HasPrefix(line, "deleted file") || strings.HasPrefix(line, "old mode") ||
			strings.HasPrefix(line, "new mode") || strings.HasPrefix(line, "similarity") ||
			strings.HasPrefix(line, "rename ") || strings.HasPrefix(line, "Binary") {
			current.header += "\n" + line
		} else {
			if current.patch != "" {
				current.patch += "\n" + line
			} else {
				current.patch = line
			}
		}
	}

	if current != nil {
		sections = append(sections, *current)
	}

	return sections
}
