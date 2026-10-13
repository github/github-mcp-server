package main

import (
	"runtime/debug"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveServerVersion(t *testing.T) {
	t.Parallel()
	sha := strings.Repeat("a1", 20)
	otherSHA := strings.Repeat("b2", 20)
	source := &debug.BuildInfo{
		Main: debug.Module{Version: "(devel)"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: sha}, {Key: "vcs.modified", Value: "false"},
		},
	}
	dirty := &debug.BuildInfo{
		Main: debug.Module{Version: "(devel)"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: sha}, {Key: "vcs.modified", Value: "true"},
		},
	}
	tests := []struct {
		name     string
		release  string
		revision string
		info     *debug.BuildInfo
		want     string
	}{
		{name: "release unchanged", release: "v1.2.3", revision: sha, info: dirty, want: "v1.2.3"},
		{name: "release suffix unchanged", release: "v1.2.3-rc.1+build.4", want: "v1.2.3-rc.1+build.4"},
		{name: "release before invalid revision", release: "v1.2.3", revision: "short", info: dirty, want: "v1.2.3"},
		{name: "source build", release: "version", revision: "commit", info: source, want: "vcs-" + sha},
		{
			name: "source revision before inferred module version", release: "version", revision: "commit",
			info: &debug.BuildInfo{
				Main:     debug.Module{Version: "v1.2.4-0.20260916095829-a1a1a1a1a1a1+dirty"},
				Settings: dirty.Settings,
			},
			want: "vcs-" + sha + "-dirty",
		},
		{name: "dirty source", release: "version", revision: "commit", info: dirty, want: "vcs-" + sha + "-dirty"},
		{name: "Docker revision", release: "dev", revision: sha, want: "vcs-" + sha},
		{name: "Docker revision ignores context dirty state", release: "dev", revision: sha, info: dirty, want: "vcs-" + sha},
		{name: "linked revision ignores unrelated dirty state", release: "dev", revision: otherSHA, info: dirty, want: "vcs-" + otherSHA},
		{name: "linked revision precedence", release: "dev", revision: otherSHA, info: source, want: "vcs-" + otherSHA},
		{name: "SHA256", release: "dev", revision: strings.Repeat("ab", 32), want: "vcs-" + strings.Repeat("ab", 32)},
		{name: "canonical hex", release: "dev", revision: strings.ToUpper(sha), want: "vcs-" + sha},
		{name: "installed module", info: &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}}, want: "v1.2.3"},
		{name: "absent build info", release: "version", revision: "commit", want: "dev"},
		{name: "missing VCS metadata", release: "dev", info: &debug.BuildInfo{}, want: "dev"},
		{name: "unknown placeholder", release: "unknown", want: "dev"},
		{name: "short revision", release: "dev", revision: "abcdef", want: "dev"},
		{name: "short revision falls through to VCS", release: "dev", revision: "abcdef", info: source, want: "vcs-" + sha},
		{name: "malformed linked revision falls through to VCS", release: "dev", revision: strings.Repeat("x", 40), info: source, want: "vcs-" + sha},
		{name: "malformed linked revision without VCS", release: "dev", revision: strings.Repeat("x", 40), want: "dev"},
		{
			name: "malformed VCS falls through to installed module",
			info: &debug.BuildInfo{
				Main:     debug.Module{Version: "v1.2.3"},
				Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "short"}},
			},
			want: "v1.2.3",
		},
		{
			name: "malformed VCS without installed module",
			info: &debug.BuildInfo{
				Main:     debug.Module{Version: "(devel)"},
				Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "short"}},
			},
			want: "dev",
		},
		{name: "invalid installed module", info: &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3\n"}}, want: "dev"},
		{name: "placeholder installed module", info: &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, want: "dev"},
		{name: "release whitespace", release: "v1.2.3 extra", want: "dev"},
		{name: "release newline", release: "v1.2.3\n", want: "dev"},
		{name: "release slash", release: "release/v1.2.3", want: "dev"},
		{name: "release non ASCII", release: "v1.2.3-\u00e9", want: "dev"},
		{name: "invalid release falls through to linked revision", release: "v1.2.3 extra", revision: otherSHA, info: dirty, want: "vcs-" + otherSHA},
		{name: "invalid release falls through to dirty VCS", release: "release/v1.2.3", info: dirty, want: "vcs-" + sha + "-dirty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, resolveServerVersion(tt.release, tt.revision, tt.info))
		})
	}
}

func TestResolveServerVersionSkipsNullRevisions(t *testing.T) {
	t.Parallel()
	for _, revision := range []string{strings.Repeat("0", 40), strings.Repeat("0", 64)} {
		for _, modified := range []string{"false", "true"} {
			for _, linked := range []bool{false, true} {
				name := revision + "/modified=" + modified
				if linked {
					name += "/linked"
				}
				t.Run(name, func(t *testing.T) {
					info := &debug.BuildInfo{Settings: []debug.BuildSetting{
						{Key: "vcs.revision", Value: revision},
						{Key: "vcs.modified", Value: modified},
					}}
					commit := "commit"
					want := "dev"
					if linked {
						commit = revision
						info.Settings[0].Value = strings.Repeat("ab", 20)
						want = "vcs-" + strings.Repeat("ab", 20)
						if modified == "true" {
							want += "-dirty"
						}
					}
					assert.Equal(t, want, resolveServerVersion("dev", commit, info))
				})
			}
		}
	}
}
