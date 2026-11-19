package skills

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// skillMD builds SKILL.md content with the given frontmatter lines. Tests use
// it rather than fixtures so that the exact bytes a digest covers are visible
// at the call site.
func skillMD(name, description string, extraFrontmatter ...string) []byte {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "name: %s\n", name)
	fmt.Fprintf(&b, "description: %s\n", description)
	for _, line := range extraFrontmatter {
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "# %s\n\nGuidance body.\n", name)
	return []byte(b.String())
}

// newTestSkill builds a valid skill for tests that need a well-formed one.
func newTestSkill(t *testing.T, skillPath string, extra ...File) *Skill {
	t.Helper()
	name := skillPath
	if _, last, ok := strings.CutLast(skillPath, "/"); ok {
		name = last
	}
	files := append([]File{{
		Path:    SkillFile,
		Content: skillMD(name, "Guidance for "+name+"."),
	}}, extra...)
	s, err := NewSkill(skillPath, files)
	require.NoError(t, err)
	return s
}

func TestParseURI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		raw       string
		wantSkill string
		wantFile  string
		wantErr   string
	}{
		{
			name:      "skill md resolves the skill path",
			raw:       "skill://example.com/writing/SKILL.md",
			wantSkill: "example.com/writing",
			wantFile:  SkillFile,
		},
		{
			name:      "multi segment prefix is server chosen",
			raw:       "skill://acme.dev/team/docs/writing/SKILL.md",
			wantSkill: "acme.dev/team/docs/writing",
			wantFile:  SkillFile,
		},
		{
			name:      "single segment path is permitted",
			raw:       "skill://writing/SKILL.md",
			wantSkill: "writing",
			wantFile:  SkillFile,
		},
		{
			name:      "path without a file parses as the skill root",
			raw:       "skill://example.com/writing",
			wantSkill: "example.com/writing",
			wantFile:  "",
		},
		{
			name:    "other schemes are rejected",
			raw:     "https://example.com/writing/SKILL.md",
			wantErr: "not a skill:// URI",
		},
		{
			name:    "trailing slash is rejected",
			raw:     "skill://example.com/writing/",
			wantErr: "trailing slash",
		},
		{
			name:    "empty segments are rejected",
			raw:     "skill://example.com//writing/SKILL.md",
			wantErr: "empty path segment",
		},
		{
			name:    "missing skill path is rejected",
			raw:     "skill://",
			wantErr: "missing skill path",
		},
		{
			name:    "uppercase final segment is rejected",
			raw:     "skill://example.com/Writing/SKILL.md",
			wantErr: "lowercase alphanumeric",
		},
		{
			name:    "relative segments are rejected",
			raw:     "skill://example.com/../writing/SKILL.md",
			wantErr: "relative segment",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			u, err := ParseURI(tc.raw)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantSkill, u.SkillPath)
			assert.Equal(t, tc.wantFile, u.FilePath)
			assert.Equal(t, URIScheme, u.Scheme)
		})
	}
}

// The skill root is written without a trailing slash and SKILL.md is always
// explicit, so the two are distinct URIs rather than aliases.
func TestURIRootHasNoTrailingSlashAndSkillFileIsExplicit(t *testing.T) {
	t.Parallel()

	u, err := ParseURI("skill://example.com/writing/SKILL.md")
	require.NoError(t, err)

	assert.Equal(t, "skill://example.com/writing", u.Root())
	assert.Equal(t, "skill://example.com/writing/SKILL.md", u.String())
	assert.NotEqual(t, u.Root(), u.String())
	assert.Equal(t, "writing", u.Name())
}

func TestSkillURIEscapesAndDecodesReservedPathCharacters(t *testing.T) {
	t.Parallel()

	const skillPath = "example.com/team space/writing"
	const filePath = "refs/quick start#draft?.md"
	want := "skill://example.com/team%20space/writing/refs/quick%20start%23draft%3F.md"

	assert.Equal(t, want, SkillURI(skillPath, filePath))
	assert.True(t, WithinSkill(skillPath, want))
	assert.False(t, WithinSkill(skillPath, want+"?download=1"))
	assert.False(t, WithinSkill(skillPath, want+"#draft"))
	assert.False(t, WithinSkill(skillPath, "skill://example.com/team%20space/writing/refs/a%2Fb.md"))

	parsed, err := ParseURI(SkillURI(skillPath, SkillFile))
	require.NoError(t, err)
	assert.Equal(t, skillPath, parsed.SkillPath)
	assert.Equal(t, SkillFile, parsed.FilePath)
}

func TestSkillPublishesReservedFilePathAsReadableResource(t *testing.T) {
	t.Parallel()

	content := []byte("# Quick start\n")
	skill, err := NewSkill("example.com/team space/writing", []File{
		{Path: SkillFile, Content: skillMD("writing", "Writing guidance.")},
		{Path: "refs/quick start#draft?.md", Content: content},
	})
	require.NoError(t, err)
	registry, err := NewRegistry(skill)
	require.NoError(t, err)

	files, err := registry.Files(t.Context())
	require.NoError(t, err)
	require.Len(t, files, 2)
	assert.Equal(t, "skill://example.com/team%20space/writing/refs/quick%20start%23draft%3F.md", files[1].URI)
	assert.Equal(t, content, files[1].File.Content)

	children, found, err := registry.ReadDirectory(t.Context(), "skill://example.com/team%20space")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, []string{"skill://example.com/team%20space/writing"}, childURIs(children))
}

