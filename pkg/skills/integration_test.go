package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// conn couples a connected client session with a view of the raw JSON-RPC error
// the server last put on the wire.
//
// The distinction matters over HTTP: the SDK maps a JSON-RPC error onto an HTTP
// 4xx status, and its client transport then reports that status and discards the
// error body. The error code this extension is required to return therefore has
// to be read from the response itself, which is what wireError exposes.
type conn struct {
	session *mcp.ClientSession
	// wireError returns the error object from the most recent response, or nil
	// when the transport gives no access to it.
	wireError func() *jsonrpc.Error
}

// newTestServer builds a server publishing the given source. When forceVersion
// is non-empty, receiving middleware establishes it on the request context,
// which is how a stateless front-end supplies the negotiated version its
// transport resolved.
func newTestServer(t *testing.T, source Source, forceVersion string) *mcp.Server {
	t.Helper()
	return newTestServerFor(t, NewPublisher(source), forceVersion)
}

func newTestServerFor(t *testing.T, pub *Publisher, forceVersion string) *mcp.Server {
	t.Helper()

	opts := &mcp.ServerOptions{Capabilities: &mcp.ServerCapabilities{}}
	pub.DeclareCapability(opts.Capabilities)

	s := mcp.NewServer(&mcp.Implementation{Name: "skills-test", Version: "0.0.1"}, opts)
	if forceVersion != "" {
		s.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
			return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
				return next(withProtocolVersion(ctx, forceVersion), method, req)
			}
		})
	}
	require.NoError(t, pub.Install(s))
	return s
}

// newTestClient returns a client that knows how to send the extension's custom
// methods.
func newTestClient(t *testing.T) *mcp.Client {
	t.Helper()

	c := mcp.NewClient(&mcp.Implementation{Name: "skills-test-client", Version: "0.0.1"}, nil)
	require.NoError(t, mcp.AddSendingCustomMethod[*ListSkillsParams, *ListSkillsResult](c, MethodListSkills))
	require.NoError(t, mcp.AddSendingCustomMethod[*GetSkillParams, *GetSkillResult](c, MethodGetSkill))
	require.NoError(t, mcp.AddSendingCustomMethod[*ReadDirectoryParams, *ReadDirectoryResult](c, MethodReadDirectory))
	return c
}

// connectInMemory wires a client and server over in-memory transports, which
// exercise the same session machinery as stdio.
func connectInMemory(t *testing.T, s *mcp.Server) *conn {
	t.Helper()

	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	serverSession, err := s.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Wait() })

	cs, err := newTestClient(t).Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })

	// A session transport surfaces JSON-RPC errors to the caller intact.
	return &conn{session: cs, wireError: func() *jsonrpc.Error { return nil }}
}

// connectHTTP serves the server over stateless streamable HTTP, the transport a
// remote deployment uses, where every request stands alone and no session state
// accumulates between them.
func connectHTTP(t *testing.T, s *mcp.Server) *conn {
	t.Helper()

	handler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return s },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true},
	)

	var (
		mu   sync.Mutex
		last *jsonrpc.Error
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &recordingWriter{ResponseWriter: w}
		handler.ServeHTTP(rec, r)

		mu.Lock()
		defer mu.Unlock()
		last = rec.jsonRPCError()
	}))
	t.Cleanup(srv.Close)

	cs, err := newTestClient(t).Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:             srv.URL,
		DisableStandaloneSSE: true,
	}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })

	return &conn{session: cs, wireError: func() *jsonrpc.Error {
		mu.Lock()
		defer mu.Unlock()
		return last
	}}
}

// recordingWriter keeps a copy of the response body so a test can inspect the
// JSON-RPC envelope the server actually produced.
type recordingWriter struct {
	http.ResponseWriter
	body bytes.Buffer
}

func (r *recordingWriter) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

