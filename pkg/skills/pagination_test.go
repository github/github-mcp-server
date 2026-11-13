package skills

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCursorRoundTrip(t *testing.T) {
	t.Parallel()

	for _, offset := range []int{0, 1, 50, 512, 1_000_000} {
		cursor := encodeCursor(offset)
		got, err := decodeCursor(cursor)
		require.NoError(t, err)
		assert.Equal(t, offset, got)
	}
}

// The cursor is opaque: clients must not read or construct one, so anything
// this server did not issue is rejected rather than interpreted.
func TestDecodeCursorRejectsForeignCursors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		cursor string
	}{
		{"not base64", "not base64!"},
		{"base64 of an unrelated payload", "aGVsbG8"},
		{"a bare offset", "10"},
		{"wrong prefix", encodeForeignCursor("skills.v0:", "1")},
		{"non-numeric offset", encodeForeignCursor(cursorPrefix, "abc")},
		{"negative offset", encodeForeignCursor(cursorPrefix, "-1")},
		{"empty payload", encodeForeignCursor(cursorPrefix, "")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := decodeCursor(tc.cursor)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidCursor)
		})
	}
}

func encodeForeignCursor(prefix, payload string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(prefix + payload))
}

func TestPaginate(t *testing.T) {
	t.Parallel()

	items := []string{"a", "b", "c", "d", "e"}

	t.Run("walks the whole collection exactly once", func(t *testing.T) {
		t.Parallel()
		var seen []string
		cursor := ""
		for pages := 0; ; pages++ {
			require.Less(t, pages, 10, "pagination must terminate")
			page, next, err := paginate(items, cursor, 2)
			require.NoError(t, err)
			seen = append(seen, page...)
			if next == "" {
				break
			}
			cursor = next
		}
		assert.Equal(t, items, seen)
	})

	t.Run("the last page carries no cursor", func(t *testing.T) {
		t.Parallel()
		page, next, err := paginate(items, "", len(items))
		require.NoError(t, err)
		assert.Equal(t, items, page)
		assert.Empty(t, next)
	})

	t.Run("a page size of zero falls back to the default", func(t *testing.T) {
		t.Parallel()
		page, next, err := paginate(items, "", 0)
		require.NoError(t, err)
		assert.Len(t, page, len(items))
		assert.Empty(t, next)
	})

	t.Run("an empty collection yields one empty page", func(t *testing.T) {
		t.Parallel()
		page, next, err := paginate([]string{}, "", 2)
		require.NoError(t, err)
		assert.Empty(t, page)
		assert.Empty(t, next)
	})

	// The catalog may shrink between pages. A client walking off the end
	// should see the walk finish, not fail.
	t.Run("an offset past the end yields an empty final page", func(t *testing.T) {
		t.Parallel()
		page, next, err := paginate(items, encodeCursor(len(items)+10), 2)
		require.NoError(t, err)
		assert.Empty(t, page)
		assert.Empty(t, next)
	})

	t.Run("an invalid cursor is an error, not a reset to the first page", func(t *testing.T) {
		t.Parallel()
		_, _, err := paginate(items, "bogus!", 2)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidCursor)
	})
}

// Slicing is by whole elements, so a skill's resource manifest is either
// carried complete on a page or not carried at all.
func TestPaginationNeverSplitsAnEntry(t *testing.T) {
	t.Parallel()

	r := newTestRegistry(t)
	entries, err := r.List(context.Background())
	require.NoError(t, err)
	require.Len(t, entries, 3)

	byURI := make(map[string]Entry, len(entries))
	for _, e := range entries {
		byURI[e.URI] = e
	}

	cursor := ""
	var seen int
	for {
		page, next, err := paginate(entries, cursor, 1)
		require.NoError(t, err)
		for _, e := range page {
			seen++
			// Every entry on a page is complete and self-validating: its
			// manifest is whole, so a host can budget and verify it from the
			// page alone.
			require.NoError(t, e.Validate())
			assert.Equal(t, byURI[e.URI].Resources, e.Resources)
		}
		if next == "" {
			break
		}
		cursor = next
	}
	assert.Equal(t, len(entries), seen)
}

