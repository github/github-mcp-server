package skills

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// entryResource builds a manifest entry for content served at uri.
func entryResource(uri string, content []byte) EntryResource {
	return EntryResource{URI: uri, Digest: Digest(content), Size: int64(len(content))}
}

// validEntry is a well-formed single-file entry that tests mutate to isolate
// one rule at a time.
func validEntry() Entry {
	const uri = "skill://example.com/writing/SKILL.md"
	content := skillMD("writing", "Writing guidance.")
	return Entry{
		URI:         uri,
		Frontmatter: Frontmatter{"name": "writing", "description": "Writing guidance."},
		Resources:   []EntryResource{entryResource(uri, content)},
	}
}

func TestEntryValidate(t *testing.T) {
	t.Parallel()

	require.NoError(t, validEntry().Validate())

	tests := []struct {
		name    string
		mutate  func(*Entry)
		wantErr string
	}{
		{
			name:    "empty uri",
			mutate:  func(e *Entry) { e.URI = "" },
			wantErr: "URI is empty",
		},
		{
			name:    "uri must address SKILL.md",
			mutate:  func(e *Entry) { e.URI = "skill://example.com/writing" },
			wantErr: "must address SKILL.md",
		},
		{
			name:    "frontmatter name must match the final path segment",
			mutate:  func(e *Entry) { e.Frontmatter["name"] = "other" },
			wantErr: "does not match frontmatter name",
		},
		{
			name:    "frontmatter requires a description",
			mutate:  func(e *Entry) { delete(e.Frontmatter, "description") },
			wantErr: "missing a description",
		},
		{
			name:    "manifest must include the skill's own SKILL.md",
			mutate:  func(e *Entry) { e.Resources[0].URI = "skill://example.com/writing/refs/style.md" },
			wantErr: "does not include the skill's own SKILL.md",
		},
		{
			name: "resources may not be listed twice",
			mutate: func(e *Entry) {
				e.Resources = append(e.Resources, e.Resources[0])
			},
			wantErr: "listed more than once",
		},
		{
			name:    "malformed digests are rejected",
			mutate:  func(e *Entry) { e.Resources[0].Digest = "sha256:nothex" },
			wantErr: "64 lowercase hexadecimal",
		},
		{
			name:    "negative sizes are rejected",
			mutate:  func(e *Entry) { e.Resources[0].Size = -1 },
			wantErr: "negative size",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := validEntry()
			tc.mutate(&e)
			err := e.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

// An absent resources field is not an empty manifest to be tolerated: the entry
// is invalid and a host must not load the skill.
func TestEntryMissingResourcesIsInvalid(t *testing.T) {
	t.Parallel()

	e := validEntry()
	e.Resources = nil

	err := e.Validate()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMissingResources)
}

// Origin scoping stops one skill's manifest from claiming another skill's
// files, whether the other skill is on the same server or a different one.
func TestEntryRejectsResourcesOutsideTheSkill(t *testing.T) {
	t.Parallel()

	outside := []string{
		"skill://example.com/other/SKILL.md",
		"skill://example.com/writing-guide/style.md",
		"skill://evil.example/writing/style.md",
		"https://example.com/writing/style.md",
		"skill://example.com/writing",
	}

	for _, uri := range outside {
		t.Run(uri, func(t *testing.T) {
			t.Parallel()
			e := validEntry()
			e.Resources = append(e.Resources, entryResource(uri, []byte("x")))
			err := e.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "outside the skill directory")
		})
	}
}

// Both limits are computed from the entry's own metadata, so a host can reject
// an oversized skill before fetching a single byte of it.
func TestEntryLimitsAreEnforcedFromMetadataAlone(t *testing.T) {
	t.Parallel()

	t.Run("resource count at the limit is accepted", func(t *testing.T) {
		t.Parallel()
		e := entryWithResourceCount(MaxResourcesPerSkill)
		assert.NoError(t, e.Validate())
	})

	t.Run("resource count above the limit is rejected", func(t *testing.T) {
		t.Parallel()
		e := entryWithResourceCount(MaxResourcesPerSkill + 1)
		err := e.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), fmt.Sprintf("exceeds the limit of %d", MaxResourcesPerSkill))
	})

	t.Run("total size at the limit is accepted", func(t *testing.T) {
		t.Parallel()
		e := validEntry()
		e.Resources[0].Size = MaxSkillBytes
		assert.NoError(t, e.Validate())
		assert.Equal(t, MaxSkillBytes, e.TotalSize())
	})

	t.Run("total size above the limit is rejected", func(t *testing.T) {
		t.Parallel()
		e := validEntry()
		e.Resources[0].Size = MaxSkillBytes + 1
		err := e.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds the limit")
	})

	t.Run("total size is summed across resources", func(t *testing.T) {
		t.Parallel()
		e := validEntry()
		half := MaxSkillBytes/2 + 1
		e.Resources[0].Size = half
		e.Resources = append(e.Resources, EntryResource{
			URI:    "skill://example.com/writing/refs/style.md",
			Digest: Digest([]byte("style")),
			Size:   half,
		})
		require.Greater(t, e.TotalSize(), MaxSkillBytes)
		assert.Error(t, e.Validate())
	})
}