func TestValidateName(t *testing.T) {
	t.Parallel()

	valid := []string{"writing", "code-review", "a", "plan-2-do", "x1"}
	for _, name := range valid {
		assert.NoError(t, ValidateName(name), name)
	}

	invalid := []string{"", "Writing", "code_review", "-lead", "trail-", "double--hyphen", "spaced name", strings.Repeat("a", MaxNameLength+1)}
	for _, name := range invalid {
		assert.Error(t, ValidateName(name), name)
	}
}

// Origin scoping: a skill's manifest may only name that skill's own files, so
// URIs belonging to another skill — or to the skill's own root directory — are
// not within it.
func TestWithinSkillIsOriginScoped(t *testing.T) {
	t.Parallel()

	const path = "example.com/writing"

	assert.True(t, WithinSkill(path, "skill://example.com/writing/SKILL.md"))
	assert.True(t, WithinSkill(path, "skill://example.com/writing/refs/style.md"))

	assert.False(t, WithinSkill(path, "skill://example.com/writing"), "the root is a directory, not a file of the skill")
	assert.False(t, WithinSkill(path, "skill://example.com/writing-guide/SKILL.md"), "a sibling with a shared prefix is a different skill")
	assert.False(t, WithinSkill(path, "skill://example.com/other/SKILL.md"))
	assert.False(t, WithinSkill(path, "skill://evil.example/writing/SKILL.md"), "a different origin is a different skill")
	assert.False(t, WithinSkill(path, "https://example.com/writing/SKILL.md"))
}

func TestParseFrontmatterPreservesEveryFieldVerbatim(t *testing.T) {
	t.Parallel()

	content := []byte("---\n" +
		"name: writing\n" +
		"description: Writing guidance.\n" +
		"version: \"1.2.3\"\n" +
		"license: MIT\n" +
		"metadata:\n" +
		"  audience: editors\n" +
		"keywords:\n" +
		"  - prose\n" +
		"  - style\n" +
		"---\n\n# Writing\n")

	fm, body, err := ParseFrontmatter(content)
	require.NoError(t, err)

	// Verbatim means every authored field survives, not a curated subset: a
	// host re-parses the fetched SKILL.md and compares field by field.
	assert.Equal(t, "writing", fm.Name())
	assert.Equal(t, "Writing guidance.", fm.Description())
	assert.Equal(t, "1.2.3", fm["version"])
	assert.Equal(t, "MIT", fm["license"])
	assert.Equal(t, map[string]any{"audience": "editors"}, fm["metadata"])
	assert.Equal(t, []any{"prose", "style"}, fm["keywords"])
	assert.Len(t, fm, 6)
	assert.Contains(t, body, "# Writing")
}

func TestParseFrontmatterErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{"no frontmatter", "# Writing\n"},
		{"unterminated frontmatter", "---\nname: writing\n"},
		{"malformed yaml", "---\nname: [unterminated\n---\n"},
		{"frontmatter is not a mapping", "---\n- writing\n---\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := ParseFrontmatter([]byte(tc.content))
			assert.Error(t, err)
		})
	}
}

func TestParseFrontmatterRejectsNonFiniteNumbers(t *testing.T) {
	t.Parallel()

	for _, value := range []string{".nan", ".inf", "-.inf"} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			content := []byte("---\nname: writing\ndescription: d\nmetadata:\n  score: " + value + "\n---\n")
			_, _, err := ParseFrontmatter(content)
			require.ErrorContains(t, err, "non-finite number")
		})
	}
}

// A checkout with Windows line endings must parse identically, while the digest
// keeps covering the raw bytes as served.
func TestParseFrontmatterNormalizesLineEndings(t *testing.T) {
	t.Parallel()

	crlf := []byte("---\r\nname: writing\r\ndescription: Writing guidance.\r\n---\r\n\r\n# Writing\r\n")
	fm, _, err := ParseFrontmatter(crlf)
	require.NoError(t, err)
	assert.Equal(t, "writing", fm.Name())
	assert.Equal(t, "Writing guidance.", fm.Description())
}

func TestFrontmatterValidateRequiresNameAndDescription(t *testing.T) {
	t.Parallel()

	assert.NoError(t, Frontmatter{"name": "writing", "description": "Guidance."}.Validate())
	assert.Error(t, Frontmatter{"description": "Guidance."}.Validate(), "name is required")
	assert.Error(t, Frontmatter{"name": "writing"}.Validate(), "description is required")
	assert.Error(t, Frontmatter{"name": "Writing", "description": "Guidance."}.Validate(), "name must satisfy the naming rules")
}

