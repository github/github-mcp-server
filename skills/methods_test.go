package skills_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"testing/fstest"

	"github.com/github/github-mcp-server/skills"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// connect stands up an in-memory client/server pair with the given registry
// published on the server, mirroring how NewMCPServer wires the extension.
func connect(t *testing.T, fsys fstest.MapFS, prefix string, dynamic func(*skills.Publisher)) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()

	registry, err := skills.LoadFS(fsys, prefix)
	require.NoError(t, err)
	pub := &skills.Publisher{Registry: registry}
	if dynamic != nil {
		dynamic(pub)
	}

	opts := &mcp.ServerOptions{}
	pub.DeclareCapability(opts)
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "0.0.1"}, opts)
	require.NoError(t, pub.Install(server))

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.1"}, nil)
	require.NoError(t, mcp.AddSendingCustomMethod[*skills.ListSkillsParams, *skills.ListSkillsResult](client, skills.MethodSkillsList))
	require.NoError(t, mcp.AddSendingCustomMethod[*skills.GetSkillParams, *skills.GetSkillResult](client, skills.MethodSkillsGet))
	require.NoError(t, mcp.AddSendingCustomMethod[*skills.DirectoryReadParams, *skills.DirectoryReadResult](client, skills.MethodDirectoryRead))

	clientSession, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = clientSession.Close() })
	return clientSession
}

func requireInvalidParams(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	var jsonrpcErr *jsonrpc.Error
	require.ErrorAs(t, err, &jsonrpcErr)
	assert.EqualValues(t, jsonrpc.CodeInvalidParams, jsonrpcErr.Code)
}

func TestExtensionCapabilityDeclared(t *testing.T) {
	cs := connect(t, testFS, "acme", nil)

	caps := cs.InitializeResult().Capabilities
	require.NotNil(t, caps)
	settings, ok := caps.Extensions[skills.ExtensionID]
	require.True(t, ok, "initialize response must declare the skills extension")
	assert.Equal(t, map[string]any{"directoryRead": true}, settings)
}

func TestSkillsListOverProtocol(t *testing.T) {
	cs := connect(t, testFS, "acme", nil)
	ctx := context.Background()

	res, err := mcp.CallCustomMethod[*skills.ListSkillsParams, *skills.ListSkillsResult](
		ctx, cs, skills.MethodSkillsList, &skills.ListSkillsParams{})
	require.NoError(t, err)

	require.Len(t, res.Skills, 2)
	assert.Empty(t, res.NextCursor, "an entry is atomic and the catalog fits one page")
	assert.Positive(t, res.TTLMs)
	assert.Equal(t, "public", res.CacheScope)
	assert.Equal(t, "complete", res.ResultType)

	// Each SKILL.md listed is readable via plain resources/read, and its
	// content matches the advertised digest.
	for _, entry := range res.Skills {
		rr, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: entry.URI})
		require.NoError(t, err)
		require.Len(t, rr.Contents, 1)
		sum := sha256.Sum256([]byte(rr.Contents[0].Text))
		assert.Equal(t, entry.Resources[0].Digest, "sha256:"+hex.EncodeToString(sum[:]))
		assert.Equal(t, int64(len(rr.Contents[0].Text)), entry.Resources[0].Size)

		// Frontmatter identity requirement: a host parsing the fetched
		// SKILL.md must find frontmatter identical to the entry's.
		fm, err := skills.ParseFrontmatter([]byte(rr.Contents[0].Text))
		require.NoError(t, err)
		assert.Equal(t, entry.Frontmatter, fm)
	}
}

func TestSkillsGetOverProtocol(t *testing.T) {
	cs := connect(t, testFS, "acme", nil)
	ctx := context.Background()

	t.Run("known skill", func(t *testing.T) {
		res, err := mcp.CallCustomMethod[*skills.GetSkillParams, *skills.GetSkillResult](
			ctx, cs, skills.MethodSkillsGet,
			&skills.GetSkillParams{URI: "skill://acme/pdf-processing/SKILL.md"})
		require.NoError(t, err)
		assert.Equal(t, "skill://acme/pdf-processing/SKILL.md", res.Skill.URI)
		assert.Equal(t, "pdf-processing", res.Skill.Frontmatter["name"])
		assert.Len(t, res.Skill.Resources, 6)
		assert.Equal(t, "complete", res.ResultType)
	})

	t.Run("unknown skill is -32602", func(t *testing.T) {
		_, err := mcp.CallCustomMethod[*skills.GetSkillParams, *skills.GetSkillResult](
			ctx, cs, skills.MethodSkillsGet,
			&skills.GetSkillParams{URI: "skill://acme/nope/SKILL.md"})
		requireInvalidParams(t, err)
	})

	t.Run("missing uri is -32602", func(t *testing.T) {
		_, err := mcp.CallCustomMethod[*skills.GetSkillParams, *skills.GetSkillResult](
			ctx, cs, skills.MethodSkillsGet, &skills.GetSkillParams{})
		requireInvalidParams(t, err)
	})
}

