package skills

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistryForTools(t *testing.T) {
	t.Parallel()
	skill := newTestSkill(t, "example.com/writing")
	skill.RequiredTools = [][]string{{"read", "read_granular"}, {"write"}}
	skill.OptionalTools = []string{"extra"}
	source, err := NewRegistry(skill)
	require.NoError(t, err)

	for _, tt := range []struct {
		name  string
		tools map[string]bool
		want  int
	}{
		{"missing core action", map[string]bool{"read": true, "extra": true}, 0},
		{"all required", map[string]bool{"read": true, "write": true}, 1},
		{"alternative", map[string]bool{"read_granular": true, "write": true}, 1},
		{"no tools", map[string]bool{}, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			filtered, err := source.ForTools(func(name string) bool { return tt.tools[name] })
			require.NoError(t, err)
			assert.Equal(t, tt.want, filtered.Len())
			_, found, err := filtered.Get(context.Background(), skill.URI())
			require.NoError(t, err)
			assert.Equal(t, tt.want == 1, found)
			files, err := filtered.Files(context.Background())
			require.NoError(t, err)
			assert.Len(t, files, tt.want)
		})
	}
	assert.Equal(t, 1, source.Len())
	_, err = source.ForTools(nil)
	require.Error(t, err)
}

func TestRegistryForToolsDoesNotLeakWithheldNestedSkills(t *testing.T) {
	t.Parallel()
	child := newTestSkill(t, "example.com/parent/child")
	child.RequiredTools = [][]string{{"write"}}
	childFile, ok := child.File(SkillFile)
	require.True(t, ok)
	parent := newTestSkill(t, "example.com/parent", File{Path: "child/SKILL.md", Content: childFile.Content})
	source, err := NewRegistry(parent, child)
	require.NoError(t, err)
	filtered, err := source.ForTools(func(string) bool { return false })
	require.NoError(t, err)
	assert.Zero(t, filtered.Len())
	files, err := filtered.Files(context.Background())
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestRegistryWithSkills(t *testing.T) {
	t.Parallel()
	source, err := NewRegistry(newTestSkill(t, "example.com/first"))
	require.NoError(t, err)
	second := newTestSkill(t, "example.com/second")
	combined, err := source.WithSkills(second)
	require.NoError(t, err)
	assert.Equal(t, 2, combined.Len())
	assert.Equal(t, 1, source.Len())
	_, err = combined.WithSkills(second)
	require.Error(t, err)
}

func TestModernSkillResponseCacheFields(t *testing.T) {
	t.Parallel()
	source, err := NewRegistry(newTestSkill(t, "example.com/writing"))
	require.NoError(t, err)
	p := NewPublisher(source)
	list, err := p.handleList(modernCtx(), nil, &ListSkillsParams{})
	require.NoError(t, err)
	get, err := p.handleGet(modernCtx(), nil, &GetSkillParams{URI: "skill://example.com/writing/SKILL.md"})
	require.NoError(t, err)
	directory, err := p.handleReadDirectory(modernCtx(), nil, &ReadDirectoryParams{URI: "skill://example.com/writing"})
	require.NoError(t, err)
	for _, result := range []any{list, get, directory} {
		raw, err := json.Marshal(result)
		require.NoError(t, err)
		var wire map[string]any
		require.NoError(t, json.Unmarshal(raw, &wire))
		assert.Equal(t, "complete", wire["resultType"])
		assert.Equal(t, float64(ListTTL), wire["ttlMs"])
		assert.Equal(t, "private", wire["cacheScope"])
	}
}
