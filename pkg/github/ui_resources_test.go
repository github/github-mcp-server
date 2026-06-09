package github

import (
	"context"
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegisterUIResources_ReadableViaClient verifies that each UI resource URI
// advertised by an MCP App-enabled tool (e.g. issue_write, create_pull_request,
// get_me) actually resolves to a registered resource on the server.
//
// Regression test for the "Error loading MCP App: MPC -32002: Resource not
// found" bug reported in issue #2467, where the HTTP/remote server returned a
// resource URI in the tool's _meta.ui block but never registered the matching
// resource — so the follow-up resources/read call from the client failed.
func TestRegisterUIResources_ReadableViaClient(t *testing.T) {
	t.Parallel()

	if !UIAssetsAvailable() {
		t.Skip("UI assets not built; run script/build-ui to enable this test")
	}

	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)
	RegisterUIResources(srv)

	// Connect an in-memory client/server pair and read each advertised URI.
	st, ct := mcp.NewInMemoryTransports()

	type clientResult struct {
		session *mcp.ClientSession
		err     error
	}
	clientCh := make(chan clientResult, 1)
	go func() {
		client := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil)
		cs, err := client.Connect(context.Background(), ct, nil)
		clientCh <- clientResult{session: cs, err: err}
	}()

	ss, err := srv.Connect(context.Background(), st, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ss.Close() })

	got := <-clientCh
	require.NoError(t, got.err)
	t.Cleanup(func() { _ = got.session.Close() })

	uris := []string{
		GetMeUIResourceURI,
		IssueWriteUIResourceURI,
		PullRequestWriteUIResourceURI,
		PullRequestEditUIResourceURI,
	}
	for _, uri := range uris {
		t.Run(uri, func(t *testing.T) {
			res, err := got.session.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: uri})
			require.NoError(t, err, "resource %s should be registered (got -32002 means it isn't)", uri)
			require.NotNil(t, res)
			require.NotEmpty(t, res.Contents)
			assert.Equal(t, uri, res.Contents[0].URI)
			assert.Equal(t, MCPAppMIMEType, res.Contents[0].MIMEType)
			assert.NotEmpty(t, res.Contents[0].Text, "UI resource should return HTML body")
		})
	}
}

// TestNewMCPServer_RegistersUIResources verifies that NewMCPServer — the
// shared constructor used by both the stdio and HTTP entry points — registers
// the UI resources when UI assets are embedded. Previously this registration
// only happened in the stdio bootstrap, so remote/HTTP clients hit -32002.
func TestNewMCPServer_RegistersUIResources(t *testing.T) {
	t.Parallel()

	if !UIAssetsAvailable() {
		t.Skip("UI assets not built; run script/build-ui to enable this test")
	}

	srv, err := NewMCPServer(context.Background(), &MCPServerConfig{
		Version:    "test",
		Translator: stubTranslator,
	}, stubDeps{t: stubTranslator}, mustEmptyInventory(t))
	require.NoError(t, err)

	st, ct := mcp.NewInMemoryTransports()

	type clientResult struct {
		session *mcp.ClientSession
		err     error
	}
	clientCh := make(chan clientResult, 1)
	go func() {
		client := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil)
		cs, err := client.Connect(context.Background(), ct, nil)
		clientCh <- clientResult{session: cs, err: err}
	}()

	ss, err := srv.Connect(context.Background(), st, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ss.Close() })

	got := <-clientCh
	require.NoError(t, got.err)
	t.Cleanup(func() { _ = got.session.Close() })

	res, err := got.session.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: IssueWriteUIResourceURI})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.NotEmpty(t, res.Contents)
	assert.Equal(t, MCPAppMIMEType, res.Contents[0].MIMEType)
}

// mustEmptyInventory builds an empty inventory for tests that only care about
// resources/prompts registered outside the inventory (such as the UI resources).
func mustEmptyInventory(t *testing.T) *inventory.Inventory {
	t.Helper()
	inv, err := NewInventory(stubTranslator).WithToolsets([]string{}).Build()
	require.NoError(t, err)
	return inv
}
