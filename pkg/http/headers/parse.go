package headers

import "strings"

// ParseCommaSeparated splits a header value by comma, trims whitespace,
// and filters out empty values
func ParseCommaSeparated(value string) []string {
	if value == "" {
		return []string{}
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
