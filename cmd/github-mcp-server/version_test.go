package main

import (
	"runtime/debug"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		wantErr  string
	}{
		{name: "release unchanged", release: "v1.2.3", revision: sha, info: dirty, want: "v1.2.3"},
		{name: "release suffix unchanged", release: "v1.2.3-rc.1+build.4", want: "v1.2.3-rc.1+build.4"},
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
		{name: "linked revision precedence", release: "dev", revision: otherSHA, info: source, want: "vcs-" + otherSHA},
		{name: "SHA256", release: "dev", revision: strings.Repeat("ab", 32), want: "vcs-" + strings.Repeat("ab", 32)},
		{name: "canonical hex", release: "dev", revision: strings.ToUpper(sha), want: "vcs-" + sha},
		{name: "installed module", info: &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}}, want: "v1.2.3"},
		{name: "absent build info", release: "version", revision: "commit", wantErr: "no release or revision"},
		{name: "missing VCS metadata", release: "dev", info: &debug.BuildInfo{}, wantErr: "no release or revision"},
		{name: "unknown placeholder", release: "unknown", wantErr: "no release or revision"},
		{name: "short revision", release: "dev", revision: "abcdef", wantErr: "full SHA-1 or SHA-256"},
		{name: "malformed linked revision", release: "dev", revision: strings.Repeat("x", 40), info: source, wantErr: "invalid server build revision"},
		{name: "release whitespace", release: "v1.2.3 extra", wantErr: "HTTP product token"},
		{name: "release newline", release: "v1.2.3\n", wantErr: "HTTP product token"},
		{name: "release slash", release: "release/v1.2.3", wantErr: "HTTP product token"},
		{name: "release non ASCII", release: "v1.2.3-\u00e9", wantErr: "HTTP product token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveServerVersion(tt.release, tt.revision, tt.info)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				assert.Empty(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
