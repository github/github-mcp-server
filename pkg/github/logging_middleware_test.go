package github

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/pkg/observability"
	"github.com/github/github-mcp-server/pkg/observability/metrics"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeToolDeps implements just enough of ToolDependencies to drive the
// logging middleware. Unused methods panic so we notice if callers grow a
// dependency on them.
type fakeToolDeps struct {
	ToolDependencies
	logger *slog.Logger
}

func (f fakeToolDeps) Logger(_ context.Context) *slog.Logger { return f.logger }

func newTestLogger(level slog.Level) (*slog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	h := slog.NewTextHandler(buf, &slog.HandlerOptions{Level: level})
	return slog.New(h), buf
}

func callToolRequest(name string) mcp.Request {
	return &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: name}}
}

func TestToolLoggingMiddleware_LogsToolSuccessAtDebug(t *testing.T) {
	logger, buf := newTestLogger(slog.LevelDebug)
	deps := fakeToolDeps{logger: logger}

	ctx := ContextWithDeps(context.Background(), deps)

	handler := ToolLoggingMiddleware()(func(ctx context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
		// Tool handlers should see the enriched logger via the context.
		assert.NotNil(t, observability.LoggerFromContext(ctx))
		return &mcp.CallToolResult{}, nil
	})

	_, err := handler(ctx, MCPMethodCallTool, callToolRequest("create_issue"))
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "level=DEBUG")
	assert.Contains(t, out, `msg="tool call succeeded"`)
	assert.Contains(t, out, "mcp.method=tools/call")
	assert.Contains(t, out, "mcp.tool=create_issue")
	assert.Contains(t, out, "duration=")
}

func TestToolLoggingMiddleware_LogsToolErrorAtError(t *testing.T) {
	logger, buf := newTestLogger(slog.LevelDebug)
	deps := fakeToolDeps{logger: logger}
	ctx := ContextWithDeps(context.Background(), deps)

	wantErr := errors.New("boom")
	handler := ToolLoggingMiddleware()(func(_ context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
		return nil, wantErr
	})

	_, err := handler(ctx, MCPMethodCallTool, callToolRequest("create_issue"))
	require.ErrorIs(t, err, wantErr)

	out := buf.String()
	assert.Contains(t, out, "level=ERROR")
	assert.Contains(t, out, `msg="tool call failed"`)
	assert.Contains(t, out, "mcp.tool=create_issue")
	assert.Contains(t, out, "error=boom")
}

func TestToolLoggingMiddleware_LogsIsErrorResult(t *testing.T) {
	logger, buf := newTestLogger(slog.LevelDebug)
	deps := fakeToolDeps{logger: logger}
	ctx := ContextWithDeps(context.Background(), deps)

	handler := ToolLoggingMiddleware()(func(_ context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
		return &mcp.CallToolResult{IsError: true}, nil
	})

	_, err := handler(ctx, MCPMethodCallTool, callToolRequest("get_repo"))
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "level=ERROR")
	assert.Contains(t, out, `msg="tool call returned error result"`)
}

func TestToolLoggingMiddleware_NonToolMethodSilent(t *testing.T) {
	logger, buf := newTestLogger(slog.LevelDebug)
	deps := fakeToolDeps{logger: logger}
	ctx := ContextWithDeps(context.Background(), deps)

	var sawLogger *slog.Logger
	handler := ToolLoggingMiddleware()(func(innerCtx context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
		sawLogger = observability.LoggerFromContext(innerCtx)
		return nil, nil
	})

	_, err := handler(ctx, "tools/list", &mcp.ListToolsRequest{Params: &mcp.ListToolsParams{}})
	require.NoError(t, err)

	assert.NotNil(t, sawLogger, "non-tool methods should still get the enriched logger")
	// No success/failure log lines for non-tool methods.
	assert.False(t, strings.Contains(buf.String(), "tool call"),
		"middleware should not log tool outcomes for non-tool methods; got: %s", buf.String())
}

func TestToolLoggingMiddleware_MissingDepsPassesThrough(t *testing.T) {
	called := false
	handler := ToolLoggingMiddleware()(func(_ context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
		called = true
		return nil, nil
	})

	// No deps injected — middleware must not panic and must still call next.
	_, err := handler(context.Background(), MCPMethodCallTool, callToolRequest("x"))
	require.NoError(t, err)
	assert.True(t, called)
}

// Exercise the Logger(ctx) fallback in BaseDeps: when the context carries
// an enriched logger (as set by ToolLoggingMiddleware), deps.Logger(ctx)
// should return it rather than the base logger.
func TestBaseDeps_Logger_UsesContextLogger(t *testing.T) {
	base, _ := newTestLogger(slog.LevelInfo)
	obsv, err := observability.NewExporters(base, metrics.NewNoopMetrics())
	require.NoError(t, err)
	d := BaseDeps{Obsv: obsv}

	enriched := base.With("tool", "x")
	ctx := observability.ContextWithLogger(context.Background(), enriched)

	assert.Equal(t, enriched, d.Logger(ctx))
	assert.Equal(t, base, d.Logger(context.Background()))
}
