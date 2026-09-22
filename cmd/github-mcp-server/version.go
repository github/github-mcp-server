package main

import (
	"encoding/hex"
	"fmt"
	"runtime/debug"
	"strings"
	"unicode"
)

func resolveServerVersion(release, revision string, info *debug.BuildInfo) (string, error) {
	isPlaceholder := func(value string) bool {
		switch value {
		case "", "version", "dev", "unknown", "(devel)":
			return true
		default:
			return false
		}
	}
	validateRelease := func(value string) (string, error) {
		for _, r := range value {
			if r > unicode.MaxASCII || r <= ' ' || r == 127 || strings.ContainsRune("()<>@,;:\\\"/[]?={}", r) {
				return "", fmt.Errorf("server version must be an HTTP product token: %q", value)
			}
		}
		return value, nil
	}
	if !isPlaceholder(release) {
		return validateRelease(release)
	}

	var vcsRevision string
	var dirty bool
	if info != nil {
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				vcsRevision = setting.Value
			case "vcs.modified":
				dirty = setting.Value == "true"
			}
		}
	}
	if revision == "" || revision == "commit" {
		revision = vcsRevision
		if revision == "" && info != nil && !isPlaceholder(info.Main.Version) {
			return validateRelease(info.Main.Version)
		}
	}
	if revision == "" {
		return "", fmt.Errorf("server build has no release or revision: build the package with VCS metadata or set main.version/main.commit using -ldflags")
	}
	if len(revision) != 40 && len(revision) != 64 {
		return "", fmt.Errorf("server build revision must be a full SHA-1 or SHA-256: %q", revision)
	}
	if _, err := hex.DecodeString(revision); err != nil {
		return "", fmt.Errorf("invalid server build revision: %w", err)
	}
	resolved := "vcs-" + strings.ToLower(revision)
	if dirty {
		resolved += "-dirty"
	}
	return resolved, nil
}
