package skills_test

import (
	"testing"
	"testing/fstest"

	"github.com/github/github-mcp-server/skills"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testFS mirrors the SEP's pdf-processing example: a multi-file skill with a
// nested subdirectory, alongside a single-file skill.
var testFS = fstest.MapFS{
	"pdf-processing/SKILL.md": {Data: []byte(
		"---\nname: pdf-processing\ndescription: Extract, fill, and assemble PDF documents\nmetadata:\n  version: \"2.1.0\"\n---\nbody\n")},
	"pdf-processing/references/FORMS.md":              {Data: []byte("forms reference")},
	"pdf-processing/scripts/extract.py":               {Data: []byte("print('extract')")},
	"pdf-processing/templates/invoice.md":             {Data: []byte("invoice")},
	"pdf-processing/templates/purchase-order.md":      {Data: []byte("po")},
	"pdf-processing/templates/regional/eu-invoice.md": {Data: []byte("eu invoice")},
	"git-workflow/SKILL.md": {Data: []byte(
		"---\nname: git-workflow\ndescription: Follow this team's Git conventions\n---\nbody\n")},
}

func loadTestRegistry(t *testing.T) *skills.Registry {
	t.Helper()
	r, err := skills.LoadFS(testFS, "acme")
	require.NoError(t, err)
	return r
}

func TestLoadFSEntries(t *testing.T) {
	r := loadTestRegistry(t)

	entries := r.Entries()
	require.Len(t, entries, 2)

	// Sorted by skill-path.
	assert.Equal(t, "skill://acme/git-workflow/SKILL.md", entries[0].URI)
	assert.Equal(t, "skill://acme/pdf-processing/SKILL.md", entries[1].URI)

	pdf := entries[1]
	assert.Equal(t, "pdf-processing", pdf.Frontmatter["name"])
	assert.Equal(t, map[string]any{"version": "2.1.0"}, pdf.Frontmatter["metadata"])

	// resources is complete: every file exactly once, SKILL.md included.
	uris := make([]string, 0, len(pdf.Resources))
	for _, res := range pdf.Resources {
		uris = append(uris, res.URI)
	}
	assert.Equal(t, []string{
		"skill://acme/pdf-processing/SKILL.md",
		"skill://acme/pdf-processing/references/FORMS.md",
		"skill://acme/pdf-processing/scripts/extract.py",
		"skill://acme/pdf-processing/templates/invoice.md",
		"skill://acme/pdf-processing/templates/purchase-order.md",
		"skill://acme/pdf-processing/templates/regional/eu-invoice.md",
	}, uris)
}

func TestRegistryGet(t *testing.T) {
	r := loadTestRegistry(t)

	s, ok := r.Get("skill://acme/pdf-processing/SKILL.md")
	require.True(t, ok)
	assert.Equal(t, "pdf-processing", s.Name)

	// Only the SKILL.md URI names a skill — supporting files and roots don't.
	for _, uri := range []string{
		"skill://acme/pdf-processing",
		"skill://acme/pdf-processing/references/FORMS.md",
		"skill://acme/unknown/SKILL.md",
	} {
		_, ok := r.Get(uri)
		assert.False(t, ok, uri)
	}
}

func TestRegistryDirectory(t *testing.T) {
	r := loadTestRegistry(t)

	t.Run("authority prefix lists skill roots", func(t *testing.T) {
		children, ok := r.Directory("skill://acme")
		require.True(t, ok)
		require.Len(t, children, 2)
		assert.Equal(t, "skill://acme/git-workflow", children[0].URI)
		assert.Equal(t, skills.DirectoryMIMEType, children[0].MIMEType)
		assert.Equal(t, "skill://acme/pdf-processing", children[1].URI)
	})

	t.Run("skill root lists files and subdirectories", func(t *testing.T) {
		children, ok := r.Directory("skill://acme/pdf-processing")
		require.True(t, ok)
		require.Len(t, children, 4)
		assert.Equal(t, "skill://acme/pdf-processing/SKILL.md", children[0].URI)
		assert.Equal(t, "text/markdown", children[0].MIMEType)
		// The SKILL.md resource carries frontmatter-derived metadata.
		assert.Equal(t, "pdf-processing", children[0].Name)
		assert.Equal(t, "Extract, fill, and assemble PDF documents", children[0].Description)
		assert.Equal(t, "skill://acme/pdf-processing/references", children[1].URI)
		assert.Equal(t, skills.DirectoryMIMEType, children[1].MIMEType)
		assert.Equal(t, "skill://acme/pdf-processing/scripts", children[2].URI)
		assert.Equal(t, "skill://acme/pdf-processing/templates", children[3].URI)
	})

	t.Run("subdirectory listing is not recursive", func(t *testing.T) {
		children, ok := r.Directory("skill://acme/pdf-processing/templates")
		require.True(t, ok)
		require.Len(t, children, 3)
		assert.Equal(t, "skill://acme/pdf-processing/templates/invoice.md", children[0].URI)
		assert.Equal(t, "invoice.md", children[0].Name)
		assert.Equal(t, "skill://acme/pdf-processing/templates/purchase-order.md", children[1].URI)
		assert.Equal(t, "skill://acme/pdf-processing/templates/regional", children[2].URI)
		assert.Equal(t, skills.DirectoryMIMEType, children[2].MIMEType)
	})

	t.Run("file and unknown URIs are not directories", func(t *testing.T) {
		for _, uri := range []string{
			"skill://acme/pdf-processing/SKILL.md",
			"skill://acme/nope",
			"skill://other",
		} {
			_, ok := r.Directory(uri)
			assert.False(t, ok, uri)
		}
	})
}

func TestLoadRejectsDuplicates(t *testing.T) {
	s1, err := skills.New("demo", []skills.File{{Path: "SKILL.md", Content: []byte(minimalSkillMD)}})
	require.NoError(t, err)
	// New validates name == final segment, so build the duplicate from the
	// same definition.
	s2, err := skills.New("demo", []skills.File{{Path: "SKILL.md", Content: []byte(minimalSkillMD)}})
	require.NoError(t, err)

	_, err = skills.Load(s1, s2)
	assert.ErrorContains(t, err, "duplicate")
}