// The comparison a host performs after fetching SKILL.md is field-by-field, so
// any drift between the listed frontmatter and the file's own is detectable.
func TestFrontmatterEqualDetectsDrift(t *testing.T) {
	t.Parallel()

	base := Frontmatter{
		"name":        "writing",
		"description": "Writing guidance.",
		"metadata":    map[string]any{"audience": "editors"},
		"keywords":    []any{"prose"},
	}

	same := Frontmatter{
		"name":        "writing",
		"description": "Writing guidance.",
		"metadata":    map[string]any{"audience": "editors"},
		"keywords":    []any{"prose"},
	}
	assert.True(t, FrontmatterEqual(base, same))

	changedValue := Frontmatter{
		"name":        "writing",
		"description": "Different guidance.",
		"metadata":    map[string]any{"audience": "editors"},
		"keywords":    []any{"prose"},
	}
	assert.False(t, FrontmatterEqual(base, changedValue))

	changedNested := Frontmatter{
		"name":        "writing",
		"description": "Writing guidance.",
		"metadata":    map[string]any{"audience": "everyone"},
		"keywords":    []any{"prose"},
	}
	assert.False(t, FrontmatterEqual(base, changedNested), "nested drift must be caught")

	droppedField := Frontmatter{
		"name":        "writing",
		"description": "Writing guidance.",
		"metadata":    map[string]any{"audience": "editors"},
	}
	assert.False(t, FrontmatterEqual(base, droppedField), "a dropped field is drift")
}

func TestDigestFormat(t *testing.T) {
	t.Parallel()

	d := Digest([]byte("hello"))
	assert.True(t, strings.HasPrefix(d, DigestPrefix))
	assert.Equal(t, "sha256:2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", d)
	assert.NoError(t, ValidateDigest(d))

	// The digest covers raw bytes: an empty file still has one.
	assert.NoError(t, ValidateDigest(Digest(nil)))
}

func TestValidateDigestRejectsMalformedValues(t *testing.T) {
	t.Parallel()

	valid := Digest([]byte("hello"))
	invalid := []string{
		"",
		strings.TrimPrefix(valid, DigestPrefix), // missing algorithm prefix
		"sha512:" + strings.TrimPrefix(valid, DigestPrefix), // wrong algorithm
		strings.ToUpper(valid),                              // uppercase hex
		DigestPrefix + strings.Repeat("a", 63),              // too short
		DigestPrefix + strings.Repeat("a", 65),              // too long
		DigestPrefix + strings.Repeat("g", 64),              // not hex
		DigestPrefix + " " + strings.Repeat("a", 63),        // whitespace
	}
	for _, d := range invalid {
		assert.Error(t, ValidateDigest(d), d)
	}
}

// A size mismatch is a verification failure equivalent to a digest mismatch, so
// it is reported even when the digest itself would have matched.
func TestVerify(t *testing.T) {
	t.Parallel()

	content := []byte("# Writing\n")
	want := EntryResource{
		URI:    "skill://example.com/writing/SKILL.md",
		Digest: Digest(content),
		Size:   int64(len(content)),
	}

	require.NoError(t, Verify(content, want))

	t.Run("digest mismatch", func(t *testing.T) {
		t.Parallel()
		tampered := []byte("# Rewritten\n")
		bad := want
		bad.Size = int64(len(tampered))
		err := Verify(tampered, bad)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "digest")
	})

	t.Run("size mismatch", func(t *testing.T) {
		t.Parallel()
		bad := want
		bad.Size = want.Size + 1
		err := Verify(content, bad)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "size")
	})

	t.Run("truncated read with recomputed digest still fails on size", func(t *testing.T) {
		t.Parallel()
		truncated := content[:len(content)-1]
		err := Verify(truncated, want)
		assert.Error(t, err, "a short read must fail before the digest is trusted")
	})
}

func TestIsTextAndMIMEType(t *testing.T) {
	t.Parallel()

	assert.True(t, IsText([]byte("plain text")))
	assert.False(t, IsText([]byte{0x00, 0x01, 0x02}), "NUL bytes mean binary")
	assert.False(t, IsText([]byte{0xff, 0xfe, 0xfd}), "invalid UTF-8 means binary")

	assert.Equal(t, SkillMIMEType, FileMIMEType("SKILL.md", []byte("# hi")))
	assert.Equal(t, SkillMIMEType, FileMIMEType("refs/style.md", []byte("# hi")))
	assert.Contains(t, FileMIMEType("data.json", []byte("{}")), "json")
	assert.Equal(t, "application/octet-stream", FileMIMEType("blob.bin", []byte{0x00, 0x01}))
}

func TestFileMediaTypeHonoursExplicitOverride(t *testing.T) {
	t.Parallel()

	f := File{Path: "data.bin", Content: []byte("{}"), MIMEType: "application/json"}
	assert.Equal(t, "application/json", f.MediaType())

	inferred := File{Path: "SKILL.md", Content: []byte("# hi")}
	assert.Equal(t, SkillMIMEType, inferred.MediaType())
}