func TestDirectoryReadOverProtocol(t *testing.T) {
	cs := connect(t, testFS, "acme", nil)
	ctx := context.Background()

	t.Run("descends like a filesystem", func(t *testing.T) {
		res, err := mcp.CallCustomMethod[*skills.DirectoryReadParams, *skills.DirectoryReadResult](
			ctx, cs, skills.MethodDirectoryRead,
			&skills.DirectoryReadParams{URI: "skill://acme/pdf-processing/templates"})
		require.NoError(t, err)
		require.Len(t, res.Resources, 3)
		assert.Equal(t, "complete", res.ResultType)
		assert.Equal(t, "regional", res.Resources[2].Name)
		assert.Equal(t, skills.DirectoryMIMEType, res.Resources[2].MIMEType)

		// Descend into the subdirectory the listing marked.
		res, err = mcp.CallCustomMethod[*skills.DirectoryReadParams, *skills.DirectoryReadResult](
			ctx, cs, skills.MethodDirectoryRead,
			&skills.DirectoryReadParams{URI: res.Resources[2].URI})
		require.NoError(t, err)
		require.Len(t, res.Resources, 1)
		assert.Equal(t, "skill://acme/pdf-processing/templates/regional/eu-invoice.md", res.Resources[0].URI)
	})

	t.Run("file URI is -32602", func(t *testing.T) {
		_, err := mcp.CallCustomMethod[*skills.DirectoryReadParams, *skills.DirectoryReadResult](
			ctx, cs, skills.MethodDirectoryRead,
			&skills.DirectoryReadParams{URI: "skill://acme/pdf-processing/SKILL.md"})
		requireInvalidParams(t, err)
	})

	t.Run("unknown URI is -32602", func(t *testing.T) {
		_, err := mcp.CallCustomMethod[*skills.DirectoryReadParams, *skills.DirectoryReadResult](
			ctx, cs, skills.MethodDirectoryRead,
			&skills.DirectoryReadParams{URI: "skill://acme/nope"})
		requireInvalidParams(t, err)
	})
}

func TestSkillFilesAppearInResourcesList(t *testing.T) {
	cs := connect(t, testFS, "acme", nil)
	ctx := context.Background()

	res, err := cs.ListResources(ctx, nil)
	require.NoError(t, err)

	byURI := make(map[string]*mcp.Resource, len(res.Resources))
	for _, r := range res.Resources {
		byURI[r.URI] = r
	}
	skillMD, ok := byURI["skill://acme/git-workflow/SKILL.md"]
	require.True(t, ok, "SKILL.md resources are ordinary listed resources")
	assert.Equal(t, "git-workflow", skillMD.Name)
	assert.Equal(t, "text/markdown", skillMD.MIMEType)
	assert.Contains(t, byURI, "skill://acme/pdf-processing/scripts/extract.py")
}

func TestDynamicHooks(t *testing.T) {
	dynamicEntry := skills.Entry{
		URI:         "skill://dyn/generated/SKILL.md",
		Frontmatter: map[string]any{"name": "generated", "description": "made on demand"},
		// Dynamically generated content cannot be pre-digested; the entry's
		// resources field serializes as the string "dynamic".
		Dynamic: true,
	}
	cs := connect(t, testFS, "acme", func(p *skills.Publisher) {
		p.DynamicGet = func(_ context.Context, uri string) (*skills.Entry, error) {
			if uri == dynamicEntry.URI {
				return &dynamicEntry, nil
			}
			return nil, nil
		}
		p.DynamicDirectoryRead = func(_ context.Context, uri string) ([]*mcp.Resource, error) {
			if uri == "skill://dyn/generated" {
				return []*mcp.Resource{{URI: dynamicEntry.URI, Name: "SKILL.md", MIMEType: "text/markdown"}}, nil
			}
			return nil, nil
		}
	})
	ctx := context.Background()

	t.Run("skills/get consults the hook for unlisted URIs", func(t *testing.T) {
		res, err := mcp.CallCustomMethod[*skills.GetSkillParams, *skills.GetSkillResult](
			ctx, cs, skills.MethodSkillsGet, &skills.GetSkillParams{URI: dynamicEntry.URI})
		require.NoError(t, err)
		assert.Equal(t, dynamicEntry.URI, res.Skill.URI)
		// The "dynamic" marker survives the wire roundtrip.
		assert.True(t, res.Skill.Dynamic)
		assert.Empty(t, res.Skill.Resources)
	})

	t.Run("unknown URIs still fail after the hook", func(t *testing.T) {
		_, err := mcp.CallCustomMethod[*skills.GetSkillParams, *skills.GetSkillResult](
			ctx, cs, skills.MethodSkillsGet, &skills.GetSkillParams{URI: "skill://dyn/other/SKILL.md"})
		requireInvalidParams(t, err)
	})

	t.Run("directory read consults the hook", func(t *testing.T) {
		res, err := mcp.CallCustomMethod[*skills.DirectoryReadParams, *skills.DirectoryReadResult](
			ctx, cs, skills.MethodDirectoryRead, &skills.DirectoryReadParams{URI: "skill://dyn/generated"})
		require.NoError(t, err)
		require.Len(t, res.Resources, 1)
	})
}
