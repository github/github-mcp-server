package observability

import (
	"fmt"
	"log/slog"
	"strings"
)

// ParseLogLevel parses a textual log level (case-insensitive) into a slog.Level.
// Accepts "debug", "info", "warn"/"warning", "error". An empty string returns
// the provided default. Unknown values produce an error.
func ParseLogLevel(s string, def slog.Level) (slog.Level, error) {
	if strings.TrimSpace(s) == "" {
		return def, nil
	}
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return def, fmt.Errorf("unknown log level %q (want one of: debug, info, warn, error)", s)
	}
}
