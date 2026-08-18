package github

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/github/github-mcp-server/pkg/scopes"
)

const workflowPathPrefix = ".github/workflows/"

func validateRelativePath(value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("path must not be empty")
	}
	if path.IsAbs(value) {
		return "", fmt.Errorf("path must be relative")
	}
	if strings.Contains(value, `\`) {
		return "", fmt.Errorf("path must use forward slashes")
	}
	if slices.Contains(strings.Split(value, "/"), "..") {
		return "", fmt.Errorf("path must not contain parent directory traversal")
	}

	cleaned := path.Clean(value)
	if cleaned == "." {
		return "", fmt.Errorf("path must identify a file")
	}
	return cleaned, nil
}

func isWorkflowPath(value string) bool {
	return strings.HasPrefix(value, workflowPathPrefix) && len(value) > len(workflowPathPrefix)
}

func workflowScopeForPath(arguments map[string]any) []string {
	value, ok := arguments["path"].(string)
	if !ok {
		return nil
	}
	cleaned, err := validateRelativePath(value)
	if err != nil || !isWorkflowPath(cleaned) {
		return nil
	}
	return []string{string(scopes.Workflow)}
}

func workflowScopeForFiles(arguments map[string]any) []string {
	files, ok := arguments["files"].([]any)
	if !ok {
		return nil
	}
	for _, file := range files {
		fileMap, ok := file.(map[string]any)
		if !ok {
			continue
		}
		if len(workflowScopeForPath(fileMap)) > 0 {
			return []string{string(scopes.Workflow)}
		}
	}
	return nil
}
