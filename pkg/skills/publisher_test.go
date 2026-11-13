package skills

import (
	"context"
	"errors"
	"sync"
	"testing"

	ghcontext "github.com/github/github-mcp-server/v2/pkg/context"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	modernVersion = "2026-07-28"
	legacyVersion = "2025-06-18"
)

func withProtocolVersion(ctx context.Context, version string) context.Context {
	return ghcontext.WithMCPMethodInfo(ctx, &ghcontext.MCPMethodInfo{ProtocolVersion: version})
}

// modernCtx carries a negotiated protocol version the way a stateless HTTP
// server establishes one per request.
func modernCtx() context.Context {
	return withProtocolVersion(context.Background(), modernVersion)
}

func legacyCtx() context.Context {
	return withProtocolVersion(context.Background(), legacyVersion)
}

// listOnlySource serves skills but cannot enumerate directories, which is the
// shape of a source that must declare the extension with no optional features.
type listOnlySource struct {
	entries  []Entry
	listErr  error
	getErr   error
	listCall int
}

func (s *listOnlySource) List(context.Context) ([]Entry, error) {
	s.listCall++
	return s.entries, s.listErr
}

func (s *listOnlySource) Get(_ context.Context, uri string) (Entry, bool, error) {
	if s.getErr != nil {
		return Entry{}, false, s.getErr
	}
	for _, e := range s.entries {
		if e.URI == uri {
			return e, true, nil
		}
	}
	return Entry{}, false, nil
}

func (s *listOnlySource) Files(context.Context) ([]ResourceFile, error) {
	return nil, nil
}

type sharedDirectorySource struct {
	*listOnlySource
	children []Child
}

func (s *sharedDirectorySource) ReadDirectory(context.Context, string) ([]Child, bool, error) {
	return s.children, true, nil
}

func TestCapabilityDeclaration(t *testing.T) {
	t.Parallel()

	t.Run("a source without directory support declares an empty settings object", func(t *testing.T) {
		t.Parallel()
		p := NewPublisher(&listOnlySource{})
		require.False(t, p.SupportsDirectoryRead())

		caps := &mcp.ServerCapabilities{}
		p.DeclareCapability(caps)

		settings, ok := caps.Extensions[ExtensionID].(map[string]any)
		require.True(t, ok, "the extension is declared under its identifier")
		assert.Empty(t, settings, "an unsupported optional feature is absent, not false")
	})

	t.Run("directoryRead is declared only when it is genuinely supported", func(t *testing.T) {
		t.Parallel()
		p := NewPublisher(newTestRegistry(t))
		require.True(t, p.SupportsDirectoryRead())

		caps := &mcp.ServerCapabilities{}
		p.DeclareCapability(caps)

		settings := caps.Extensions[ExtensionID].(map[string]any)
		assert.Equal(t, true, settings[SettingDirectoryRead])
	})

	t.Run("existing extensions are preserved", func(t *testing.T) {
		t.Parallel()
		caps := &mcp.ServerCapabilities{Extensions: map[string]any{"com.example/other": map[string]any{}}}
		NewPublisher(&listOnlySource{}).DeclareCapability(caps)
		assert.Len(t, caps.Extensions, 2)
	})

	t.Run("nil capabilities are ignored", func(t *testing.T) {
		t.Parallel()
		assert.NotPanics(t, func() { NewPublisher(&listOnlySource{}).DeclareCapability(nil) })
	})
}