func (r *recordingWriter) jsonRPCError() *jsonrpc.Error {
	var envelope struct {
		Error *jsonrpc.Error `json:"error"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(r.body.Bytes()), &envelope); err != nil {
		return nil
	}
	return envelope.Error
}

type transport struct {
	name    string
	connect func(*testing.T, *mcp.Server) *conn
}

// transports enumerates the two ways this server is deployed, so every
// behavioural assertion below is made against both.
func transports() []transport {
	return []transport{
		{"stdio", connectInMemory},
		{"http", connectHTTP},
	}
}

// requireCode asserts the JSON-RPC error code the server put on the wire,
// preferring the raw response when the transport exposes it.
func requireCode(t *testing.T, c *conn, err error, want int64) {
	t.Helper()
	require.Error(t, err)

	if wire := c.wireError(); wire != nil {
		assert.Equal(t, want, wire.Code)
		return
	}
	var rpcErr *jsonrpc.Error
	require.ErrorAs(t, err, &rpcErr)
	assert.Equal(t, want, rpcErr.Code)
}

func listSkills(ctx context.Context, c *conn, params *ListSkillsParams) (*ListSkillsResult, error) {
	return mcp.CallCustomMethod[*ListSkillsParams, *ListSkillsResult](ctx, c.session, MethodListSkills, params)
}

func getSkill(ctx context.Context, c *conn, uri string) (*GetSkillResult, error) {
	return mcp.CallCustomMethod[*GetSkillParams, *GetSkillResult](ctx, c.session, MethodGetSkill, &GetSkillParams{URI: uri})
}

func readDirectory(ctx context.Context, c *conn, uri string) (*ReadDirectoryResult, error) {
	return mcp.CallCustomMethod[*ReadDirectoryParams, *ReadDirectoryResult](ctx, c.session, MethodReadDirectory, &ReadDirectoryParams{URI: uri})
}

func TestExtensionIsDeclaredOnInitialize(t *testing.T) {
	t.Parallel()

	for _, tr := range transports() {
		t.Run(tr.name, func(t *testing.T) {
			t.Parallel()
			c := tr.connect(t, newTestServer(t, newTestRegistry(t), ""))

			res := c.session.InitializeResult()
			require.NotNil(t, res.Capabilities)
			settings, ok := res.Capabilities.Extensions[ExtensionID].(map[string]any)
			require.True(t, ok, "the extension is declared under %q", ExtensionID)
			assert.Equal(t, true, settings[SettingDirectoryRead])
		})
	}
}

func TestSkillsListOverTheWire(t *testing.T) {
	t.Parallel()

	for _, tr := range transports() {
		t.Run(tr.name, func(t *testing.T) {
			t.Parallel()
			c := tr.connect(t, newTestServer(t, newTestRegistry(t), ""))

			got, err := listSkills(context.Background(), c, &ListSkillsParams{})
			require.NoError(t, err)

			assert.Equal(t, []string{
				"skill://example.com/billing/refunds/SKILL.md",
				"skill://example.com/support/refunds/SKILL.md",
				"skill://example.com/writing/SKILL.md",
			}, uris(got.Skills))

			for _, e := range got.Skills {
				// Every entry survives the round trip intact: its manifest is
				// complete and it satisfies the rules a host applies before
				// loading anything.
				require.NoError(t, e.Validate(), e.URI)
				assert.NotEmpty(t, e.Frontmatter.Name())
				assert.NotEmpty(t, e.Frontmatter.Description())
			}
		})
	}
}

// The digest a host verifies must match the bytes the server actually serves
// through resources/read, and the frontmatter it re-parses after fetching must
// match the frontmatter that was published.
func TestPublishedDigestsAndFrontmatterMatchServedContent(t *testing.T) {
	t.Parallel()

	for _, tr := range transports() {
		t.Run(tr.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			c := tr.connect(t, newTestServer(t, newTestRegistry(t), ""))

			listed, err := listSkills(ctx, c, &ListSkillsParams{})
			require.NoError(t, err)
			require.NotEmpty(t, listed.Skills)

			for _, entry := range listed.Skills {
				for _, want := range entry.Resources {
					read, err := c.session.ReadResource(ctx, &mcp.ReadResourceParams{URI: want.URI})
					require.NoError(t, err, want.URI)
					require.Len(t, read.Contents, 1)

					content := []byte(read.Contents[0].Text)
					if read.Contents[0].Blob != nil {
						content = read.Contents[0].Blob
					}
					assert.NoError(t, Verify(content, want), want.URI)

					if want.URI != entry.URI {
						continue
					}
					parsed, _, err := ParseFrontmatter(content)
					require.NoError(t, err)
					assert.True(t, FrontmatterEqual(parsed, entry.Frontmatter),
						"published frontmatter drifted from served content for %s", entry.URI)
				}
			}
		})
	}
}

func TestSkillsGetOverTheWire(t *testing.T) {
	t.Parallel()

	for _, tr := range transports() {
		t.Run(tr.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			c := tr.connect(t, newTestServer(t, newTestRegistry(t), ""))

			got, err := getSkill(ctx, c, "skill://example.com/writing/SKILL.md")
			require.NoError(t, err)
			assert.Equal(t, "skill://example.com/writing/SKILL.md", got.Skill.URI)
			assert.Equal(t, "writing", got.Skill.Frontmatter.Name())
			require.NoError(t, got.Skill.Validate())

			// An unknown skill is invalid params, not a resource-not-found
			// error, on either transport.
			_, err = getSkill(ctx, c, "skill://example.com/nope/SKILL.md")
			requireCode(t, c, err, jsonrpc.CodeInvalidParams)
		})
	}
}

func TestDirectoryReadOverTheWire(t *testing.T) {
	t.Parallel()

	for _, tr := range transports() {
		t.Run(tr.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			c := tr.connect(t, newTestServer(t, newTestRegistry(t), ""))

			got, err := readDirectory(ctx, c, "skill://example.com/writing")
			require.NoError(t, err)
			require.Len(t, got.Resources, 2)
			assert.Equal(t, "skill://example.com/writing/SKILL.md", got.Resources[0].URI)
			assert.Equal(t, "skill://example.com/writing/refs", got.Resources[1].URI)
			assert.Equal(t, DirectoryMIMEType, got.Resources[1].MIMEType)

			t.Run("a file is not a directory", func(t *testing.T) {
				_, err := readDirectory(ctx, c, "skill://example.com/writing/SKILL.md")
				requireCode(t, c, err, jsonrpc.CodeInvalidParams)
			})
			t.Run("and neither is something absent", func(t *testing.T) {
				_, err := readDirectory(ctx, c, "skill://example.com/nowhere")
				requireCode(t, c, err, jsonrpc.CodeInvalidParams)
			})
		})
	}
}

// A client that negotiated a version predating the caching attributes must not
// be sent them, whichever transport carried the request.
func TestVersionSkewOverTheWire(t *testing.T) {
	t.Parallel()

	for _, tr := range transports() {
		t.Run(tr.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			legacy, err := listSkills(ctx, tr.connect(t, newTestServer(t, newTestRegistry(t), legacyVersion)), &ListSkillsParams{})
			require.NoError(t, err)
			assert.Empty(t, legacy.ResultType)
			assert.Nil(t, legacy.TTLMs)
			assert.Empty(t, legacy.CacheScope)
			assert.Len(t, legacy.Skills, 3, "the listing itself is unchanged by version")

			modern, err := listSkills(ctx, tr.connect(t, newTestServer(t, newTestRegistry(t), modernVersion)), &ListSkillsParams{})
			require.NoError(t, err)
			assert.Equal(t, ResultTypeComplete, modern.ResultType)
			require.NotNil(t, modern.TTLMs)
			assert.Equal(t, ListTTL, *modern.TTLMs)
			assert.NotEmpty(t, modern.CacheScope)
		})
	}
}

// The transport must not change the answer: the same source answers stdio and
// HTTP clients identically.
func TestTransportParity(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	server := newTestServer(t, newTestRegistry(t), "")

	overStdio, err := listSkills(ctx, connectInMemory(t, server), &ListSkillsParams{})
	require.NoError(t, err)

	overHTTP, err := listSkills(ctx, connectHTTP(t, server), &ListSkillsParams{})
	require.NoError(t, err)

	assert.Equal(t, overStdio.ResultType, overHTTP.ResultType)
	assert.Equal(t, overStdio.NextCursor, overHTTP.NextCursor)
	assert.Equal(t, overStdio.CacheScope, overHTTP.CacheScope)
	assert.Equal(t, overStdio.Skills, overHTTP.Skills)
}

// A stateless deployment keeps no session state between requests, so a cursor
// issued on one request must still be honoured on the next.
func TestPaginationIsStateless(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pub := NewPublisher(newTestRegistry(t))
	pub.PageSize = 1
	c := connectHTTP(t, newTestServerFor(t, pub, ""))

	var seen []string
	cursor := ""
	for pages := 0; ; pages++ {
		require.Less(t, pages, 10, "pagination must terminate")

		got, err := listSkills(ctx, c, &ListSkillsParams{Cursor: cursor})
		require.NoError(t, err)
		require.Len(t, got.Skills, 1)
		require.NoError(t, got.Skills[0].Validate(), "an entry is never split across pages")

		seen = append(seen, got.Skills[0].URI)
		if got.NextCursor == "" {
			break
		}
		cursor = got.NextCursor
	}
	assert.Len(t, seen, 3)
	assert.Len(t, dedupe(seen), 3)

	// A cursor this server did not issue is rejected rather than interpreted.
	_, err := listSkills(ctx, c, &ListSkillsParams{Cursor: "handmade"})
	requireCode(t, c, err, jsonrpc.CodeInvalidParams)
}

// A server that publishes nothing still declares the extension and still
// answers both methods; an empty listing is not proof of absence.
func TestEmptySourceOverTheWire(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	empty, err := NewRegistry()
	require.NoError(t, err)

	c := connectInMemory(t, newTestServer(t, empty, ""))
	_, declared := c.session.InitializeResult().Capabilities.Extensions[ExtensionID]
	assert.True(t, declared, "the extension is declared even with nothing to publish")

	got, err := listSkills(ctx, c, &ListSkillsParams{})
	require.NoError(t, err)
	assert.Empty(t, got.Skills)
	assert.Empty(t, got.NextCursor)

	_, err = getSkill(ctx, c, "skill://example.com/writing/SKILL.md")
	requireCode(t, c, err, jsonrpc.CodeInvalidParams)
}

// A source that cannot enumerate directories must neither declare directoryRead
// nor answer the method.
func TestDirectoryReadIsAbsentWhenUnsupported(t *testing.T) {
	t.Parallel()

	c := connectInMemory(t, newTestServer(t, &listOnlySource{}, ""))

	settings, ok := c.session.InitializeResult().Capabilities.Extensions[ExtensionID].(map[string]any)
	require.True(t, ok)
	_, declared := settings[SettingDirectoryRead]
	assert.False(t, declared)

	_, err := readDirectory(context.Background(), c, "skill://example.com/writing")
	requireCode(t, c, err, jsonrpc.CodeMethodNotFound)
}

// Reading a SKILL.md through resources/read is an ordinary resource read. It
// yields the file's bytes and nothing else: the extension defines no activation
// semantics, and a host that reads a skill this way has not invoked anything.
func TestSkillFilesAreOrdinaryResources(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	c := connectInMemory(t, newTestServer(t, newTestRegistry(t), ""))

	got, err := c.session.ReadResource(ctx, &mcp.ReadResourceParams{
		URI: "skill://example.com/writing/refs/style.md",
	})
	require.NoError(t, err)
	require.Len(t, got.Contents, 1)
	assert.Equal(t, "# Style", got.Contents[0].Text)

	// A resource that was never published is not readable.
	_, err = c.session.ReadResource(ctx, &mcp.ReadResourceParams{
		URI: "skill://example.com/writing/refs/absent.md",
	})
	require.Error(t, err)
}
