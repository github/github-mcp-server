package github

import (
	"context"
	"slices"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func listUIResourceNames(t *testing.T, readOnly bool) []string {
	t.Helper()

	srv := mcp.NewServer(&mcp.Implementation{Name: "test"}, nil)
	RegisterUIResources(srv, readOnly)

	st, ct := mcp.NewInMemoryTransports()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil)

	type clientResult struct {
		res *mcp.ListResourcesResult
		err error
	}
	clientResultCh := make(chan clientResult, 1)
	go func() {
		cs, err := client.Connect(context.Background(), ct, nil)
		if err != nil {
			clientResultCh <- clientResult{err: err}
			return
		}
		defer func() { _ = cs.Close() }()

		res, err := cs.ListResources(context.Background(), nil)
		clientResultCh <- clientResult{res: res, err: err}
	}()

	ss, err := srv.Connect(context.Background(), st, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ss.Close() })

	got := <-clientResultCh
	require.NoError(t, got.err)
	require.NotNil(t, got.res)

	names := make([]string, 0, len(got.res.Resources))
	for _, res := range got.res.Resources {
		names = append(names, res.Name)
	}
	slices.Sort(names)
	return names
}

func TestRegisterUIResources(t *testing.T) {
	t.Parallel()

	t.Run("registers all UI resources by default", func(t *testing.T) {
		t.Parallel()

		names := listUIResourceNames(t, false)
		require.Equal(t, []string{"get_me_ui", "issue_write_ui", "pr_write_ui"}, names)
	})

	t.Run("skips write UI resources in read-only mode", func(t *testing.T) {
		t.Parallel()

		names := listUIResourceNames(t, true)
		require.Equal(t, []string{"get_me_ui"}, names)
	})
}