// Declaring the extension commits the server to both skills/list and
// skills/get, and to resources/directory/read exactly when directoryRead was
// advertised.
func TestInstallRegistersExactlyTheDeclaredMethods(t *testing.T) {
	t.Parallel()

	t.Run("without directory support", func(t *testing.T) {
		t.Parallel()
		p := NewPublisher(&listOnlySource{})
		s := newBareServer(t)
		require.NoError(t, p.Install(s))

		// Re-registering a method the publisher already installed replaces it,
		// so a second Install must not fail; a method it did not install must
		// still be free.
		require.NoError(t, mcp.AddReceivingCustomMethod(s, MethodReadDirectory, p.handleReadDirectory),
			"resources/directory/read was left unregistered")
	})

	t.Run("with directory support", func(t *testing.T) {
		t.Parallel()
		p := NewPublisher(newTestRegistry(t))
		require.NoError(t, p.Install(newBareServer(t)))
	})

	t.Run("a source without a content provider is rejected", func(t *testing.T) {
		t.Parallel()
		source := struct{ Source }{Source: &listOnlySource{}}
		err := NewPublisher(source).Install(newBareServer(t))
		require.ErrorContains(t, err, "requires a ContentSource")
	})

	t.Run("listed resources must be supplied by the content provider", func(t *testing.T) {
		t.Parallel()
		entry := newTestSkill(t, "example.com/writing").Entry()
		source := &listOnlySource{entries: []Entry{entry}}
		err := NewPublisher(source).Install(newBareServer(t))
		require.ErrorContains(t, err, "content source does not provide it")
	})

	t.Run("skill files are registered as ordinary resources", func(t *testing.T) {
		t.Parallel()
		// Registration itself is exercised end to end in the integration
		// tests; here the concern is that every file the source offers is
		// accepted by the server, including URIs in the skill scheme.
		require.NoError(t, NewPublisher(newTestRegistry(t)).Install(newBareServer(t)))
	})
}

// Skill content is served through ordinary resources/read; the extension
// defines no skill-specific read semantics, and reading a SKILL.md this way
// does not activate the skill.
func TestResourceHandlerServesFileBytes(t *testing.T) {
	t.Parallel()

	t.Run("text is sent as text", func(t *testing.T) {
		t.Parallel()
		h := resourceHandler(ResourceFile{
			URI:  "skill://example.com/writing/refs/style.md",
			File: File{Path: "refs/style.md", Content: []byte("# Style")},
		})
		got, err := h(context.Background(), nil)
		require.NoError(t, err)
		require.Len(t, got.Contents, 1)
		assert.Equal(t, "# Style", got.Contents[0].Text)
		assert.Nil(t, got.Contents[0].Blob)
		assert.Equal(t, "skill://example.com/writing/refs/style.md", got.Contents[0].URI)
	})

	t.Run("binary content is sent as a blob", func(t *testing.T) {
		t.Parallel()
		content := []byte{0x89, 'P', 'N', 'G', 0x00, 0x01}
		h := resourceHandler(ResourceFile{
			URI:  "skill://example.com/writing/logo.png",
			File: File{Path: "logo.png", Content: content},
		})
		got, err := h(context.Background(), nil)
		require.NoError(t, err)
		require.Len(t, got.Contents, 1)
		assert.Empty(t, got.Contents[0].Text)
		assert.Equal(t, content, got.Contents[0].Blob)
	})
}

func newBareServer(t *testing.T) *mcp.Server {
	t.Helper()
	return mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)
}

func TestHandleList(t *testing.T) {
	t.Parallel()

	p := NewPublisher(newTestRegistry(t))

	t.Run("modern clients receive resultType and the caching hint", func(t *testing.T) {
		t.Parallel()
		got, err := p.handleList(modernCtx(), nil, &ListSkillsParams{})
		require.NoError(t, err)
		assert.Equal(t, ResultTypeComplete, got.ResultType)
		require.NotNil(t, got.TTLMs)
		assert.Equal(t, ListTTL, *got.TTLMs)
		assert.NotEmpty(t, got.CacheScope)
		assert.Len(t, got.Skills, 3)
	})

	// The caching attributes and resultType are defined from 2026-07-28; an
	// older client must not be sent fields its protocol version does not
	// define.
	t.Run("legacy clients receive neither", func(t *testing.T) {
		t.Parallel()
		got, err := p.handleList(legacyCtx(), nil, &ListSkillsParams{})
		require.NoError(t, err)
		assert.Empty(t, got.ResultType)
		assert.Nil(t, got.TTLMs)
		assert.Empty(t, got.CacheScope)
		assert.Len(t, got.Skills, 3, "the listing itself is unchanged by version")
	})

	t.Run("an unknown version is treated as legacy", func(t *testing.T) {
		t.Parallel()
		got, err := p.handleList(context.Background(), nil, &ListSkillsParams{})
		require.NoError(t, err)
		assert.Empty(t, got.ResultType)
		assert.Nil(t, got.TTLMs)
	})

	t.Run("omitted params are treated as an uncursored first page", func(t *testing.T) {
		t.Parallel()
		got, err := p.handleList(modernCtx(), nil, nil)
		require.NoError(t, err)
		assert.Len(t, got.Skills, 3)
	})

	t.Run("every published entry is valid", func(t *testing.T) {
		t.Parallel()
		got, err := p.handleList(modernCtx(), nil, &ListSkillsParams{})
		require.NoError(t, err)
		for _, e := range got.Skills {
			assert.NoError(t, e.Validate(), e.URI)
		}
	})

	t.Run("a foreign cursor is invalid params", func(t *testing.T) {
		t.Parallel()
		_, err := p.handleList(modernCtx(), nil, &ListSkillsParams{Cursor: "not-ours"})
		requireInvalidParams(t, err)
	})

	t.Run("source failures propagate as errors, not as an empty listing", func(t *testing.T) {
		t.Parallel()
		boom := errors.New("boom")
		_, err := NewPublisher(&listOnlySource{listErr: boom}).handleList(modernCtx(), nil, &ListSkillsParams{})
		assert.ErrorIs(t, err, boom)
	})
}

