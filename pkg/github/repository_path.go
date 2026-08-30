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

func workflowScopeChallengeForPath(arguments map[string]any, activeScopes []string) []string {
	value, ok := arguments["path"].(string)
	if !ok {
		return nil
	}
	cleaned, err := validateRelativePath(value)
	if err != nil {
		return nil
	}
	if !isWorkflowPath(cleaned) {
		return scopes.ChallengeAll(activeScopes, scopes.Repo)
	}
	return scopes.ChallengeAll(activeScopes, scopes.Repo, scopes.Workflow)
}

func workflowScopeChallengeForFiles(arguments map[string]any, activeScopes []string) []string {
	files, ok := arguments["files"].([]any)
	if !ok {
		return nil
	}
	containsWorkflow := false
	for _, file := range files {
		fileMap, ok := file.(map[string]any)
		if !ok {
			return nil
		}
		value, ok := fileMap["path"].(string)
		if !ok {
			return nil
		}
		cleaned, err := validateRelativePath(value)
		if err != nil {
			return nil
		}
		if isWorkflowPath(cleaned) {
			containsWorkflow = true
		}
	}
	if containsWorkflow {
		return scopes.ChallengeAll(activeScopes, scopes.Repo, scopes.Workflow)
	}
	return scopes.ChallengeAll(activeScopes, scopes.Repo)
}
