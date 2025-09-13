package main

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed build_info/commit.txt build_info/date.txt build_info/version.txt
var versionFS embed.FS

type buildInfoStruct struct {
	commit  string
	date    string
	version string
}

func (b buildInfoStruct) String() string {
	return fmt.Sprintf("Commit: %s\nBuild Date: %s\nVersion: %s", b.commit, b.date, b.version)
}

var buildInfo = func() buildInfoStruct {
	readFile := func(path, fallback string) string {
		if content, err := versionFS.ReadFile(path); err == nil {
			return strings.TrimSpace(string(content))
		}
		return fallback
	}

	return buildInfoStruct{
		commit:  readFile("build_info/commit.txt", "unknown commit"),
		date:    readFile("build_info/date.txt", "unknown date"),
		version: readFile("build_info/version.txt", "unknown version"),
	}
}()