// A server may publish nothing and still serve skills. An empty listing is a
// valid answer, and clients must not read it as proof of absence.
func TestEmptyListingIsValidAndNotProofOfAbsence(t *testing.T) {
	t.Parallel()

	hidden := newTestSkill(t, "example.com/writing").Entry()
	source := &listOnlySource{entries: []Entry{}}
	p := NewPublisher(source)

	got, err := p.handleList(modernCtx(), nil, &ListSkillsParams{})
	require.NoError(t, err)
	require.NotNil(t, got.Skills, "skills is an empty array, never null")
	assert.Empty(t, got.Skills)
	assert.Empty(t, got.NextCursor)

	// The same server answers skills/get for a skill it declined to enumerate.
	source.entries = []Entry{hidden}
	unlisted := NewPublisher(&unlistedSource{entry: hidden})
	res, err := unlisted.handleGet(modernCtx(), nil, &GetSkillParams{URI: hidden.URI})
	require.NoError(t, err)
	assert.Equal(t, hidden.URI, res.Skill.URI)

	listed, err := unlisted.handleList(modernCtx(), nil, &ListSkillsParams{})
	require.NoError(t, err)
	assert.Empty(t, listed.Skills)
}

// unlistedSource enumerates nothing but answers for the skill it serves.
type unlistedSource struct{ entry Entry }

func (s *unlistedSource) List(context.Context) ([]Entry, error) { return nil, nil }
func (s *unlistedSource) Get(_ context.Context, uri string) (Entry, bool, error) {
	if uri == s.entry.URI {
		return s.entry, true, nil
	}
	return Entry{}, false, nil
}

