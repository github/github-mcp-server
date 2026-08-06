package skills_test

import (
	"strings"
	"testing"

	"github.com/github/github-mcp-server/skills"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const minimalSkillMD = `---
name: demo
description: A demo skill.
---

## Workflow

Do the thing.
`

func TestParseFrontmatterVerbatim(t *testing.T) {
	content := strings.ReplaceAll(minimalSkillMD, "description: A demo skill.",
		"description: A demo skill.\nlicense: Apache-2.0\nmetadata:\n  version: \"2.1.0\"")

	fm, err := skills.ParseFrontmatter([]byte(content))
	require.NoError(t, err)

	// Every field the author wrote passes through — not a curated subset.
	assert.Equal(t, "demo", fm["name"])
	assert.Equal(t, "A demo skill.", fm["description"])
	assert.Equal(t, "Apache-2.0", fm["license"])
	assert.Equal(t, map[string]any{"version": "2.1.0"}, fm["metadata"])
}

func TestParseFrontmatterErrors(t *testing.T) {
	for name, content := range map[string]string{
		"no frontmatter":  "# Just markdown\n",
		"unterminated":    "---\nname: x\ndescription: y\n",
		"emptyerror":      "---\n---\n",
		"leading content": "\n---\nname: x\n---\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := skills.ParseFrontmatter([]byte(content))
			assert.Error(t, err)
		})
	}
}

func TestParseFrontmatterCRLF(t *testing.T) {
	content := strings.ReplaceAll(minimalSkillMD, "\n", "\r\n")
	fm, err := skills.ParseFrontmatter([]byte(content))
	require.NoError(t, err)
	assert.Equal(t, "demo", fm["name"])
}

func TestDigestFormat(t *testing.T) {
	// SHA-256 of the empty string, a fixed vector.
	assert.Equal(t,
		"sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		skills.Digest(nil))
}

func TestNewValidation(t *testing.T) {
	skillMD := skills.File{Path: "SKILL.md", Content: []byte(minimalSkillMD)}

	t.Run("valid", func(t *testing.T) {
		s, err := skills.New("acme/billing/demo", []skills.File{skillMD})
		require.NoError(t, err)
		assert.Equal(t, "demo", s.Name)
		assert.Equal(t, "skill://acme/billing/demo/SKILL.md", s.URI())
		assert.Equal(t, "skill://acme/billing/demo", s.RootURI())
		assert.Equal(t, "A demo skill.", s.Description)
	})

	t.Run("missing SKILL.md", func(t *testing.T) {
		_, err := skills.New("demo", []skills.File{{Path: "README.md", Content: []byte("x")}})
		assert.ErrorContains(t, err, "missing SKILL.md")
	})

	t.Run("name mismatch", func(t *testing.T) {
		_, err := skills.New("other-name", []skills.File{skillMD})
		assert.ErrorContains(t, err, "does not match")
	})

	t.Run("invalid name", func(t *testing.T) {
		_, err := skills.New("Bad_Name", []skills.File{skillMD})
		assert.ErrorContains(t, err, "naming rules")
	})

	t.Run("missing description", func(t *testing.T) {
		content := "---\nname: demo\n---\nbody\n"
		_, err := skills.New("demo", []skills.File{{Path: "SKILL.md", Content: []byte(content)}})
		assert.ErrorContains(t, err, "description")
	})
}

func TestSkillEntryOrdering(t *testing.T) {
	s, err := skills.New("demo", []skills.File{
		{Path: "scripts/extract.py", Content: []byte("print()")},
		{Path: "SKILL.md", Content: []byte(minimalSkillMD)},
		{Path: "references/GUIDE.md", Content: []byte("guide")},
	})
	require.NoError(t, err)

	entry := s.Entry()
	require.Len(t, entry.Resources, 3)
	// SKILL.md first — its entry matches the skill's top-level uri and
	// carries the digest of SKILL.md itself — then supporting files by path.
	assert.Equal(t, entry.URI, entry.Resources[0].URI)
	assert.Equal(t, "skill://demo/references/GUIDE.md", entry.Resources[1].URI)
	assert.Equal(t, "skill://demo/scripts/extract.py", entry.Resources[2].URI)
	for _, r := range entry.Resources {
		assert.Regexp(t, `^sha256:[0-9a-f]{64}$`, r.Digest)
	}
}

func TestFileMIMEType(t *testing.T) {
	assert.Equal(t, "text/markdown", skills.FileMIMEType("SKILL.md"))
	assert.Equal(t, "text/markdown", skills.FileMIMEType("references/GUIDE.md"))
	assert.Equal(t, "text/x-python", skills.FileMIMEType("scripts/extract.py"))
	assert.Equal(t, "text/plain", skills.FileMIMEType("LICENSE"))
}