// SKILL.md counts towards the 512-resource limit, so the boundary case is
// constructed with it included.
func entryWithResourceCount(n int) Entry {
	e := validEntry()
	for i := len(e.Resources); i < n; i++ {
		uri := fmt.Sprintf("skill://example.com/writing/refs/file-%d.md", i)
		e.Resources = append(e.Resources, entryResource(uri, []byte(uri)))
	}
	return e
}

// A dynamic skill is deliberately unverifiable rather than malformed: it
// publishes the marker instead of a manifest, and must not publish both.
func TestDynamicEntry(t *testing.T) {
	t.Parallel()

	e := validEntry()
	e.Resources = nil
	e.Dynamic = true
	require.NoError(t, e.Validate())

	raw, err := json.Marshal(e)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"resources":"dynamic"`)

	var round Entry
	require.NoError(t, json.Unmarshal(raw, &round))
	assert.True(t, round.Dynamic)
	assert.Empty(t, round.Resources)

	t.Run("dynamic and a manifest are mutually exclusive", func(t *testing.T) {
		t.Parallel()
		both := validEntry()
		both.Dynamic = true
		assert.Error(t, both.Validate())
		_, err := json.Marshal(both)
		assert.Error(t, err)
	})
}

func TestEntryMarshalAlwaysEmitsResources(t *testing.T) {
	t.Parallel()

	e := validEntry()
	raw, err := json.Marshal(e)
	require.NoError(t, err)

	var wire map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &wire))
	require.Contains(t, wire, "resources")
	require.Contains(t, wire, "uri")
	require.Contains(t, wire, "frontmatter")

	// An entry built by hand with no manifest is invalid, but it must still
	// never serialize as null: null is not one of the two permitted forms.
	empty := Entry{URI: e.URI, Frontmatter: e.Frontmatter}
	raw, err = json.Marshal(empty)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"resources":[]`)
	assert.NotContains(t, string(raw), `"resources":null`)
}

func TestEntryUnmarshalRejectsInvalidResourceForms(t *testing.T) {
	t.Parallel()

	const head = `{"uri":"skill://example.com/writing/SKILL.md","frontmatter":{"name":"writing","description":"d"}`

	tests := []struct {
		name string
		body string
	}{
		{"absent", head + `}`},
		{"null", head + `,"resources":null}`},
		{"unknown string marker", head + `,"resources":"static"}`},
		{"object", head + `,"resources":{"uri":"x"}}`},
		{"number", head + `,"resources":7}`},
		{"boolean", head + `,"resources":true}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var e Entry
			err := json.Unmarshal([]byte(tc.body), &e)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrMissingResources)
		})
	}
}

func TestEntryRoundTripsThroughJSON(t *testing.T) {
	t.Parallel()

	e := validEntry()
	e.Frontmatter["metadata"] = map[string]any{"audience": "editors"}
	e.Resources = append(e.Resources, entryResource("skill://example.com/writing/refs/style.md", []byte("style")))

	raw, err := json.Marshal(e)
	require.NoError(t, err)

	var round Entry
	require.NoError(t, json.Unmarshal(raw, &round))

	assert.Equal(t, e.URI, round.URI)
	assert.Equal(t, e.Resources, round.Resources)
	assert.False(t, round.Dynamic)
	assert.True(t, FrontmatterEqual(e.Frontmatter, round.Frontmatter), "frontmatter must survive the round trip verbatim")
}
