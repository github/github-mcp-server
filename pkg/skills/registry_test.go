package skills

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSkill(t *testing.T) {
	t.Parallel()

	s, err := NewSkill("example.com/writing", []File{
		{Path: SkillFile, Content: skillMD("writing", "Writing guidance.")},
		{Path: "refs/style.md", Content: []byte("# Style")},
	})
	require.NoError(t, err)

	assert.Equal(t, "writing", s.Name())
	assert.Equal(t, "Writing guidance.", s.Description())
	assert.Equal(t, "skill://example.com/writing/SKILL.md", s.URI())
	assert.Equal(t, "skill://example.com/writing", s.Root())

	// Files are ordered by path so that entries and pages are reproducible.
	require.Len(t, s.Files(), 2)
	assert.Equal(t, SkillFile, s.Files()[0].Path)
	assert.Equal(t, "refs/style.md", s.Files()[1].Path)

	f, ok := s.File("refs/style.md")
	require.True(t, ok)
	assert.Equal(t, []byte("# Style"), f.Content)

	_, ok = s.File("missing.md")
	assert.False(t, ok)
}

func TestNewSkillRejectsMalformedInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		skillPath string
		files     []File
		wantErr   string
	}{
		{
			name:      "no files",
			skillPath: "example.com/writing",
			wantErr:   "no files",
		},
		{
			name:      "missing SKILL.md",
			skillPath: "example.com/writing",
			files:     []File{{Path: "refs/style.md", Content: []byte("# Style")}},
			wantErr:   "missing SKILL.md",
		},
		{
			name:      "frontmatter name must match the final path segment",
			skillPath: "example.com/writing",
			files:     []File{{Path: SkillFile, Content: skillMD("editing", "d")}},
			wantErr:   "does not match final path segment",
		},
		{
			name:      "frontmatter must carry a description",
			skillPath: "example.com/writing",
			files:     []File{{Path: SkillFile, Content: []byte("---\nname: writing\n---\nbody\n")}},
			wantErr:   "missing a description",
		},
		{
			name:      "missing frontmatter",
			skillPath: "example.com/writing",
			files:     []File{{Path: SkillFile, Content: []byte("# Writing\n")}},
			wantErr:   "frontmatter",
		},
		{
			name:      "duplicate file paths",
			skillPath: "example.com/writing",
			files: []File{
				{Path: SkillFile, Content: skillMD("writing", "d")},
				{Path: "refs/style.md", Content: []byte("a")},
				{Path: "refs/style.md", Content: []byte("b")},
			},
			wantErr: "listed more than once",
		},
		{
			name:      "invalid skill path",
			skillPath: "example.com/Writing",
			files:     []File{{Path: SkillFile, Content: skillMD("Writing", "d")}},
			wantErr:   "name",
		},
		{
			name:      "path traversal in a file path",
			skillPath: "example.com/writing",
			files: []File{
				{Path: SkillFile, Content: skillMD("writing", "d")},
				{Path: "../escape.md", Content: []byte("x")},
			},
			wantErr: "..",
		},
		{
			name:      "absolute file path",
			skillPath: "example.com/writing",
			files: []File{
				{Path: SkillFile, Content: skillMD("writing", "d")},
				{Path: "/etc/passwd", Content: []byte("x")},
			},
			wantErr: "path",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewSkill(tc.skillPath, tc.files)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

// The per-skill limits are enforced at construction so a registry can never
// serve an entry a conforming host would be obliged to refuse.
func TestNewSkillEnforcesLimits(t *testing.T) {
	t.Parallel()

	t.Run("too many files", func(t *testing.T) {
		t.Parallel()
		files := []File{{Path: SkillFile, Content: skillMD("writing", "d")}}
		for i := len(files); i <= MaxResourcesPerSkill; i++ {
			files = append(files, File{Path: "refs/f" + itoa(i) + ".md", Content: []byte("x")})
		}
		require.Greater(t, len(files), MaxResourcesPerSkill)
		_, err := NewSkill("example.com/writing", files)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds the limit")
	})

	t.Run("too many bytes", func(t *testing.T) {
		t.Parallel()
		_, err := NewSkill("example.com/writing", []File{
			{Path: SkillFile, Content: skillMD("writing", "d")},
			{Path: "big.bin", Content: make([]byte, MaxSkillBytes)},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds the limit")
	})
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for ; i > 0; i /= 10 {
		b = append([]byte{byte('0' + i%10)}, b...)
	}
	return string(b)
}

// Every entry a skill publishes must satisfy the same rules a host applies to
// it, including the requirement that the manifest list the skill's own
// SKILL.md.
func TestSkillEntryIsSelfConsistent(t *testing.T) {
	t.Parallel()

	skillDoc := skillMD("writing", "Writing guidance.")
	s, err := NewSkill("example.com/writing", []File{
		{Path: SkillFile, Content: skillDoc},
		{Path: "refs/style.md", Content: []byte("# Style")},
	})
	require.NoError(t, err)

	e := s.Entry()
	require.NoError(t, e.Validate())

	assert.Equal(t, s.URI(), e.URI)
	assert.False(t, e.Dynamic)
	require.Len(t, e.Resources, 2)
	assert.Equal(t, EntryResource{
		URI:    "skill://example.com/writing/SKILL.md",
		Digest: Digest(skillDoc),
		Size:   int64(len(skillDoc)),
	}, e.Resources[0])
	assert.Equal(t, int64(len(skillDoc)+len("# Style")), e.TotalSize())

	// The published frontmatter is the file's own, verbatim.
	parsed, _, err := ParseFrontmatter(skillDoc)
	require.NoError(t, err)
	assert.True(t, FrontmatterEqual(parsed, e.Frontmatter))
}

func TestDynamicSkillPublishesTheMarkerInsteadOfDigests(t *testing.T) {
	t.Parallel()

	s, err := NewSkill("example.com/report", []File{
		{Path: SkillFile, Content: skillMD("report", "Generated report guidance.")},
	})
	require.NoError(t, err)
	s.Dynamic = true

	e := s.Entry()
	require.NoError(t, e.Validate())
	assert.True(t, e.Dynamic)
	assert.Empty(t, e.Resources)
}

func TestRegistryListAndGet(t *testing.T) {
	t.Parallel()

	r := newTestRegistry(t)

	entries, err := r.List(context.Background())
	require.NoError(t, err)
	require.Len(t, entries, 3)
	// Ordered by skill path so that page boundaries are reproducible.
	assert.Equal(t, []string{
		"skill://example.com/billing/refunds/SKILL.md",
		"skill://example.com/support/refunds/SKILL.md",
		"skill://example.com/writing/SKILL.md",
	}, uris(entries))
	for _, e := range entries {
		assert.NoError(t, e.Validate(), e.URI)
	}

	got, ok, err := r.Get(context.Background(), "skill://example.com/writing/SKILL.md")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "skill://example.com/writing/SKILL.md", got.URI)

	_, ok, err = r.Get(context.Background(), "skill://example.com/missing/SKILL.md")
	require.NoError(t, err)
	assert.False(t, ok, "an unknown URI is reported as not found, not as an error")

	// The skill root is not the SKILL.md; the file is always addressed
	// explicitly.
	_, ok, err = r.Get(context.Background(), "skill://example.com/writing")
	require.NoError(t, err)
	assert.False(t, ok)
}

// Names are labels, not identifiers: two skills at different paths may share a
// final segment, and a registry must serve both.
func TestRegistryAllowsDuplicateNamesButNotDuplicatePaths(t *testing.T) {
	t.Parallel()

	r := newTestRegistry(t)
	entries, err := r.List(context.Background())
	require.NoError(t, err)

	var refunds int
	for _, e := range entries {
		if e.Frontmatter.Name() == "refunds" {
			refunds++
		}
	}
	assert.Equal(t, 2, refunds, "both skills named refunds must be served")

	dup := newTestSkill(t, "example.com/writing")
	_, err = NewRegistry(newTestSkill(t, "example.com/writing"), dup)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate skill path")
}

func TestRegistryRejectsNilSkills(t *testing.T) {
	t.Parallel()

	_, err := NewRegistry(newTestSkill(t, "example.com/writing"), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil skill")
}

// A nested skill's files are ordinary supporting files of the enclosing skill,
// so the same URI is legitimately claimed twice and served once.
func TestRegistryServesNestedSkills(t *testing.T) {
	t.Parallel()

	nestedDoc := skillMD("refunds", "Refund handling.")
	outer, err := NewSkill("example.com/billing", []File{
		{Path: SkillFile, Content: skillMD("billing", "Billing guidance.")},
		{Path: "refunds/SKILL.md", Content: nestedDoc},
	})
	require.NoError(t, err)
	inner, err := NewSkill("example.com/billing/refunds", []File{
		{Path: SkillFile, Content: nestedDoc},
	})
	require.NoError(t, err)

	r, err := NewRegistry(outer, inner)
	require.NoError(t, err)

	// Publication is flat: two ordinary entries, nothing marking the nesting.
	entries, err := r.List(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{
		"skill://example.com/billing/SKILL.md",
		"skill://example.com/billing/refunds/SKILL.md",
	}, uris(entries))

	// The nested SKILL.md is served once, described by the skill that roots
	// at its directory.
	files, err := r.Files(context.Background())
	require.NoError(t, err)
	var matched int
	for _, f := range files {
		if f.URI == "skill://example.com/billing/refunds/SKILL.md" {
			matched++
			assert.Equal(t, "example.com/billing/refunds", f.SkillPath)
			assert.Equal(t, "refunds", f.Name)
		}
	}
	assert.Equal(t, 1, matched)

	t.Run("conflicting content is rejected", func(t *testing.T) {
		t.Parallel()
		conflicting, err := NewSkill("example.com/billing", []File{
			{Path: SkillFile, Content: skillMD("billing", "Billing guidance.")},
			{Path: "refunds/SKILL.md", Content: skillMD("refunds", "A different description.")},
		})
		require.NoError(t, err)
		_, err = NewRegistry(conflicting, inner)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "different content")
	})
}

func TestRegistryReadDirectory(t *testing.T) {
	t.Parallel()

	r := newTestRegistry(t)
	ctx := context.Background()

	t.Run("skill root lists direct children only", func(t *testing.T) {
		t.Parallel()
		children, ok, err := r.ReadDirectory(ctx, "skill://example.com/writing")
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, []string{
			"skill://example.com/writing/SKILL.md",
			"skill://example.com/writing/refs",
		}, childURIs(children))

		for _, c := range children {
			if c.Name == "refs" {
				assert.True(t, c.IsDir())
				assert.Equal(t, DirectoryMIMEType, c.MIMEType)
				assert.Zero(t, c.Size, "a directory has no byte length")
			} else {
				assert.False(t, c.IsDir())
				assert.Equal(t, SkillMIMEType, c.MIMEType)
				assert.NotZero(t, c.Size)
			}
		}
	})

	t.Run("subdirectory", func(t *testing.T) {
		t.Parallel()
		children, ok, err := r.ReadDirectory(ctx, "skill://example.com/writing/refs")
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, []string{"skill://example.com/writing/refs/style.md"}, childURIs(children))
	})

	t.Run("ancestor directories are traversable", func(t *testing.T) {
		t.Parallel()
		children, ok, err := r.ReadDirectory(ctx, "skill://example.com")
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, []string{
			"skill://example.com/billing",
			"skill://example.com/support",
			"skill://example.com/writing",
		}, childURIs(children))
		for _, c := range children {
			assert.True(t, c.IsDir())
		}
	})

	t.Run("a file is not a directory", func(t *testing.T) {
		t.Parallel()
		_, ok, err := r.ReadDirectory(ctx, "skill://example.com/writing/SKILL.md")
		require.NoError(t, err)
		assert.False(t, ok, "a file URI is invalid params, not an empty directory")
	})

	t.Run("unknown directory", func(t *testing.T) {
		t.Parallel()
		_, ok, err := r.ReadDirectory(ctx, "skill://example.com/nowhere")
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("an empty directory is not an error", func(t *testing.T) {
		t.Parallel()
		empty, err := NewSkill("example.com/deep", []File{
			{Path: SkillFile, Content: skillMD("deep", "d")},
			{Path: "a/b/c.md", Content: []byte("c")},
		})
		require.NoError(t, err)
		reg, err := NewRegistry(empty)
		require.NoError(t, err)

		children, ok, err := reg.ReadDirectory(ctx, "skill://example.com/deep/a")
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, []string{"skill://example.com/deep/a/b"}, childURIs(children))
	})

	t.Run("returned slices are copies", func(t *testing.T) {
		t.Parallel()
		first, ok, err := r.ReadDirectory(ctx, "skill://example.com/writing")
		require.NoError(t, err)
		require.True(t, ok)
		first[0].URI = "mutated"

		second, ok, err := r.ReadDirectory(ctx, "skill://example.com/writing")
		require.NoError(t, err)
		require.True(t, ok)
		assert.NotEqual(t, "mutated", second[0].URI)
	})
}

func TestRegistryFiles(t *testing.T) {
	t.Parallel()

	r := newTestRegistry(t)
	files, err := r.Files(context.Background())
	require.NoError(t, err)
	require.Len(t, files, 4)

	// Ordered by URI, and a SKILL.md carries the skill's own name and
	// description as its resource metadata.
	assert.Equal(t, "skill://example.com/billing/refunds/SKILL.md", files[0].URI)
	assert.Equal(t, "refunds", files[0].Name)
	assert.NotEmpty(t, files[0].Description)

	for _, f := range files {
		if strings.HasSuffix(f.URI, "/refs/style.md") {
			assert.Equal(t, "style.md", f.Name, "a supporting file is named after its own path")
			assert.Empty(t, f.Description)
		}
	}
}

func TestRegistrySkillLookup(t *testing.T) {
	t.Parallel()

	r := newTestRegistry(t)
	assert.Equal(t, 3, r.Len())

	s, ok := r.Skill("example.com/writing")
	require.True(t, ok)
	assert.Equal(t, "writing", s.Name())

	_, ok = r.Skill("example.com/missing")
	assert.False(t, ok)
}

func TestEmptyRegistryIsValid(t *testing.T) {
	t.Parallel()

	r, err := NewRegistry()
	require.NoError(t, err)
	assert.Zero(t, r.Len())

	entries, err := r.List(context.Background())
	require.NoError(t, err)
	assert.Empty(t, entries)

	files, err := r.Files(context.Background())
	require.NoError(t, err)
	assert.Empty(t, files)
}

// newTestRegistry builds a registry with two skills sharing the name "refunds"
// at different paths, plus one with a supporting file in a subdirectory.
func newTestRegistry(t *testing.T) *Registry {
	t.Helper()

	writing, err := NewSkill("example.com/writing", []File{
		{Path: SkillFile, Content: skillMD("writing", "Writing guidance.")},
		{Path: "refs/style.md", Content: []byte("# Style")},
	})
	require.NoError(t, err)

	r, err := NewRegistry(
		writing,
		newTestSkill(t, "example.com/billing/refunds"),
		newTestSkill(t, "example.com/support/refunds"),
	)
	require.NoError(t, err)
	return r
}

func uris(entries []Entry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.URI)
	}
	return out
}

func childURIs(children []Child) []string {
	out := make([]string, 0, len(children))
	for _, c := range children {
		out = append(out, c.URI)
	}
	return out
}
