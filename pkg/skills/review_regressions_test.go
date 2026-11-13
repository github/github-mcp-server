package skills

import (
	"context"
	"encoding/json"
	"math"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkillResourceHandlerIsPrivate(t *testing.T) {
	t.Parallel()
	result, err := resourceHandler(ResourceFile{
		URI: "skill://example.com/writing/SKILL.md", File: File{Content: skillMD("writing", "d")},
	})(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, "private", result.CacheScope)
}

func TestEntrySizeCannotOverflowValidation(t *testing.T) {
	t.Parallel()
	e := validEntry()
	e.Resources[0].Size = math.MaxInt64
	e.Resources = append(e.Resources, EntryResource{
		URI: "skill://example.com/writing/other.md", Digest: Digest(nil), Size: math.MaxInt64,
	})
	require.ErrorContains(t, e.Validate(), "exceeds the limit")
}

func TestEntryRejectsRelativeResourceSegments(t *testing.T) {
	t.Parallel()
	for _, suffix := range []string{"../other/SKILL.md", "./style.md", "refs/../../other.md", "", "/style.md"} {
		t.Run(suffix, func(t *testing.T) {
			e := validEntry()
			uri := "skill://example.com/writing/" + suffix
			e.Resources = append(e.Resources, entryResource(uri, nil))
			assert.False(t, WithinSkill("example.com/writing", uri))
			require.ErrorContains(t, e.Validate(), "outside the skill directory")
		})
	}
}

func TestSkillCopiesCallerOwnedFileContentAndFiles(t *testing.T) {
	t.Parallel()

	main := skillMD("writing", "Writing guidance.")
	originalMain := slices.Clone(main)
	supporting := []byte("# Original\n")
	skill, err := NewSkill("example.com/writing", []File{
		{Path: SkillFile, Content: main},
		{Path: "refs/style.md", Content: supporting},
	})
	require.NoError(t, err)

	main[0] = 'x'
	supporting[0] = 'x'
	gotFiles := skill.Files()
	require.Len(t, gotFiles, 2)
	assert.Equal(t, originalMain, gotFiles[0].Content)
	assert.Equal(t, []byte("# Original\n"), gotFiles[1].Content)
	gotFiles[1].Path = "renamed.md"
	gotFiles[1].Content[0] = 'x'

	gotFile, ok := skill.File("refs/style.md")
	require.True(t, ok)
	assert.Equal(t, []byte("# Original\n"), gotFile.Content)
	gotFile.Content[0] = 'x'

	assert.Equal(t, "refs/style.md", skill.Files()[1].Path)
	assert.Equal(t, originalMain, skill.Files()[0].Content)
	assert.Equal(t, []byte("# Original\n"), skill.Files()[1].Content)
	require.NoError(t, skill.Entry().Validate())
}

func TestFrontmatterNumbersMatchJSONWithoutLosingPrecision(t *testing.T) {
	t.Parallel()
	parsed, _, err := ParseFrontmatter([]byte("---\nname: writing\ndescription: d\nversion: 2\nmetadata:\n  priority: 1\n  weights: [0.5, 2]\n---\n"))
	require.NoError(t, err)
	raw, err := json.Marshal(parsed)
	require.NoError(t, err)
	var wire Frontmatter
	require.NoError(t, json.Unmarshal(raw, &wire))
	assert.True(t, FrontmatterEqual(parsed, wire))
	assert.True(t, FrontmatterEqual(wire, parsed))
	wire["version"] = float64(3)
	assert.False(t, FrontmatterEqual(parsed, wire))
	assert.False(t, FrontmatterEqual(Frontmatter{"n": int64(9007199254740993)}, Frontmatter{"n": float64(9007199254740992)}))
	assert.False(t, FrontmatterEqual(Frontmatter{"n": 2}, Frontmatter{"n": "2"}))
	assert.False(t, FrontmatterEqual(Frontmatter{"n": math.NaN()}, Frontmatter{"n": math.NaN()}))
	assert.True(t, FrontmatterEqual(Frontmatter{"n": int64(9007199254740993)}, Frontmatter{"n": json.Number("9007199254740993")}))
}

func TestRegistryRejectsFileDirectoryURICollision(t *testing.T) {
	t.Parallel()

	parent := newTestSkill(t, "example.com/parent", File{
		Path: "child", Content: []byte("a file, not a directory"),
	})
	child := newTestSkill(t, "example.com/parent/child")

	_, err := NewRegistry(parent, child)
	require.ErrorContains(t, err, "both a file and a directory")
}

func TestRegistryRejectsIncompleteEnclosingManifest(t *testing.T) {
	t.Parallel()
	child := newTestSkill(t, "example.com/parent/child", File{Path: "refs/note.md", Content: []byte("note")})
	childMain, ok := child.File(SkillFile)
	require.True(t, ok)
	for _, partial := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing child", true: "missing supporting file"}[partial], func(t *testing.T) {
			var files []File
			if partial {
				files = append(files, File{Path: "child/SKILL.md", Content: childMain.Content})
			}
			parent := newTestSkill(t, "example.com/parent", files...)
			for _, order := range [][]*Skill{{parent, child}, {child, parent}} {
				_, err := NewRegistry(order...)
				require.ErrorContains(t, err, "enclosing manifest is missing")
			}
		})
	}
	parentFiles := make([]File, 0, len(child.Files()))
	for _, file := range child.Files() {
		file.Path = "child/" + file.Path
		parentFiles = append(parentFiles, file)
	}
	parent := newTestSkill(t, "example.com/parent", parentFiles...)
	registry, err := NewRegistry(parent, child)
	require.NoError(t, err)
	_, err = registry.WithSkills(newTestSkill(t, "example.com/parent/another"))
	require.ErrorContains(t, err, "enclosing manifest is missing")
	dynamic := newTestSkill(t, "example.com/parent")
	dynamic.Dynamic = true
	_, err = NewRegistry(dynamic, child)
	require.NoError(t, err, "dynamic entries do not claim a complete manifest")
}
