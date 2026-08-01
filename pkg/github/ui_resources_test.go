package github

import (
	"context"
	"slices"
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegisterUIResources_ReadableViaClient verifies that 每个UI 资源 URI
// advertised by 一个MCP App-启用 工具 (e.g. 议题_写入, 创建_pull_请求,
// 获取_me) actually resolves to 一个registered 资源 在服务器.
//
// Regression test 用于"Err或loading MCP App: MPC -32002: Resource not
// found" bug reported in 议题 #2467, 其中HTTP/remote 服务器 返回ed a
// 资源 URI 在工具's _meta.ui block 但绝不registered matching
// 资源 — so follow-up 资源/读取 c所有来自客户端 failed.
func TestRegisterUIResources_ReadableViaClient(t *testing.T) {
	t.Parallel()

	if !UIAssetsAvailable() {
		t.Skip("UI assets not built; run script/build-ui to enable this test")
	}

	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)
	RegisterUIResources(srv, false)

	// Connect 一个in-memory 客户端/服务器 pair 和读取 每个advertised URI.
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
// shared construct或由以下内容使用： both stdio 和HTTP entry points — registers
// UI 资源 when UI assets are embedded. Previously this registration
// 仅happened 在stdio bootstrap, so remote/HTTP 客户端s hit -32002.
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

func TestRegisterUIResources_ReadOnlySkipsWriteResources(t *testing.T) {
	t.Parallel()

	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)
	RegisterUIResources(srv, true)

	st, ct := mcp.NewInMemoryTransports()

	type clientResult struct {
		res *mcp.ListResourcesResult
		err error
	}
	clientCh := make(chan clientResult, 1)
	go func() {
		client := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil)
		cs, err := client.Connect(context.Background(), ct, nil)
		if err != nil {
			clientCh <- clientResult{err: err}
			return
		}
		defer func() { _ = cs.Close() }()

		res, err := cs.ListResources(context.Background(), nil)
		clientCh <- clientResult{res: res, err: err}
	}()

	ss, err := srv.Connect(context.Background(), st, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ss.Close() })

	got := <-clientCh
	require.NoError(t, got.err)
	require.NotNil(t, got.res)

	names := make([]string, 0, len(got.res.Resources))
	for _, res := range got.res.Resources {
		names = append(names, res.Name)
	}
	slices.Sort(names)

	assert.Equal(t, []string{"get_me_ui"}, names)
}

// mustEmptyInventory builds 一个空 inventory f或tests that 仅care about
// 资源/提示 registered outside inventory (such as UI 资源).
func mustEmptyInventory(t *testing.T) *inventory.Inventory {
	t.Helper()
	inv, err := NewInventory(stubTranslator).WithToolsets([]string{}).Build()
	require.NoError(t, err)
	return inv
}
