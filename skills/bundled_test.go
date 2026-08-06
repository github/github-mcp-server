package skills_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/skills"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBundledSkillsValid is the gate that turns a malformed bundled skill
// into a test failure instead of a server-startup panic.
func TestBundledSkillsValid(t *testing.T) {
	r := skills.Bundled()
	require.NotEmpty(t, r.Skills())

	for _, s := range r.Skills() {
		t.Run(s.Name, func(t *testing.T) {
			// The final skill-path segment is the skill name, under the
			// bundled organizational prefix.
			assert.Equal(t, "github/"+s.Name, s.Path)
			assert.Equal(t, "skill://github/"+s.Name+"/SKILL.md", s.URI())

			// Frontmatter identity: entry frontmatter equals what a read of
			// SKILL.md would parse, and name matches the URI's final segment.
			entry := s.Entry()
			assert.Equal(t, s.Name, entry.Frontmatter["name"])
			desc, _ := entry.Frontmatter["description"].(string)
			assert.NotEmpty(t, desc)

			// resources is complete and self-consistent: first entry is the
			// skill's own uri carrying the digest of SKILL.md.
			require.NotEmpty(t, entry.Resources)
			assert.Equal(t, entry.URI, entry.Resources[0].URI)
			assert.Equal(t, skills.Digest(s.Files[0].Content), entry.Resources[0].Digest)
			assert.Len(t, entry.Resources, len(s.Files))
		})
	}
}

// TestBundledSkillsMatchSourceTree verifies the embedded content serves
// exactly what is on disk in this package's directory — the skill files are
// the primary artifact; the embed is a delivery detail.
func TestBundledSkillsMatchSourceTree(t *testing.T) {
	entries, err := os.ReadDir(".")
	require.NoError(t, err)

	var wantNames []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		content, err := os.ReadFile(filepath.Join(e.Name(), "SKILL.md"))
		require.NoError(t, err, "every directory in skills/ must contain a SKILL.md")
		wantNames = append(wantNames, e.Name())

		s, ok := skills.Bundled().Get("skill://github/" + e.Name() + "/SKILL.md")
		require.True(t, ok, "skill %s not in bundled registry", e.Name())
		assert.Equal(t, skills.Digest(content), s.Files[0].Digest)
	}

	var gotNames []string
	for _, s := range skills.Bundled().Skills() {
		gotNames = append(gotNames, s.Name)
	}
	assert.Equal(t, strings.Join(wantNames, ","), strings.Join(gotNames, ","))
}