func TestHandleListPaginatesAtomically(t *testing.T) {
	t.Parallel()

	p := NewPublisher(newTestRegistry(t))
	p.PageSize = 2

	first, err := p.handleList(modernCtx(), nil, &ListSkillsParams{})
	require.NoError(t, err)
	require.Len(t, first.Skills, 2)
	require.NotEmpty(t, first.NextCursor)
	for _, e := range first.Skills {
		require.NoError(t, e.Validate(), "an entry arrives with its manifest whole")
	}

	second, err := p.handleList(modernCtx(), nil, &ListSkillsParams{Cursor: first.NextCursor})
	require.NoError(t, err)
	require.Len(t, second.Skills, 1)
	assert.Empty(t, second.NextCursor, "the last page carries no cursor")

	seen := append(uris(first.Skills), uris(second.Skills)...)
	assert.Len(t, seen, 3)
	assert.Len(t, dedupe(seen), 3, "no entry is repeated across pages")
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func TestHandleGet(t *testing.T) {
	t.Parallel()

	p := NewPublisher(newTestRegistry(t))

	t.Run("modern clients receive resultType", func(t *testing.T) {
		t.Parallel()
		got, err := p.handleGet(modernCtx(), nil, &GetSkillParams{URI: "skill://example.com/writing/SKILL.md"})
		require.NoError(t, err)
		assert.Equal(t, ResultTypeComplete, got.ResultType)
		assert.Equal(t, "skill://example.com/writing/SKILL.md", got.Skill.URI)
		assert.NoError(t, got.Skill.Validate())
	})

	t.Run("legacy clients do not", func(t *testing.T) {
		t.Parallel()
		got, err := p.handleGet(legacyCtx(), nil, &GetSkillParams{URI: "skill://example.com/writing/SKILL.md"})
		require.NoError(t, err)
		assert.Empty(t, got.ResultType)
		assert.Equal(t, "skill://example.com/writing/SKILL.md", got.Skill.URI)
	})

	// The extension specifies invalid params for an unknown skill, which is the
	// same code resources/read uses for an unknown resource — not the
	// resource-not-found code.
	t.Run("an unknown skill is invalid params", func(t *testing.T) {
		t.Parallel()
		_, err := p.handleGet(modernCtx(), nil, &GetSkillParams{URI: "skill://example.com/nope/SKILL.md"})
		requireInvalidParams(t, err)
	})

	t.Run("a skill root is not a SKILL.md", func(t *testing.T) {
		t.Parallel()
		_, err := p.handleGet(modernCtx(), nil, &GetSkillParams{URI: "skill://example.com/writing"})
		requireInvalidParams(t, err)
	})

	t.Run("a missing uri is invalid params", func(t *testing.T) {
		t.Parallel()
		_, err := p.handleGet(modernCtx(), nil, &GetSkillParams{})
		requireInvalidParams(t, err)
		_, err = p.handleGet(modernCtx(), nil, nil)
		requireInvalidParams(t, err)
	})

	t.Run("source failures propagate", func(t *testing.T) {
		t.Parallel()
		boom := errors.New("boom")
		_, err := NewPublisher(&listOnlySource{getErr: boom}).handleGet(modernCtx(), nil, &GetSkillParams{URI: "skill://x/y/SKILL.md"})
		assert.ErrorIs(t, err, boom)
	})
}

func TestHandleReadDirectory(t *testing.T) {
	t.Parallel()

	p := NewPublisher(newTestRegistry(t))

	t.Run("direct children only, with subdirectories marked", func(t *testing.T) {
		t.Parallel()
		got, err := p.handleReadDirectory(modernCtx(), nil, &ReadDirectoryParams{URI: "skill://example.com/writing"})
		require.NoError(t, err)
		assert.Equal(t, ResultTypeComplete, got.ResultType)
		require.Len(t, got.Resources, 2)

		assert.Equal(t, "skill://example.com/writing/SKILL.md", got.Resources[0].URI)
		assert.NotZero(t, got.Resources[0].Size)

		assert.Equal(t, "skill://example.com/writing/refs", got.Resources[1].URI)
		assert.Equal(t, DirectoryMIMEType, got.Resources[1].MIMEType)
		assert.Zero(t, got.Resources[1].Size, "a directory carries no size")
	})

	t.Run("legacy clients receive no resultType", func(t *testing.T) {
		t.Parallel()
		got, err := p.handleReadDirectory(legacyCtx(), nil, &ReadDirectoryParams{URI: "skill://example.com/writing"})
		require.NoError(t, err)
		assert.Empty(t, got.ResultType)
	})

	t.Run("an empty directory is an empty array, not an error", func(t *testing.T) {
		t.Parallel()
		got, err := p.handleReadDirectory(modernCtx(), nil, &ReadDirectoryParams{URI: "skill://example.com/writing/refs"})
		require.NoError(t, err)
		require.NotNil(t, got.Resources)
		assert.Len(t, got.Resources, 1)
	})

	// A URI that does not exist and one that exists but is not a directory are
	// both invalid params.
	t.Run("a non-directory is invalid params", func(t *testing.T) {
		t.Parallel()
		_, err := p.handleReadDirectory(modernCtx(), nil, &ReadDirectoryParams{URI: "skill://example.com/writing/SKILL.md"})
		requireInvalidParams(t, err)
	})

	t.Run("an unknown directory is invalid params", func(t *testing.T) {
		t.Parallel()
		_, err := p.handleReadDirectory(modernCtx(), nil, &ReadDirectoryParams{URI: "skill://example.com/nowhere"})
		requireInvalidParams(t, err)
	})

	t.Run("a missing uri is invalid params", func(t *testing.T) {
		t.Parallel()
		_, err := p.handleReadDirectory(modernCtx(), nil, &ReadDirectoryParams{})
		requireInvalidParams(t, err)
		_, err = p.handleReadDirectory(modernCtx(), nil, nil)
		requireInvalidParams(t, err)
	})

	t.Run("a source that cannot enumerate directories refuses the method", func(t *testing.T) {
		t.Parallel()
		_, err := NewPublisher(&listOnlySource{}).handleReadDirectory(modernCtx(), nil, &ReadDirectoryParams{URI: "skill://example.com/writing"})
		requireInvalidParams(t, err)
	})

	t.Run("a foreign cursor is invalid params", func(t *testing.T) {
		t.Parallel()
		_, err := p.handleReadDirectory(modernCtx(), nil, &ReadDirectoryParams{
			URI:    "skill://example.com/writing",
			Cursor: "not-ours",
		})
		requireInvalidParams(t, err)
	})

	t.Run("pagination", func(t *testing.T) {
		t.Parallel()
		paged := NewPublisher(newTestRegistry(t))
		paged.PageSize = 1

		first, err := paged.handleReadDirectory(modernCtx(), nil, &ReadDirectoryParams{URI: "skill://example.com/writing"})
		require.NoError(t, err)
		require.Len(t, first.Resources, 1)
		require.NotEmpty(t, first.NextCursor)

		second, err := paged.handleReadDirectory(modernCtx(), nil, &ReadDirectoryParams{
			URI:    "skill://example.com/writing",
			Cursor: first.NextCursor,
		})
		require.NoError(t, err)
		require.Len(t, second.Resources, 1)
		assert.Empty(t, second.NextCursor)
		assert.NotEqual(t, first.Resources[0].URI, second.Resources[0].URI)
	})
}

func TestHandleReadDirectoryDoesNotMutateSourceChildren(t *testing.T) {
	t.Parallel()

	children := []Child{
		{URI: "skill://example.com/writing/z.md", Name: "z.md", MIMEType: "text/markdown"},
		{URI: "skill://example.com/writing/a.md", Name: "a.md", MIMEType: "text/markdown"},
	}
	source := &sharedDirectorySource{listOnlySource: &listOnlySource{}, children: children}
	publisher := NewPublisher(source)

	const requests = 8
	var wg sync.WaitGroup
	results := make(chan []string, requests)
	errs := make(chan error, requests)
	for range requests {
		wg.Go(func() {
			got, err := publisher.handleReadDirectory(modernCtx(), nil, &ReadDirectoryParams{URI: "skill://example.com/writing"})
			if err != nil {
				errs <- err
				return
			}
			uris := make([]string, len(got.Resources))
			for i, resource := range got.Resources {
				uris[i] = resource.URI
			}
			results <- uris
		})
	}
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	for got := range results {
		assert.Equal(t, []string{
			"skill://example.com/writing/a.md",
			"skill://example.com/writing/z.md",
		}, got)
	}
	assert.Equal(t, "skill://example.com/writing/z.md", children[0].URI, "publisher sorting must not mutate source-owned memory")
}

// A stateless server answers each request on its own terms, so two requests on
// the same publisher may resolve to different protocol versions.
func TestCapabilitiesAreResolvedPerRequest(t *testing.T) {
	t.Parallel()

	p := NewPublisher(newTestRegistry(t))

	modern, err := p.handleList(modernCtx(), nil, &ListSkillsParams{})
	require.NoError(t, err)
	legacy, err := p.handleList(legacyCtx(), nil, &ListSkillsParams{})
	require.NoError(t, err)
	again, err := p.handleList(modernCtx(), nil, &ListSkillsParams{})
	require.NoError(t, err)

	assert.Equal(t, ResultTypeComplete, modern.ResultType)
	assert.Empty(t, legacy.ResultType, "an earlier modern request must not leak into a legacy one")
	assert.Equal(t, ResultTypeComplete, again.ResultType, "nor a legacy request into a later modern one")

	assert.NotNil(t, modern.TTLMs)
	assert.Nil(t, legacy.TTLMs)
	assert.NotNil(t, again.TTLMs)
}

func requireInvalidParams(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	var rpcErr *jsonrpc.Error
	require.ErrorAs(t, err, &rpcErr)
	assert.Equal(t, int64(jsonrpc.CodeInvalidParams), rpcErr.Code, "must be -32602")
}
