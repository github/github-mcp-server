package github

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
)

const workflowPathPrefix = ".github/workflows/"

func validateRelativePath(value string) (string, error) {
	value = strings.TrimPrefix(value, "/")
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

func workflowScopePolicyForPath(arguments map[string]any) inventory.ScopePolicy {
	value, ok := arguments["path"].(string)
	if !ok {
		return scopes.UnscopedScopePolicy()
	}
	cleaned, err := validateRelativePath(value)
	if err != nil {
		return scopes.UnscopedScopePolicy()
	}
	if !isWorkflowPath(cleaned) {
		return scopes.AllOfScopePolicy(scopes.Repo)
	}
	return scopes.AllOfScopePolicy(scopes.Repo, scopes.Workflow)
}

func workflowScopePolicyForFiles(arguments map[string]any) inventory.ScopePolicy {
	files, ok := arguments["files"].([]any)
	if !ok {
		return scopes.UnscopedScopePolicy()
	}
	for _, file := range files {
		fileMap, ok := file.(map[string]any)
		if !ok {
			return scopes.UnscopedScopePolicy()
		}
		value, ok := fileMap["path"].(string)
		if !ok {
			return scopes.UnscopedScopePolicy()
		}
		cleaned, err := validateRelativePath(value)
		if err != nil {
			return scopes.UnscopedScopePolicy()
		}
		if isWorkflowPath(cleaned) {
			return scopes.AllOfScopePolicy(scopes.Repo, scopes.Workflow)
		}
	}
	return scopes.AllOfScopePolicy(scopes.Repo)
}
