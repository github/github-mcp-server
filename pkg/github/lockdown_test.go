package github

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_authorLockdownResult(t *testing.T) {
	t.Parallel()

	t.Run("missing cache returns error", func(t *testing.T) {
		result, err := authorLockdownResult(context.Background(), nil, "owner", "repo", "author", lockdownIssueRestrictedMessage)
		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("empty author fails closed", func(t *testing.T) {
		cache := stubRepoAccessCache(nil, time.Minute)
		result, err := authorLockdownResult(context.Background(), cache, "owner", "repo", "", lockdownIssueRestrictedMessage)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, lockdownIssueRestrictedMessage)
	})

	t.Run("lookup failure returns tool-result error", func(t *testing.T) {
		cache := stubRepoAccessCache(nil, time.Minute)
		result, err := authorLockdownResult(context.Background(), cache, "owner", "repo", "author", lockdownIssueRestrictedMessage)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "failed to check lockdown mode")
	})
}
