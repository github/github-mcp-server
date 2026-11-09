package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

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

// boolSettings lists the boolean configuration keys shared by the server commands.
var boolSettings = []string{
	"read-only",
	"lockdown-mode",
	"insiders",
	"export-translations",
	"enable-command-logging",
}

// validateBoolSettings rejects boolean settings whose raw value is not a valid
// boolean. viper.GetBool silently treats unparsable values such as "yes" as
// false, which would leave e.g. read-only mode off without any warning. Flags
// are already parsed strictly by pflag, so in practice this catches invalid
// values coming from environment variables.
func validateBoolSettings(keys ...string) error {
	for _, key := range keys {
		value, ok := viper.Get(key).(string)
		if !ok {
			continue
		}
		if _, err := strconv.ParseBool(value); err != nil {
			envName := "GITHUB_" + strings.ToUpper(strings.ReplaceAll(key, "-", "_"))
			return fmt.Errorf("invalid value %q for %s: must be a boolean (1, t, true, 0, f, false)", value, envName)
		}
	}
	return nil
}