func TestNegotiatedProtocolVersion(t *testing.T) {
	t.Parallel()

	const (
		modern = "2026-07-28"
		legacy = "2025-06-18"
	)

	metaParams := func(version string) *ListSkillsParams {
		p := &ListSkillsParams{}
		p.SetMeta(mcp.Meta{mcp.MetaKeyProtocolVersion: version})
		return p
	}

	t.Run("the request context wins", func(t *testing.T) {
		t.Parallel()
		ctx := withProtocolVersion(context.Background(), modern)
		assert.Equal(t, modern, NegotiatedProtocolVersion(ctx, metaParams(legacy), nil))
	})

	t.Run("request meta is consulted next", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, modern, NegotiatedProtocolVersion(context.Background(), metaParams(modern), nil))
	})

	t.Run("an empty context value does not win", func(t *testing.T) {
		t.Parallel()
		ctx := withProtocolVersion(context.Background(), "")
		assert.Equal(t, legacy, NegotiatedProtocolVersion(ctx, metaParams(legacy), nil))
	})

	t.Run("no source of truth is unknown, not an assumption", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, NegotiatedProtocolVersion(context.Background(), &ListSkillsParams{}, nil))
	})

	t.Run("nil params and nil session are safe", func(t *testing.T) {
		t.Parallel()
		var p *ListSkillsParams
		assert.Empty(t, NegotiatedProtocolVersion(context.Background(), p, nil))
		assert.Empty(t, NegotiatedProtocolVersion(context.Background(), nil, nil))
	})

	t.Run("non-string meta values are ignored", func(t *testing.T) {
		t.Parallel()
		p := &ListSkillsParams{}
		p.SetMeta(mcp.Meta{mcp.MetaKeyProtocolVersion: 42})
		assert.Empty(t, NegotiatedProtocolVersion(context.Background(), p, nil))
	})

	// A stateless server sees a different version on each request, so the
	// answer must follow the request rather than any remembered value.
	t.Run("each request is resolved independently", func(t *testing.T) {
		t.Parallel()
		first := withProtocolVersion(context.Background(), modern)
		second := withProtocolVersion(context.Background(), legacy)
		assert.Equal(t, modern, NegotiatedProtocolVersion(first, nil, nil))
		assert.Equal(t, legacy, NegotiatedProtocolVersion(second, nil, nil))
		assert.Equal(t, modern, NegotiatedProtocolVersion(first, nil, nil))
	})
}

// MCP protocol versions are ISO dates, so ordering is chronological.
func TestSupportsRequestScopedContext(t *testing.T) {
	t.Parallel()

	tests := map[string]bool{
		"":           false,
		"2024-11-05": false,
		"2025-03-26": false,
		"2025-06-18": false,
		"2026-07-27": false,
		"2026-07-28": true,
		"2026-07-29": true,
		"2027-01-01": true,
	}

	for version, want := range tests {
		assert.Equal(t, want, SupportsRequestScopedContext(version), version)
	}
}

func TestResultTypeIsEraCorrect(t *testing.T) {
	t.Parallel()

	assert.Empty(t, resultTypeFor(""), "an unknown version is treated as older")
	assert.Empty(t, resultTypeFor("2025-06-18"))
	assert.Equal(t, ResultTypeComplete, resultTypeFor("2026-07-28"))
	assert.Equal(t, ResultTypeComplete, resultTypeFor("2027-03-01"))
}

// The result types must omit every optional field a client's protocol version
// does not define, rather than sending it empty.
func TestResultsOmitUndefinedFields(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(ListSkillsResult{Skills: []Entry{}})
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"skills":[]`, "skills is always rendered")
	for _, field := range []string{"resultType", "nextCursor", "ttlMs", "cacheScope"} {
		assert.NotContains(t, string(raw), `"`+field+`"`)
	}

	ttl := ListTTL
	raw, err = json.Marshal(ListSkillsResult{
		ResultType: ResultTypeComplete,
		Skills:     []Entry{},
		NextCursor: "abc",
		TTLMs:      &ttl,
		CacheScope: "session",
	})
	require.NoError(t, err)
	for _, field := range []string{"resultType", "nextCursor", "ttlMs", "cacheScope"} {
		assert.Contains(t, string(raw), `"`+field+`"`)
	}

	// A zero TTL is a meaningful value — do not cache — so it must survive.
	zero := 0
	raw, err = json.Marshal(ListSkillsResult{Skills: []Entry{}, TTLMs: &zero})
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"ttlMs":0`)

	raw, err = json.Marshal(ReadDirectoryResult{Resources: []*mcp.Resource{}})
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"resources":[]`)
	assert.NotContains(t, string(raw), `"nextCursor"`)
}
