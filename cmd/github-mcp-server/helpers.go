package main

import "strings"

// formatToolsetName converts a toolset ID to a human-readable name.
// Used by both generate_docs.go and list_scopes.go for consistent formatting.
func formatToolsetName(name string) string {
	switch name {
	case "pull_requests":
		return "Pull Requests"
	case "repos":
		return "Repositories"
	case "code_security":
		return "Code Security"
	case "secret_protection":
		return "Secret Protection"
	case "orgs":
		return "Organizations"
	default:
		// Fallback: capitalize first letter and replace underscores with spaces
		parts := strings.Split(name, "_")
		for i, part := range parts {
			if len(part) > 0 {
				parts[i] = strings.ToUpper(string(part[0])) + part[1:]
			}
		}
		return strings.Join(parts, " ")
	}
}
