package main

import (
	"encoding/hex"
	"runtime/debug"
	"strings"
	"unicode"
)

const developmentServerVersion = "dev"

func resolveServerVersion(release, revision string, info *debug.BuildInfo) string {
	isPlaceholder := func(value string) bool {
		switch value {
		case "", "version", "dev", "unknown", "(devel)":
			return true
		default:
			return false
		}
	}
	validRelease := func(value string) bool {
		if isPlaceholder(value) {
			return false
		}
		for _, r := range value {
			if r > unicode.MaxASCII || r <= ' ' || r == 127 || strings.ContainsRune("()<>@,;:\\\"/[]?={}", r) {
				return false
			}
		}
		return true
	}
	if validRelease(release) {
		return release
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
	revisions := []struct {
		value string
		dirty bool
	}{
		{value: revision},
		{value: vcsRevision, dirty: dirty},
	}
	for _, candidate := range revisions {
		if len(candidate.value) != 40 && len(candidate.value) != 64 {
			continue
		}
		if _, err := hex.DecodeString(candidate.value); err != nil {
			continue
		}
		if strings.Trim(candidate.value, "0") == "" {
			continue
		}
		resolved := "vcs-" + strings.ToLower(candidate.value)
		if candidate.dirty {
			resolved += "-dirty"
		}
		return resolved
	}
	if info != nil && validRelease(info.Main.Version) {
		return info.Main.Version
	}
	return developmentServerVersion
}
