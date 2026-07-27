package github

import (
	"fmt"
	"slices"
	"strings"
)

func validateEnumParam(name, value string, allowed ...string) error {
	if value == "" {
		return nil
	}
	if slices.Contains(allowed, value) {
		return nil
	}
	return fmt.Errorf("%s must be one of: %s", name, strings.Join(allowed, ", "))
}

func validateRepoRelativePath(name, path string) error {
	if path == "" {
		return fmt.Errorf("%s must not be empty", name)
	}
	if strings.HasPrefix(path, "/") {
		return fmt.Errorf("%s must be relative to the repository root (no leading '/')", name)
	}
	if slices.Contains(strings.Split(path, "/"), "..") {
		return fmt.Errorf("%s must not contain '..' segments", name)
	}
	for _, r := range path {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("%s must not contain control characters", name)
		}
	}
	return nil
}

func validateOptionalRepoRelativePath(name, path string) error {
	if path == "" {
		return nil
	}
	return validateRepoRelativePath(name, path)
}

func validateRepoRelativePathOrRoot(name, path string) error {
	if path == "" || path == "/" {
		return nil
	}
	return validateRepoRelativePath(name, path)
}

func normalizeRepoRelativePathOrRoot(path string) string {
	if path == "/" {
		return ""
	}
	return path
}
